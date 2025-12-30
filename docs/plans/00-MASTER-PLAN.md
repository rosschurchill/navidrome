# Navidrome Fork - Master Project Plan

> **For AI Sessions**: This is the entry point. Read this first, then dive into specific feature plans as needed.

## Quick Status

| Phase | Status | Plan Document |
|-------|--------|---------------|
| 1. Security Hardening | ✅ Complete | `01-SECURITY-HARDENING.md` |
| 2. Metadata (WAV/RIFF) | ✅ Complete | N/A (simple implementation) |
| 3a. Smart Playlists UI | ✅ Complete | N/A |
| 3b. Advanced Search | ✅ Complete | N/A |
| 3c. Split Albums Fix | ✅ Complete | N/A |
| 3d. Song Info Panel | ✅ Complete | N/A |
| 3e. **Sonos SMAPI** | 🔄 Phase 1 ✅ (Security), Phase 2 (Testing) | `02-SONOS-SMAPI.md` |
| 3f. **DLNA/UPnP** | 🔄 Phase 1-2 ✅, Phase 3-4 Pending | `03-DLNA-UPNP.md` |
| 3g. **Gapless Playback** | 🔄 Phase 1 ✅ (Metadata), Phase 2-4 Pending | `04-GAPLESS-PLAYBACK.md` |
| 3h. **Audio Fingerprinting** | 🔄 Phase 1 ✅ (Core), Phase 2-4 Pending | `05-AUDIO-FINGERPRINTING.md` |

---

## Handover Documents

| Session | Feature | Document |
|---------|---------|----------|
| A | Sonos Security | `HANDOVER-SONOS-SECURITY.md` |
| B | DLNA/UPnP | `DLNA-HANDOVER.md` |
| C | Audio Fingerprinting | `SESSION-C-FINGERPRINTING-HANDOVER.md` |
| D | Gapless Playback | `HANDOVER-SESSION-D-GAPLESS.md` |

## Action Plan

See `ACTION-PLAN.md` for consolidated implementation and testing checklist.

---

## Plan Document Index

| Document | Purpose | Complexity | Dependencies |
|----------|---------|------------|--------------|
| `00-MASTER-PLAN.md` | Entry point, status tracking, project overview | - | - |
| `01-SECURITY-HARDENING.md` | Security fixes (SHA3-256, crypto/rand, CSP) | Low | None |
| `02-SONOS-SMAPI.md` | Native Sonos integration with security hardening | High | PasswordEncryptionKey |
| `03-DLNA-UPNP.md` | DLNA/UPnP media server for TVs, consoles | High | None |
| `04-GAPLESS-PLAYBACK.md` | Seamless track transitions | Medium | Client support |
| `05-AUDIO-FINGERPRINTING.md` | AcoustID/MusicBrainz integration | High | External services |

---

## Project Overview

### What Is This?
A security-hardened fork of [Navidrome](https://github.com/navidrome/navidrome) (v0.54.x) with:
- Critical security fixes (MD5→SHA3-256, crypto/rand, CSP headers, XSS fixes)
- Enhanced metadata support (WAV RIFF INFO chunks)
- New features (Smart Playlists, Native Sonos, etc.)

### Repository
- **Location**: `/home/ross/Documents/general/navidrome`
- **Language**: Go (backend), React (frontend)
- **Key Dependencies**: TagLib (C++), SQLite, chi router

### Test Environment
- **QNAP Host**: `192.168.1.200` (ssh: `ross@tvsp01`)
- **Test Container**: `navidrome-test` at http://192.168.1.200:4535
- **Original Container**: `navidrome` at http://192.168.1.200:4533

---

## Completed Work Summary

### Phase 1: Security (DONE)
- SHA3-256 migration (replaced MD5 in 17 files)
- crypto/rand for all randomness
- Salt entropy increased (3→16 bytes)
- CSP headers enabled
- XSS sanitization in React components

### Phase 2: Metadata (DONE)
- WAV RIFF INFO chunk extraction in `taglib_wrapper.cpp`
- Diagnostic logging for missing metadata

### Phase 3: Features (PARTIAL)
| Feature | Status | Key Files |
|---------|--------|-----------|
| Smart Playlists UI | ✅ | `ui/src/playlist/SmartPlaylistRulesBuilder.jsx` |
| Advanced Search | ✅ | `persistence/advanced_search.go`, `sql_search.go` |
| Album Artist Derivation | ✅ | `model/metadata/map_participants.go` |
| Cover Art Fallback | ✅ | `core/artwork/reader_album.go` |
| Split Albums Detection | ✅ | `server/nativeapi/split_albums.go`, UI dialog |
| Enhanced Song Info | ✅ | `ui/src/common/SongInfo.jsx` |
| **Sonos SMAPI** | 🔄 | `server/sonos/*` - See detailed plan |
| DLNA/UPnP | ⏳ | Not started - See detailed plan |
| Gapless Playback | ⏳ | Not started - See detailed plan |
| Audio Fingerprinting | ⏳ | Not started - See detailed plan |

---

## Architecture Overview

```
navidrome/
├── adapters/taglib/       # C++ TagLib wrapper for metadata
├── cmd/                   # Entry points, Wire DI
├── conf/                  # Configuration (viper-based)
├── core/                  # Business logic (artwork, playback, etc.)
├── db/migrations/         # Goose migrations
├── model/                 # Data models & repository interfaces
├── persistence/           # SQLite implementations
├── server/
│   ├── nativeapi/         # REST API for web UI
│   ├── subsonic/          # Subsonic API compatibility
│   ├── sonos/             # NEW: Sonos SMAPI (SOAP)
│   └── public/            # Public endpoints (shares)
└── ui/                    # React frontend
```

---

## How to Work on This Project

### 1. Reading Code
```bash
# Key files to understand the codebase
cat CLAUDE.md                    # Project context
cat docs/plans/00-MASTER-PLAN.md # This file
```

### 2. Building
```bash
# Requires: Go 1.21+, Node 18+, TagLib (libtag1-dev)
go build ./...                   # Backend (needs TagLib)
cd ui && npm install && npm build # Frontend
```

### 3. Testing Locally
```bash
# Non-TagLib tests
go test ./model/... ./persistence/...

# With TagLib
go test ./adapters/taglib/...
```

### 4. Deploying to Test Container
```bash
# Build for QNAP (linux/amd64)
docker build --platform linux/amd64 -t navidrome-fork:latest .

# Transfer and load
docker save navidrome-fork:latest | gzip > /tmp/navidrome-fork.tar.gz
rsync -avP /tmp/navidrome-fork.tar.gz ross@tvsp01:/share/Container/navidrome-test/
ssh ross@tvsp01 "gunzip -c /share/Container/navidrome-test/navidrome-fork.tar.gz | /share/ZFS530_DATA/.qpkg/container-station/bin/docker load"
ssh ross@tvsp01 "/share/ZFS530_DATA/.qpkg/container-station/bin/docker restart navidrome-test"
```

### 5. Viewing Logs
```bash
ssh ross@tvsp01 "/share/ZFS530_DATA/.qpkg/container-station/bin/docker logs navidrome-test --tail 100"
```

---

## Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2024-12-29 | SHA3-256 over SHA-256 | Future-proof, no known weaknesses |
| 2024-12-29 | Native Sonos over bonob | Eliminate external dependency |
| 2024-12-30 | Token hashing over encryption | One-way = more secure for storage |
| 2024-12-30 | Argon2id for key derivation | Memory-hard, brute-force resistant |

---

## Open Questions (Project-Wide)

1. **Release Strategy**: When to open-source the fork?
2. **Upstream Contributions**: Which fixes to PR back to Navidrome?
3. **Versioning**: How to version relative to upstream?

---

## Next Steps (Priority Order)

1. **Complete Sonos SMAPI** - Review `02-SONOS-SMAPI.md`, implement security hardening
2. **Plan DLNA/UPnP** - Research, document in `03-DLNA-UPNP.md`
3. **Plan Gapless Playback** - Document in `04-GAPLESS-PLAYBACK.md`
4. **Plan Audio Fingerprinting** - Document in `05-AUDIO-FINGERPRINTING.md`

---

*Last Updated: 2024-12-30*
