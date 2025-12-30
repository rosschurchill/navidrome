package sonos

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
)

// SonosDevice represents a discovered Sonos device
type SonosDevice struct {
	IP       string
	Port     int
	Name     string
	Model    string
	Location string
}

// DiscoverDevices finds Sonos devices on the local network using SSDP
func DiscoverDevices(ctx context.Context, timeout time.Duration) ([]SonosDevice, error) {
	const ssdpAddr = "239.255.255.250:1900"
	const searchTarget = "urn:schemas-upnp-org:device:ZonePlayer:1"

	// Create UDP socket
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: 0})
	if err != nil {
		return nil, fmt.Errorf("failed to create UDP socket: %w", err)
	}
	defer conn.Close()

	// Set read deadline
	conn.SetReadDeadline(time.Now().Add(timeout))

	// Build M-SEARCH request
	searchRequest := fmt.Sprintf(
		"M-SEARCH * HTTP/1.1\r\n"+
			"HOST: %s\r\n"+
			"MAN: \"ssdp:discover\"\r\n"+
			"MX: 2\r\n"+
			"ST: %s\r\n"+
			"\r\n",
		ssdpAddr, searchTarget)

	// Send to multicast address
	addr, err := net.ResolveUDPAddr("udp4", ssdpAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve SSDP address: %w", err)
	}

	if _, err := conn.WriteToUDP([]byte(searchRequest), addr); err != nil {
		return nil, fmt.Errorf("failed to send SSDP request: %w", err)
	}

	// Collect responses
	devices := make(map[string]SonosDevice) // Dedupe by IP
	buf := make([]byte, 4096)

	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			// Timeout is expected
			break
		}

		response := string(buf[:n])

		// Parse LOCATION header
		locationMatch := regexp.MustCompile(`(?i)LOCATION:\s*(.+?)\r\n`).FindStringSubmatch(response)
		if len(locationMatch) < 2 {
			continue
		}

		location := strings.TrimSpace(locationMatch[1])
		device := SonosDevice{
			IP:       remoteAddr.IP.String(),
			Port:     1400, // Standard Sonos port
			Location: location,
		}

		// Fetch device description for friendly name
		if desc, err := fetchDeviceDescription(ctx, location); err == nil {
			device.Name = desc.FriendlyName
			device.Model = desc.ModelName
		}

		devices[device.IP] = device
	}

	// Convert map to slice
	result := make([]SonosDevice, 0, len(devices))
	for _, d := range devices {
		result = append(result, d)
	}

	return result, nil
}

// DeviceDescription contains relevant fields from Sonos device XML
type DeviceDescription struct {
	FriendlyName string
	ModelName    string
}

// fetchDeviceDescription retrieves the device description XML
func fetchDeviceDescription(ctx context.Context, location string) (*DeviceDescription, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(location)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Simple regex extraction (avoid full XML parsing for this)
	desc := &DeviceDescription{}

	if match := regexp.MustCompile(`<friendlyName>(.+?)</friendlyName>`).FindStringSubmatch(string(body)); len(match) > 1 {
		desc.FriendlyName = match[1]
	}
	if match := regexp.MustCompile(`<modelName>(.+?)</modelName>`).FindStringSubmatch(string(body)); len(match) > 1 {
		desc.ModelName = match[1]
	}

	return desc, nil
}

// RegisterWithDevice registers this music service with a Sonos device
func RegisterWithDevice(ctx context.Context, device SonosDevice, serviceURL string, serviceName string) error {
	log.Info(ctx, "Registering Sonos music service", "device", device.Name, "ip", device.IP, "serviceURL", serviceURL)

	customsdURL := fmt.Sprintf("http://%s:%d/customsd", device.IP, device.Port)

	// First, get the CSRF token from the form
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(customsdURL)
	if err != nil {
		return fmt.Errorf("failed to fetch customsd page: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read customsd page: %w", err)
	}

	// Extract CSRF token
	csrfMatch := regexp.MustCompile(`name="csrfToken"\s+value="([^"]+)"`).FindStringSubmatch(string(body))
	if len(csrfMatch) < 2 {
		return fmt.Errorf("CSRF token not found in customsd page")
	}
	csrfToken := csrfMatch[1]

	// Build service registration form data
	parsedURL, _ := url.Parse(serviceURL)
	secureURL := serviceURL
	if parsedURL.Scheme == "http" {
		// For local testing, use http
		secureURL = strings.Replace(serviceURL, "http://", "https://", 1)
	}

	formData := url.Values{
		"csrfToken":        {csrfToken},
		"sid":              {"255"}, // Custom service ID (246-255 for custom services)
		"name":             {serviceName},
		"uri":              {serviceURL + "/ws/sonos"},
		"secureUri":        {secureURL + "/ws/sonos"},
		"pollInterval":     {"3600"},
		"authType":         {"AppLink"},
		"stringsVersion":   {"1"},
		"stringsUri":       {serviceURL + "/strings.xml"},
		"presentMapVersion": {"1"},
		"presentMapUri":    {serviceURL + "/presentationMap.xml"},
		"containerType":    {"MService"},
		"caps":             {"search", "trFavorites", "alFavorites", "arFavorites", "extendedMD", "logging"},
	}

	// POST registration
	resp, err = client.PostForm(customsdURL, formData)
	if err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(body))
	}

	log.Info(ctx, "Successfully registered Sonos music service", "device", device.Name)
	return nil
}

// AutoRegister discovers Sonos devices and registers this service with the first one found
func AutoRegister(ctx context.Context) error {
	baseURL := conf.Server.BaseURL
	if baseURL == "" {
		log.Warn(ctx, "Sonos auto-registration requires BaseURL to be set")
		return fmt.Errorf("BaseURL not configured")
	}

	serviceURL := baseURL + "/sonos"
	serviceName := "Navidrome"

	log.Info(ctx, "Discovering Sonos devices...")
	devices, err := DiscoverDevices(ctx, 5*time.Second)
	if err != nil {
		return fmt.Errorf("device discovery failed: %w", err)
	}

	if len(devices) == 0 {
		log.Warn(ctx, "No Sonos devices found on the network")
		return fmt.Errorf("no devices found")
	}

	log.Info(ctx, "Found Sonos devices", "count", len(devices))
	for _, d := range devices {
		log.Debug(ctx, "Discovered device", "name", d.Name, "model", d.Model, "ip", d.IP)
	}

	// Register with the first device (it will propagate to all devices in the household)
	return RegisterWithDevice(ctx, devices[0], serviceURL, serviceName)
}
