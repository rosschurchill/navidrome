# Feature Plan: DLNA/UPnP Media Server

> **Status**: ⏳ Planned (Not Started)
> **Priority**: Medium
> **Complexity**: High
> **Dependencies**: None

---

## 1. Overview

### What Is This?
Implementation of DLNA (Digital Living Network Alliance) / UPnP (Universal Plug and Play) media server capabilities, allowing TVs, game consoles, and media players to discover and stream from Navidrome.

### Why Do We Need It?
- **Device Compatibility**: Smart TVs, Xbox, PlayStation, Roku, etc.
- **No App Required**: Works with built-in media apps
- **Local Network**: Zero-config discovery
- **Industry Standard**: Widely supported protocol

### What Devices Would Work?
- Samsung/LG/Sony Smart TVs
- Xbox One/Series X
- PlayStation 4/5
- Roku
- VLC (as renderer)
- Any DLNA-certified device

---

## 2. Technical Background

### Protocol Stack
```
┌─────────────────────────────────────────┐
│           DLNA Guidelines               │  ← Device classes, profiles
├─────────────────────────────────────────┤
│         UPnP AV Architecture            │  ← ContentDirectory, AVTransport
├─────────────────────────────────────────┤
│              UPnP Device                │  ← Device description, services
├─────────────────────────────────────────┤
│                SSDP                     │  ← Discovery (multicast UDP)
├─────────────────────────────────────────┤
│            HTTP / SOAP                  │  ← Control, eventing
└─────────────────────────────────────────┘
```

### Key Components

| Component | Protocol | Purpose |
|-----------|----------|---------|
| **SSDP** | UDP multicast (239.255.255.250:1900) | Device discovery |
| **Device Description** | HTTP GET (XML) | Describe server capabilities |
| **ContentDirectory** | SOAP | Browse/search media library |
| **ConnectionManager** | SOAP | Protocol negotiation |
| **AVTransport** | SOAP | Playback control (optional) |

### DLNA Device Classes
| Class | Code | Description |
|-------|------|-------------|
| Digital Media Server | DMS | What we're implementing |
| Digital Media Player | DMP | TVs, consoles (clients) |
| Digital Media Renderer | DMR | Speakers, displays |
| Digital Media Controller | DMC | Remote control apps |

---

## 3. Implementation Approach

### Option A: Use Existing Go Library
**Library**: [goupnp](https://github.com/huin/goupnp) + custom ContentDirectory

**Pros**:
- SSDP and basic UPnP handling done
- Active community

**Cons**:
- Still need to implement ContentDirectory ourselves
- May not have full DLNA compliance

### Option B: Port from Existing Server
**Reference**: [ReadyMedia/MiniDLNA](https://sourceforge.net/projects/minidlna/) (C)

**Pros**:
- Battle-tested DLNA compliance
- Known to work with problematic devices

**Cons**:
- C codebase, need to port concepts
- Can't directly use code

### Option C: Full Custom Implementation
**Approach**: Implement from UPnP/DLNA specs

**Pros**:
- Full control
- Optimized for Navidrome

**Cons**:
- Most work
- Risk of compatibility issues

### Recommendation: Option A + B
Use goupnp for SSDP/HTTP, reference MiniDLNA for ContentDirectory implementation details and DLNA profile handling.

---

## 4. Architecture Design

### Package Structure
```
server/dlna/
├── dlna.go           # Main router, initialization
├── ssdp.go           # SSDP discovery (UDP multicast)
├── device.go         # Device description XML
├── content_directory.go  # Browse/Search implementation
├── connection_manager.go # Protocol info
├── didl.go           # DIDL-Lite XML generation
└── transcoding.go    # Format negotiation
```

### Integration Points
```
┌─────────────────────────────────────────────────────────────┐
│                      Navidrome                               │
├─────────────────┬─────────────────┬─────────────────────────┤
│   Web UI        │   Subsonic API  │   DLNA Server (NEW)     │
├─────────────────┴─────────────────┴─────────────────────────┤
│                    Core Services                             │
│  (DataStore, Artwork, MediaStreamer, Transcoding)           │
└─────────────────────────────────────────────────────────────┘
```

---

## 5. Key Implementation Details

### 5.1 SSDP Discovery

```go
// Announce server on network
func (s *Server) announcePresence() {
    // Send NOTIFY to 239.255.255.250:1900
    // Include: NT, NTS, USN, LOCATION, CACHE-CONTROL
}

// Respond to M-SEARCH queries
func (s *Server) handleMSearch(req *ssdp.Request) {
    // Check if searching for our device type
    // Respond with device location
}
```

### 5.2 ContentDirectory Service

**Required Actions**:
| Action | Purpose |
|--------|---------|
| `Browse` | Navigate folder hierarchy |
| `Search` | Search by criteria (optional but recommended) |
| `GetSystemUpdateID` | Check for library changes |
| `GetSearchCapabilities` | What fields can be searched |
| `GetSortCapabilities` | What fields can be sorted |

**Browse Implementation**:
```go
func (s *Server) Browse(objectID, browseFlag string, filter string,
                        startingIndex, requestedCount int) (*BrowseResult, error) {
    switch objectID {
    case "0":
        return s.browseRoot()
    case "music":
        return s.browseMusicRoot()
    case "music/artists":
        return s.browseArtists(startingIndex, requestedCount)
    // ... etc
    }
}
```

### 5.3 DIDL-Lite Content Format

```xml
<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/"
           xmlns:dc="http://purl.org/dc/elements/1.1/"
           xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/">
  <item id="track-123" parentID="album-456" restricted="1">
    <dc:title>Song Title</dc:title>
    <dc:creator>Artist Name</dc:creator>
    <upnp:class>object.item.audioItem.musicTrack</upnp:class>
    <upnp:album>Album Name</upnp:album>
    <upnp:artist>Artist Name</upnp:artist>
    <res protocolInfo="http-get:*:audio/flac:*"
         duration="0:03:45"
         size="12345678">
      http://server:4533/rest/stream?id=123
    </res>
    <upnp:albumArtURI>http://server:4533/rest/getCoverArt?id=123</upnp:albumArtURI>
  </item>
</DIDL-Lite>
```

### 5.4 Media Format Profiles

DLNA requires specific format profiles. Key audio profiles:

| Profile | Format | Container | Notes |
|---------|--------|-----------|-------|
| LPCM | PCM | WAV | Universal support |
| MP3 | MP3 | MP3 | Widely supported |
| AAC_ISO | AAC | MP4 | Most devices |
| FLAC | FLAC | FLAC | Limited device support |

**Transcoding Strategy**:
- Advertise multiple `<res>` elements per track
- Native format + LPCM fallback
- Let client choose preferred format

---

## 6. Configuration

```toml
[DLNA]
Enabled = false           # Must explicitly enable
ServerName = "Navidrome"  # Shown on devices
Interface = ""            # Network interface (empty = all)
Port = 0                  # SSDP port (0 = auto)
TranscodeProfile = "auto" # auto, lpcm, mp3, none
```

---

## 7. Implementation Checklist

### Phase 1: Foundation
- [ ] Create `server/dlna/` package structure
- [ ] Implement SSDP announcement (multicast)
- [ ] Implement M-SEARCH response
- [ ] Create device description XML
- [ ] Add config options

### Phase 2: ContentDirectory
- [ ] Implement Browse action (root, artists, albums, tracks)
- [ ] Implement DIDL-Lite generation
- [ ] Map Navidrome data to UPnP classes
- [ ] Handle pagination (startingIndex, requestedCount)

### Phase 3: Streaming
- [ ] Generate proper `<res>` elements with protocolInfo
- [ ] Test with real devices (TV, console)
- [ ] Implement transcoding profile selection

### Phase 4: Polish
- [ ] Implement Search action
- [ ] Add album art support
- [ ] Handle GetSystemUpdateID for library updates
- [ ] Test with problematic devices

---

## 8. Testing Plan

### Test Devices
- [ ] Samsung Smart TV
- [ ] LG Smart TV
- [ ] VLC (Windows/Mac/Linux)
- [ ] Xbox (if available)
- [ ] BubbleUPnP (Android)

### Test Cases
1. **Discovery**: Device appears in TV's media sources
2. **Browse**: Navigate Artists → Albums → Tracks
3. **Playback**: Play track, verify audio
4. **Album Art**: Verify artwork displays
5. **Search**: Search for artist/track (if implemented)

---

## 9. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Device compatibility | High | Test with multiple devices, reference MiniDLNA |
| Format support | Medium | Provide LPCM fallback transcoding |
| Network issues | Medium | Handle multiple interfaces, NAT |
| Performance (large libraries) | Low | Pagination, caching |

---

## 10. Research Links

- [UPnP Device Architecture](http://upnp.org/specs/arch/UPnP-arch-DeviceArchitecture-v1.1.pdf)
- [UPnP ContentDirectory](http://upnp.org/specs/av/UPnP-av-ContentDirectory-v1-Service.pdf)
- [DLNA Guidelines](https://spirespark.com/dlna/guidelines) (summary)
- [goupnp library](https://github.com/huin/goupnp)
- [MiniDLNA source](https://sourceforge.net/p/minidlna/git/ci/master/tree/)

---

## 11. Open Questions

1. **Scope**: Full DLNA or just UPnP AV?
2. **Transcoding**: On-demand or pre-generate profiles?
3. **AVTransport**: Implement for remote control capability?
4. **Playlists**: Expose as DLNA containers?

---

*Last Updated: 2024-12-30*
*Status: Research/Planning*
