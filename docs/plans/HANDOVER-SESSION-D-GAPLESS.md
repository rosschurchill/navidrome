# Session D Handover: Gapless Playback Metadata

> **Date**: 2024-12-30
> **Session**: D (Gapless Playback)
> **Status**: Phase 1 Complete

---

## Summary

Implemented Phase 1 of gapless playback support: **metadata extraction** from audio files. The server now extracts encoder delay, encoder padding, and total sample counts from supported audio formats during library scanning.

---

## What Was Done

### 1. C++ TagLib Wrapper (`adapters/taglib/taglib_wrapper.cpp`)

Added extraction of gapless playback metadata:

| Format | Data Extracted | Method |
|--------|---------------|--------|
| **MP3** | Total samples (calculated) | Duration × sample rate |
| **M4A/AAC** | Encoder delay, padding, total samples | iTunSMPB atom parsing |
| **FLAC** | Total samples | `sampleFrames()` from stream info |
| **Opus** | Total samples (calculated) | Duration × sample rate |
| **Vorbis** | Total samples (calculated) | Duration × sample rate |
| **WavPack** | Total samples | `sampleFrames()` |
| **AIFF** | Total samples | `sampleFrames()` |
| **WAV** | Total samples | `sampleFrames()` |
| **DSF** | Total samples | `sampleCount()` |
| **APE** | Total samples | `sampleFrames()` |

**Note**: The code was modified by linter/user after initial implementation. The MP3 LAME header extraction was simplified to calculate total samples from duration instead of using `encoderDelay()`/`encoderPadding()` methods that may not be available in all TagLib versions. A TODO was added to parse LAME header manually if needed.

**iTunSMPB Parsing** (M4A/AAC):
```cpp
// Format: " 00000000 XXXXXXXX YYYYYYYY ZZZZZZZZZZZZZZZZ"
// where X = encoder delay (hex), Y = padding (hex), Z = total samples (hex)
if (item.first == "----:com.apple.iTunes:iTunSMPB") {
    // Parse hex values and send to Go
}
```

### 2. Go TagLib Adapter (`adapters/taglib/taglib.go`)

Added parsing of new properties:
```go
// Parse gapless playback properties
ap.EncoderDelay = parseProp(tags, "__encoderdelay")
ap.EncoderPadding = parseProp(tags, "__encoderpadding")
ap.TotalSamples = parsePropInt64(tags, "__totalsamples")
```

Added new helper function:
```go
func parsePropInt64(tags map[string][]string, prop string) int64
```

### 3. Metadata Layer (`model/metadata/metadata.go`)

Extended `AudioProperties` struct:
```go
type AudioProperties struct {
    Duration       time.Duration
    BitRate        int
    BitDepth       int
    SampleRate     int
    Channels       int
    EncoderDelay   int   // Samples to skip at start (for gapless playback)
    EncoderPadding int   // Samples to skip at end (for gapless playback)
    TotalSamples   int64 // Total sample count (for frame-accurate seeking)
}
```

### 4. Metadata Mapping (`model/metadata/map_mediafile.go`)

Added mapping to MediaFile:
```go
// Gapless playback properties
mf.EncoderDelay = md.AudioProperties().EncoderDelay
mf.EncoderPadding = md.AudioProperties().EncoderPadding
mf.TotalSamples = md.AudioProperties().TotalSamples
```

### 5. Model (`model/mediafile.go`)

Added new fields:
```go
EncoderDelay   int   `structs:"encoder_delay" json:"encoderDelay,omitempty"`
EncoderPadding int   `structs:"encoder_padding" json:"encoderPadding,omitempty"`
TotalSamples   int64 `structs:"total_samples" json:"totalSamples,omitempty"`
```

### 6. Database Migration

Created `db/migrations/20251230130000_add_gapless_playback_columns.go`:
```sql
ALTER TABLE media_file ADD COLUMN encoder_delay INTEGER DEFAULT 0;
ALTER TABLE media_file ADD COLUMN encoder_padding INTEGER DEFAULT 0;
ALTER TABLE media_file ADD COLUMN total_samples INTEGER DEFAULT 0;
```

---

## Files Modified

| File | Type | Description |
|------|------|-------------|
| `adapters/taglib/taglib_wrapper.cpp` | Modified | C++ gapless extraction |
| `adapters/taglib/taglib.go` | Modified | Go property parsing |
| `model/metadata/metadata.go` | Modified | AudioProperties struct |
| `model/metadata/map_mediafile.go` | Modified | Property mapping |
| `model/mediafile.go` | Modified | New model fields |
| `db/migrations/20251230130000_add_gapless_playback_columns.go` | **New** | DB migration |
| `docs/plans/04-GAPLESS-PLAYBACK.md` | Modified | Updated status |
| `docs/plans/00-MASTER-PLAN.md` | Modified | Updated project status |
| `CLAUDE.md` | Modified | Added documentation |

---

## Build Status

- **Go code**: Compiles successfully with `-tags=notag`
- **C++ code**: Requires TagLib library (`libtag1-dev`) to compile
- **No conflicts** with other sessions (Sonos, DLNA, Fingerprinting)

---

## What's Left (Phase 2-4)

### Phase 2: API Exposure
- [ ] Add `encoderDelay`, `encoderPadding`, `totalSamples` to Subsonic `Child` response
- [ ] Add fields to native API song response
- [ ] Document API extensions

### Phase 3: Web Player
- [ ] Implement track preloading
- [ ] Calculate transition points using gapless info
- [ ] Add optional crossfade
- [ ] Test with gapped and gapless albums

### Phase 4: Testing
- [ ] Test with LAME-encoded MP3s
- [ ] Test with iTunes AAC files
- [ ] Test with FLAC albums
- [ ] Test with mixed-format playlists
- [ ] Verify no regression for non-gapless content

---

## Known Limitations

1. **MP3 LAME Header**: The current implementation calculates total samples from duration rather than parsing the actual LAME header encoder delay/padding values. A TODO was left to parse the LAME header manually if more precise gapless info is needed.

2. **TagLib Version**: Some older TagLib versions may not expose `encoderDelay()`/`encoderPadding()` methods. The code was simplified to work with any TagLib version.

3. **Transcoding**: Gapless info may be lost when transcoding. This needs consideration in Phase 3.

---

## Test Recommendations

When testing Phase 1, rescan the library and check:
1. FLAC files should have `total_samples` populated
2. M4A files with iTunSMPB should have all three fields populated
3. MP3 files should have `total_samples` calculated from duration
4. Values should be visible in the database: `SELECT encoder_delay, encoder_padding, total_samples FROM media_file WHERE total_samples > 0 LIMIT 10;`

---

*Last Updated: 2024-12-30*
