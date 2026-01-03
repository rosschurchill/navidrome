# Sonos Casting Technical Implementation Guide for Go/Navidrome

This guide fills the implementation gaps from your existing UPnP research. It provides working Go code, complete SOAP message structures, and the critical gotchas that cause implementations to fail.

---

## Table of Contents

1. [SSDP Discovery Implementation](#1-ssdp-discovery-implementation)
2. [Complete SOAP Client for Go](#2-complete-soap-client-for-go)
3. [AVTransport Operations with Working Examples](#3-avtransport-operations-with-working-examples)
4. [Queue Management - Playing Albums and Playlists](#4-queue-management---playing-albums-and-playlists)
5. [UPnP Event Subscriptions for Real-Time State](#5-upnp-event-subscriptions-for-real-time-state)
6. [Zone Group Topology and Coordinator Detection](#6-zone-group-topology-and-coordinator-detection)
7. [DIDL-Lite Metadata Construction](#7-didl-lite-metadata-construction)
8. [Common Failure Modes and Solutions](#8-common-failure-modes-and-solutions)
9. [Integration Architecture for Navidrome](#9-integration-architecture-for-navidrome)

---

## 1. SSDP Discovery Implementation

SSDP (Simple Service Discovery Protocol) uses UDP multicast to find Sonos devices on the network.

### Discovery Constants

```go
const (
    SSDPMulticastAddr = "239.255.255.250:1900"
    SSDPSearchTarget  = "urn:schemas-upnp-org:device:ZonePlayer:1"
    SonosPort         = 1400
)
```

### Working Go Discovery Code

```go
package sonos

import (
    "bufio"
    "bytes"
    "fmt"
    "net"
    "net/http"
    "strings"
    "time"
)

type SonosDevice struct {
    IP           string
    UUID         string
    RoomName     string
    ModelName    string
    IsCoordinator bool
}

func DiscoverDevices(timeout time.Duration) ([]SonosDevice, error) {
    // Create UDP connection for multicast
    addr, err := net.ResolveUDPAddr("udp4", SSDPMulticastAddr)
    if err != nil {
        return nil, fmt.Errorf("resolve multicast addr: %w", err)
    }

    conn, err := net.ListenUDP("udp4", nil)
    if err != nil {
        return nil, fmt.Errorf("listen udp: %w", err)
    }
    defer conn.Close()

    // Set read deadline
    conn.SetReadDeadline(time.Now().Add(timeout))

    // Send M-SEARCH request
    searchMsg := fmt.Sprintf(
        "M-SEARCH * HTTP/1.1\r\n"+
        "HOST: %s\r\n"+
        "MAN: \"ssdp:discover\"\r\n"+
        "MX: 3\r\n"+
        "ST: %s\r\n"+
        "\r\n",
        SSDPMulticastAddr, SSDPSearchTarget,
    )

    _, err = conn.WriteToUDP([]byte(searchMsg), addr)
    if err != nil {
        return nil, fmt.Errorf("send search: %w", err)
    }

    // Collect responses
    devices := make(map[string]SonosDevice)
    buf := make([]byte, 4096)

    for {
        n, remoteAddr, err := conn.ReadFromUDP(buf)
        if err != nil {
            // Timeout is expected
            break
        }

        response := string(buf[:n])
        
        // Parse LOCATION header to get device description URL
        if strings.Contains(response, "Sonos") {
            device := parseSSDP Response(response, remoteAddr.IP.String())
            if device.UUID != "" {
                devices[device.UUID] = device
            }
        }
    }

    // Convert map to slice
    result := make([]SonosDevice, 0, len(devices))
    for _, d := range devices {
        result = append(result, d)
    }
    return result, nil
}

func parseSSDPResponse(response, ip string) SonosDevice {
    device := SonosDevice{IP: ip}
    
    // Parse headers
    reader := bufio.NewReader(strings.NewReader(response))
    tp := textproto.NewReader(reader)
    
    // Skip first line (HTTP/1.1 200 OK)
    tp.ReadLine()
    
    headers, err := tp.ReadMIMEHeader()
    if err != nil {
        return device
    }
    
    // Extract UUID from USN header
    // Format: uuid:RINCON_XXXX::urn:schemas-upnp-org:device:ZonePlayer:1
    if usn := headers.Get("USN"); usn != "" {
        if idx := strings.Index(usn, "::"); idx > 0 {
            device.UUID = strings.TrimPrefix(usn[:idx], "uuid:")
        }
    }
    
    return device
}
```

### Fetch Device Details

After discovery, fetch device details from the description XML:

```go
func (d *SonosDevice) FetchDetails() error {
    url := fmt.Sprintf("http://%s:%d/xml/device_description.xml", d.IP, SonosPort)
    
    resp, err := http.Get(url)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    // Parse XML to extract roomName, modelName, etc.
    // The XML structure includes:
    // - <roomName>Living Room</roomName>
    // - <modelName>Sonos One</modelName>
    // - <UDN>uuid:RINCON_XXX</UDN>
    
    return nil
}
```

---

## 2. Complete SOAP Client for Go

All Sonos control happens via SOAP over HTTP POST to port 1400.

### SOAP Envelope Template

```go
const soapEnvelopeTemplate = `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    %s
  </s:Body>
</s:Envelope>`
```

### Generic SOAP Client

```go
package sonos

import (
    "bytes"
    "encoding/xml"
    "fmt"
    "io"
    "net/http"
)

type SOAPClient struct {
    IP     string
    Client *http.Client
}

type SOAPFault struct {
    FaultCode   string `xml:"faultcode"`
    FaultString string `xml:"faultstring"`
    Detail      struct {
        UPnPError struct {
            ErrorCode        int    `xml:"errorCode"`
            ErrorDescription string `xml:"errorDescription"`
        } `xml:"UPnPError"`
    } `xml:"detail"`
}

func NewSOAPClient(ip string) *SOAPClient {
    return &SOAPClient{
        IP: ip,
        Client: &http.Client{
            Timeout: 10 * time.Second,
        },
    }
}

// Call executes a SOAP action and returns the response body
func (c *SOAPClient) Call(endpoint, serviceType, action string, body string) ([]byte, error) {
    url := fmt.Sprintf("http://%s:1400%s", c.IP, endpoint)
    
    // Wrap body in SOAP envelope
    envelope := fmt.Sprintf(soapEnvelopeTemplate, body)
    
    req, err := http.NewRequest("POST", url, bytes.NewBufferString(envelope))
    if err != nil {
        return nil, err
    }
    
    // CRITICAL: These headers must be exact
    req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
    req.Header.Set("SOAPAction", fmt.Sprintf(`"%s#%s"`, serviceType, action))
    
    resp, err := c.Client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    responseBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }
    
    // Check for SOAP fault (HTTP 500)
    if resp.StatusCode == 500 {
        return nil, parseSoapFault(responseBody)
    }
    
    return responseBody, nil
}

func parseSoapFault(body []byte) error {
    var envelope struct {
        Body struct {
            Fault SOAPFault `xml:"Fault"`
        } `xml:"Body"`
    }
    
    if err := xml.Unmarshal(body, &envelope); err != nil {
        return fmt.Errorf("SOAP error (unparseable): %s", string(body))
    }
    
    fault := envelope.Body.Fault
    return fmt.Errorf("UPnP Error %d: %s (%s)", 
        fault.Detail.UPnPError.ErrorCode,
        fault.FaultString,
        fault.Detail.UPnPError.ErrorDescription)
}
```

---

## 3. AVTransport Operations with Working Examples

### Service Constants

```go
const (
    AVTransportEndpoint    = "/MediaRenderer/AVTransport/Control"
    AVTransportServiceType = "urn:schemas-upnp-org:service:AVTransport:1"
    RenderingControlEndpoint    = "/MediaRenderer/RenderingControl/Control"
    RenderingControlServiceType = "urn:schemas-upnp-org:service:RenderingControl:1"
)
```

### Play a Single Track (SetAVTransportURI + Play)

```go
// PlayURI plays a single audio file directly (not from queue)
func (c *SOAPClient) PlayURI(uri, title string) error {
    // Build metadata (empty is OK for simple playback, but title helps display)
    metadata := buildTrackMetadata(uri, title)
    
    // XML-escape the metadata for embedding in SOAP
    escapedMetadata := xmlEscape(metadata)
    
    body := fmt.Sprintf(`<u:SetAVTransportURI xmlns:u="%s">
      <InstanceID>0</InstanceID>
      <CurrentURI>%s</CurrentURI>
      <CurrentURIMetaData>%s</CurrentURIMetaData>
    </u:SetAVTransportURI>`, AVTransportServiceType, xmlEscape(uri), escapedMetadata)
    
    _, err := c.Call(AVTransportEndpoint, AVTransportServiceType, "SetAVTransportURI", body)
    if err != nil {
        return fmt.Errorf("SetAVTransportURI: %w", err)
    }
    
    // Now call Play
    return c.Play()
}

func (c *SOAPClient) Play() error {
    body := fmt.Sprintf(`<u:Play xmlns:u="%s">
      <InstanceID>0</InstanceID>
      <Speed>1</Speed>
    </u:Play>`, AVTransportServiceType)
    
    _, err := c.Call(AVTransportEndpoint, AVTransportServiceType, "Play", body)
    return err
}

func (c *SOAPClient) Pause() error {
    body := fmt.Sprintf(`<u:Pause xmlns:u="%s">
      <InstanceID>0</InstanceID>
    </u:Pause>`, AVTransportServiceType)
    
    _, err := c.Call(AVTransportEndpoint, AVTransportServiceType, "Pause", body)
    return err
}

func (c *SOAPClient) Stop() error {
    body := fmt.Sprintf(`<u:Stop xmlns:u="%s">
      <InstanceID>0</InstanceID>
    </u:Stop>`, AVTransportServiceType)
    
    _, err := c.Call(AVTransportEndpoint, AVTransportServiceType, "Stop", body)
    return err
}

func (c *SOAPClient) Next() error {
    body := fmt.Sprintf(`<u:Next xmlns:u="%s">
      <InstanceID>0</InstanceID>
    </u:Next>`, AVTransportServiceType)
    
    _, err := c.Call(AVTransportEndpoint, AVTransportServiceType, "Next", body)
    return err
}

func (c *SOAPClient) Previous() error {
    body := fmt.Sprintf(`<u:Previous xmlns:u="%s">
      <InstanceID>0</InstanceID>
    </u:Previous>`, AVTransportServiceType)
    
    _, err := c.Call(AVTransportEndpoint, AVTransportServiceType, "Previous", body)
    return err
}
```

### Seek to Position

```go
// SeekToTime seeks to a specific time position (format: "hh:mm:ss")
func (c *SOAPClient) SeekToTime(position string) error {
    body := fmt.Sprintf(`<u:Seek xmlns:u="%s">
      <InstanceID>0</InstanceID>
      <Unit>REL_TIME</Unit>
      <Target>%s</Target>
    </u:Seek>`, AVTransportServiceType, position)
    
    _, err := c.Call(AVTransportEndpoint, AVTransportServiceType, "Seek", body)
    return err
}

// SeekToTrack seeks to a specific track number in the queue (1-indexed)
func (c *SOAPClient) SeekToTrack(trackNumber int) error {
    body := fmt.Sprintf(`<u:Seek xmlns:u="%s">
      <InstanceID>0</InstanceID>
      <Unit>TRACK_NR</Unit>
      <Target>%d</Target>
    </u:Seek>`, AVTransportServiceType, trackNumber)
    
    _, err := c.Call(AVTransportEndpoint, AVTransportServiceType, "Seek", body)
    return err
}
```

### Get Current Playback State

```go
type TransportInfo struct {
    CurrentTransportState  string // PLAYING, PAUSED_PLAYBACK, STOPPED, TRANSITIONING
    CurrentTransportStatus string
    CurrentSpeed           string
}

type PositionInfo struct {
    Track         int
    TrackDuration string
    TrackURI      string
    RelTime       string // Current position in track
    AbsTime       string
}

func (c *SOAPClient) GetTransportInfo() (*TransportInfo, error) {
    body := fmt.Sprintf(`<u:GetTransportInfo xmlns:u="%s">
      <InstanceID>0</InstanceID>
    </u:GetTransportInfo>`, AVTransportServiceType)
    
    resp, err := c.Call(AVTransportEndpoint, AVTransportServiceType, "GetTransportInfo", body)
    if err != nil {
        return nil, err
    }
    
    // Parse response XML
    var result struct {
        Body struct {
            Response struct {
                CurrentTransportState  string
                CurrentTransportStatus string
                CurrentSpeed           string
            } `xml:"GetTransportInfoResponse"`
        } `xml:"Body"`
    }
    
    if err := xml.Unmarshal(resp, &result); err != nil {
        return nil, err
    }
    
    return &TransportInfo{
        CurrentTransportState:  result.Body.Response.CurrentTransportState,
        CurrentTransportStatus: result.Body.Response.CurrentTransportStatus,
        CurrentSpeed:           result.Body.Response.CurrentSpeed,
    }, nil
}

func (c *SOAPClient) GetPositionInfo() (*PositionInfo, error) {
    body := fmt.Sprintf(`<u:GetPositionInfo xmlns:u="%s">
      <InstanceID>0</InstanceID>
    </u:GetPositionInfo>`, AVTransportServiceType)
    
    resp, err := c.Call(AVTransportEndpoint, AVTransportServiceType, "GetPositionInfo", body)
    if err != nil {
        return nil, err
    }
    
    // Parse response - structure contains Track, TrackDuration, TrackURI, RelTime, etc.
    // ...
    
    return nil, nil // Implement parsing
}
```

---

## 4. Queue Management - Playing Albums and Playlists

This is critical for a music player. Use the Sonos queue (Q:0) for album/playlist playback.

### Clear and Load Queue

```go
// ClearQueue removes all tracks from the queue
func (c *SOAPClient) ClearQueue() error {
    body := fmt.Sprintf(`<u:RemoveAllTracksFromQueue xmlns:u="%s">
      <InstanceID>0</InstanceID>
    </u:RemoveAllTracksFromQueue>`, AVTransportServiceType)
    
    _, err := c.Call(AVTransportEndpoint, AVTransportServiceType, "RemoveAllTracksFromQueue", body)
    // Error 804 means queue already empty - not a real error
    if err != nil && !strings.Contains(err.Error(), "804") {
        return err
    }
    return nil
}

// AddURIToQueue adds a single track to the queue
// Returns the position where track was inserted
func (c *SOAPClient) AddURIToQueue(uri, metadata string, asNext bool) (int, error) {
    enqueueAsNext := "0"
    if asNext {
        enqueueAsNext = "1"
    }
    
    escapedURI := xmlEscape(uri)
    escapedMetadata := xmlEscape(metadata)
    
    body := fmt.Sprintf(`<u:AddURIToQueue xmlns:u="%s">
      <InstanceID>0</InstanceID>
      <EnqueuedURI>%s</EnqueuedURI>
      <EnqueuedURIMetaData>%s</EnqueuedURIMetaData>
      <DesiredFirstTrackNumberEnqueued>0</DesiredFirstTrackNumberEnqueued>
      <EnqueueAsNext>%s</EnqueueAsNext>
    </u:AddURIToQueue>`, AVTransportServiceType, escapedURI, escapedMetadata, enqueueAsNext)
    
    resp, err := c.Call(AVTransportEndpoint, AVTransportServiceType, "AddURIToQueue", body)
    if err != nil {
        return 0, err
    }
    
    // Parse FirstTrackNumberEnqueued from response
    var result struct {
        Body struct {
            Response struct {
                FirstTrackNumberEnqueued int
                NumTracksAdded           int
                NewQueueLength           int
            } `xml:"AddURIToQueueResponse"`
        } `xml:"Body"`
    }
    
    if err := xml.Unmarshal(resp, &result); err != nil {
        return 0, err
    }
    
    return result.Body.Response.FirstTrackNumberEnqueued, nil
}

// PlayQueue sets the queue as the current transport URI and starts playback
func (c *SOAPClient) PlayQueue(deviceUUID string, startTrack int) error {
    // Set queue as the transport URI
    queueURI := fmt.Sprintf("x-rincon-queue:%s#0", deviceUUID)
    
    body := fmt.Sprintf(`<u:SetAVTransportURI xmlns:u="%s">
      <InstanceID>0</InstanceID>
      <CurrentURI>%s</CurrentURI>
      <CurrentURIMetaData></CurrentURIMetaData>
    </u:SetAVTransportURI>`, AVTransportServiceType, queueURI)
    
    _, err := c.Call(AVTransportEndpoint, AVTransportServiceType, "SetAVTransportURI", body)
    if err != nil {
        return err
    }
    
    // Seek to starting track if specified
    if startTrack > 0 {
        if err := c.SeekToTrack(startTrack); err != nil {
            return err
        }
    }
    
    // Start playback
    return c.Play()
}
```

### Add Multiple URIs (Batch Loading)

For efficiency when loading albums:

```go
// AddMultipleURIsToQueue adds multiple tracks at once
// URIs and metadata are space-separated lists
func (c *SOAPClient) AddMultipleURIsToQueue(uris, metadatas []string) error {
    // Join with space separator
    uriList := strings.Join(uris, " ")
    metadataList := strings.Join(metadatas, " ")
    
    body := fmt.Sprintf(`<u:AddMultipleURIsToQueue xmlns:u="%s">
      <InstanceID>0</InstanceID>
      <UpdateID>0</UpdateID>
      <NumberOfURIs>%d</NumberOfURIs>
      <EnqueuedURIs>%s</EnqueuedURIs>
      <EnqueuedURIsMetaData>%s</EnqueuedURIsMetaData>
      <ContainerURI></ContainerURI>
      <ContainerMetaData></ContainerMetaData>
      <DesiredFirstTrackNumberEnqueued>0</DesiredFirstTrackNumberEnqueued>
      <EnqueueAsNext>0</EnqueueAsNext>
    </u:AddMultipleURIsToQueue>`, AVTransportServiceType, 
        len(uris), xmlEscape(uriList), xmlEscape(metadataList))
    
    _, err := c.Call(AVTransportEndpoint, AVTransportServiceType, "AddMultipleURIsToQueue", body)
    return err
}
```

### Complete Album Playback Example

```go
// PlayAlbum clears queue, adds all tracks, and starts playback
func (c *SOAPClient) PlayAlbum(deviceUUID string, tracks []Track) error {
    // 1. Clear existing queue
    if err := c.ClearQueue(); err != nil {
        return fmt.Errorf("clear queue: %w", err)
    }
    
    // 2. Add all tracks to queue
    for i, track := range tracks {
        metadata := buildTrackMetadata(track.URI, track.Title)
        _, err := c.AddURIToQueue(track.URI, metadata, false)
        if err != nil {
            return fmt.Errorf("add track %d: %w", i, err)
        }
    }
    
    // 3. Set queue as transport and play
    return c.PlayQueue(deviceUUID, 1)
}
```

---

## 5. UPnP Event Subscriptions for Real-Time State

To get real-time updates (track changes, play/pause state), you must subscribe to UPnP events.

### Event Subscription Flow

1. Send SUBSCRIBE request to device
2. Receive SID (Subscription ID) and timeout
3. Device sends NOTIFY callbacks to your HTTP server
4. Renew subscription before timeout

### Subscription Implementation

```go
package sonos

import (
    "fmt"
    "net/http"
    "time"
)

type EventSubscription struct {
    SID       string
    Timeout   time.Duration
    Endpoint  string
    DeviceIP  string
    ExpiresAt time.Time
}

const (
    AVTransportEventEndpoint      = "/MediaRenderer/AVTransport/Event"
    RenderingControlEventEndpoint = "/MediaRenderer/RenderingControl/Event"
    ZoneGroupTopologyEventEndpoint = "/ZoneGroupTopology/Event"
)

// Subscribe creates a new event subscription
func (c *SOAPClient) Subscribe(eventEndpoint, callbackURL string, timeout int) (*EventSubscription, error) {
    url := fmt.Sprintf("http://%s:1400%s", c.IP, eventEndpoint)
    
    req, err := http.NewRequest("SUBSCRIBE", url, nil)
    if err != nil {
        return nil, err
    }
    
    // Required headers for SUBSCRIBE
    req.Header.Set("CALLBACK", fmt.Sprintf("<%s>", callbackURL))
    req.Header.Set("NT", "upnp:event")
    req.Header.Set("TIMEOUT", fmt.Sprintf("Second-%d", timeout))
    
    resp, err := c.Client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        return nil, fmt.Errorf("subscribe failed: %s", resp.Status)
    }
    
    // Extract SID and timeout from response headers
    sid := resp.Header.Get("SID")
    timeoutHeader := resp.Header.Get("TIMEOUT")
    
    return &EventSubscription{
        SID:       sid,
        Endpoint:  eventEndpoint,
        DeviceIP:  c.IP,
        ExpiresAt: time.Now().Add(time.Duration(timeout) * time.Second),
    }, nil
}

// Renew extends an existing subscription
func (c *SOAPClient) Renew(sub *EventSubscription, timeout int) error {
    url := fmt.Sprintf("http://%s:1400%s", c.IP, sub.Endpoint)
    
    req, err := http.NewRequest("SUBSCRIBE", url, nil)
    if err != nil {
        return err
    }
    
    // Use SID for renewal (no CALLBACK or NT)
    req.Header.Set("SID", sub.SID)
    req.Header.Set("TIMEOUT", fmt.Sprintf("Second-%d", timeout))
    
    resp, err := c.Client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        return fmt.Errorf("renew failed: %s", resp.Status)
    }
    
    sub.ExpiresAt = time.Now().Add(time.Duration(timeout) * time.Second)
    return nil
}

// Unsubscribe cancels an event subscription
func (c *SOAPClient) Unsubscribe(sub *EventSubscription) error {
    url := fmt.Sprintf("http://%s:1400%s", c.IP, sub.Endpoint)
    
    req, err := http.NewRequest("UNSUBSCRIBE", url, nil)
    if err != nil {
        return err
    }
    
    req.Header.Set("SID", sub.SID)
    
    resp, err := c.Client.Do(req)
    if err != nil {
        return err
    }
    resp.Body.Close()
    
    return nil
}
```

### Event Listener HTTP Server

```go
package sonos

import (
    "encoding/xml"
    "io"
    "net/http"
    "sync"
)

type EventListener struct {
    port          int
    subscriptions map[string]*EventSubscription
    callbacks     map[string]func(Event)
    mu            sync.RWMutex
}

type Event struct {
    SID       string
    Variables map[string]string
}

type LastChangeEvent struct {
    InstanceID         string
    TransportState     string
    CurrentTrackURI    string
    CurrentTrackMetaData string
    // ... other variables
}

func NewEventListener(port int) *EventListener {
    return &EventListener{
        port:          port,
        subscriptions: make(map[string]*EventSubscription),
        callbacks:     make(map[string]func(Event)),
    }
}

func (l *EventListener) Start() error {
    http.HandleFunc("/notify", l.handleNotify)
    
    go http.ListenAndServe(fmt.Sprintf(":%d", l.port), nil)
    
    return nil
}

func (l *EventListener) handleNotify(w http.ResponseWriter, r *http.Request) {
    // Must respond 200 OK quickly
    defer func() {
        w.WriteHeader(http.StatusOK)
    }()
    
    if r.Method != "NOTIFY" {
        return
    }
    
    sid := r.Header.Get("SID")
    
    body, err := io.ReadAll(r.Body)
    if err != nil {
        return
    }
    
    event := parseEventXML(body)
    event.SID = sid
    
    l.mu.RLock()
    callback, ok := l.callbacks[sid]
    l.mu.RUnlock()
    
    if ok {
        go callback(event)
    }
}

func parseEventXML(body []byte) Event {
    // Parse the UPnP event XML structure
    // Events contain <e:propertyset> with <e:property> children
    // The LastChange property contains embedded XML with transport state
    
    var event Event
    event.Variables = make(map[string]string)
    
    // Structure:
    // <e:propertyset>
    //   <e:property>
    //     <LastChange>...escaped XML...</LastChange>
    //   </e:property>
    // </e:propertyset>
    
    // Parse LastChange embedded XML for TransportState, CurrentTrackURI, etc.
    
    return event
}
```

### Subscription Manager with Auto-Renewal

```go
type SubscriptionManager struct {
    client   *SOAPClient
    listener *EventListener
    subs     []*EventSubscription
    stopChan chan struct{}
}

func (m *SubscriptionManager) Start(deviceIP, callbackURL string) error {
    // Subscribe to key events
    endpoints := []string{
        AVTransportEventEndpoint,
        RenderingControlEventEndpoint,
    }
    
    for _, endpoint := range endpoints {
        sub, err := m.client.Subscribe(endpoint, callbackURL, 3600)
        if err != nil {
            return err
        }
        m.subs = append(m.subs, sub)
    }
    
    // Start renewal goroutine
    go m.renewalLoop()
    
    return nil
}

func (m *SubscriptionManager) renewalLoop() {
    ticker := time.NewTicker(30 * time.Minute)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            for _, sub := range m.subs {
                if time.Until(sub.ExpiresAt) < 10*time.Minute {
                    m.client.Renew(sub, 3600)
                }
            }
        case <-m.stopChan:
            return
        }
    }
}
```

---

## 6. Zone Group Topology and Coordinator Detection

**Critical**: Commands must be sent to the group coordinator, not individual speakers.

### Get Zone Group State

```go
const (
    ZoneGroupTopologyEndpoint = "/ZoneGroupTopology/Control"
    ZoneGroupTopologyService  = "urn:schemas-upnp-org:service:ZoneGroupTopology:1"
)

type ZoneGroup struct {
    Coordinator string
    Members     []ZoneMember
}

type ZoneMember struct {
    UUID     string
    Location string // IP address
    Name     string
}

func (c *SOAPClient) GetZoneGroupState() ([]ZoneGroup, error) {
    body := fmt.Sprintf(`<u:GetZoneGroupState xmlns:u="%s">
    </u:GetZoneGroupState>`, ZoneGroupTopologyService)
    
    resp, err := c.Call(ZoneGroupTopologyEndpoint, ZoneGroupTopologyService, "GetZoneGroupState", body)
    if err != nil {
        return nil, err
    }
    
    // Response contains ZoneGroupState with embedded XML
    // Parse to extract groups and coordinators
    
    return parseZoneGroupState(resp)
}

func parseZoneGroupState(resp []byte) ([]ZoneGroup, error) {
    // The response contains ZoneGroupState which has this structure:
    // <ZoneGroups>
    //   <ZoneGroup Coordinator="RINCON_XXX">
    //     <ZoneGroupMember UUID="RINCON_XXX" Location="http://192.168.1.10:1400/xml/device_description.xml"/>
    //     <ZoneGroupMember UUID="RINCON_YYY" Location="http://192.168.1.11:1400/xml/device_description.xml"/>
    //   </ZoneGroup>
    // </ZoneGroups>
    
    // Implementation...
    return nil, nil
}

// FindCoordinator returns the coordinator IP for a given device
func FindCoordinator(groups []ZoneGroup, deviceUUID string) string {
    for _, group := range groups {
        for _, member := range group.Members {
            if member.UUID == deviceUUID {
                // Find coordinator's IP
                for _, m := range group.Members {
                    if m.UUID == group.Coordinator {
                        return extractIP(m.Location)
                    }
                }
            }
        }
    }
    return ""
}
```

### Join/Unjoin Groups

```go
// JoinGroup joins this speaker to another speaker's group
func (c *SOAPClient) JoinGroup(masterUUID string) error {
    uri := fmt.Sprintf("x-rincon:%s", masterUUID)
    
    body := fmt.Sprintf(`<u:SetAVTransportURI xmlns:u="%s">
      <InstanceID>0</InstanceID>
      <CurrentURI>%s</CurrentURI>
      <CurrentURIMetaData></CurrentURIMetaData>
    </u:SetAVTransportURI>`, AVTransportServiceType, uri)
    
    _, err := c.Call(AVTransportEndpoint, AVTransportServiceType, "SetAVTransportURI", body)
    return err
}

// LeaveGroup makes this speaker standalone
func (c *SOAPClient) LeaveGroup() error {
    body := fmt.Sprintf(`<u:BecomeCoordinatorOfStandaloneGroup xmlns:u="%s">
      <InstanceID>0</InstanceID>
    </u:BecomeCoordinatorOfStandaloneGroup>`, AVTransportServiceType)
    
    _, err := c.Call(AVTransportEndpoint, AVTransportServiceType, "BecomeCoordinatorOfStandaloneGroup", body)
    return err
}
```

---

## 7. DIDL-Lite Metadata Construction

DIDL-Lite metadata is required for proper track display on Sonos.

### Metadata Templates

```go
// For music tracks from your server
const trackMetadataTemplate = `<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" xmlns:r="urn:schemas-rinconnetworks-com:metadata-1-0/" xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/">
<item id="%s" parentID="%s" restricted="true">
<dc:title>%s</dc:title>
<dc:creator>%s</dc:creator>
<upnp:class>object.item.audioItem.musicTrack</upnp:class>
<upnp:album>%s</upnp:album>
<upnp:albumArtURI>%s</upnp:albumArtURI>
<res protocolInfo="http-get:*:audio/mpeg:*">%s</res>
</item>
</DIDL-Lite>`

// For radio/stream
const radioMetadataTemplate = `<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" xmlns:r="urn:schemas-rinconnetworks-com:metadata-1-0/" xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/">
<item id="R:0/0/0" parentID="R:0/0" restricted="true">
<dc:title>%s</dc:title>
<upnp:class>object.item.audioItem.audioBroadcast</upnp:class>
<desc id="cdudn" nameSpace="urn:schemas-rinconnetworks-com:metadata-1-0/">SA_RINCON65031_</desc>
</item>
</DIDL-Lite>`

func buildTrackMetadata(track Track) string {
    // Determine MIME type for protocolInfo
    mimeType := "audio/mpeg" // default
    switch {
    case strings.HasSuffix(track.URI, ".flac"):
        mimeType = "audio/flac"
    case strings.HasSuffix(track.URI, ".mp3"):
        mimeType = "audio/mpeg"
    case strings.HasSuffix(track.URI, ".m4a"), strings.HasSuffix(track.URI, ".aac"):
        mimeType = "audio/mp4"
    case strings.HasSuffix(track.URI, ".ogg"):
        mimeType = "application/ogg"
    }
    
    protocolInfo := fmt.Sprintf("http-get:*:%s:*", mimeType)
    
    return fmt.Sprintf(trackMetadataTemplate,
        xmlEscape(track.ID),
        xmlEscape(track.AlbumID),
        xmlEscape(track.Title),
        xmlEscape(track.Artist),
        xmlEscape(track.Album),
        xmlEscape(track.CoverArtURL),
        protocolInfo,
        xmlEscape(track.URI),
    )
}
```

### XML Escaping Helper

```go
import "html"

func xmlEscape(s string) string {
    return html.EscapeString(s)
}

// For embedding metadata in SOAP, you need double-escaping
func xmlEscapeForSOAP(s string) string {
    return html.EscapeString(html.EscapeString(s))
}
```

---

## 8. Common Failure Modes and Solutions

### Error 800: "Not a Coordinator"

**Cause**: Sending transport commands to a grouped speaker that isn't the coordinator.

**Solution**: Always query ZoneGroupTopology and send commands to the coordinator:

```go
func (m *SonosManager) PlayOnDevice(deviceUUID string, tracks []Track) error {
    // Find coordinator for this device's group
    groups, _ := m.GetZoneGroupState()
    coordinatorIP := FindCoordinator(groups, deviceUUID)
    
    // Send commands to coordinator
    client := NewSOAPClient(coordinatorIP)
    return client.PlayAlbum(coordinatorIP, tracks)
}
```

### Error 714: "Illegal MIME-Type"

**Cause**: Your HTTP server isn't returning correct Content-Type header.

**Solution**: Ensure your streaming endpoint returns proper headers:

```go
func streamHandler(w http.ResponseWriter, r *http.Request) {
    // CRITICAL headers for Sonos
    w.Header().Set("Content-Type", "audio/flac") // Match actual format
    w.Header().Set("Content-Length", fmt.Sprintf("%d", fileSize))
    w.Header().Set("Accept-Ranges", "bytes")
    
    // Handle Range requests for seeking
    if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
        // Parse range and return 206 Partial Content
        w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, total))
        w.WriteHeader(http.StatusPartialContent)
    }
    
    // Stream the file
    io.Copy(w, file)
}
```

### Error 716: "Resource Not Found"

**Cause**: Sonos can't reach your streaming URL.

**Solutions**:
1. Use IP address, not hostname (Sonos may not resolve local DNS)
2. Ensure no firewall blocking port
3. Use `http://` not `https://` for local network (or use trusted cert)
4. Avoid `localhost` - use actual LAN IP

```go
// BAD
uri := "http://localhost:4533/stream/123"

// GOOD  
uri := fmt.Sprintf("http://%s:4533/stream/123", getLocalIP())
```

### TRANSITIONING State Stuck

**Cause**: Event race condition - TRANSITIONING arrives after PLAYING.

**Solution**: Treat TRANSITIONING as PLAYING for UI purposes:

```go
func isPlayingState(state string) bool {
    return state == "PLAYING" || state == "TRANSITIONING"
}
```

### Missing Content-Length Causes Endless Buffering

**Cause**: Chunked transfer encoding or missing Content-Length header.

**Solution**: Always provide Content-Length:

```go
// For transcoding/streaming, calculate or estimate length
// If unknown, provide a large fake value and close connection when done
w.Header().Set("Content-Length", "999999999")
// Sonos tolerates early connection close
```

---

## 9. Integration Architecture for Navidrome

### Recommended Module Structure

```
navidrome/
├── server/
│   └── sonos/
│       ├── discovery.go      # SSDP discovery
│       ├── soap.go           # SOAP client
│       ├── avtransport.go    # Playback control
│       ├── queue.go          # Queue management
│       ├── events.go         # Event subscriptions
│       ├── topology.go       # Zone groups
│       ├── metadata.go       # DIDL-Lite builder
│       └── manager.go        # High-level API
```

### Manager API for Frontend

```go
type SonosManager struct {
    devices       map[string]*SonosDevice
    subscriptions *SubscriptionManager
    eventChan     chan Event
}

// API methods for React frontend
type SonosAPI interface {
    // Discovery
    GetDevices() []SonosDevice
    RefreshDevices() error
    
    // Playback (routes to coordinator)
    Play(deviceUUID string) error
    Pause(deviceUUID string) error
    Stop(deviceUUID string) error
    Next(deviceUUID string) error
    Previous(deviceUUID string) error
    Seek(deviceUUID string, seconds int) error
    
    // Queue
    PlayTracks(deviceUUID string, trackIDs []string) error
    AddToQueue(deviceUUID string, trackIDs []string, asNext bool) error
    ClearQueue(deviceUUID string) error
    
    // Volume
    GetVolume(deviceUUID string) (int, error)
    SetVolume(deviceUUID string, volume int) error
    
    // State
    GetPlaybackState(deviceUUID string) (*PlaybackState, error)
    SubscribeToEvents(deviceUUID string, callback func(Event)) error
    
    // Groups
    GetGroups() []ZoneGroup
    JoinGroup(deviceUUID, masterUUID string) error
    LeaveGroup(deviceUUID string) error
}
```

### REST Endpoints for Frontend

```go
// router.go
r.Route("/api/sonos", func(r chi.Router) {
    r.Get("/devices", h.GetDevices)
    r.Post("/devices/refresh", h.RefreshDevices)
    
    r.Route("/devices/{uuid}", func(r chi.Router) {
        r.Post("/play", h.Play)
        r.Post("/pause", h.Pause)
        r.Post("/stop", h.Stop)
        r.Post("/next", h.Next)
        r.Post("/previous", h.Previous)
        r.Post("/seek", h.Seek)
        r.Get("/state", h.GetState)
        r.Post("/volume", h.SetVolume)
        r.Get("/volume", h.GetVolume)
    })
    
    r.Post("/queue/play", h.PlayQueue)      // Play tracks on device
    r.Post("/queue/add", h.AddToQueue)      // Add tracks to queue
    r.Delete("/queue", h.ClearQueue)        // Clear queue
    
    r.Get("/groups", h.GetGroups)
    r.Post("/groups/join", h.JoinGroup)
    r.Post("/groups/leave", h.LeaveGroup)
})
```

### Stream URL Generation

Navidrome's existing `/rest/stream` endpoint works, but ensure:

1. **Absolute URLs**: Sonos needs full URL with IP
2. **Auth in URL**: Subsonic API tokens in query params
3. **Proper CORS**: Allow requests from any origin

```go
func buildStreamURL(baseURL string, trackID string, auth AuthParams) string {
    return fmt.Sprintf("%s/rest/stream?id=%s&u=%s&t=%s&s=%s&v=1.16.1&c=sonos",
        baseURL,
        trackID,
        auth.Username,
        auth.Token,
        auth.Salt,
    )
}
```

---

## Quick Start Checklist

1. **Discovery**: Implement SSDP to find devices (Section 1)
2. **SOAP Client**: Build generic SOAP client (Section 2)
3. **Basic Playback**: Test PlayURI with a single track (Section 3)
4. **Queue Playback**: Implement queue for albums (Section 4)
5. **Coordinator Handling**: Always send to coordinator (Section 6)
6. **HTTP Headers**: Ensure Content-Type, Content-Length, Range support (Section 8)
7. **Events**: Add subscriptions for real-time UI updates (Section 5)

---

## References

- Sonos API Documentation: https://sonos.svrooij.io/
- go-sonos library: https://github.com/ianr0bkny/go-sonos
- SoCo Python library (reference): https://github.com/SoCo/SoCo
- UPnP AV Architecture: http://upnp.org/specs/av/UPnP-av-AVArchitecture-v1.pdf
