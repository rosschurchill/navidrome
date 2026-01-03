package sonos_cast

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
)

// SonosCast is the main service for Sonos speaker control
type SonosCast struct {
	discovery  *Discovery
	transport  *AVTransport
	rendering  *RenderingControl
	running    bool
	stopCh     chan struct{}
	wg         sync.WaitGroup
	mu         sync.RWMutex
}

// NewSonosCast creates a new SonosCast service
func NewSonosCast() *SonosCast {
	return &SonosCast{
		discovery: NewDiscovery(),
		transport: NewAVTransport(),
		rendering: NewRenderingControl(),
		stopCh:    make(chan struct{}),
	}
}

// Start begins the SonosCast service with periodic discovery
func (s *SonosCast) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.mu.Unlock()

	log.Info(ctx, "Starting Sonos Cast service")

	// Initial discovery
	s.runDiscovery(ctx)

	// Start periodic discovery
	interval := conf.Server.SonosCast.DiscoveryInterval
	if interval == 0 {
		interval = 5 * time.Minute
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.runDiscovery(ctx)
			case <-s.stopCh:
				log.Info(ctx, "Sonos Cast discovery stopped")
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// Shutdown stops the SonosCast service
func (s *SonosCast) Shutdown() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopCh)
	s.wg.Wait()
}

// runDiscovery performs SSDP discovery and fetches zone topology
func (s *SonosCast) runDiscovery(ctx context.Context) {
	devices, err := s.discovery.Scan(ctx)
	if err != nil {
		log.Error(ctx, "Sonos discovery failed", err)
		return
	}

	log.Debug(ctx, "Sonos SSDP discovery found devices", "count", len(devices))

	// Fetch zone topology from first available device
	if len(devices) > 0 {
		if err := s.discovery.FetchZoneGroupTopology(ctx, devices[0]); err != nil {
			log.Warn(ctx, "Failed to fetch zone topology - marking all devices as coordinators", err)
			// Fallback: mark all devices as their own coordinator so casting works
			// This means grouped speakers won't be detected, but at least standalone
			// speakers will work correctly
			for _, device := range devices {
				device.IsCoordinator = true
				device.GroupID = device.UUID
				device.GroupMembers = []string{device.UUID}
				s.discovery.cache.Set(device)
				log.Debug(ctx, "Marked device as coordinator (fallback)", "roomName", device.RoomName, "uuid", device.UUID)
			}
		}
	}
}

// RefreshDevices forces a new discovery scan
func (s *SonosCast) RefreshDevices(ctx context.Context) error {
	s.runDiscovery(ctx)
	return nil
}

// GetDevices returns all discovered Sonos devices
func (s *SonosCast) GetDevices() []*SonosDevice {
	return s.discovery.GetDevices()
}

// GetDevice returns a specific device by UUID
func (s *SonosCast) GetDevice(uuid string) (*SonosDevice, bool) {
	return s.discovery.GetDevice(uuid)
}

// getCoordinator returns the group coordinator for a device
// If the device is already a coordinator, it returns the device itself
// If the device is part of a group, it returns the coordinator of that group
func (s *SonosCast) getCoordinator(ctx context.Context, uuid string) (*SonosDevice, error) {
	device, ok := s.GetDevice(uuid)
	if !ok {
		return nil, ErrDeviceNotFound
	}

	// If this device is already a coordinator, use it directly
	if device.IsCoordinator {
		return device, nil
	}

	// Device is part of a group - find and use the coordinator
	// The GroupID contains the coordinator's UUID (format: RINCON_xxx:nnn)
	if device.GroupID != "" {
		// Extract coordinator UUID from GroupID (before the colon)
		coordUUID := device.GroupID
		if idx := len(coordUUID) - 1; idx > 0 {
			// GroupID format is typically "RINCON_xxx:nnn" - we need just "RINCON_xxx"
			for i := len(device.GroupID) - 1; i >= 0; i-- {
				if device.GroupID[i] == ':' {
					coordUUID = device.GroupID[:i]
					break
				}
			}
		}

		// Try to find the coordinator device
		coordinator, ok := s.GetDevice(coordUUID)
		if ok && coordinator.IsCoordinator {
			log.Debug(ctx, "Redirecting command to group coordinator",
				"requested", device.RoomName, "coordinator", coordinator.RoomName)
			return coordinator, nil
		}
	}

	// Couldn't find coordinator - log warning but try the original device anyway
	// (it may work for some commands like volume on individual speakers)
	log.Warn(ctx, "Device is not a coordinator and coordinator not found",
		"device", device.RoomName, "groupId", device.GroupID)
	return device, nil
}

// Play starts playback on a device
func (s *SonosCast) Play(ctx context.Context, uuid string) error {
	device, err := s.getCoordinator(ctx, uuid)
	if err != nil {
		return err
	}
	return s.transport.Play(ctx, device)
}

// PlayURI sets a URI and starts playback
func (s *SonosCast) PlayURI(ctx context.Context, uuid string, uri string, metadata string) error {
	device, err := s.getCoordinator(ctx, uuid)
	if err != nil {
		return err
	}
	return s.transport.PlayURI(ctx, device, uri, metadata)
}

// Pause pauses playback on a device
func (s *SonosCast) Pause(ctx context.Context, uuid string) error {
	device, err := s.getCoordinator(ctx, uuid)
	if err != nil {
		return err
	}
	return s.transport.Pause(ctx, device)
}

// Stop stops playback on a device
func (s *SonosCast) Stop(ctx context.Context, uuid string) error {
	device, err := s.getCoordinator(ctx, uuid)
	if err != nil {
		return err
	}
	return s.transport.Stop(ctx, device)
}

// Seek seeks to a position on a device
func (s *SonosCast) Seek(ctx context.Context, uuid string, position time.Duration) error {
	device, err := s.getCoordinator(ctx, uuid)
	if err != nil {
		return err
	}
	return s.transport.Seek(ctx, device, position)
}

// Next skips to the next track on a device
func (s *SonosCast) Next(ctx context.Context, uuid string) error {
	device, err := s.getCoordinator(ctx, uuid)
	if err != nil {
		return err
	}
	return s.transport.Next(ctx, device)
}

// Previous goes to the previous track on a device
func (s *SonosCast) Previous(ctx context.Context, uuid string) error {
	device, err := s.getCoordinator(ctx, uuid)
	if err != nil {
		return err
	}
	return s.transport.Previous(ctx, device)
}

// GetPlaybackState gets the current playback state of a device
func (s *SonosCast) GetPlaybackState(ctx context.Context, uuid string) (*PlaybackState, error) {
	device, ok := s.GetDevice(uuid)
	if !ok {
		return nil, ErrDeviceNotFound
	}

	// Get transport state
	transportState, err := s.transport.GetTransportInfo(ctx, device)
	if err != nil {
		return nil, err
	}

	// Get position info
	track, err := s.transport.GetPositionInfo(ctx, device)
	if err != nil {
		return nil, err
	}

	// Get volume
	volume, err := s.rendering.GetVolume(ctx, device)
	if err != nil {
		// Non-fatal, continue without volume
		log.Warn(ctx, "Failed to get volume", err)
		volume = -1
	}

	// Get mute state
	muted, err := s.rendering.GetMute(ctx, device)
	if err != nil {
		// Non-fatal, continue without mute state
		log.Warn(ctx, "Failed to get mute state", err)
	}

	return &PlaybackState{
		State:        transportState,
		CurrentTrack: track,
		Volume:       volume,
		Muted:        muted,
	}, nil
}

// SetVolume sets the volume on a device
func (s *SonosCast) SetVolume(ctx context.Context, uuid string, volume int) error {
	device, ok := s.GetDevice(uuid)
	if !ok {
		return ErrDeviceNotFound
	}
	return s.rendering.SetVolume(ctx, device, volume)
}

// GetVolume gets the volume from a device
func (s *SonosCast) GetVolume(ctx context.Context, uuid string) (int, error) {
	device, ok := s.GetDevice(uuid)
	if !ok {
		return 0, ErrDeviceNotFound
	}
	return s.rendering.GetVolume(ctx, device)
}

// SetMute sets the mute state on a device
func (s *SonosCast) SetMute(ctx context.Context, uuid string, mute bool) error {
	device, ok := s.GetDevice(uuid)
	if !ok {
		return ErrDeviceNotFound
	}
	return s.rendering.SetMute(ctx, device, mute)
}

// ToggleMute toggles mute on a device
func (s *SonosCast) ToggleMute(ctx context.Context, uuid string) (bool, error) {
	device, ok := s.GetDevice(uuid)
	if !ok {
		return false, ErrDeviceNotFound
	}
	return s.rendering.ToggleMute(ctx, device)
}

// BuildTrackMetadata creates DIDL-Lite metadata for a track
// streamURI and mimeType are required for Sonos to understand the content type
func (s *SonosCast) BuildTrackMetadata(id, title, artist, album, albumArtURL, streamURI, mimeType string) string {
	return BuildDIDLMetadata(id, title, artist, album, albumArtURL, streamURI, mimeType)
}

// Discovery returns the underlying discovery service
func (s *SonosCast) Discovery() *Discovery {
	return s.discovery
}

// Transport returns the underlying AVTransport service
func (s *SonosCast) Transport() *AVTransport {
	return s.transport
}

// Rendering returns the underlying RenderingControl service
func (s *SonosCast) Rendering() *RenderingControl {
	return s.rendering
}

// GetStreamBaseURL returns the base URL for Sonos to stream from
// This needs to be an absolute URL reachable from the LAN
// Sonos speakers are on the local network, so we use HTTP and internal IP
func (s *SonosCast) GetStreamBaseURL() string {
	// Use configured BaseURL if set (should be LAN-accessible HTTP URL)
	if conf.Server.BaseURL != "" {
		return conf.Server.BaseURL
	}

	// Fallback: construct from Address and Port
	// Note: conf.Server.Address may be "0.0.0.0" which won't work for Sonos
	// In that case, the admin should set BaseURL explicitly
	port := conf.Server.Port
	if port == 0 {
		port = 4533
	}

	address := conf.Server.Address
	if address == "" || address == "0.0.0.0" {
		// Can't determine LAN IP automatically - log warning
		log.Warn("Sonos Cast: BaseURL not configured and Address is 0.0.0.0. Set ND_BASEURL to your LAN-accessible URL (e.g., http://192.168.1.x:4533)")
		// Return localhost as fallback (will likely fail, but at least it's clear why)
		address = "127.0.0.1"
	}

	return fmt.Sprintf("http://%s:%d", address, port)
}
