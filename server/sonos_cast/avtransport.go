package sonos_cast

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/navidrome/navidrome/log"
)

// AVTransport provides playback control for Sonos devices
type AVTransport struct {
	client *http.Client
}

// NewAVTransport creates a new AVTransport controller
func NewAVTransport() *AVTransport {
	return &AVTransport{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SetAVTransportURI sets the playback URI on the device
func (a *AVTransport) SetAVTransportURI(ctx context.Context, device *SonosDevice, uri string, metadata string) error {
	action := SetAVTransportURIAction{
		XmlnsU:             AVTransportURN,
		InstanceID:         0,
		CurrentURI:         uri,
		CurrentURIMetaData: metadata,
	}

	_, err := a.sendAction(ctx, device, "SetAVTransportURI", action)
	if err != nil {
		return fmt.Errorf("SetAVTransportURI failed: %w", err)
	}

	log.Debug(ctx, "Set transport URI", "device", device.RoomName, "uri", uri)
	return nil
}

// SetNextAVTransportURI sets the next track for gapless playback
func (a *AVTransport) SetNextAVTransportURI(ctx context.Context, device *SonosDevice, uri string, metadata string) error {
	action := SetNextAVTransportURIAction{
		XmlnsU:          AVTransportURN,
		InstanceID:      0,
		NextURI:         uri,
		NextURIMetaData: metadata,
	}

	_, err := a.sendAction(ctx, device, "SetNextAVTransportURI", action)
	if err != nil {
		return fmt.Errorf("SetNextAVTransportURI failed: %w", err)
	}

	log.Debug(ctx, "Set next transport URI", "device", device.RoomName, "uri", uri)
	return nil
}

// Play starts or resumes playback
func (a *AVTransport) Play(ctx context.Context, device *SonosDevice) error {
	action := PlayAction{
		XmlnsU:     AVTransportURN,
		InstanceID: 0,
		Speed:      "1",
	}

	_, err := a.sendAction(ctx, device, "Play", action)
	if err != nil {
		return fmt.Errorf("Play failed: %w", err)
	}

	log.Debug(ctx, "Started playback", "device", device.RoomName)
	return nil
}

// Pause pauses playback
func (a *AVTransport) Pause(ctx context.Context, device *SonosDevice) error {
	action := PauseAction{
		XmlnsU:     AVTransportURN,
		InstanceID: 0,
	}

	_, err := a.sendAction(ctx, device, "Pause", action)
	if err != nil {
		return fmt.Errorf("Pause failed: %w", err)
	}

	log.Debug(ctx, "Paused playback", "device", device.RoomName)
	return nil
}

// Stop stops playback
func (a *AVTransport) Stop(ctx context.Context, device *SonosDevice) error {
	action := StopAction{
		XmlnsU:     AVTransportURN,
		InstanceID: 0,
	}

	_, err := a.sendAction(ctx, device, "Stop", action)
	if err != nil {
		return fmt.Errorf("Stop failed: %w", err)
	}

	log.Debug(ctx, "Stopped playback", "device", device.RoomName)
	return nil
}

// Seek seeks to a position in the current track
func (a *AVTransport) Seek(ctx context.Context, device *SonosDevice, position time.Duration) error {
	// Format as HH:MM:SS
	hours := int(position.Hours())
	minutes := int(position.Minutes()) % 60
	seconds := int(position.Seconds()) % 60
	target := fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)

	action := SeekAction{
		XmlnsU:     AVTransportURN,
		InstanceID: 0,
		Unit:       "REL_TIME",
		Target:     target,
	}

	_, err := a.sendAction(ctx, device, "Seek", action)
	if err != nil {
		return fmt.Errorf("Seek failed: %w", err)
	}

	log.Debug(ctx, "Seeked to position", "device", device.RoomName, "position", target)
	return nil
}

// Next skips to the next track in the queue
func (a *AVTransport) Next(ctx context.Context, device *SonosDevice) error {
	action := NextAction{
		XmlnsU:     AVTransportURN,
		InstanceID: 0,
	}

	_, err := a.sendAction(ctx, device, "Next", action)
	if err != nil {
		return fmt.Errorf("Next failed: %w", err)
	}

	log.Debug(ctx, "Skipped to next track", "device", device.RoomName)
	return nil
}

// Previous goes to the previous track in the queue
func (a *AVTransport) Previous(ctx context.Context, device *SonosDevice) error {
	action := PreviousAction{
		XmlnsU:     AVTransportURN,
		InstanceID: 0,
	}

	_, err := a.sendAction(ctx, device, "Previous", action)
	if err != nil {
		return fmt.Errorf("Previous failed: %w", err)
	}

	log.Debug(ctx, "Went to previous track", "device", device.RoomName)
	return nil
}

// GetPositionInfo gets the current playback position and track info
func (a *AVTransport) GetPositionInfo(ctx context.Context, device *SonosDevice) (*Track, error) {
	action := GetPositionInfoAction{
		XmlnsU:     AVTransportURN,
		InstanceID: 0,
	}

	respBody, err := a.sendAction(ctx, device, "GetPositionInfo", action)
	if err != nil {
		return nil, fmt.Errorf("GetPositionInfo failed: %w", err)
	}

	// Parse response
	var resp GetPositionInfoResponse
	if err := extractSOAPResponse(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse GetPositionInfo response: %w", err)
	}

	track := &Track{
		URI:      resp.TrackURI,
		TrackNum: resp.Track,
		Position: parseDuration(resp.RelTime),
		Duration: parseDuration(resp.TrackDuration),
	}

	// Parse metadata if available
	if resp.TrackMetaData != "" {
		a.parseTrackMetadata(resp.TrackMetaData, track)
	}

	return track, nil
}

// GetTransportInfo gets the current transport state
func (a *AVTransport) GetTransportInfo(ctx context.Context, device *SonosDevice) (string, error) {
	action := GetTransportInfoAction{
		XmlnsU:     AVTransportURN,
		InstanceID: 0,
	}

	respBody, err := a.sendAction(ctx, device, "GetTransportInfo", action)
	if err != nil {
		return "", fmt.Errorf("GetTransportInfo failed: %w", err)
	}

	// Parse response
	var resp GetTransportInfoResponse
	if err := extractSOAPResponse(respBody, &resp); err != nil {
		return "", fmt.Errorf("failed to parse GetTransportInfo response: %w", err)
	}

	return resp.CurrentTransportState, nil
}

// PlayURI sets the URI and starts playback in one call
func (a *AVTransport) PlayURI(ctx context.Context, device *SonosDevice, uri string, metadata string) error {
	if err := a.SetAVTransportURI(ctx, device, uri, metadata); err != nil {
		return err
	}
	return a.Play(ctx, device)
}

// sendAction sends a SOAP action to the device
func (a *AVTransport) sendAction(ctx context.Context, device *SonosDevice, actionName string, action interface{}) ([]byte, error) {
	// Build SOAP envelope
	envelope := SOAPEnvelope{
		XmlnsS:        "http://schemas.xmlsoap.org/soap/envelope/",
		EncodingStyle: "http://schemas.xmlsoap.org/soap/encoding/",
		Body: SOAPBody{
			Content: action,
		},
	}

	body, err := xml.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal SOAP envelope: %w", err)
	}

	// Add XML declaration
	body = append([]byte(xml.Header), body...)

	// Build request
	url := fmt.Sprintf("http://%s:%d%s", device.IP, device.Port, AVTransportControlURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPACTION", fmt.Sprintf("\"%s#%s\"", AVTransportURN, actionName))

	// DEBUG: Log the full SOAP request
	log.Debug(ctx, "SOAP Request", "url", url, "action", actionName, "body", string(body))

	// Send request
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		// Try to parse SOAP fault for better error messages
		if upnpErr := parseSOAPFault(respBody); upnpErr != nil {
			log.Error(ctx, "SOAP fault received", "action", actionName,
				"code", upnpErr.Code, "description", upnpErr.Description)
			return nil, upnpErr
		}
		return nil, fmt.Errorf("SOAP request failed: %d - %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// parseSOAPFault attempts to parse a SOAP fault response and return a UPnPError
func parseSOAPFault(body []byte) *UPnPError {
	bodyStr := string(body)

	// Look for UPnP error code in the fault response
	// Format: <errorCode>714</errorCode>
	codeStart := strings.Index(bodyStr, "<errorCode>")
	if codeStart == -1 {
		return nil
	}
	codeStart += len("<errorCode>")
	codeEnd := strings.Index(bodyStr[codeStart:], "</errorCode>")
	if codeEnd == -1 {
		return nil
	}

	codeStr := bodyStr[codeStart : codeStart+codeEnd]
	code, err := strconv.Atoi(codeStr)
	if err != nil {
		return nil
	}

	// Get human-readable description
	description := upnpErrorDescription(code)

	// Also try to extract the device's error description if present
	descStart := strings.Index(bodyStr, "<errorDescription>")
	if descStart != -1 {
		descStart += len("<errorDescription>")
		descEnd := strings.Index(bodyStr[descStart:], "</errorDescription>")
		if descEnd != -1 {
			deviceDesc := bodyStr[descStart : descStart+descEnd]
			if deviceDesc != "" {
				description = fmt.Sprintf("%s (%s)", description, deviceDesc)
			}
		}
	}

	return &UPnPError{
		Code:        code,
		Description: description,
	}
}

// parseTrackMetadata parses DIDL-Lite metadata to extract track info
func (a *AVTransport) parseTrackMetadata(metadata string, track *Track) {
	// Decode HTML entities
	metadata = html.UnescapeString(metadata)

	// Extract title
	if start := strings.Index(metadata, "<dc:title>"); start != -1 {
		start += len("<dc:title>")
		if end := strings.Index(metadata[start:], "</dc:title>"); end != -1 {
			track.Title = metadata[start : start+end]
		}
	}

	// Extract artist/creator
	if start := strings.Index(metadata, "<dc:creator>"); start != -1 {
		start += len("<dc:creator>")
		if end := strings.Index(metadata[start:], "</dc:creator>"); end != -1 {
			track.Artist = metadata[start : start+end]
		}
	}

	// Extract album
	if start := strings.Index(metadata, "<upnp:album>"); start != -1 {
		start += len("<upnp:album>")
		if end := strings.Index(metadata[start:], "</upnp:album>"); end != -1 {
			track.Album = metadata[start : start+end]
		}
	}

	// Extract album art
	if start := strings.Index(metadata, "<upnp:albumArtURI>"); start != -1 {
		start += len("<upnp:albumArtURI>")
		if end := strings.Index(metadata[start:], "</upnp:albumArtURI>"); end != -1 {
			track.AlbumArt = metadata[start : start+end]
		}
	}
}

// parseDuration parses HH:MM:SS format to seconds
func parseDuration(duration string) int {
	parts := strings.Split(duration, ":")
	if len(parts) != 3 {
		return 0
	}

	hours, _ := strconv.Atoi(parts[0])
	minutes, _ := strconv.Atoi(parts[1])
	seconds, _ := strconv.Atoi(parts[2])

	return hours*3600 + minutes*60 + seconds
}

// extractSOAPResponse extracts the response body from SOAP envelope
func extractSOAPResponse(body []byte, v interface{}) error {
	// Simple extraction - find the response element and unmarshal it
	bodyStr := string(body)

	// Find the Body element content
	startBody := strings.Index(bodyStr, "<s:Body>")
	if startBody == -1 {
		startBody = strings.Index(bodyStr, "<Body>")
	}
	if startBody == -1 {
		return fmt.Errorf("no SOAP Body found")
	}

	endBody := strings.Index(bodyStr, "</s:Body>")
	if endBody == -1 {
		endBody = strings.Index(bodyStr, "</Body>")
	}
	if endBody == -1 {
		return fmt.Errorf("no SOAP Body end found")
	}

	// Extract body content
	startBody += len("<s:Body>")
	if strings.HasPrefix(bodyStr[startBody:], ">") {
		startBody++
	}
	content := strings.TrimSpace(bodyStr[startBody:endBody])

	// Remove namespace prefix from response element
	content = strings.ReplaceAll(content, "u:", "")

	return xml.Unmarshal([]byte(content), v)
}

// BuildDIDLMetadata creates DIDL-Lite metadata for a track
// Uses musicTrack format for discrete file playback
// The streamURI and mimeType are REQUIRED for Sonos to understand the content
// durationSecs is the track duration in seconds (0 to omit)
func BuildDIDLMetadata(id, title, artist, album, albumArtURL, streamURI, mimeType string, durationSecs float32) string {
	// Build metadata with proper artist/album info for discrete tracks
	var albumArtElement string
	if albumArtURL != "" {
		albumArtElement = fmt.Sprintf("<upnp:albumArtURI>%s</upnp:albumArtURI>\n", html.EscapeString(albumArtURL))
	}

	var creatorElement string
	if artist != "" {
		creatorElement = fmt.Sprintf("<dc:creator>%s</dc:creator>\n", html.EscapeString(artist))
	}

	var albumElement string
	if album != "" {
		albumElement = fmt.Sprintf("<upnp:album>%s</upnp:album>\n", html.EscapeString(album))
	}

	// Default MIME type if not specified
	if mimeType == "" {
		mimeType = "audio/flac"
	}

	// The <res> element is CRITICAL - it tells Sonos the protocol and MIME type
	// Without it, Sonos returns error 714 (Illegal MIME-Type)
	// Include duration attribute if provided (format: H:MM:SS or H:MM:SS.mmm)
	protocolInfo := fmt.Sprintf("http-get:*:%s:*", mimeType)
	var durationAttr string
	if durationSecs > 0 {
		hours := int(durationSecs) / 3600
		minutes := (int(durationSecs) % 3600) / 60
		seconds := int(durationSecs) % 60
		durationAttr = fmt.Sprintf(" duration=\"%d:%02d:%02d\"", hours, minutes, seconds)
	}
	resElement := fmt.Sprintf("<res protocolInfo=\"%s\"%s>%s</res>\n", protocolInfo, durationAttr, html.EscapeString(streamURI))

	return fmt.Sprintf(`<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/">
<item id="%s" parentID="0" restricted="true">
<dc:title>%s</dc:title>
%s%s%s%s<upnp:class>object.item.audioItem.musicTrack</upnp:class>
</item>
</DIDL-Lite>`,
		html.EscapeString(id),
		html.EscapeString(title),
		creatorElement,
		albumElement,
		albumArtElement,
		resElement)
}
