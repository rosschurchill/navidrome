package dlna

import (
	"encoding/xml"
	"fmt"
	"net/http"

	"github.com/navidrome/navidrome/consts"
)

// UPnP device description XML structures

// DeviceDescription is the root element of device.xml
type DeviceDescription struct {
	XMLName     xml.Name `xml:"urn:schemas-upnp-org:device-1-0 root"`
	SpecVersion SpecVersion
	Device      Device
}

// SpecVersion defines the UPnP spec version
type SpecVersion struct {
	Major int `xml:"major"`
	Minor int `xml:"minor"`
}

// Device describes the DLNA media server
type Device struct {
	DeviceType       string    `xml:"deviceType"`
	FriendlyName     string    `xml:"friendlyName"`
	Manufacturer     string    `xml:"manufacturer"`
	ManufacturerURL  string    `xml:"manufacturerURL,omitempty"`
	ModelDescription string    `xml:"modelDescription,omitempty"`
	ModelName        string    `xml:"modelName"`
	ModelNumber      string    `xml:"modelNumber,omitempty"`
	ModelURL         string    `xml:"modelURL,omitempty"`
	SerialNumber     string    `xml:"serialNumber,omitempty"`
	UDN              string    `xml:"UDN"`
	IconList         *IconList `xml:"iconList,omitempty"`
	ServiceList      ServiceList
	PresentationURL  string `xml:"presentationURL,omitempty"`
}

// IconList contains device icons
type IconList struct {
	Icons []Icon `xml:"icon"`
}

// Icon describes a device icon
type Icon struct {
	MIMEType string `xml:"mimetype"`
	Width    int    `xml:"width"`
	Height   int    `xml:"height"`
	Depth    int    `xml:"depth"`
	URL      string `xml:"url"`
}

// ServiceList contains device services
type ServiceList struct {
	Services []Service `xml:"service"`
}

// Service describes a UPnP service
type Service struct {
	ServiceType string `xml:"serviceType"`
	ServiceID   string `xml:"serviceId"`
	SCPDURL     string `xml:"SCPDURL"`
	ControlURL  string `xml:"controlURL"`
	EventSubURL string `xml:"eventSubURL"`
}

// handleDeviceDescription returns the UPnP device description XML
func (r *Router) handleDeviceDescription(w http.ResponseWriter, req *http.Request) {
	baseURL := r.getBaseURL(req)

	desc := DeviceDescription{
		SpecVersion: SpecVersion{Major: 1, Minor: 1},
		Device: Device{
			DeviceType:       deviceType,
			FriendlyName:     r.serverName,
			Manufacturer:     "Navidrome",
			ManufacturerURL:  "https://www.navidrome.org",
			ModelDescription: "Navidrome Music Server with DLNA support",
			ModelName:        "Navidrome",
			ModelNumber:      consts.Version,
			ModelURL:         "https://www.navidrome.org",
			UDN:              r.uuid,
			IconList: &IconList{
				Icons: []Icon{
					{MIMEType: "image/png", Width: 48, Height: 48, Depth: 24, URL: fmt.Sprintf("%s/dlna/icon/48.png", baseURL)},
					{MIMEType: "image/png", Width: 120, Height: 120, Depth: 24, URL: fmt.Sprintf("%s/dlna/icon/120.png", baseURL)},
				},
			},
			ServiceList: ServiceList{
				Services: []Service{
					{
						ServiceType: contentDirectoryType,
						ServiceID:   "urn:upnp-org:serviceId:ContentDirectory",
						SCPDURL:     fmt.Sprintf("%s/dlna/ContentDirectory.xml", baseURL),
						ControlURL:  fmt.Sprintf("%s/dlna/ContentDirectory/control", baseURL),
						EventSubURL: "",
					},
					{
						ServiceType: connectionManagerType,
						ServiceID:   "urn:upnp-org:serviceId:ConnectionManager",
						SCPDURL:     fmt.Sprintf("%s/dlna/ConnectionManager.xml", baseURL),
						ControlURL:  fmt.Sprintf("%s/dlna/ConnectionManager/control", baseURL),
						EventSubURL: "",
					},
				},
			},
			PresentationURL: baseURL + "/",
		},
	}

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(desc); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleContentDirectoryDescription returns the ContentDirectory service description
func (r *Router) handleContentDirectoryDescription(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Write([]byte(contentDirectorySCPD))
}

// handleConnectionManagerDescription returns the ConnectionManager service description
func (r *Router) handleConnectionManagerDescription(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Write([]byte(connectionManagerSCPD))
}

// handleIcon serves device icons
func (r *Router) handleIcon(w http.ResponseWriter, req *http.Request) {
	// Return a simple placeholder response for now
	// In production, this would serve actual icon images
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	// TODO: Serve actual icon from resources
}

// getBaseURL returns the base URL for device description URLs
func (r *Router) getBaseURL(req *http.Request) string {
	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s", scheme, req.Host)
}

// ContentDirectory Service Control Protocol Description
var contentDirectorySCPD = `<?xml version="1.0" encoding="utf-8"?>
<scpd xmlns="urn:schemas-upnp-org:service-1-0">
  <specVersion>
    <major>1</major>
    <minor>0</minor>
  </specVersion>
  <actionList>
    <action>
      <name>Browse</name>
      <argumentList>
        <argument>
          <name>ObjectID</name>
          <direction>in</direction>
          <relatedStateVariable>A_ARG_TYPE_ObjectID</relatedStateVariable>
        </argument>
        <argument>
          <name>BrowseFlag</name>
          <direction>in</direction>
          <relatedStateVariable>A_ARG_TYPE_BrowseFlag</relatedStateVariable>
        </argument>
        <argument>
          <name>Filter</name>
          <direction>in</direction>
          <relatedStateVariable>A_ARG_TYPE_Filter</relatedStateVariable>
        </argument>
        <argument>
          <name>StartingIndex</name>
          <direction>in</direction>
          <relatedStateVariable>A_ARG_TYPE_Index</relatedStateVariable>
        </argument>
        <argument>
          <name>RequestedCount</name>
          <direction>in</direction>
          <relatedStateVariable>A_ARG_TYPE_Count</relatedStateVariable>
        </argument>
        <argument>
          <name>SortCriteria</name>
          <direction>in</direction>
          <relatedStateVariable>A_ARG_TYPE_SortCriteria</relatedStateVariable>
        </argument>
        <argument>
          <name>Result</name>
          <direction>out</direction>
          <relatedStateVariable>A_ARG_TYPE_Result</relatedStateVariable>
        </argument>
        <argument>
          <name>NumberReturned</name>
          <direction>out</direction>
          <relatedStateVariable>A_ARG_TYPE_Count</relatedStateVariable>
        </argument>
        <argument>
          <name>TotalMatches</name>
          <direction>out</direction>
          <relatedStateVariable>A_ARG_TYPE_Count</relatedStateVariable>
        </argument>
        <argument>
          <name>UpdateID</name>
          <direction>out</direction>
          <relatedStateVariable>A_ARG_TYPE_UpdateID</relatedStateVariable>
        </argument>
      </argumentList>
    </action>
    <action>
      <name>GetSearchCapabilities</name>
      <argumentList>
        <argument>
          <name>SearchCaps</name>
          <direction>out</direction>
          <relatedStateVariable>SearchCapabilities</relatedStateVariable>
        </argument>
      </argumentList>
    </action>
    <action>
      <name>GetSortCapabilities</name>
      <argumentList>
        <argument>
          <name>SortCaps</name>
          <direction>out</direction>
          <relatedStateVariable>SortCapabilities</relatedStateVariable>
        </argument>
      </argumentList>
    </action>
    <action>
      <name>GetSystemUpdateID</name>
      <argumentList>
        <argument>
          <name>Id</name>
          <direction>out</direction>
          <relatedStateVariable>SystemUpdateID</relatedStateVariable>
        </argument>
      </argumentList>
    </action>
  </actionList>
  <serviceStateTable>
    <stateVariable sendEvents="no">
      <name>A_ARG_TYPE_ObjectID</name>
      <dataType>string</dataType>
    </stateVariable>
    <stateVariable sendEvents="no">
      <name>A_ARG_TYPE_Result</name>
      <dataType>string</dataType>
    </stateVariable>
    <stateVariable sendEvents="no">
      <name>A_ARG_TYPE_BrowseFlag</name>
      <dataType>string</dataType>
      <allowedValueList>
        <allowedValue>BrowseMetadata</allowedValue>
        <allowedValue>BrowseDirectChildren</allowedValue>
      </allowedValueList>
    </stateVariable>
    <stateVariable sendEvents="no">
      <name>A_ARG_TYPE_Filter</name>
      <dataType>string</dataType>
    </stateVariable>
    <stateVariable sendEvents="no">
      <name>A_ARG_TYPE_SortCriteria</name>
      <dataType>string</dataType>
    </stateVariable>
    <stateVariable sendEvents="no">
      <name>A_ARG_TYPE_Index</name>
      <dataType>ui4</dataType>
    </stateVariable>
    <stateVariable sendEvents="no">
      <name>A_ARG_TYPE_Count</name>
      <dataType>ui4</dataType>
    </stateVariable>
    <stateVariable sendEvents="no">
      <name>A_ARG_TYPE_UpdateID</name>
      <dataType>ui4</dataType>
    </stateVariable>
    <stateVariable sendEvents="yes">
      <name>SystemUpdateID</name>
      <dataType>ui4</dataType>
    </stateVariable>
    <stateVariable sendEvents="no">
      <name>SearchCapabilities</name>
      <dataType>string</dataType>
    </stateVariable>
    <stateVariable sendEvents="no">
      <name>SortCapabilities</name>
      <dataType>string</dataType>
    </stateVariable>
  </serviceStateTable>
</scpd>`

// ConnectionManager Service Control Protocol Description
var connectionManagerSCPD = `<?xml version="1.0" encoding="utf-8"?>
<scpd xmlns="urn:schemas-upnp-org:service-1-0">
  <specVersion>
    <major>1</major>
    <minor>0</minor>
  </specVersion>
  <actionList>
    <action>
      <name>GetProtocolInfo</name>
      <argumentList>
        <argument>
          <name>Source</name>
          <direction>out</direction>
          <relatedStateVariable>SourceProtocolInfo</relatedStateVariable>
        </argument>
        <argument>
          <name>Sink</name>
          <direction>out</direction>
          <relatedStateVariable>SinkProtocolInfo</relatedStateVariable>
        </argument>
      </argumentList>
    </action>
    <action>
      <name>GetCurrentConnectionIDs</name>
      <argumentList>
        <argument>
          <name>ConnectionIDs</name>
          <direction>out</direction>
          <relatedStateVariable>CurrentConnectionIDs</relatedStateVariable>
        </argument>
      </argumentList>
    </action>
    <action>
      <name>GetCurrentConnectionInfo</name>
      <argumentList>
        <argument>
          <name>ConnectionID</name>
          <direction>in</direction>
          <relatedStateVariable>A_ARG_TYPE_ConnectionID</relatedStateVariable>
        </argument>
        <argument>
          <name>RcsID</name>
          <direction>out</direction>
          <relatedStateVariable>A_ARG_TYPE_RcsID</relatedStateVariable>
        </argument>
        <argument>
          <name>AVTransportID</name>
          <direction>out</direction>
          <relatedStateVariable>A_ARG_TYPE_AVTransportID</relatedStateVariable>
        </argument>
        <argument>
          <name>ProtocolInfo</name>
          <direction>out</direction>
          <relatedStateVariable>A_ARG_TYPE_ProtocolInfo</relatedStateVariable>
        </argument>
        <argument>
          <name>PeerConnectionManager</name>
          <direction>out</direction>
          <relatedStateVariable>A_ARG_TYPE_ConnectionManager</relatedStateVariable>
        </argument>
        <argument>
          <name>PeerConnectionID</name>
          <direction>out</direction>
          <relatedStateVariable>A_ARG_TYPE_ConnectionID</relatedStateVariable>
        </argument>
        <argument>
          <name>Direction</name>
          <direction>out</direction>
          <relatedStateVariable>A_ARG_TYPE_Direction</relatedStateVariable>
        </argument>
        <argument>
          <name>Status</name>
          <direction>out</direction>
          <relatedStateVariable>A_ARG_TYPE_ConnectionStatus</relatedStateVariable>
        </argument>
      </argumentList>
    </action>
  </actionList>
  <serviceStateTable>
    <stateVariable sendEvents="yes">
      <name>SourceProtocolInfo</name>
      <dataType>string</dataType>
    </stateVariable>
    <stateVariable sendEvents="yes">
      <name>SinkProtocolInfo</name>
      <dataType>string</dataType>
    </stateVariable>
    <stateVariable sendEvents="yes">
      <name>CurrentConnectionIDs</name>
      <dataType>string</dataType>
    </stateVariable>
    <stateVariable sendEvents="no">
      <name>A_ARG_TYPE_ConnectionStatus</name>
      <dataType>string</dataType>
      <allowedValueList>
        <allowedValue>OK</allowedValue>
        <allowedValue>ContentFormatMismatch</allowedValue>
        <allowedValue>InsufficientBandwidth</allowedValue>
        <allowedValue>UnreliableChannel</allowedValue>
        <allowedValue>Unknown</allowedValue>
      </allowedValueList>
    </stateVariable>
    <stateVariable sendEvents="no">
      <name>A_ARG_TYPE_ConnectionManager</name>
      <dataType>string</dataType>
    </stateVariable>
    <stateVariable sendEvents="no">
      <name>A_ARG_TYPE_Direction</name>
      <dataType>string</dataType>
      <allowedValueList>
        <allowedValue>Input</allowedValue>
        <allowedValue>Output</allowedValue>
      </allowedValueList>
    </stateVariable>
    <stateVariable sendEvents="no">
      <name>A_ARG_TYPE_ProtocolInfo</name>
      <dataType>string</dataType>
    </stateVariable>
    <stateVariable sendEvents="no">
      <name>A_ARG_TYPE_ConnectionID</name>
      <dataType>i4</dataType>
    </stateVariable>
    <stateVariable sendEvents="no">
      <name>A_ARG_TYPE_AVTransportID</name>
      <dataType>i4</dataType>
    </stateVariable>
    <stateVariable sendEvents="no">
      <name>A_ARG_TYPE_RcsID</name>
      <dataType>i4</dataType>
    </stateVariable>
  </serviceStateTable>
</scpd>`
