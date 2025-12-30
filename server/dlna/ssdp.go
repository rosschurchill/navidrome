package dlna

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/navidrome/navidrome/log"
)

const (
	// SSDP message types
	ssdpAlive  = "ssdp:alive"
	ssdpByeBye = "ssdp:byebye"
	ssdpAll    = "ssdp:all"

	// Cache control max-age in seconds
	cacheMaxAge = 1800

	// Announcement interval
	announceInterval = 30 * time.Minute
)

// startSSDP initializes the SSDP listener for M-SEARCH requests
func (r *Router) startSSDP() error {
	// Parse multicast address
	addr, err := net.ResolveUDPAddr("udp4", ssdpAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve SSDP address: %w", err)
	}

	// Listen on all interfaces for multicast
	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		return fmt.Errorf("failed to listen on multicast: %w", err)
	}

	// Set read buffer size
	if err := conn.SetReadBuffer(65535); err != nil {
		log.Warn(r.ctx, "Failed to set SSDP read buffer", err)
	}

	r.ssdpConn = conn

	// Start listening for M-SEARCH requests
	go r.listenSSDP()

	// Start periodic announcements
	go r.periodicAnnounce()

	return nil
}

// listenSSDP handles incoming SSDP M-SEARCH requests
func (r *Router) listenSSDP() {
	buf := make([]byte, 2048)

	for {
		select {
		case <-r.ctx.Done():
			return
		default:
		}

		// Set read deadline to allow checking context
		if err := r.ssdpConn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			continue
		}

		n, remoteAddr, err := r.ssdpConn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			log.Error(r.ctx, "Error reading SSDP packet", err)
			continue
		}

		msg := string(buf[:n])
		if strings.HasPrefix(msg, "M-SEARCH") {
			r.handleMSearch(msg, remoteAddr)
		}
	}
}

// handleMSearch responds to SSDP M-SEARCH discovery requests
func (r *Router) handleMSearch(msg string, remoteAddr *net.UDPAddr) {
	// Parse the search target
	st := extractHeader(msg, "ST")
	if st == "" {
		return
	}

	// Check if we should respond to this search
	shouldRespond := false
	var respondTargets []string

	switch st {
	case ssdpAll:
		shouldRespond = true
		respondTargets = r.getAllServiceTypes()
	case "upnp:rootdevice":
		shouldRespond = true
		respondTargets = []string{"upnp:rootdevice"}
	case deviceType:
		shouldRespond = true
		respondTargets = []string{deviceType}
	case contentDirectoryType:
		shouldRespond = true
		respondTargets = []string{contentDirectoryType}
	case connectionManagerType:
		shouldRespond = true
		respondTargets = []string{connectionManagerType}
	default:
		// Check if it's our UUID
		if st == r.uuid {
			shouldRespond = true
			respondTargets = []string{r.uuid}
		}
	}

	if !shouldRespond {
		return
	}

	log.Debug(r.ctx, "Responding to M-SEARCH", "st", st, "from", remoteAddr.String())

	// Send responses for each target
	for _, target := range respondTargets {
		r.sendSearchResponse(target, remoteAddr)
	}
}

// sendSearchResponse sends an M-SEARCH response to the requester
func (r *Router) sendSearchResponse(st string, remoteAddr *net.UDPAddr) {
	location := r.getDeviceURL()
	usn := r.getUSN(st)

	response := fmt.Sprintf("HTTP/1.1 200 OK\r\n"+
		"CACHE-CONTROL: max-age=%d\r\n"+
		"DATE: %s\r\n"+
		"EXT:\r\n"+
		"LOCATION: %s\r\n"+
		"SERVER: %s\r\n"+
		"ST: %s\r\n"+
		"USN: %s\r\n"+
		"BOOTID.UPNP.ORG: 1\r\n"+
		"CONFIGID.UPNP.ORG: 1\r\n"+
		"\r\n",
		cacheMaxAge,
		time.Now().UTC().Format(time.RFC1123),
		location,
		r.getServerString(),
		st,
		usn,
	)

	conn, err := net.DialUDP("udp4", nil, remoteAddr)
	if err != nil {
		log.Error(r.ctx, "Failed to dial for M-SEARCH response", err)
		return
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(response)); err != nil {
		log.Error(r.ctx, "Failed to send M-SEARCH response", err)
	}
}

// announcePresence sends SSDP NOTIFY alive messages for all services
func (r *Router) announcePresence() {
	for _, target := range r.getAllServiceTypes() {
		r.sendNotify(target, ssdpAlive)
	}
}

// sendByeBye sends SSDP NOTIFY byebye messages for all services
func (r *Router) sendByeBye() {
	for _, target := range r.getAllServiceTypes() {
		r.sendNotify(target, ssdpByeBye)
	}
}

// periodicAnnounce sends announcements at regular intervals
func (r *Router) periodicAnnounce() {
	ticker := time.NewTicker(announceInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.announcePresence()
		}
	}
}

// sendNotify sends an SSDP NOTIFY message
func (r *Router) sendNotify(nt, nts string) {
	location := r.getDeviceURL()
	usn := r.getUSN(nt)

	var msg string
	if nts == ssdpByeBye {
		msg = fmt.Sprintf("NOTIFY * HTTP/1.1\r\n"+
			"HOST: %s\r\n"+
			"NT: %s\r\n"+
			"NTS: %s\r\n"+
			"USN: %s\r\n"+
			"BOOTID.UPNP.ORG: 1\r\n"+
			"CONFIGID.UPNP.ORG: 1\r\n"+
			"\r\n",
			ssdpAddr,
			nt,
			nts,
			usn,
		)
	} else {
		msg = fmt.Sprintf("NOTIFY * HTTP/1.1\r\n"+
			"HOST: %s\r\n"+
			"CACHE-CONTROL: max-age=%d\r\n"+
			"LOCATION: %s\r\n"+
			"NT: %s\r\n"+
			"NTS: %s\r\n"+
			"SERVER: %s\r\n"+
			"USN: %s\r\n"+
			"BOOTID.UPNP.ORG: 1\r\n"+
			"CONFIGID.UPNP.ORG: 1\r\n"+
			"\r\n",
			ssdpAddr,
			cacheMaxAge,
			location,
			nt,
			nts,
			r.getServerString(),
			usn,
		)
	}

	// Send to multicast address
	addr, err := net.ResolveUDPAddr("udp4", ssdpAddr)
	if err != nil {
		log.Error(r.ctx, "Failed to resolve SSDP address for notify", err)
		return
	}

	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		log.Error(r.ctx, "Failed to dial for NOTIFY", err)
		return
	}
	defer conn.Close()

	// Send notification multiple times for reliability
	for i := 0; i < 3; i++ {
		if _, err := conn.Write([]byte(msg)); err != nil {
			log.Error(r.ctx, "Failed to send NOTIFY", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// getAllServiceTypes returns all service types to advertise
func (r *Router) getAllServiceTypes() []string {
	return []string{
		"upnp:rootdevice",
		r.uuid,
		deviceType,
		contentDirectoryType,
		connectionManagerType,
	}
}

// getUSN returns the Unique Service Name for a given service type
func (r *Router) getUSN(st string) string {
	if st == r.uuid {
		return r.uuid
	}
	return fmt.Sprintf("%s::%s", r.uuid, st)
}

// getDeviceURL returns the URL to the device description
func (r *Router) getDeviceURL() string {
	localIP := getLocalIP()
	baseURL := fmt.Sprintf("http://%s:%d", localIP, r.httpPort)
	return fmt.Sprintf("%s/dlna/device.xml", baseURL)
}

// getServerString returns the SERVER header value
func (r *Router) getServerString() string {
	return fmt.Sprintf("Linux/1.0 UPnP/1.1 %s/1.0", r.serverName)
}

// extractHeader extracts a header value from an SSDP message
func extractHeader(msg, header string) string {
	headerPrefix := header + ":"
	for _, line := range strings.Split(msg, "\r\n") {
		if strings.HasPrefix(strings.ToUpper(line), strings.ToUpper(headerPrefix)) {
			return strings.TrimSpace(line[len(headerPrefix):])
		}
	}
	return ""
}
