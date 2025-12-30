# Navidrome Enhanced Fork

> **A security-hardened, feature-rich fork of [Navidrome](https://github.com/navidrome/navidrome)**

This fork addresses critical security vulnerabilities and adds highly-requested features while maintaining full compatibility with the upstream codebase.

---

## Security Hardening

### Critical Fixes

| Issue | Severity | Fix Applied |
|-------|----------|-------------|
| **MD5 Hash Algorithm** | CRITICAL | Migrated to **SHA3-256** across 17 files for ID generation |
| **Weak Random Number Generation** | CRITICAL | Replaced `math/rand` with **crypto/rand** for all security-sensitive operations |
| **Insufficient Salt Entropy** | HIGH | Increased salt from 3 bytes to **16 bytes** in `server/auth.go` |
| **Missing CSP Headers** | HIGH | Enabled **Content Security Policy** headers in `server/middlewares.go` |
| **XSS Vulnerabilities** | HIGH | Added HTML sanitization in React components (`AlbumDetails.jsx`, etc.) |

### Files Modified for SHA3-256 Migration
- `model/id/id.go` - Core ID generation
- `model/metadata/legacy_ids.go` - Legacy ID compatibility
- `server/auth.go` - Authentication tokens
- `server/subsonic/middlewares.go` - Subsonic API auth
- `core/agents/lastfm/client.go` - Last.fm integration
- `core/artwork/reader_album.go`, `reader_artist.go` - Artwork caching
- `model/mediafile.go`, `participants.go`, `tag.go` - Data models
- `persistence/sql_base_repository.go` - Database operations
- `plugins/runtime.go` - Plugin system
- `scanner/folder_entry.go` - File scanning

---

## New Features

### 1. DLNA/UPnP Media Server

**Status**: Phase 1-2 Complete

Enables discovery and streaming from Smart TVs, game consoles, and DLNA-certified devices without any additional software.

**Capabilities**:
- SSDP device discovery (UDP multicast)
- ContentDirectory browsing with full navigation hierarchy
- Streaming via authenticated Subsonic URLs
- Album art support

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

**Configuration**:
```toml
[DLNA]
Enabled = true
ServerName = "Navidrome"
Interface = ""  # Empty = all interfaces
```

**Files Added**:
- `server/dlna/dlna.go` - Main router and lifecycle
- `server/dlna/ssdp.go` - SSDP discovery
- `server/dlna/device.go` - Device description XML
- `server/dlna/control.go` - SOAP handling
- `server/dlna/content_directory.go` - Browse implementation
- `server/dlna/connection_manager.go` - Protocol info

---

### 2. Gapless Playback Metadata

**Status**: Phase 1 Complete (Metadata Extraction)

Extracts encoder delay, padding, and sample counts for seamless track transitions - essential for listening to albums as intended.

**New MediaFile Fields**:
| Field | Type | Description |
|-------|------|-------------|
| `encoder_delay` | int | Samples to skip at track start |
| `encoder_padding` | int | Samples to skip at track end |
| `total_samples` | int64 | Total sample count for frame-accurate seeking |

**Format Support**:
| Format | Data Extracted |
|--------|----------------|
| **MP3** | Encoder delay/padding from LAME/Xing header |
| **M4A/AAC** | Delay/padding/samples from iTunSMPB atom |
| **FLAC** | Total samples from stream info |
| **Opus/Vorbis** | Total samples from stream info |
| **WavPack/AIFF/WAV/DSF** | Total samples from stream properties |

**Files Modified**:
- `adapters/taglib/taglib_wrapper.cpp` - C++ extraction logic
- `adapters/taglib/taglib.go` - Go parsing
- `model/mediafile.go` - New fields
- `model/metadata/metadata.go` - AudioProperties struct
- `db/migrations/20251230130000_add_gapless_playback_columns.go`

---

### 3. Audio Fingerprinting Core

**Status**: Phase 1 Complete (Core Implementation)

Foundation for AcoustID/MusicBrainz integration to automatically identify and tag unknown music.

**Configuration**:
```toml
[Fingerprint]
Enabled = true
AcoustIDApiKey = "your-api-key"
FpcalcPath = ""  # Auto-detect
CacheResults = true
AutoIdentify = false
BatchSize = 100
```

**Files Added**:
- `core/fingerprint/fingerprint.go` - Main service
- `core/fingerprint/chromaprint.go` - Chromaprint integration
- `core/fingerprint/acoustid.go` - AcoustID API client
- `core/fingerprint/musicbrainz.go` - MusicBrainz lookups

---

### 4. Smart Playlists UI

**Status**: Complete

Visual rules builder for creating dynamic, auto-updating playlists based on criteria like genre, year, rating, play count, and more.

**Files Added**:
- `ui/src/playlist/SmartPlaylistRulesBuilder.jsx` - React component

**Files Modified**:
- `ui/src/playlist/PlaylistCreate.jsx`
- `ui/src/playlist/PlaylistEdit.jsx`
- `ui/src/playlist/PlaylistList.jsx`

---

### 5. Advanced Search

**Status**: Complete

Enhanced search query parser supporting complex operators and filters.

**Files Added**:
- `persistence/advanced_search.go` - Query parser
- `persistence/advanced_search_test.go` - Test coverage

**Files Modified**:
- `persistence/sql_search.go` - Search integration

---

### 6. Split Albums Detection & Fix

**Status**: Complete

Admin UI to detect and fix albums incorrectly split due to different album artists (common with featured artists).

**Features**:
- Detects albums with same name but different album artists
- Intelligently identifies compilations vs. single-artist albums
- Suggests the best album artist to merge under
- Bulk selection and merging

**API Endpoints** (admin-only):
- `GET /api/splitAlbums` - List split albums with suggestions
- `POST /api/splitAlbums/merge` - Merge selected albums

**Files Added**:
- `server/nativeapi/split_albums.go` - API handlers
- `ui/src/dialogs/SplitAlbumsDialog.jsx` - React dialog

**Files Modified**:
- `model/album.go` - SplitAlbum struct
- `persistence/album_repository.go` - GetSplitAlbums(), MergeAlbums()
- `ui/src/album/AlbumListActions.jsx` - Toolbar button

---

### 7. Enhanced Song Info Panel

**Status**: Complete

Comprehensive 4-tab info panel showing detailed track information.

**Tabs**:
1. **Overview** - Basic track info
2. **File** - Path, format, bitrate, sample rate
3. **IDs** - MusicBrainz IDs, internal IDs
4. **Raw Tags** - All metadata tags as stored

**Files Modified**:
- `ui/src/common/SongInfo.jsx`

---

## Metadata Improvements

### WAV RIFF INFO Chunk Support

Previously only ID3v2 tags in WAV files were extracted. Now RIFF INFO chunks are properly handled.

**Files Modified**:
- `adapters/taglib/taglib_wrapper.cpp` - Added RIFF INFO extraction
- `resources/mappings.yaml` - Tag aliases

---

### Album Artist Derivation Fix

**Problem**: Albums without explicit `albumartist` tags were being split when tracks had different artists (e.g., "Dezza", "Dezza & Lauren L'aimant", "Dezza feat. EMME").

**Solution**: Added `extractPrimaryArtists()` function that:
1. Takes only the first artist from parsed artists list
2. Strips featuring patterns: ` feat. `, ` ft. `, ` & `, ` x `, ` vs `
3. Creates clean artist entry for album grouping

**Example**: "Dezza & Lauren L'aimant" → "Dezza" for album artist derivation

**Files Modified**:
- `model/metadata/map_participants.go`

---

### Cover Art Fallback

When standard cover art patterns don't match, the system now falls back to using any available image in the album folder.

**Files Modified**:
- `core/artwork/reader_album.go`
- `core/artwork/sources.go`

---

### FFmpeg Artwork Extraction Fix

**Problem**: Albums with broken embedded artwork and non-standard external image filenames would show no cover art. The FFmpeg extraction would return a reader that failed later during caching/resizing, after fallback sources were no longer available.

**Solution**: Modified `fromFFmpegTag` to validate extraction by reading the entire image into memory before returning. This catches FFmpeg errors at source selection time, allowing `selectImageReader` to try fallback sources.

**Example**: Album with `FD12925_Global-Underground_Adapt-Artwork_1-WEB.jpg` (non-standard name) and broken WAV embedded art now correctly displays the external image.

**Files Modified**:
- `core/artwork/sources.go`

---

### Diagnostic Logging

Added logging for files with missing metadata to help identify tagging issues.

**Files Modified**:
- `adapters/taglib/taglib.go`

---

## Configuration Reference

```toml
# DLNA/UPnP Server
[DLNA]
Enabled = false
ServerName = "Navidrome"
Interface = ""
TranscodeProfile = "auto"

# Audio Fingerprinting
[Fingerprint]
Enabled = false
AcoustIDApiKey = ""
FpcalcPath = ""
CacheResults = true
AutoIdentify = false
BatchSize = 100
```

**Environment Variables**:
```bash
ND_DLNA_ENABLED=true
ND_DLNA_SERVERNAME="My Music Server"
ND_FINGERPRINT_ENABLED=true
ND_FINGERPRINT_ACOUSTIDAPIKEY="your-key"
```

---

## Pending Work

| Feature | Status | Next Steps |
|---------|--------|------------|
| DLNA Phase 3-4 | Pending | Transcoding, search, AVTransport |
| Gapless Phase 2-4 | Pending | API exposure, web player integration |
| Fingerprinting Phase 2-4 | Pending | DB caching, API endpoints, UI |
| Sonos SMAPI | On Hold | Available on `feature/sonos-smapi` branch; blocked by Sonos S2 firmware disabling custom service registration |

---

## Compatibility

- **Upstream Version**: Based on Navidrome v0.54.x
- **Subsonic API**: Fully compatible
- **Existing Clients**: DSub, Symfonium, play:Sub, etc. all work
- **Database**: Migration scripts included, non-destructive

---

## Contributing Back

We welcome the Navidrome team to review and merge any of these improvements. Each feature has been implemented with:
- Minimal changes to existing code
- Comprehensive test coverage where applicable
- Documentation in code comments
- Backwards compatibility in mind

**Priority suggestions for upstream**:
1. Security fixes (SHA3-256, crypto/rand, salt entropy)
2. Album artist derivation fix
3. DLNA/UPnP support
4. Gapless playback metadata

---

## License

This fork maintains the same GPL-3.0 license as the original Navidrome project.

---

*Fork maintained by [@rosschurchill](https://github.com/rosschurchill)*
