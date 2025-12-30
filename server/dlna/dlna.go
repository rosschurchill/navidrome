package dlna

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/artwork"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

const (
	// SSDP multicast address and port
	ssdpAddr = "239.255.255.250:1900"
	// UPnP device type for media server
	deviceType = "urn:schemas-upnp-org:device:MediaServer:1"
	// UPnP service types
	contentDirectoryType  = "urn:schemas-upnp-org:service:ContentDirectory:1"
	connectionManagerType = "urn:schemas-upnp-org:service:ConnectionManager:1"
)

// Router handles DLNA/UPnP requests
type Router struct {
	ds         model.DataStore
	artwork    artwork.Artwork
	serverName string
	uuid       string
	httpPort   int
	interfaces []net.Interface
	ssdpConn   *net.UDPConn
	mu         sync.RWMutex
	running    bool
	ctx        context.Context
	cancel     context.CancelFunc
}

// New creates a new DLNA router
func New(ds model.DataStore, artwork artwork.Artwork) *Router {
	serverName := conf.Server.DLNA.ServerName
	if serverName == "" {
		serverName = "Navidrome"
	}

	// Generate a stable UUID based on server config
	uuid := generateUUID(serverName, conf.Server.Port)

	return &Router{
		ds:         ds,
		artwork:    artwork,
		serverName: serverName,
		uuid:       uuid,
		httpPort:   conf.Server.Port,
	}
}

// Routes returns the chi router for DLNA HTTP endpoints
func (r *Router) Routes() chi.Router {
	router := chi.NewRouter()

	// Device description
	router.Get("/device.xml", r.handleDeviceDescription)

	// ContentDirectory service
	router.Get("/ContentDirectory.xml", r.handleContentDirectoryDescription)
	router.Post("/ContentDirectory/control", r.handleContentDirectoryControl)

	// ConnectionManager service
	router.Get("/ConnectionManager.xml", r.handleConnectionManagerDescription)
	router.Post("/ConnectionManager/control", r.handleConnectionManagerControl)

	// Icons
	router.Get("/icon/{size}.png", r.handleIcon)

	return router
}

// Start begins SSDP announcements and M-SEARCH handling
func (r *Router) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return nil
	}
	r.ctx, r.cancel = context.WithCancel(ctx)
	r.running = true
	r.mu.Unlock()

	// Get network interfaces
	ifaces, err := getActiveInterfaces()
	if err != nil {
		return fmt.Errorf("failed to get network interfaces: %w", err)
	}
	r.interfaces = ifaces

	// Start SSDP listener
	if err := r.startSSDP(); err != nil {
		return fmt.Errorf("failed to start SSDP: %w", err)
	}

	// Send initial announcements
	r.announcePresence()

	log.Info(r.ctx, "DLNA server started", "name", r.serverName, "uuid", r.uuid)

	return nil
}

// Stop halts SSDP announcements and closes connections
func (r *Router) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return
	}

	// Send byebye notifications
	r.sendByeBye()

	if r.cancel != nil {
		r.cancel()
	}

	if r.ssdpConn != nil {
		r.ssdpConn.Close()
	}

	r.running = false
	log.Info("DLNA server stopped")
}

// generateUUID creates a stable UUID for this server instance
func generateUUID(serverName string, port int) string {
	// Use a combination of server name and port for stability
	// In production, this should be persisted to maintain the same UUID across restarts
	return fmt.Sprintf("uuid:navidrome-%s-%d", serverName, port)
}

// getActiveInterfaces returns network interfaces that are up and have addresses
func getActiveInterfaces() ([]net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var active []net.Interface
	for _, iface := range ifaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		// Check if interface has usable addresses
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ipnet.IP.To4() != nil && !ipnet.IP.IsLoopback() {
					active = append(active, iface)
					break
				}
			}
		}
	}

	return active, nil
}

// getLocalIP returns the first non-loopback IPv4 address
func getLocalIP() string {
	ifaces, err := getActiveInterfaces()
	if err != nil || len(ifaces) == 0 {
		return "127.0.0.1"
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ipnet.IP.To4() != nil && !ipnet.IP.IsLoopback() {
					return ipnet.IP.String()
				}
			}
		}
	}

	return "127.0.0.1"
}

// getAlbumArtURL returns the URL for album artwork
func (r *Router) getAlbumArtURL(albumID string) string {
	baseURL := conf.Server.BaseURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://%s:%d", getLocalIP(), r.httpPort)
	}
	return fmt.Sprintf("%s/rest/getCoverArt?id=%s&size=300", baseURL, albumID)
}
