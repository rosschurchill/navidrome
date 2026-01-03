# Sonos Cast Feature - Implementation Plan

> **Created**: 2025-12-30
> **Status**: Planning
> **Risk Level**: Medium (undocumented API)

---

## Overview

Implement a "Cast to Sonos" feature that allows users to play Navidrome audio on Sonos speakers via local UPnP control. Unlike the SMAPI approach (which required Sonos partner registration), this approach controls speakers directly over the local network.

### How It Works

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Navidrome  │────▶│   Browser   │────▶│   Sonos     │
│   Server    │◀────│   (UI)      │     │   Speaker   │
└─────────────┘     └─────────────┘     └──────┬──────┘
       │                                        │
       │         HTTP Stream URL                │
       └────────────────────────────────────────┘
                 Speaker fetches audio
```

1. User clicks "Cast" on a track/album/playlist
2. UI shows discovered Sonos speakers
3. User selects speaker(s)
4. Backend sends `SetAVTransportURI` SOAP command to speaker
5. Speaker fetches audio stream directly from Navidrome

---

## Phase 1: Sonos Discovery & Basic Control

### 1.1 SSDP Discovery (Reuse DLNA Code)

**Goal**: Discover Sonos speakers on the local network

**Approach**: Modify existing `server/dlna/ssdp.go` to also discover Sonos devices

**Sonos SSDP Details**:
- Search target: `urn:schemas-upnp-org:device:ZonePlayer:1`
- Port: 1400
- Device description: `http://{ip}:1400/xml/device_description.xml`

**Files to Create/Modify**:
- `server/sonos_cast/discovery.go` - Sonos-specific discovery
- `server/sonos_cast/types.go` - Device structs

**Speaker Info to Extract**:
```go
type SonosDevice struct {
    IP           string
    Port         int
    UUID         string
    RoomName     string    // "Living Room", "Kitchen", etc.
    ModelName    string    // "Sonos One", "Beam", etc.
    ModelNumber  string
    IsCoordinator bool     // Group coordinator
    GroupID      string    // For grouped speakers
}
```

### 1.2 Device Description Parsing

**Goal**: Get speaker details from device description XML

**Endpoint**: `GET http://{ip}:1400/xml/device_description.xml`

**Parse**:
- `<roomName>` - Human-readable room name
- `<displayName>` - Model name
- `<modelNumber>` - Model identifier
- `<UDN>` - Unique device ID

### 1.3 Zone/Group Topology

**Goal**: Understand speaker grouping (important for multi-room)

**Endpoint**: Subscribe to `ZoneGroupTopology` service or query directly

**Key Concepts**:
- **Coordinator**: The "master" speaker in a group
- **Satellites**: Speakers following the coordinator
- Must send commands to coordinator, not satellites

---

## Phase 2: Playback Control

### 2.1 AVTransport Service

**Goal**: Control playback on Sonos speakers

**Endpoint**: `POST http://{ip}:1400/MediaRenderer/AVTransport/Control`

**SOAP Actions**:

| Action | Description |
|--------|-------------|
| `SetAVTransportURI` | Set the audio source URL |
| `Play` | Start playback |
| `Pause` | Pause playback |
| `Stop` | Stop playback |
| `Seek` | Seek to position |
| `Next` | Next track (queue) |
| `Previous` | Previous track (queue) |
| `GetPositionInfo` | Current position/track info |
| `GetTransportInfo` | Playing/Paused/Stopped state |

**SetAVTransportURI Example**:
```xml
<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"
            s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:SetAVTransportURI xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
      <InstanceID>0</InstanceID>
      <CurrentURI>http://navidrome:4533/rest/stream?id=xxx&amp;u=user&amp;t=token&amp;s=salt&amp;v=1.16.1&amp;c=SonosCast</CurrentURI>
      <CurrentURIMetaData>&lt;DIDL-Lite ...&gt;...&lt;/DIDL-Lite&gt;</CurrentURIMetaData>
    </u:SetAVTransportURI>
  </s:Body>
</s:Envelope>
```

**DIDL-Lite Metadata** (for track info display on Sonos):
```xml
<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/"
           xmlns:dc="http://purl.org/dc/elements/1.1/"
           xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/">
  <item id="track-123" parentID="album-456" restricted="1">
    <dc:title>Track Name</dc:title>
    <dc:creator>Artist Name</dc:creator>
    <upnp:album>Album Name</upnp:album>
    <upnp:albumArtURI>http://navidrome:4533/rest/getCoverArt?id=xxx</upnp:albumArtURI>
    <res protocolInfo="http-get:*:audio/flac:*">http://navidrome:4533/rest/stream?id=xxx...</res>
  </item>
</DIDL-Lite>
```

### 2.2 RenderingControl Service

**Goal**: Volume and mute control

**Endpoint**: `POST http://{ip}:1400/MediaRenderer/RenderingControl/Control`

**SOAP Actions**:

| Action | Description |
|--------|-------------|
| `GetVolume` | Get current volume (0-100) |
| `SetVolume` | Set volume |
| `GetMute` | Get mute state |
| `SetMute` | Set mute state |

### 2.3 Queue Management (Optional)

**Goal**: Build playlists on Sonos queue

**Actions**:
- `AddURIToQueue` - Add track to queue
- `RemoveAllTracksFromQueue` - Clear queue
- `RemoveTrackFromQueue` - Remove specific track

---

## Phase 3: Backend API

### 3.1 REST Endpoints

**Base Path**: `/api/cast/sonos`

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/devices` | List discovered Sonos speakers |
| `POST` | `/devices/refresh` | Force re-discovery |
| `GET` | `/devices/{id}` | Get device details + state |
| `POST` | `/devices/{id}/play` | Play URI on device |
| `POST` | `/devices/{id}/pause` | Pause playback |
| `POST` | `/devices/{id}/stop` | Stop playback |
| `POST` | `/devices/{id}/volume` | Set volume |
| `GET` | `/devices/{id}/state` | Get playback state |

### 3.2 Play Request Body

```json
{
  "type": "track|album|playlist",
  "id": "media-id",
  "startIndex": 0,        // For albums/playlists
  "shuffle": false
}
```

### 3.3 State Response

```json
{
  "deviceId": "RINCON_xxx",
  "roomName": "Living Room",
  "state": "playing|paused|stopped",
  "currentTrack": {
    "id": "track-123",
    "title": "Track Name",
    "artist": "Artist",
    "album": "Album",
    "duration": 240,
    "position": 45,
    "albumArt": "/api/artwork/..."
  },
  "volume": 45,
  "muted": false
}
```

---

## Phase 4: Frontend UI

### 4.1 Cast Button Component

**Location**: Track list actions, album header, playlist header, now playing bar

**Behavior**:
1. Click opens speaker picker popover
2. Shows discovered Sonos devices with room names
3. Grouped speakers shown together
4. Click speaker to start playback
5. Visual indicator for currently casting

### 4.2 Speaker Picker

```
┌─────────────────────────────┐
│ Cast to Sonos               │
├─────────────────────────────┤
│ ○ Living Room (Sonos One)   │
│ ○ Kitchen (Sonos Move)      │
│ ● Bedroom (Sonos Beam)  ▶   │  ← Currently playing
│ ○ Office (Play:5)           │
├─────────────────────────────┤
│ [Group: Downstairs]         │
│   Living Room + Kitchen     │
└─────────────────────────────┘
```

### 4.3 Mini Player (When Casting)

Show in the now-playing bar:
- Sonos icon + room name
- Current track info
- Play/Pause/Stop
- Volume slider
- Disconnect button

### 4.4 Files to Create/Modify

- `ui/src/cast/SonosCastButton.jsx` - Cast button component
- `ui/src/cast/SonosSpeakerPicker.jsx` - Speaker selection UI
- `ui/src/cast/SonosMiniPlayer.jsx` - Mini player controls
- `ui/src/cast/useSonosCast.js` - React hook for cast state

---

## Phase 5: Configuration

### 5.1 Server Config

```toml
[SonosCast]
Enabled = true
DiscoveryInterval = "5m"    # How often to scan for new speakers
StreamFormat = "flac"       # Preferred format for casting
```

### 5.2 Environment Variables

```bash
ND_SONOSCAST_ENABLED=true
ND_SONOSCAST_DISCOVERYINTERVAL=5m
ND_SONOSCAST_STREAMFORMAT=flac
```

---

## Phase 6: Testing

### 6.1 Unit Tests

- SSDP discovery parsing
- SOAP message generation
- DIDL-Lite metadata generation
- API endpoint handlers

### 6.2 Integration Tests

- Discovery with mock Sonos responses
- Playback control with mock speaker
- Queue management

### 6.3 Manual Testing

| Test | Expected Result |
|------|-----------------|
| Discover speakers | All Sonos devices on network shown |
| Play single track | Track plays on selected speaker |
| Play album | Album queued and plays |
| Play playlist | Playlist queued and plays |
| Pause/Resume | Playback pauses/resumes |
| Volume control | Volume changes on speaker |
| Multi-room | Grouped speakers play in sync |

---

## File Structure

```
server/sonos_cast/
├── sonos_cast.go         # Main router, initialization
├── discovery.go          # SSDP discovery for Sonos
├── types.go              # Device, state structs
├── avtransport.go        # AVTransport SOAP client
├── rendering.go          # RenderingControl SOAP client
├── topology.go           # Zone group topology
├── didl.go               # DIDL-Lite metadata generation
└── api.go                # REST API handlers

ui/src/cast/
├── SonosCastButton.jsx
├── SonosSpeakerPicker.jsx
├── SonosMiniPlayer.jsx
├── useSonosCast.js
└── sonosApi.js           # API client
```

---

## Dependencies

### Go Libraries

| Library | Purpose |
|---------|---------|
| `github.com/huin/goupnp` | UPnP/SOAP client (or custom) |
| Built-in `net/http` | HTTP requests |
| Built-in `encoding/xml` | XML parsing/generation |

### Existing Code Reuse

| Component | Source |
|-----------|--------|
| SSDP multicast | `server/dlna/ssdp.go` |
| SOAP envelope | `server/dlna/control.go` |
| Auth token generation | `server/subsonic/` |

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Sonos deprecates UPnP | Feature flag to disable; document as "experimental" |
| Speaker firmware breaks it | Version detection; graceful degradation |
| Network issues | Timeout handling; retry logic; clear error messages |
| Multi-room complexity | Start with single speaker; add grouping later |

---

## Implementation Order

### MVP (2-3 days)
1. [ ] Discovery - Find Sonos speakers
2. [ ] Play single track - SetAVTransportURI + Play
3. [ ] Basic UI - Cast button + speaker picker

### Enhanced (2-3 days)
4. [ ] Playback controls - Pause/Stop/Seek/Volume
5. [ ] State polling - Current track, position
6. [ ] Mini player UI - Show what's playing

### Advanced (2-3 days)
7. [ ] Queue support - Albums/playlists
8. [ ] Multi-room - Group topology awareness
9. [ ] Event subscription - Real-time state updates

---

## Success Criteria

### MVP
- [ ] Discover Sonos speakers on network
- [ ] Play a track on any speaker
- [ ] UI shows speaker list

### Full Feature
- [ ] Play tracks, albums, playlists
- [ ] Full playback control (pause, seek, volume)
- [ ] Multi-room/grouped speaker support
- [ ] Real-time state updates in UI

---

## References

- [Sonos UPnP API Documentation](https://sonos.svrooij.io/)
- [AVTransport Service](https://sonos.svrooij.io/services/av-transport)
- [SoCo Python Library](https://github.com/SoCo/SoCo)
- [node-sonos](https://github.com/bencevans/node-sonos)
- [go-sonos](https://github.com/ianr0bkny/go-sonos)

---

*Last Updated: 2025-12-30*
