# Feature Plan: Audio Fingerprinting Integration

> **Status**: ⏳ Planned (Not Started)
> **Priority**: Medium
> **Complexity**: High
> **Dependencies**: External service (AcoustID/MusicBrainz)

---

## 1. Overview

### What Is This?
Integration with audio fingerprinting services (AcoustID/Chromaprint) and metadata databases (MusicBrainz) to automatically identify and tag music files.

### Why Do We Need It?
- **Unknown files**: Identify poorly tagged or untagged music
- **Metadata correction**: Fix incorrect artist/album/track info
- **Duplicate detection**: Find duplicate tracks (different encodings)
- **Library cleanup**: Mass-identify files in bulk

### The Challenge
Audio fingerprinting requires:
1. **Chromaprint library**: C library for generating fingerprints
2. **AcoustID API**: Web service to match fingerprints to recordings
3. **MusicBrainz API**: Rich metadata database
4. **Rate limiting**: Both services have strict rate limits

---

## 2. Technical Background

### How Audio Fingerprinting Works

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   Audio File    │ ──▶ │   Chromaprint    │ ──▶ │   Fingerprint   │
│   (any format)  │     │   (local calc)   │     │   (hash string) │
└─────────────────┘     └──────────────────┘     └────────┬────────┘
                                                          │
                                                          ▼
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   Metadata      │ ◀── │   MusicBrainz    │ ◀── │    AcoustID     │
│   (tags)        │     │   (metadata DB)  │     │   (fingerprint  │
└─────────────────┘     └──────────────────┘     │    lookup)      │
                                                 └─────────────────┘
```

### Key Components

| Component | Purpose | Rate Limit |
|-----------|---------|------------|
| **Chromaprint** | Generate audio fingerprint locally | None (local) |
| **AcoustID API** | Match fingerprint to recording ID | 3 req/sec |
| **MusicBrainz API** | Fetch detailed metadata | 1 req/sec |

### Fingerprint Format
```
Chromaprint Output:
- Duration: 180 (seconds)
- Fingerprint: AQADtImSZEmSJEn...  (base64-encoded)

AcoustID Response:
{
  "results": [{
    "id": "abcd1234-...",
    "recordings": [{
      "id": "recording-mbid-...",
      "title": "Song Title",
      "artists": [{"name": "Artist"}]
    }]
  }]
}
```

---

## 3. Implementation Approaches

### Approach A: CGO Chromaprint Integration (Recommended)

Direct integration with libchromaprint via CGO:

```go
// adapters/chromaprint/chromaprint.go
/*
#cgo pkg-config: libchromaprint
#include <chromaprint.h>
*/
import "C"

func GenerateFingerprint(audioPath string) (string, int, error) {
    // Use ffmpeg to decode audio to raw PCM
    // Feed to chromaprint_feed()
    // Get fingerprint with chromaprint_get_fingerprint()
}
```

**Pros**:
- Fast, native performance
- No external process spawning
- Full control over audio decoding

**Cons**:
- Requires libchromaprint and FFmpeg development headers
- Increases build complexity

### Approach B: fpcalc External Command

Shell out to `fpcalc` command-line tool:

```go
func GenerateFingerprint(audioPath string) (string, int, error) {
    cmd := exec.Command("fpcalc", "-json", audioPath)
    output, err := cmd.Output()
    // Parse JSON output
}
```

**Pros**:
- Simpler implementation
- No CGO complexity
- Easy to test

**Cons**:
- Requires fpcalc binary installed
- Process spawning overhead
- Less control over errors

### Approach C: Hybrid

Use CGO when available, fall back to fpcalc:

```go
func GenerateFingerprint(audioPath string) (string, int, error) {
    if chromaprintAvailable {
        return generateWithCGO(audioPath)
    }
    return generateWithFpcalc(audioPath)
}
```

### Recommendation: Approach B for Initial Implementation
- Simpler to implement and test
- Can upgrade to CGO later for performance
- Most users can install fpcalc package

---

## 4. Architecture Design

### Package Structure
```
core/fingerprint/
├── fingerprint.go      # Main interface
├── chromaprint.go      # Fingerprint generation (fpcalc wrapper)
├── acoustid.go         # AcoustID API client
├── musicbrainz.go      # MusicBrainz API client
├── matcher.go          # Match coordination logic
├── ratelimit.go        # Rate limiting for APIs
└── cache.go            # Fingerprint/result caching
```

### Integration Points
```
┌─────────────────────────────────────────────────────────────┐
│                      Navidrome                               │
├─────────────────┬───────────────────┬───────────────────────┤
│   Scanner       │   Native API      │   Subsonic API        │
│   (batch ID)    │   (single ID)     │   (getLyrics etc)     │
├─────────────────┴───────────────────┴───────────────────────┤
│                 Fingerprint Service                          │
│  ┌────────────┐  ┌────────────┐  ┌────────────────────┐     │
│  │ Chromaprint │  │  AcoustID  │  │   MusicBrainz      │     │
│  │ (local)    │  │  (remote)  │  │   (remote)         │     │
│  └────────────┘  └────────────┘  └────────────────────┘     │
└─────────────────────────────────────────────────────────────┘
```

---

## 5. Implementation Details

### 5.1 Fingerprint Generation

```go
// core/fingerprint/chromaprint.go

type FingerprintResult struct {
    Duration    int    `json:"duration"`
    Fingerprint string `json:"fingerprint"`
}

func Generate(ctx context.Context, filePath string) (*FingerprintResult, error) {
    // Check if fpcalc is available
    fpcalcPath, err := exec.LookPath("fpcalc")
    if err != nil {
        return nil, fmt.Errorf("fpcalc not found: %w", err)
    }

    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, fpcalcPath, "-json", filePath)
    output, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("fpcalc failed: %w", err)
    }

    var result FingerprintResult
    if err := json.Unmarshal(output, &result); err != nil {
        return nil, fmt.Errorf("parse fpcalc output: %w", err)
    }

    return &result, nil
}
```

### 5.2 AcoustID Lookup

```go
// core/fingerprint/acoustid.go

const acoustIDURL = "https://api.acoustid.org/v2/lookup"

type AcoustIDClient struct {
    apiKey     string
    httpClient *http.Client
    limiter    *rate.Limiter  // 3 req/sec
}

type AcoustIDResult struct {
    Results []struct {
        ID         string  `json:"id"`
        Score      float64 `json:"score"`
        Recordings []struct {
            ID      string `json:"id"`  // MusicBrainz Recording ID
            Title   string `json:"title"`
            Artists []struct {
                ID   string `json:"id"`
                Name string `json:"name"`
            } `json:"artists"`
        } `json:"recordings"`
    } `json:"results"`
}

func (c *AcoustIDClient) Lookup(ctx context.Context, fingerprint string, duration int) (*AcoustIDResult, error) {
    // Wait for rate limiter
    if err := c.limiter.Wait(ctx); err != nil {
        return nil, err
    }

    params := url.Values{
        "client":      {c.apiKey},
        "fingerprint": {fingerprint},
        "duration":    {strconv.Itoa(duration)},
        "meta":        {"recordings"},
    }

    resp, err := c.httpClient.Get(acoustIDURL + "?" + params.Encode())
    // ... parse response
}
```

### 5.3 MusicBrainz Enrichment

```go
// core/fingerprint/musicbrainz.go

const musicBrainzURL = "https://musicbrainz.org/ws/2"

type MusicBrainzClient struct {
    httpClient *http.Client
    limiter    *rate.Limiter  // 1 req/sec
    userAgent  string         // Required by MB
}

type Recording struct {
    ID           string `json:"id"`
    Title        string `json:"title"`
    Length       int    `json:"length"`
    ArtistCredit []struct {
        Name   string `json:"name"`
        Artist struct {
            ID   string `json:"id"`
            Name string `json:"name"`
        } `json:"artist"`
    } `json:"artist-credit"`
    Releases []struct {
        ID    string `json:"id"`
        Title string `json:"title"`
        Date  string `json:"date"`
    } `json:"releases"`
}

func (c *MusicBrainzClient) GetRecording(ctx context.Context, mbid string) (*Recording, error) {
    // Wait for rate limiter
    if err := c.limiter.Wait(ctx); err != nil {
        return nil, err
    }

    url := fmt.Sprintf("%s/recording/%s?fmt=json&inc=artists+releases", musicBrainzURL, mbid)
    // ... fetch and parse
}
```

### 5.4 Caching Layer

```go
// core/fingerprint/cache.go

// Cache fingerprints in database to avoid regeneration
type FingerprintCache struct {
    ds model.DataStore
}

// Table: media_file_fingerprint
// - media_file_id (FK)
// - fingerprint (string)
// - duration (int)
// - acoustid_id (string, nullable)
// - musicbrainz_id (string, nullable)
// - cached_at (datetime)
```

### 5.5 API Endpoints

**Native API**:
```go
// POST /api/fingerprint/{id}
// Generate fingerprint and lookup metadata for a single track

// POST /api/fingerprint/batch
// Batch fingerprint multiple tracks (background job)

// GET /api/fingerprint/{id}/suggestions
// Get metadata suggestions from fingerprint match
```

---

## 6. Configuration

```toml
[Fingerprint]
Enabled = false              # Must explicitly enable
AcoustIDApiKey = ""          # Required for lookups
FpcalcPath = ""              # Override fpcalc location (auto-detect if empty)
CacheResults = true          # Cache fingerprints in DB
AutoIdentify = false         # Auto-fingerprint on scan (resource intensive)
BatchSize = 100              # Max tracks per batch job
```

### Required External Setup
1. Register at https://acoustid.org/ to get API key
2. Install `fpcalc` (part of chromaprint-tools package)

```bash
# Ubuntu/Debian
apt install libchromaprint-tools

# macOS
brew install chromaprint

# Alpine (Docker)
apk add chromaprint
```

---

## 7. Implementation Checklist

### Phase 1: Foundation
- [ ] Create `core/fingerprint/` package structure
- [ ] Implement fpcalc wrapper with timeout handling
- [ ] Add rate-limited HTTP clients for APIs
- [ ] Create database table for fingerprint cache
- [ ] Add configuration options

### Phase 2: API Integration
- [ ] Implement AcoustID lookup client
- [ ] Implement MusicBrainz recording lookup
- [ ] Add caching for API responses
- [ ] Handle API errors gracefully (rate limits, timeouts)

### Phase 3: User Interface
- [ ] Add "Identify" action to track context menu
- [ ] Show metadata suggestions dialog
- [ ] Allow user to accept/reject suggestions
- [ ] Batch identification UI

### Phase 4: Scanner Integration
- [ ] Optional auto-fingerprint during scan
- [ ] Background job for batch fingerprinting
- [ ] Progress reporting for large libraries

---

## 8. Database Schema

### Table: `media_file_fingerprint`
```sql
CREATE TABLE media_file_fingerprint (
    id              VARCHAR(255) PRIMARY KEY,
    media_file_id   VARCHAR(255) NOT NULL REFERENCES media_file(id) ON DELETE CASCADE,
    fingerprint     TEXT NOT NULL,
    duration        INTEGER NOT NULL,
    acoustid_id     VARCHAR(255),
    musicbrainz_id  VARCHAR(255),
    confidence      REAL,           -- Match confidence score
    cached_at       DATETIME NOT NULL,
    UNIQUE(media_file_id)
);

CREATE INDEX idx_fingerprint_mbid ON media_file_fingerprint(musicbrainz_id);
CREATE INDEX idx_fingerprint_acoustid ON media_file_fingerprint(acoustid_id);
```

---

## 9. API Rate Limiting Strategy

### Rate Limit Handling
```go
// Sliding window rate limiter per service
type RateLimiter struct {
    limiter *rate.Limiter
    backoff time.Duration
}

// AcoustID: 3 req/sec
acoustID := rate.NewLimiter(rate.Every(334*time.Millisecond), 1)

// MusicBrainz: 1 req/sec (be conservative)
musicBrainz := rate.NewLimiter(rate.Every(1100*time.Millisecond), 1)
```

### Batch Processing
- Queue fingerprint requests
- Process in background with rate limiting
- Report progress to user
- Allow cancellation

---

## 10. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Rate limit exceeded | High | Strict client-side rate limiting, exponential backoff |
| fpcalc not installed | Medium | Clear error message, installation instructions |
| Poor match quality | Medium | Show confidence score, require user confirmation |
| API service down | Low | Graceful degradation, cached results |
| Large library scan | Medium | Background processing, progress indication |

---

## 11. Testing Plan

### Unit Tests
- [ ] Fingerprint generation mock
- [ ] AcoustID response parsing
- [ ] MusicBrainz response parsing
- [ ] Rate limiter behavior
- [ ] Cache hit/miss logic

### Integration Tests
- [ ] Full fingerprint → lookup → enrich flow
- [ ] Rate limit compliance (don't exceed)
- [ ] Error handling (network, API errors)
- [ ] Cache persistence across restarts

### Test Files
- Create small audio test fixtures (10-second clips)
- Use known recordings with verified AcoustID entries
- Test various formats (MP3, FLAC, M4A)

---

## 12. User Experience

### Workflow: Single Track
1. User right-clicks track → "Identify with Fingerprint"
2. System shows spinner while fingerprinting
3. Dialog shows top matches with confidence scores
4. User selects correct match (or "None match")
5. System updates metadata (with undo option)

### Workflow: Batch
1. User selects multiple tracks → "Identify All"
2. Background job starts with progress bar
3. When complete, show summary: "42 identified, 8 uncertain, 3 not found"
4. User can review uncertain matches

### UI Components
- Confidence indicator (green/yellow/red)
- Side-by-side current vs. suggested metadata
- Album art preview from MusicBrainz
- "Apply" / "Skip" / "Apply All" buttons

---

## 13. Research Links

- [Chromaprint Documentation](https://acoustid.org/chromaprint)
- [AcoustID Web Service](https://acoustid.org/webservice)
- [MusicBrainz API](https://musicbrainz.org/doc/MusicBrainz_API)
- [fpcalc Usage](https://acoustid.org/chromaprint#fpcalc)
- [Rate Limiting Best Practices](https://musicbrainz.org/doc/MusicBrainz_API/Rate_Limiting)

---

## 14. Open Questions

1. **Auto-identify**: Should new files be fingerprinted automatically during scan?
2. **Confidence threshold**: What score requires user confirmation vs auto-apply?
3. **MusicBrainz Picard**: Integration with existing Picard workflows?
4. **Cover art**: Also fetch album art from Cover Art Archive?
5. **Submission**: Allow submitting new fingerprints to AcoustID?

---

*Last Updated: 2024-12-30*
*Status: Research/Planning*
