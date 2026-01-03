package sonos_cast

import (
	"encoding/xml"
	"sync"
	"time"
)

// SonosDevice represents a discovered Sonos speaker
type SonosDevice struct {
	IP            string    `json:"ip"`
	Port          int       `json:"port"`
	UUID          string    `json:"uuid"`
	RoomName      string    `json:"roomName"`
	ModelName     string    `json:"modelName"`
	ModelNumber   string    `json:"modelNumber"`
	SoftwareGen   string    `json:"softwareGen"` // S1 or S2
	IsCoordinator bool      `json:"isCoordinator"`
	GroupID       string    `json:"groupId"`
	GroupMembers  []string  `json:"groupMembers,omitempty"` // UUIDs of group members
	LastSeen      time.Time `json:"lastSeen"`
}

// PlaybackState represents the current playback state of a speaker
type PlaybackState struct {
	State        string `json:"state"` // PLAYING, PAUSED_PLAYBACK, STOPPED
	CurrentTrack *Track `json:"currentTrack,omitempty"`
	Volume       int    `json:"volume"`
	Muted        bool   `json:"muted"`
}

// Track represents currently playing track info
type Track struct {
	URI       string `json:"uri"`
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	Album     string `json:"album"`
	AlbumArt  string `json:"albumArt"`
	Duration  int    `json:"duration"`  // seconds
	Position  int    `json:"position"`  // seconds
	TrackNum  int    `json:"trackNum"`
	QueueSize int    `json:"queueSize"`

	// Quality info
	Format      string `json:"format,omitempty"`      // FLAC, MP3, AAC, etc.
	BitRate     int    `json:"bitRate,omitempty"`     // kbps
	SampleRate  int    `json:"sampleRate,omitempty"`  // Hz (e.g., 44100, 48000)
	BitDepth    int    `json:"bitDepth,omitempty"`    // bits (e.g., 16, 24)
	Transcoding bool   `json:"transcoding,omitempty"` // true if stream is being transcoded
}

// PlayRequest is the request body for playing media
type PlayRequest struct {
	Type       string `json:"type"`       // track, album, playlist
	ID         string `json:"id"`         // media ID
	StartIndex int    `json:"startIndex"` // for albums/playlists
	Shuffle    bool   `json:"shuffle"`
}

// VolumeRequest is the request body for volume control
type VolumeRequest struct {
	Volume int `json:"volume"` // 0-100
}

// DeviceCache holds discovered devices with thread-safe access
type DeviceCache struct {
	mu      sync.RWMutex
	devices map[string]*SonosDevice // keyed by UUID
}

func NewDeviceCache() *DeviceCache {
	return &DeviceCache{
		devices: make(map[string]*SonosDevice),
	}
}

func (c *DeviceCache) Set(device *SonosDevice) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.devices[device.UUID] = device
}

func (c *DeviceCache) Get(uuid string) (*SonosDevice, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	d, ok := c.devices[uuid]
	return d, ok
}

func (c *DeviceCache) GetAll() []*SonosDevice {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]*SonosDevice, 0, len(c.devices))
	for _, d := range c.devices {
		result = append(result, d)
	}
	return result
}

func (c *DeviceCache) Remove(uuid string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.devices, uuid)
}

// XML types for parsing Sonos device description

type DeviceDescription struct {
	XMLName     xml.Name `xml:"root"`
	Device      Device   `xml:"device"`
	DisplayName string   `xml:"displayName"` // Sonos-specific
}

type Device struct {
	DeviceType       string    `xml:"deviceType"`
	FriendlyName     string    `xml:"friendlyName"`
	Manufacturer     string    `xml:"manufacturer"`
	ModelName        string    `xml:"modelName"`
	ModelNumber      string    `xml:"modelNumber"`
	SoftwareVersion  string    `xml:"softwareVersion"`
	HardwareVersion  string    `xml:"hardwareVersion"`
	SerialNum        string    `xml:"serialNum"`
	UDN              string    `xml:"UDN"` // uuid:RINCON_xxx
	RoomName         string    `xml:"roomName"`
	DisplayName      string    `xml:"displayName"`
	ServiceList      []Service `xml:"serviceList>service"`
	SoftwareGen      string    `xml:"swGen"` // 1 or 2 (S1/S2)
	MinCompatibleVer string    `xml:"minCompatibleVersion"`
}

type Service struct {
	ServiceType string `xml:"serviceType"`
	ServiceId   string `xml:"serviceId"`
	ControlURL  string `xml:"controlURL"`
	EventSubURL string `xml:"eventSubURL"`
	SCPDURL     string `xml:"SCPDURL"`
}

// XML types for ZoneGroupTopology

type ZoneGroupState struct {
	XMLName    xml.Name    `xml:"ZoneGroupState"`
	ZoneGroups []ZoneGroup `xml:"ZoneGroups>ZoneGroup"`
}

type ZoneGroup struct {
	Coordinator string       `xml:"Coordinator,attr"`
	ID          string       `xml:"ID,attr"`
	Members     []ZoneMember `xml:"ZoneGroupMember"`
}

type ZoneMember struct {
	UUID     string `xml:"UUID,attr"`
	Location string `xml:"Location,attr"`
	ZoneName string `xml:"ZoneName,attr"`
}

// SOAP envelope types

type SOAPEnvelope struct {
	XMLName       xml.Name `xml:"s:Envelope"`
	XmlnsS        string   `xml:"xmlns:s,attr"`
	EncodingStyle string   `xml:"s:encodingStyle,attr"`
	Body          SOAPBody `xml:"s:Body"`
}

type SOAPBody struct {
	Content interface{} `xml:",any"`
}

// AVTransport SOAP actions

type SetAVTransportURIAction struct {
	XMLName            xml.Name `xml:"u:SetAVTransportURI"`
	XmlnsU             string   `xml:"xmlns:u,attr"`
	InstanceID         int      `xml:"InstanceID"`
	CurrentURI         string   `xml:"CurrentURI"`
	CurrentURIMetaData string   `xml:"CurrentURIMetaData"`
}

type PlayAction struct {
	XMLName    xml.Name `xml:"u:Play"`
	XmlnsU     string   `xml:"xmlns:u,attr"`
	InstanceID int      `xml:"InstanceID"`
	Speed      string   `xml:"Speed"`
}

type PauseAction struct {
	XMLName    xml.Name `xml:"u:Pause"`
	XmlnsU     string   `xml:"xmlns:u,attr"`
	InstanceID int      `xml:"InstanceID"`
}

type StopAction struct {
	XMLName    xml.Name `xml:"u:Stop"`
	XmlnsU     string   `xml:"xmlns:u,attr"`
	InstanceID int      `xml:"InstanceID"`
}

type SeekAction struct {
	XMLName    xml.Name `xml:"u:Seek"`
	XmlnsU     string   `xml:"xmlns:u,attr"`
	InstanceID int      `xml:"InstanceID"`
	Unit       string   `xml:"Unit"`
	Target     string   `xml:"Target"`
}

type NextAction struct {
	XMLName    xml.Name `xml:"u:Next"`
	XmlnsU     string   `xml:"xmlns:u,attr"`
	InstanceID int      `xml:"InstanceID"`
}

type PreviousAction struct {
	XMLName    xml.Name `xml:"u:Previous"`
	XmlnsU     string   `xml:"xmlns:u,attr"`
	InstanceID int      `xml:"InstanceID"`
}

type GetPositionInfoAction struct {
	XMLName    xml.Name `xml:"u:GetPositionInfo"`
	XmlnsU     string   `xml:"xmlns:u,attr"`
	InstanceID int      `xml:"InstanceID"`
}

type GetTransportInfoAction struct {
	XMLName    xml.Name `xml:"u:GetTransportInfo"`
	XmlnsU     string   `xml:"xmlns:u,attr"`
	InstanceID int      `xml:"InstanceID"`
}

// AVTransport SOAP responses

type GetPositionInfoResponse struct {
	XMLName       xml.Name `xml:"GetPositionInfoResponse"`
	Track         int      `xml:"Track"`
	TrackDuration string   `xml:"TrackDuration"`
	TrackMetaData string   `xml:"TrackMetaData"`
	TrackURI      string   `xml:"TrackURI"`
	RelTime       string   `xml:"RelTime"`
	AbsTime       string   `xml:"AbsTime"`
	RelCount      int      `xml:"RelCount"`
	AbsCount      int      `xml:"AbsCount"`
}

type GetTransportInfoResponse struct {
	XMLName               xml.Name `xml:"GetTransportInfoResponse"`
	CurrentTransportState string   `xml:"CurrentTransportState"`
	CurrentSpeed          string   `xml:"CurrentTransportSpeed"`
}

// RenderingControl SOAP actions

type GetVolumeAction struct {
	XMLName    xml.Name `xml:"u:GetVolume"`
	XmlnsU     string   `xml:"xmlns:u,attr"`
	InstanceID int      `xml:"InstanceID"`
	Channel    string   `xml:"Channel"`
}

type SetVolumeAction struct {
	XMLName       xml.Name `xml:"u:SetVolume"`
	XmlnsU        string   `xml:"xmlns:u,attr"`
	InstanceID    int      `xml:"InstanceID"`
	Channel       string   `xml:"Channel"`
	DesiredVolume int      `xml:"DesiredVolume"`
}

type GetMuteAction struct {
	XMLName    xml.Name `xml:"u:GetMute"`
	XmlnsU     string   `xml:"xmlns:u,attr"`
	InstanceID int      `xml:"InstanceID"`
	Channel    string   `xml:"Channel"`
}

type SetMuteAction struct {
	XMLName     xml.Name `xml:"u:SetMute"`
	XmlnsU      string   `xml:"xmlns:u,attr"`
	InstanceID  int      `xml:"InstanceID"`
	Channel     string   `xml:"Channel"`
	DesiredMute int      `xml:"DesiredMute"` // 0 or 1
}

// RenderingControl SOAP responses

type GetVolumeResponse struct {
	XMLName       xml.Name `xml:"GetVolumeResponse"`
	CurrentVolume int      `xml:"CurrentVolume"`
}

type GetMuteResponse struct {
	XMLName     xml.Name `xml:"GetMuteResponse"`
	CurrentMute int      `xml:"CurrentMute"`
}

// Constants
const (
	SonosPort = 1400

	// Service URNs
	AVTransportURN       = "urn:schemas-upnp-org:service:AVTransport:1"
	RenderingControlURN  = "urn:schemas-upnp-org:service:RenderingControl:1"
	ZoneGroupTopologyURN = "urn:upnp-org:serviceId:ZoneGroupTopology"

	// Control URLs
	AVTransportControlURL      = "/MediaRenderer/AVTransport/Control"
	RenderingControlControlURL = "/MediaRenderer/RenderingControl/Control"
	ZoneGroupTopologyURL       = "/ZoneGroupTopology/Control"

	// Transport states
	StatePlaying = "PLAYING"
	StatePaused  = "PAUSED_PLAYBACK"
	StateStopped = "STOPPED"
)
