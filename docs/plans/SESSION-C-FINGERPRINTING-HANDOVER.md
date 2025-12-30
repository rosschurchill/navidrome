# Session C: Audio Fingerprinting - Handover Document

> **Date**: 2024-12-30
> **Status**: Phase 1 Complete
> **Next Session**: Phase 2 (API & Database)

---

## Summary

Created the `core/fingerprint/` package providing audio fingerprinting integration with AcoustID and MusicBrainz for automatic track identification.

---

## Completed Work (Phase 1: Foundation)

### Files Created

| File | Purpose |
|------|---------|
| `core/fingerprint/fingerprint.go` | Main service interface and implementation |
| `core/fingerprint/chromaprint.go` | fpcalc wrapper with timeout handling |
| `core/fingerprint/acoustid.go` | Rate-limited AcoustID API client (3 req/sec) |
| `core/fingerprint/musicbrainz.go` | Rate-limited MusicBrainz API client (1 req/sec) |

### Files Modified

| File | Changes |
|------|---------|
| `conf/configuration.go` | Added `fingerprintOptions` struct and defaults |

### Configuration Options Added

```toml
[Fingerprint]
Enabled = false              # Must explicitly enable
AcoustIDApiKey = ""          # Required for lookups (get from acoustid.org)
FpcalcPath = ""              # Override fpcalc location (auto-detect if empty)
CacheResults = true          # Cache fingerprints in DB (Phase 2)
AutoIdentify = false         # Auto-fingerprint on scan (Phase 4)
BatchSize = 100              # Max tracks per batch job
```

Environment variables:
- `ND_FINGERPRINT_ENABLED=true`
- `ND_FINGERPRINT_ACOUSTIDAPIKEY=your_key`
- `ND_FINGERPRINT_FPCALCPATH=/usr/bin/fpcalc`

---

## Service API

```go
// core/fingerprint/fingerprint.go

type Service interface {
    // IsEnabled returns true if fingerprinting is configured and available
    IsEnabled() bool

    // Generate creates an audio fingerprint for the given file
    Generate(ctx context.Context, filePath string) (*FingerprintResult, error)

    // Lookup searches for matches using the given fingerprint
    Lookup(ctx context.Context, fingerprint string, duration int) ([]MatchResult, error)

    // Identify generates a fingerprint and looks up matches in one call
    Identify(ctx context.Context, filePath string) ([]MatchResult, error)
}

// Create service with:
svc := fingerprint.NewService()
```

---

## Remaining Work

### Phase 2: Database & Caching
- [ ] Create database migration for `media_file_fingerprint` table:
  ```sql
  CREATE TABLE media_file_fingerprint (
      id              VARCHAR(255) PRIMARY KEY,
      media_file_id   VARCHAR(255) NOT NULL REFERENCES media_file(id) ON DELETE CASCADE,
      fingerprint     TEXT NOT NULL,
      duration        INTEGER NOT NULL,
      acoustid_id     VARCHAR(255),
      musicbrainz_id  VARCHAR(255),
      confidence      REAL,
      cached_at       DATETIME NOT NULL,
      UNIQUE(media_file_id)
  );
  ```
- [ ] Add model and repository for fingerprint cache
- [ ] Implement cache layer in service

### Phase 3: API Endpoints
- [ ] `POST /api/fingerprint/{id}` - Fingerprint single track
- [ ] `POST /api/fingerprint/batch` - Batch fingerprint (background job)
- [ ] `GET /api/fingerprint/{id}/suggestions` - Get metadata suggestions

### Phase 4: UI & Scanner Integration
- [ ] Add "Identify" action to track context menu
- [ ] Metadata suggestions dialog
- [ ] Batch identification UI
- [ ] Optional auto-fingerprint during scan

---

## External Dependencies

```bash
# fpcalc must be installed on the system
apt install libchromaprint-tools  # Ubuntu/Debian
brew install chromaprint          # macOS
apk add chromaprint               # Alpine (Docker)
```

Users must register at https://acoustid.org/ to get an API key.

---

## Testing Notes

Build verification:
```bash
go build -tags=notag ./core/fingerprint/...
go build -tags=notag ./conf/...
```

Both packages compile successfully. No tests written yet (Phase 2 task).

---

## Key Design Decisions

1. **fpcalc over CGO**: Chose to shell out to `fpcalc` rather than CGO integration with libchromaprint for simplicity. Can upgrade to CGO later for performance.

2. **Rate limiting**: Used `golang.org/x/time/rate` for API rate limiting:
   - AcoustID: 3 requests/second
   - MusicBrainz: 1 request/second (conservative)

3. **Disabled by default**: Fingerprinting is disabled by default and requires explicit opt-in plus API key.

4. **Disabled with external services**: When `EnableExternalServices=false`, fingerprinting is also disabled.

---

## Reference Documents

- `docs/plans/05-AUDIO-FINGERPRINTING.md` - Full implementation plan
- `core/agents/lastfm/client.go` - Pattern reference for HTTP clients

---

*Session C completed: 2024-12-30*
