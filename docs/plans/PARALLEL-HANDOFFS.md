# Parallel Development Handoff Instructions

> **Purpose**: Copy-paste ready instructions for spinning up parallel Claude sessions
> **Last Updated**: 2024-12-30

---

## Shared Context (Include with ALL handoffs)

```markdown
## Project Context
- **Repository**: `/home/ross/Documents/general/navidrome`
- **What**: Navidrome fork with security hardening and new features
- **Language**: Go (backend), React (frontend), C++ (TagLib wrapper)
- **Entry Point**: Read `docs/plans/00-MASTER-PLAN.md` first

## Build Commands
```bash
# Backend compile check (no TagLib)
go build -tags=notag ./server/...

# Backend compile (with TagLib - requires libtag1-dev)
go build ./...

# Frontend
cd ui && npm install && npm run build
```

## Test Environment
- **QNAP Host**: `192.168.1.200` (ssh: `ross@tvsp01`)
- **Docker Path**: `/share/ZFS530_DATA/.qpkg/container-station/bin/docker`
- **Test Container**: `navidrome-test` at http://192.168.1.200:4535
```

---

## Session A: Sonos Security Hardening

**Copy everything below this line:**

---

```markdown
# Task: Sonos SMAPI Security Hardening

## Project Context
- **Repository**: `/home/ross/Documents/general/navidrome`
- **What**: Navidrome fork with security hardening and new features
- **Language**: Go (backend), React (frontend), C++ (TagLib wrapper)
- **Entry Point**: Read `docs/plans/00-MASTER-PLAN.md` first

## Build Commands
```bash
go build -tags=notag ./server/...
```

## Your Task
Harden the security of the existing Sonos SMAPI implementation.

## Required Reading (in order)
1. `docs/plans/00-MASTER-PLAN.md` - Project overview
2. `docs/plans/02-SONOS-SMAPI.md` - Full plan with security checklist
3. `server/sonos/auth.go` - Current auth implementation (needs fixes)
4. `SONOS_SECURITY_PLAN.md` - Detailed security analysis

## Current State
- Core SMAPI is implemented and working
- Auth flow exists but has security weaknesses
- Database migration and model already created

## Your Focus: Phase 1 Security Hardening
From `02-SONOS-SMAPI.md` Section 3, implement:

- [ ] **3.1 Token Storage**: Replace encryption with SHA-256 hashing
- [ ] **3.2 Password Validation**: Use bcrypt.CompareHashAndPassword (fix timing attack)
- [ ] **3.3 Rate Limiting**: Add sliding window rate limiter to auth endpoints
- [ ] **3.4 Token Expiration**: Enforce 90-day expiry on validation
- [ ] **3.5 HTTPS Enforcement**: Make configurable (warn vs block)
- [ ] **3.6 Key Derivation**: Implement Argon2id instead of simple SHA-256
- [ ] **3.7 XSS Prevention**: html.EscapeString on all outputs

## Key Files to Modify
| File | Changes |
|------|---------|
| `server/sonos/auth.go` | All security fixes |
| `server/sonos/smapi.go` | Rate limiting middleware |

## Do NOT Modify (other sessions working on these)
- `server/dlna/*` - DLNA session
- `core/fingerprint/*` - Fingerprinting session
- `adapters/taglib/*` - Gapless session

## Reference Code Patterns
- bcrypt usage: `server/auth.go` (main Navidrome auth)
- Rate limiting: Consider `golang.org/x/time/rate`

## Coordination
Another session is working on DLNA simultaneously. No file conflicts expected.
```

---

## Session B: DLNA/UPnP Foundation

**Copy everything below this line:**

---

```markdown
# Task: DLNA/UPnP Media Server Implementation

## Project Context
- **Repository**: `/home/ross/Documents/general/navidrome`
- **What**: Navidrome fork with security hardening and new features
- **Language**: Go (backend), React (frontend), C++ (TagLib wrapper)
- **Entry Point**: Read `docs/plans/00-MASTER-PLAN.md` first

## Build Commands
```bash
go build -tags=notag ./server/...
```

## Your Task
Create a new DLNA/UPnP media server package from scratch.

## Required Reading (in order)
1. `docs/plans/00-MASTER-PLAN.md` - Project overview, architecture
2. `docs/plans/03-DLNA-UPNP.md` - Full DLNA implementation plan
3. `server/sonos/smapi.go` - Reference for how protocol servers are structured
4. `cmd/root.go` - How routers are conditionally mounted

## Current State
- Nothing implemented yet - you're starting fresh
- Plan document has full architecture and code samples

## Your Focus: Phase 1 Foundation
From `03-DLNA-UPNP.md` Section 7, implement:

- [ ] Create `server/dlna/` package structure
- [ ] Implement SSDP announcement (UDP multicast 239.255.255.250:1900)
- [ ] Implement M-SEARCH response handler
- [ ] Create device description XML endpoint
- [ ] Add config options to `conf/configuration.go`

## Package Structure to Create
```
server/dlna/
├── dlna.go              # Main router, initialization
├── ssdp.go              # SSDP discovery (UDP multicast)
├── device.go            # Device description XML
└── routes.go            # HTTP routes setup
```

## Key Integration Points
| What | Where | Pattern to Follow |
|------|-------|-------------------|
| Config options | `conf/configuration.go` | Copy `sonosOptions` struct pattern |
| Router mounting | `cmd/root.go` | Copy Sonos conditional mount pattern |
| Core services | Inject `model.DataStore` | Same as other routers |

## Do NOT Modify Yet (Phase 2)
- `cmd/root.go` - Just create package first, integrate later
- `conf/configuration.go` - Just create package first, integrate later

## Reference Libraries
- Consider [goupnp](https://github.com/huin/goupnp) for SSDP
- Reference [MiniDLNA](https://sourceforge.net/p/minidlna/git/ci/master/tree/) for DLNA compliance

## Coordination
Another session is working on Sonos security simultaneously. No file conflicts expected.
```

---

## Session C: Audio Fingerprinting

**Copy everything below this line:**

---

```markdown
# Task: Audio Fingerprinting Integration

## Project Context
- **Repository**: `/home/ross/Documents/general/navidrome`
- **What**: Navidrome fork with security hardening and new features
- **Language**: Go (backend), React (frontend), C++ (TagLib wrapper)
- **Entry Point**: Read `docs/plans/00-MASTER-PLAN.md` first

## Build Commands
```bash
go build -tags=notag ./core/...
```

## Your Task
Create audio fingerprinting integration with AcoustID/MusicBrainz.

## Required Reading (in order)
1. `docs/plans/00-MASTER-PLAN.md` - Project overview, architecture
2. `docs/plans/05-AUDIO-FINGERPRINTING.md` - Full implementation plan
3. `core/agents/lastfm/client.go` - Reference for external API client pattern

## Current State
- Nothing implemented yet - you're starting fresh
- Plan document has full architecture and code samples

## Your Focus: Phase 1 Foundation
From `05-AUDIO-FINGERPRINTING.md` Section 7, implement:

- [ ] Create `core/fingerprint/` package structure
- [ ] Implement fpcalc wrapper (shell out to chromaprint)
- [ ] Create rate-limited AcoustID API client
- [ ] Create rate-limited MusicBrainz API client
- [ ] Add config options

## Package Structure to Create
```
core/fingerprint/
├── fingerprint.go      # Main interface
├── chromaprint.go      # fpcalc wrapper
├── acoustid.go         # AcoustID API client
├── musicbrainz.go      # MusicBrainz API client
└── ratelimit.go        # Rate limiting (3/sec AcoustID, 1/sec MB)
```

## Key Patterns to Follow
| What | Reference | Notes |
|------|-----------|-------|
| External API client | `core/agents/lastfm/client.go` | HTTP client pattern |
| Rate limiting | `golang.org/x/time/rate` | Use rate.Limiter |
| Config options | `conf/configuration.go` | Add FingerprintOptions |

## External Dependencies
```bash
# fpcalc must be installed on system
apt install libchromaprint-tools  # Ubuntu
brew install chromaprint          # macOS
```

## Do NOT Modify Yet (Phase 2)
- Database migrations - create package first
- API endpoints - create core logic first

## Coordination
Other sessions working on Sonos and DLNA. No file conflicts expected.
```

---

## Session D: Gapless Playback Metadata

**Copy everything below this line:**

---

```markdown
# Task: Gapless Playback Metadata Extraction

## Project Context
- **Repository**: `/home/ross/Documents/general/navidrome`
- **What**: Navidrome fork with security hardening and new features
- **Language**: Go (backend), React (frontend), C++ (TagLib wrapper)
- **Entry Point**: Read `docs/plans/00-MASTER-PLAN.md` first

## Build Commands
```bash
# Requires TagLib development headers
apt install libtag1-dev  # Ubuntu
go build ./adapters/taglib/...
```

## Your Task
Extract gapless playback metadata (encoder delay/padding) from audio files.

## Required Reading (in order)
1. `docs/plans/00-MASTER-PLAN.md` - Project overview, architecture
2. `docs/plans/04-GAPLESS-PLAYBACK.md` - Full implementation plan
3. `adapters/taglib/taglib_wrapper.cpp` - Current TagLib C++ wrapper
4. `adapters/taglib/taglib_wrapper.go` - Go FFI bindings

## Current State
- TagLib wrapper exists and extracts metadata
- No gapless info extraction yet

## Your Focus: Phase 1 Metadata Extraction
From `04-GAPLESS-PLAYBACK.md` Section 6, implement:

- [ ] Update `taglib_wrapper.cpp` to extract LAME header (encoder delay/padding)
- [ ] Extract iTunSMPB from M4A/AAC files
- [ ] Handle FLAC frame count from stream info
- [ ] Update Go struct to receive new fields
- [ ] Create database migration for new columns

## Key Files to Modify
| File | Changes |
|------|---------|
| `adapters/taglib/taglib_wrapper.cpp` | Extract gapless info from LAME header, iTunSMPB |
| `adapters/taglib/taglib_wrapper.h` | Add fields to Props struct |
| `adapters/taglib/taglib_wrapper.go` | Map C struct to Go |
| `model/mediafile.go` | Add EncoderDelay, EncoderPadding, TotalSamples fields |

## C++ Code Reference (from plan)
```cpp
// For MP3 files with LAME header
if (TagLib::MPEG::File *mpegFile = dynamic_cast<TagLib::MPEG::File*>(file)) {
    TagLib::MPEG::Properties *props = mpegFile->audioProperties();
    if (props) {
        goPutInt(id, "encoderDelay", props->encoderDelay());
        goPutInt(id, "encoderPadding", props->encoderPadding());
    }
}
```

## Database Migration
```sql
ALTER TABLE media_file ADD COLUMN encoder_delay INTEGER DEFAULT 0;
ALTER TABLE media_file ADD COLUMN encoder_padding INTEGER DEFAULT 0;
ALTER TABLE media_file ADD COLUMN total_samples INTEGER DEFAULT 0;
```

## Do NOT Modify Yet (Phase 2)
- API responses - extract metadata first
- Web player - needs API first

## Coordination
Other sessions working on Sonos and DLNA. No file conflicts expected.
This is the only session touching `adapters/taglib/`.
```

---

## Phase 2 Integration (After Phase 1 Complete)

Once all Phase 1 work is done, coordinate to merge:

| Task | Files | Owner |
|------|-------|-------|
| Add DLNA config | `conf/configuration.go` | DLNA session |
| Add DLNA router mount | `cmd/root.go` | DLNA session |
| Add Fingerprint config | `conf/configuration.go` | Fingerprint session |
| Add Gapless DB migration | `db/migrations/` | Gapless session |
| Add Gapless model fields | `model/mediafile.go` | Gapless session |

---

## Phase 3-4 (Feature Completion)

After Phase 2 integration:
- Each session continues with their plan's Phase 2-4 checklists
- API endpoints, UI components, testing

---

*This document is for development coordination. Update as sessions complete phases.*
