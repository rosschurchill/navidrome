# DLNA/UPnP Implementation Handover

> **Session**: B (DLNA/UPnP Foundation)
> **Date**: 2024-12-30
> **Status**: Phase 1 & 2 Complete

---

## Summary

Implemented a complete DLNA/UPnP Media Server for Navidrome, enabling discovery and browsing from Smart TVs, game consoles, and other DLNA-certified devices.

---

## What Was Implemented

### Phase 1: Foundation (Complete)

Created the `server/dlna/` package with core DLNA functionality:

| File | Lines | Purpose |
|------|-------|---------|
| `dlna.go` | ~200 | Main router, initialization, lifecycle management |
| `ssdp.go` | ~300 | SSDP discovery (UDP multicast), M-SEARCH handler |
| `device.go` | ~330 | Device description XML, service SCPD documents |
| `control.go` | ~145 | SOAP request/response handling |
| `content_directory.go` | ~650 | ContentDirectory Browse implementation |
| `connection_manager.go` | ~135 | ConnectionManager protocol info |

### Phase 2: Integration (Complete)

Integrated DLNA into Navidrome's configuration and startup:

| File | Changes |
|------|---------|
| `conf/configuration.go` | Added `dlnaOptions` struct and viper defaults |
| `consts/consts.go` | Added `URLPathDLNA = "/dlna"` |
| `cmd/wire_injectors.go` | Added Wire provider and `CreateDLNARouter()` |
| `cmd/wire_gen.go` | Regenerated with DLNA support |
| `cmd/root.go` | Conditional router mounting + SSDP startup |

---

## Features Implemented

### SSDP Discovery
- Multicast announcement to `239.255.255.250:1900`
- M-SEARCH response handling for device discovery
- Periodic keep-alive notifications (30 min interval)
- ByeBye messages on shutdown
- Multi-interface support

### Device Description
- UPnP 1.1 compliant device XML
- MediaServer:1 device type
- ContentDirectory:1 service
- ConnectionManager:1 service

### ContentDirectory Service
- **Browse action** with BrowseMetadata and BrowseDirectChildren
- **DIDL-Lite** response format
- **Navigation hierarchy**:
  ```
  Root (0)
  └── Music
      ├── Artists → Albums → Tracks
      ├── Albums → Tracks
      ├── Genres → Albums → Tracks
      └── Playlists → Tracks
  ```
- Pagination support (startingIndex, requestedCount)
- GetSearchCapabilities, GetSortCapabilities, GetSystemUpdateID

### ConnectionManager Service
- GetProtocolInfo (MP3, FLAC, WAV, AAC, OGG, OPUS, WMA)
- GetCurrentConnectionIDs
- GetCurrentConnectionInfo

### Database Integration
- Full queries for Artists, Albums, Genres, Playlists, Tracks
- Proper filtering (by artist, album, genre)
- Pagination with offset/limit
- Track metadata with duration, bitrate, sample rate, channels

### Streaming
- Subsonic-compatible stream URLs (`/rest/stream?id=...`)
- Album art URLs (`/rest/getCoverArt?id=...`)
- DLNA protocol info strings for all audio formats

---

## Configuration

### navidrome.toml
```toml
[DLNA]
Enabled = true              # Enable DLNA server (default: false)
ServerName = "Navidrome"    # Name shown on DLNA devices
Interface = ""              # Network interface (empty = all)
TranscodeProfile = "auto"   # Transcoding profile
```

### Environment Variables
```bash
ND_DLNA_ENABLED=true
ND_DLNA_SERVERNAME="My Music Server"
ND_DLNA_INTERFACE=""
ND_DLNA_TRANSCODEPROFILE="auto"
```

---

## HTTP Endpoints (when enabled)

| Endpoint | Purpose |
|----------|---------|
| `GET /dlna/device.xml` | UPnP device description |
| `GET /dlna/ContentDirectory.xml` | ContentDirectory SCPD |
| `POST /dlna/ContentDirectory/control` | ContentDirectory SOAP actions |
| `GET /dlna/ConnectionManager.xml` | ConnectionManager SCPD |
| `POST /dlna/ConnectionManager/control` | ConnectionManager SOAP actions |
| `GET /dlna/icon/{size}.png` | Device icons |

---

## Testing

### Build Verification
```bash
# Compile without TagLib (development)
go build -tags=notag ./server/...

# Regenerate Wire
go generate ./cmd/...
```

### Manual Testing
1. Enable DLNA in config: `ND_DLNA_ENABLED=true`
2. Start Navidrome
3. Look for "DLNA server started" in logs
4. Use a DLNA client (VLC, BubbleUPnP, TV) to discover

### Test Devices
- [ ] VLC (Windows/Mac/Linux)
- [ ] BubbleUPnP (Android)
- [ ] Samsung Smart TV
- [ ] LG Smart TV
- [ ] Xbox

---

## What's NOT Implemented (Phase 3-4)

### Phase 3: Streaming Enhancements
- [ ] Transcoding profile selection per-device
- [ ] Multiple `<res>` elements with format options
- [ ] LPCM fallback for universal compatibility
- [ ] Range request support for seeking

### Phase 4: Polish
- [ ] Search action implementation
- [ ] Real SystemUpdateID tracking (library changes)
- [ ] AVTransport service (remote control)
- [ ] Actual device icons from resources
- [ ] Eventing for state changes

---

## Architecture Notes

### SSDP Lifecycle
```
Server Start
    │
    ├─► Start SSDP listener (multicast UDP)
    ├─► Send NOTIFY ssdp:alive (3x for reliability)
    └─► Start periodic announcer (30 min)

Server Stop
    │
    ├─► Send NOTIFY ssdp:byebye
    └─► Close UDP connection
```

### Browse Flow
```
Client M-SEARCH → Server responds with device URL
Client GET /dlna/device.xml → Server returns device description
Client POST /dlna/ContentDirectory/control (Browse) → Server queries DB, returns DIDL-Lite
```

### Object ID Scheme
```
"0"                 → Root
"music"             → Music folder
"music/artists"     → Artists list
"music/albums"      → Albums list
"music/genres"      → Genres list
"music/playlists"   → Playlists list
"artist/{id}"       → Specific artist's albums
"album/{id}"        → Specific album's tracks
"genre/{id}"        → Specific genre's albums
"playlist/{id}"     → Specific playlist's tracks
"track/{id}"        → Specific track (for metadata)
```

---

## Known Issues / Limitations

1. **No authentication** - DLNA clients access library without login
2. **No transcoding negotiation** - Serves original format only
3. **SystemUpdateID static** - Doesn't reflect library changes
4. **No search** - Browse only, search not implemented
5. **Icons placeholder** - Returns empty response

---

## Files Reference

```
server/dlna/
├── dlna.go              # Router, Start/Stop, UUID generation
├── ssdp.go              # SSDP multicast, M-SEARCH, NOTIFY
├── device.go            # Device XML, SCPD documents
├── control.go           # SOAP envelope handling
├── content_directory.go # Browse implementation, DIDL-Lite
└── connection_manager.go# Protocol info
```

---

## Related Documents

- `docs/plans/03-DLNA-UPNP.md` - Original implementation plan
- `docs/plans/PARALLEL-HANDOFFS.md` - Session coordination

---

*Last Updated: 2024-12-30*
