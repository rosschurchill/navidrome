package sonos_cast

import (
	"bufio"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/navidrome/navidrome/log"
)

const (
	ssdpMulticastAddr = "239.255.255.250:1900"
	sonosSearchTarget = "urn:schemas-upnp-org:device:ZonePlayer:1"
	ssdpSearchTimeout = 3 * time.Second
	deviceFetchTimeout = 5 * time.Second
)

// Discovery handles Sonos device discovery via SSDP
type Discovery struct {
	cache  *DeviceCache
	client *http.Client
}

// NewDiscovery creates a new Sonos discovery service
func NewDiscovery() *Discovery {
	return &Discovery{
		cache: NewDeviceCache(),
		client: &http.Client{
			Timeout: deviceFetchTimeout,
		},
	}
}

// Scan performs SSDP discovery for Sonos devices
func (d *Discovery) Scan(ctx context.Context) ([]*SonosDevice, error) {
	log.Debug(ctx, "Starting Sonos SSDP discovery scan")

	// Create UDP connection for multicast
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return nil, fmt.Errorf("failed to create UDP listener: %w", err)
	}
	defer conn.Close()

	// Build M-SEARCH request
	searchRequest := buildMSearchRequest(sonosSearchTarget)

	// Resolve multicast address
	multicastAddr, err := net.ResolveUDPAddr("udp4", ssdpMulticastAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve multicast address: %w", err)
	}

	// Send M-SEARCH
	_, err = conn.WriteToUDP([]byte(searchRequest), multicastAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to send M-SEARCH: %w", err)
	}

	log.Debug(ctx, "Sent SSDP M-SEARCH for Sonos devices")

	// Collect responses
	locations := make(map[string]bool)
	deadline := time.Now().Add(ssdpSearchTimeout)
	conn.SetReadDeadline(deadline)

	buf := make([]byte, 2048)
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break // Expected timeout
			}
			log.Warn(ctx, "Error reading SSDP response", err)
			break
		}

		// Parse response to extract LOCATION header
		location := parseLocationFromResponse(string(buf[:n]))
		if location != "" && !locations[location] {
			locations[location] = true
			log.Debug(ctx, "Found Sonos device", "location", location)
		}
	}

	// Fetch device descriptions
	var devices []*SonosDevice
	for location := range locations {
		device, err := d.fetchDeviceDescription(ctx, location)
		if err != nil {
			log.Warn(ctx, "Failed to fetch device description", "location", location, err)
			continue
		}
		devices = append(devices, device)
		d.cache.Set(device)
	}

	log.Info(ctx, "Sonos discovery complete", "devicesFound", len(devices))
	return devices, nil
}

// GetDevices returns all cached devices
func (d *Discovery) GetDevices() []*SonosDevice {
	return d.cache.GetAll()
}

// GetDevice returns a specific device by UUID
func (d *Discovery) GetDevice(uuid string) (*SonosDevice, bool) {
	return d.cache.Get(uuid)
}

// buildMSearchRequest creates an SSDP M-SEARCH request
func buildMSearchRequest(searchTarget string) string {
	return fmt.Sprintf(
		"M-SEARCH * HTTP/1.1\r\n"+
			"HOST: %s\r\n"+
			"MAN: \"ssdp:discover\"\r\n"+
			"MX: 2\r\n"+
			"ST: %s\r\n"+
			"USER-AGENT: Navidrome/1.0 UPnP/1.0\r\n"+
			"\r\n",
		ssdpMulticastAddr, searchTarget)
}

// parseLocationFromResponse extracts the LOCATION header from SSDP response
func parseLocationFromResponse(response string) string {
	scanner := bufio.NewScanner(strings.NewReader(response))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.ToUpper(line), "LOCATION:") {
			return strings.TrimSpace(line[9:])
		}
	}
	return ""
}

// fetchDeviceDescription fetches and parses the device description XML
func (d *Discovery) fetchDeviceDescription(ctx context.Context, location string) (*SonosDevice, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", location, nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var desc DeviceDescription
	if err := xml.Unmarshal(body, &desc); err != nil {
		return nil, fmt.Errorf("failed to parse device description: %w", err)
	}

	// Extract IP and port from location URL
	ip, port := parseIPPort(location)

	// Extract UUID from UDN (format: uuid:RINCON_xxx)
	uuid := strings.TrimPrefix(desc.Device.UDN, "uuid:")

	// Determine S1 vs S2
	softwareGen := "S2"
	if desc.Device.SoftwareGen == "1" {
		softwareGen = "S1"
	}

	device := &SonosDevice{
		IP:          ip,
		Port:        port,
		UUID:        uuid,
		RoomName:    desc.Device.RoomName,
		ModelName:   desc.Device.ModelName,
		ModelNumber: desc.Device.ModelNumber,
		SoftwareGen: softwareGen,
		LastSeen:    time.Now(),
	}

	return device, nil
}

// parseIPPort extracts IP and port from a URL like http://192.168.1.10:1400/xml/device_description.xml
func parseIPPort(location string) (string, int) {
	// Remove protocol
	location = strings.TrimPrefix(location, "http://")
	location = strings.TrimPrefix(location, "https://")

	// Get host:port part
	if idx := strings.Index(location, "/"); idx != -1 {
		location = location[:idx]
	}

	// Split host and port
	host, portStr, err := net.SplitHostPort(location)
	if err != nil {
		return location, SonosPort // Default port
	}

	port := SonosPort
	fmt.Sscanf(portStr, "%d", &port)

	return host, port
}

// FetchZoneGroupTopology fetches the zone group topology to understand speaker grouping
func (d *Discovery) FetchZoneGroupTopology(ctx context.Context, device *SonosDevice) error {
	log.Debug(ctx, "Fetching zone group topology", "deviceIP", device.IP, "deviceUUID", device.UUID)

	// Build SOAP request for GetZoneGroupState
	soapAction := "urn:upnp-org:serviceId:ZoneGroupTopology#GetZoneGroupState"
	soapBody := `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:GetZoneGroupState xmlns:u="urn:upnp-org:serviceId:ZoneGroupTopology">
    </u:GetZoneGroupState>
  </s:Body>
</s:Envelope>`

	url := fmt.Sprintf("http://%s:%d/ZoneGroupTopology/Control", device.IP, device.Port)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(soapBody))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPACTION", soapAction)

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("zone topology request failed: %d - %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	log.Debug(ctx, "ZoneGroupTopology SOAP response received", "bodyLen", len(body))

	// Parse the response to extract zone groups
	// The response contains XML-encoded ZoneGroupState inside the SOAP body
	// We need to extract and parse it
	zoneGroupState := extractZoneGroupState(string(body))
	if zoneGroupState == "" {
		bodyPreview := string(body)
		if len(bodyPreview) > 500 {
			bodyPreview = bodyPreview[:500]
		}
		log.Warn(ctx, "Failed to extract ZoneGroupState from response", "body", bodyPreview)
		return fmt.Errorf("failed to extract ZoneGroupState from response")
	}

	var zgs ZoneGroupState
	if err := xml.Unmarshal([]byte(zoneGroupState), &zgs); err != nil {
		statePreview := zoneGroupState
		if len(statePreview) > 500 {
			statePreview = statePreview[:500]
		}
		log.Error(ctx, "Failed to parse ZoneGroupState XML", err, "xml", statePreview)
		return fmt.Errorf("failed to parse ZoneGroupState: %w", err)
	}

	log.Debug(ctx, "Parsed zone groups", "count", len(zgs.ZoneGroups))

	// Update device cache with group info
	updatedCount := 0
	notFoundCount := 0
	for _, group := range zgs.ZoneGroups {
		var memberUUIDs []string
		for _, member := range group.Members {
			memberUUIDs = append(memberUUIDs, member.UUID)
		}

		for _, member := range group.Members {
			if cached, ok := d.cache.Get(member.UUID); ok {
				cached.GroupID = group.ID
				cached.IsCoordinator = (member.UUID == group.Coordinator)
				cached.GroupMembers = memberUUIDs
				if cached.RoomName == "" {
					cached.RoomName = member.ZoneName
				}
				d.cache.Set(cached)
				updatedCount++
				log.Debug(ctx, "Updated device with group info",
					"roomName", cached.RoomName,
					"uuid", member.UUID,
					"isCoordinator", cached.IsCoordinator,
					"groupId", group.ID)
			} else {
				notFoundCount++
				log.Debug(ctx, "Device from zone topology not in cache", "uuid", member.UUID, "zoneName", member.ZoneName)
			}
		}
	}

	log.Debug(ctx, "Zone group topology update complete", "groups", len(zgs.ZoneGroups), "updated", updatedCount, "notFound", notFoundCount)
	return nil
}

// extractZoneGroupState extracts the ZoneGroupState XML from SOAP response
// The Sonos response typically has the ZoneGroupState content HTML-encoded
func extractZoneGroupState(body string) string {
	// Look for ZoneGroupState element and extract its content
	startTag := "<ZoneGroupState>"
	endTag := "</ZoneGroupState>"

	start := strings.Index(body, startTag)
	if start == -1 {
		return ""
	}
	start += len(startTag)

	end := strings.Index(body[start:], endTag)
	if end == -1 {
		return ""
	}

	content := body[start : start+end]

	// The content is typically HTML-encoded, so decode it
	content = strings.ReplaceAll(content, "&lt;", "<")
	content = strings.ReplaceAll(content, "&gt;", ">")
	content = strings.ReplaceAll(content, "&quot;", "\"")
	content = strings.ReplaceAll(content, "&amp;", "&")
	content = strings.ReplaceAll(content, "&apos;", "'")

	// Check if content needs double-decoding
	if !strings.Contains(content, "<ZoneGroups>") && !strings.Contains(content, "<ZoneGroup") && !strings.Contains(content, "<ZoneGroupState>") {
		content = strings.ReplaceAll(content, "&lt;", "<")
		content = strings.ReplaceAll(content, "&gt;", ">")
		content = strings.ReplaceAll(content, "&quot;", "\"")
		content = strings.ReplaceAll(content, "&amp;", "&")
	}

	// If content already starts with <ZoneGroupState>, return as-is
	// Otherwise wrap it for proper unmarshaling
	if strings.HasPrefix(strings.TrimSpace(content), "<ZoneGroupState>") {
		return content
	}
	return "<ZoneGroupState>" + content + "</ZoneGroupState>"
}
