package sonos_cast

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/navidrome/navidrome/log"
)

// RenderingControl provides volume and mute control for Sonos devices
type RenderingControl struct {
	client *http.Client
}

// NewRenderingControl creates a new RenderingControl controller
func NewRenderingControl() *RenderingControl {
	return &RenderingControl{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetVolume gets the current volume level (0-100)
func (r *RenderingControl) GetVolume(ctx context.Context, device *SonosDevice) (int, error) {
	action := GetVolumeAction{
		XmlnsU:     RenderingControlURN,
		InstanceID: 0,
		Channel:    "Master",
	}

	respBody, err := r.sendAction(ctx, device, "GetVolume", action)
	if err != nil {
		return 0, fmt.Errorf("GetVolume failed: %w", err)
	}

	var resp GetVolumeResponse
	if err := extractSOAPResponse(respBody, &resp); err != nil {
		return 0, fmt.Errorf("failed to parse GetVolume response: %w", err)
	}

	return resp.CurrentVolume, nil
}

// SetVolume sets the volume level (0-100)
func (r *RenderingControl) SetVolume(ctx context.Context, device *SonosDevice, volume int) error {
	// Clamp volume to valid range
	if volume < 0 {
		volume = 0
	}
	if volume > 100 {
		volume = 100
	}

	action := SetVolumeAction{
		XmlnsU:        RenderingControlURN,
		InstanceID:    0,
		Channel:       "Master",
		DesiredVolume: volume,
	}

	_, err := r.sendAction(ctx, device, "SetVolume", action)
	if err != nil {
		return fmt.Errorf("SetVolume failed: %w", err)
	}

	log.Debug(ctx, "Set volume", "device", device.RoomName, "volume", volume)
	return nil
}

// GetMute gets the current mute state
func (r *RenderingControl) GetMute(ctx context.Context, device *SonosDevice) (bool, error) {
	action := GetMuteAction{
		XmlnsU:     RenderingControlURN,
		InstanceID: 0,
		Channel:    "Master",
	}

	respBody, err := r.sendAction(ctx, device, "GetMute", action)
	if err != nil {
		return false, fmt.Errorf("GetMute failed: %w", err)
	}

	var resp GetMuteResponse
	if err := extractSOAPResponse(respBody, &resp); err != nil {
		return false, fmt.Errorf("failed to parse GetMute response: %w", err)
	}

	return resp.CurrentMute == 1, nil
}

// SetMute sets the mute state
func (r *RenderingControl) SetMute(ctx context.Context, device *SonosDevice, mute bool) error {
	muteVal := 0
	if mute {
		muteVal = 1
	}

	action := SetMuteAction{
		XmlnsU:      RenderingControlURN,
		InstanceID:  0,
		Channel:     "Master",
		DesiredMute: muteVal,
	}

	_, err := r.sendAction(ctx, device, "SetMute", action)
	if err != nil {
		return fmt.Errorf("SetMute failed: %w", err)
	}

	log.Debug(ctx, "Set mute", "device", device.RoomName, "muted", mute)
	return nil
}

// ToggleMute toggles the mute state and returns the new state
func (r *RenderingControl) ToggleMute(ctx context.Context, device *SonosDevice) (bool, error) {
	currentMute, err := r.GetMute(ctx, device)
	if err != nil {
		return false, err
	}

	newMute := !currentMute
	if err := r.SetMute(ctx, device, newMute); err != nil {
		return false, err
	}

	return newMute, nil
}

// AdjustVolume adjusts volume by a relative amount
func (r *RenderingControl) AdjustVolume(ctx context.Context, device *SonosDevice, delta int) (int, error) {
	currentVolume, err := r.GetVolume(ctx, device)
	if err != nil {
		return 0, err
	}

	newVolume := currentVolume + delta
	if newVolume < 0 {
		newVolume = 0
	}
	if newVolume > 100 {
		newVolume = 100
	}

	if err := r.SetVolume(ctx, device, newVolume); err != nil {
		return 0, err
	}

	return newVolume, nil
}

// sendAction sends a SOAP action to the device's RenderingControl service
func (r *RenderingControl) sendAction(ctx context.Context, device *SonosDevice, actionName string, action interface{}) ([]byte, error) {
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
	url := fmt.Sprintf("http://%s:%d%s", device.IP, device.Port, RenderingControlControlURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPACTION", fmt.Sprintf("\"%s#%s\"", RenderingControlURN, actionName))

	// Send request
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SOAP request failed: %d - %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
