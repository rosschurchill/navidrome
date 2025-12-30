# Navidrome Fork - Enhanced Music Server
**Version**: 0.1.0
**Status**: Development
**Base**: Navidrome v0.54.x fork

---

## Project Overview

This is an enhanced fork of Navidrome with security hardening, improved metadata support, and new features.

### Completed Enhancements

| Feature | Status | Files Modified |
|---------|--------|----------------|
| **SHA3-256 Migration** | Done | `model/id/id.go`, `model/metadata/legacy_ids.go`, etc. |
| **crypto/rand Usage** | Done | Replaced `math/rand` with secure RNG |
| **Salt Entropy Increase** | Done | 3 bytes -> 16 bytes in `server/auth.go` |
| **CSP Headers** | Done | `server/middlewares.go` |
| **XSS Fixes** | Done | React components sanitization |
| **WAV RIFF INFO Support** | Done | `taglib_wrapper.cpp`, `mappings.yaml` |
| **Smart Playlists UI** | Done | `SmartPlaylistRulesBuilder.jsx`, `PlaylistCreate/Edit.jsx` |
| **Advanced Search** | Done | `advanced_search.go`, `sql_search.go` |
| **Diagnostic Logging** | Done | `taglib.go` - logs files with missing metadata |
| **Album Artist Derivation** | Done | `map_participants.go` - fixes album splitting from featured artists |
| **Cover Art Fallback** | Done | `reader_album.go`, `sources.go` - uses any image when patterns don't match |
| **Split Albums Detection & Fix** | Done | `album_repository.go`, `split_albums.go`, `SplitAlbumsDialog.jsx` - UI for finding and fixing incorrectly split albums |
| **Enhanced Song Info Panel** | Done | `ui/src/common/SongInfo.jsx` - Comprehensive 4-tab info panel with file path, technical details, IDs, raw tags |
| **Gapless Playback Metadata** | Phase 1 Done | `taglib_wrapper.cpp`, `taglib.go`, `model/mediafile.go` - Extracts encoder delay/padding from LAME header (MP3), iTunSMPB (M4A), and sample counts from FLAC/Opus/Vorbis |
| **DLNA/UPnP Media Server** | Phase 1-2 Done | `server/dlna/*` - SSDP discovery, ContentDirectory browsing, streaming via Subsonic URLs |

### Pending Features

- DLNA/UPnP Phase 3-4 (transcoding, search, AVTransport)
- Gapless playback API exposure (Phase 2) and web player (Phase 3)
- Audio fingerprinting (AcoustID/MusicBrainz)

---

## Infrastructure

### Credentials Location
All credentials are stored in `.env` (gitignored):
- Vault AppRole credentials
- Portainer API key
- QNAP connection details

### QNAP Docker Host
- **IP**: 192.168.1.200
- **SSH**: `ssh ross@tvsp01`
- **Docker Path**: `/share/ZFS530_DATA/.qpkg/container-station/bin/docker`
- **Portainer**: http://192.168.1.200:9000

### Vault Integration
- **Address**: https://192.168.1.200:8200
- **Auth Method**: AppRole
- **Purpose**: Secure credential storage for production deployment

---

## Development Setup

### Build Requirements
- Go 1.21+
- Node.js 18+
- TagLib C++ library (`apt install libtag1-dev`)
- SQLite3

### Testing
```bash
# Run Go tests (non-TagLib dependent)
go test ./model/criteria/...
go test ./persistence/...

# Run with TagLib (requires library)
go test ./adapters/taglib/...
```

### Docker Testing
```bash
# Load credentials
source .env

# Create test container via Portainer API
curl -X POST \
  -H "X-API-Key: $PORTAINER_API_KEY" \
  -H "Content-Type: application/json" \
  "$PORTAINER_URL/api/stacks/create/standalone/string?endpointId=$PORTAINER_ENDPOINT_ID" \
  -d '{"name": "navidrome-test", "stackFileContent": "..."}'
```

---

## Key Files Reference

### Security
- `server/auth.go` - Authentication, password hashing
- `server/middlewares.go` - CORS, CSP headers
- `core/auth/auth.go` - JWT handling

### Metadata Extraction
- `adapters/taglib/taglib_wrapper.cpp` - C++ TagLib interface
- `adapters/taglib/taglib.go` - Go extraction logic
- `resources/mappings.yaml` - Tag alias definitions

### Smart Playlists
- `ui/src/playlist/SmartPlaylistRulesBuilder.jsx` - Rules builder UI
- `model/criteria/` - NSP criteria engine

### Advanced Search
- `persistence/advanced_search.go` - Query parser
- `persistence/sql_search.go` - Search integration

### Album Artist Derivation
- `model/metadata/map_participants.go` - Primary artist extraction

### Split Albums Detection & Fix
- `model/album.go` - `SplitAlbum` struct definition
- `persistence/album_repository.go` - `GetSplitAlbums()`, `MergeAlbums()` methods
- `server/nativeapi/split_albums.go` - API endpoints (admin-only)
- `ui/src/dialogs/SplitAlbumsDialog.jsx` - React UI component
- `ui/src/album/AlbumListActions.jsx` - "Fix Split Albums" button for admins

### DLNA/UPnP Media Server (Phase 1-2)
Enables discovery and browsing from Smart TVs, game consoles, and other DLNA-certified devices.

**Files Created**:
- `server/dlna/dlna.go` - Main router, initialization, lifecycle management
- `server/dlna/ssdp.go` - SSDP discovery (UDP multicast), M-SEARCH handler
- `server/dlna/device.go` - Device description XML, service SCPD documents
- `server/dlna/control.go` - SOAP request/response handling
- `server/dlna/content_directory.go` - ContentDirectory Browse implementation with DB queries
- `server/dlna/connection_manager.go` - ConnectionManager protocol info

**Files Modified**:
- `conf/configuration.go` - Added `dlnaOptions` struct and viper defaults
- `consts/consts.go` - Added `URLPathDLNA = "/dlna"`
- `cmd/wire_injectors.go` - Added DLNA provider and `CreateDLNARouter()`
- `cmd/wire_gen.go` - Regenerated with DLNA support
- `cmd/root.go` - Conditional router mounting + SSDP startup

**Configuration Options** (in `navidrome.toml` or environment variables):
```toml
[DLNA]
Enabled = true              # Enable DLNA server (default: false)
ServerName = "Navidrome"    # Name shown on DLNA devices
Interface = ""              # Network interface (empty = all)
TranscodeProfile = "auto"   # Transcoding profile
```

**Environment Variables**:
- `ND_DLNA_ENABLED=true`
- `ND_DLNA_SERVERNAME="My Music Server"`
- `ND_DLNA_INTERFACE=""`
- `ND_DLNA_TRANSCODEPROFILE="auto"`

**HTTP Endpoints** (when DLNA is enabled):
- `GET /dlna/device.xml` - UPnP device description
- `GET /dlna/ContentDirectory.xml` - ContentDirectory SCPD
- `POST /dlna/ContentDirectory/control` - ContentDirectory SOAP actions
- `GET /dlna/ConnectionManager.xml` - ConnectionManager SCPD
- `POST /dlna/ConnectionManager/control` - ConnectionManager SOAP actions

**Navigation Hierarchy**:
```
Root (0)
└── Music
    ├── Artists → Albums → Tracks
    ├── Albums → Tracks
    ├── Genres → Albums → Tracks
    └── Playlists → Tracks
```

**Supported Formats**: MP3, FLAC, WAV, AAC, OGG, OPUS, WMA

**Limitations (Phase 3-4 work)**:
- No authentication (DLNA clients access library without login)
- No transcoding negotiation (serves original format only)
- No search action (browse only)
- Static SystemUpdateID (doesn't reflect library changes)

**See Also**: `docs/plans/DLNA-HANDOVER.md` for complete implementation details

### Gapless Playback Metadata (Phase 1)
Extracts gapless playback information from audio files for seamless track transitions.

**Files Modified**:
- `adapters/taglib/taglib_wrapper.cpp` - C++ extraction of:
  - **MP3**: LAME header encoder delay/padding from `TagLib::MPEG::Properties`
  - **M4A/AAC**: iTunSMPB atom parsing (format: `" 00000000 DELAY PADDING TOTALSAMPLES"`)
  - **FLAC/Opus/Vorbis/WavPack/AIFF/WAV/DSF**: Sample count from stream properties
- `adapters/taglib/taglib.go` - Go parsing of `__encoderdelay`, `__encoderpadding`, `__totalsamples`
- `model/metadata/metadata.go` - Added fields to `AudioProperties` struct
- `model/metadata/map_mediafile.go` - Maps audio properties to MediaFile
- `model/mediafile.go` - Added `EncoderDelay`, `EncoderPadding`, `TotalSamples` fields
- `db/migrations/20251230130000_add_gapless_playback_columns.go` - Database migration

**New MediaFile Fields**:
| Field | Type | Description |
|-------|------|-------------|
| `encoder_delay` | int | Samples to skip at start (for gapless playback) |
| `encoder_padding` | int | Samples to skip at end (for gapless playback) |
| `total_samples` | int64 | Total sample count (for frame-accurate seeking) |

**Supported Formats**:
- **MP3**: Encoder delay/padding from LAME/Xing header
- **M4A/AAC**: Encoder delay/padding/total samples from iTunSMPB atom
- **FLAC/Opus/Vorbis/WavPack/AIFF/WAV/DSF/APE**: Total sample count from stream info

**Next Steps** (Phase 2-4):
- Expose fields in Subsonic API responses
- Add fields to native API song response
- Implement web player gapless support

---

## Notable Bug Fixes

### Album Splitting from Featured Artists (Fixed)

**Problem**: Albums without explicit `albumartist` tags were being split into multiple albums when tracks had different artists (e.g., "Dezza", "Dezza & Lauren L'aimant", "Dezza feat. EMME").

**Root Cause**: When no `albumartist` tag exists, Navidrome falls back to using the track `artist` field directly. This caused albums with featured artists to be grouped separately.

**Solution**: Added `extractPrimaryArtists()` function that:
1. Takes only the first artist from the parsed artists list
2. Strips common featuring patterns from the artist name:
   - ` feat. `, ` feat `, ` ft. `, ` ft `
   - ` & `, ` x `, ` vs `, ` vs. `
3. Creates a clean artist entry for album grouping

**Example**: "Dezza & Lauren L'aimant" -> "Dezza" for album artist derivation

**Files Modified**:
- `model/metadata/map_participants.go`:
  - Line 57: Changed fallback from `artists` to `extractPrimaryArtists(artists)`
  - Lines 241-279: New `extractPrimaryArtists()` function

**Note**: This only affects files WITHOUT an explicit `albumartist` tag. Files with proper tagging are unaffected.

**Verified**: Testing with "Dezza - 44 North, 63 West" album (19 tracks) - all tracks now grouped into a single album while preserving featured artist credits in track artist field.

### Split Albums Detection & Fix UI (New Feature)

**Purpose**: Provides an admin UI to detect and fix albums that have been incorrectly split into multiple entries due to different album artists.

**Access**: Admin users can click the "Fix Split Albums" button in the Albums list toolbar.

**Features**:
- Detects albums with the same name but different album artists
- Intelligently identifies compilations vs. single-artist albums with featuring credits
- Suggests the best album artist to merge under
- Allows bulk selection and merging

**API Endpoints** (admin-only):
- `GET /api/splitAlbums` - Returns list of split albums with suggestions
- `POST /api/splitAlbums/merge` - Merges selected albums under a target artist

**Files Added/Modified**:
- `model/album.go` - Added `SplitAlbum` struct and repository interface methods
- `persistence/album_repository.go` - Implemented `GetSplitAlbums()` and `MergeAlbums()`
- `server/nativeapi/split_albums.go` - API handler for split albums endpoints
- `ui/src/dialogs/SplitAlbumsDialog.jsx` - React dialog component
- `ui/src/album/AlbumListActions.jsx` - Added toolbar button for admins
- `ui/src/actions/dialogs.js` - Redux actions
- `ui/src/reducers/dialogReducer.js` - Redux reducer

---

## Testing Container

### Container: `navidrome-test`
- **URL**: http://192.168.1.200:4535
- **Image**: `navidrome-fork:latest`
- **Music Library**: `/share/Music` (read-only)
- **Data Path**: `/share/Container/navidrome-test`
- **Log Level**: debug

### Original Container: `navidrome`
- **URL**: http://192.168.1.200:4533
- **Image**: `deluan/navidrome:latest`

### Docker Commands
```bash
# View test container logs
ssh ross@tvsp01 "/share/ZFS530_DATA/.qpkg/container-station/bin/docker logs navidrome-test --tail 50"

# Restart test container
ssh ross@tvsp01 "/share/ZFS530_DATA/.qpkg/container-station/bin/docker restart navidrome-test"

# Rebuild and deploy
docker build --platform linux/amd64 -t navidrome-fork:latest .
docker save navidrome-fork:latest -o /tmp/navidrome-fork.tar
gzip -f /tmp/navidrome-fork.tar
rsync -avP /tmp/navidrome-fork.tar.gz ross@tvsp01:/share/Container/navidrome-test/
ssh ross@tvsp01 "gunzip -c /share/Container/navidrome-test/navidrome-fork.tar.gz | /share/ZFS530_DATA/.qpkg/container-station/bin/docker load"
ssh ross@tvsp01 "/share/ZFS530_DATA/.qpkg/container-station/bin/docker restart navidrome-test"
```

---

**Created**: 2025-12-29
**Last Updated**: 2025-12-30
