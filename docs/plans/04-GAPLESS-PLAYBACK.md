# Feature Plan: Gapless Playback

> **Status**: 🔄 In Progress (Phase 1 Complete)
> **Priority**: Medium
> **Complexity**: Medium
> **Dependencies**: Client support required

---

## 1. Overview

### What Is This?
Seamless audio playback between tracks with no silence gaps. Critical for:
- Live albums
- Classical music
- DJ mixes
- Concept albums (Pink Floyd, etc.)

### Why Do We Need It?
- Current behavior inserts audible gaps between tracks
- Ruins listening experience for continuous albums
- Expected feature in modern music servers

### The Challenge
Gapless playback requires coordination between:
1. **Server**: Provide track boundary information
2. **Client**: Pre-buffer next track, crossfade
3. **Audio Format**: Some formats have inherent padding (MP3)

---

## 2. Technical Background

### Why Gaps Exist

| Cause | Description | Solution |
|-------|-------------|----------|
| **HTTP Latency** | Time to request next track | Pre-buffering |
| **Decoder Startup** | Audio decoder initialization | Keep decoder running |
| **MP3 Padding** | Encoder adds silence frames | Trim with gapless info |
| **Track Boundaries** | Hard stop at end of track | Crossfade/overlap |

### MP3 Gapless Info

MP3 encoders add padding frames. Gapless info tells decoders how much to trim:

```
LAME Header (Xing/Info frame):
- Encoder delay: samples to skip at start
- Padding: samples to skip at end
```

**Location**: In ID3 tag or LAME header of MP3 files.

### Other Formats
| Format | Gapless Support | Notes |
|--------|-----------------|-------|
| FLAC | Native | Frame-accurate, no padding |
| AAC | iTunes metadata | `iTunSMPB` atom |
| MP3 | LAME header | If encoded with LAME |
| Opus | Native | Designed for gapless |
| WAV/AIFF | Native | No compression padding |

---

## 3. Implementation Approaches

### Approach A: Subsonic API Extension (Recommended)

Extend Subsonic API responses with gapless metadata:

```xml
<song id="123" ...
      encoderDelay="576"
      encoderPadding="1152"
      totalSamples="9876543"
      sampleRate="44100">
</song>
```

**Client Behavior**:
1. Parse gapless info from API response
2. Pre-fetch next track while current plays
3. Trim padding from current track end
4. Trim padding from next track start
5. Seamlessly transition

### Approach B: HTTP Range + Timing

Provide precise timing via HTTP headers:

```http
X-Gapless-Duration: 234567  (total samples)
X-Gapless-Delay: 576        (samples to skip at start)
X-Gapless-Padding: 1152     (samples to skip at end)
```

### Approach C: Server-Side Stitching

Server concatenates tracks into continuous stream:

```
GET /stream/playlist/456?gapless=true
```

**Pros**: Works with any client
**Cons**: Complex, loses seek ability, resource intensive

### Recommendation: Approach A
- Most flexible
- Client controls experience
- Works with existing streaming infrastructure
- Widely supported pattern (Spotify, Apple Music use similar)

---

## 4. Implementation Details

### 4.1 Extract Gapless Metadata

**TagLib Changes** (`adapters/taglib/taglib_wrapper.cpp`):

```cpp
// For MP3 files with LAME header
if (TagLib::MPEG::File *mpegFile = dynamic_cast<TagLib::MPEG::File*>(file)) {
    TagLib::MPEG::Properties *props = mpegFile->audioProperties();
    if (props) {
        // LAME header info
        goPutInt(id, "encoderDelay", props->encoderDelay());
        goPutInt(id, "encoderPadding", props->encoderPadding());
    }
}

// For AAC/M4A with iTunes gapless info
if (TagLib::MP4::File *mp4File = dynamic_cast<TagLib::MP4::File*>(file)) {
    TagLib::MP4::ItemMap items = mp4File->tag()->itemMap();
    if (items.contains("----:com.apple.iTunes:iTunSMPB")) {
        // Parse iTunSMPB atom
        // Format: " 00000000 XXXXXXXX YYYYYYYY ZZZZZZZZ..."
        // X = encoder delay, Y = padding, Z = total samples
    }
}
```

### 4.2 Store in Database

**Migration**:
```sql
ALTER TABLE media_file ADD COLUMN encoder_delay INTEGER DEFAULT 0;
ALTER TABLE media_file ADD COLUMN encoder_padding INTEGER DEFAULT 0;
ALTER TABLE media_file ADD COLUMN total_samples INTEGER DEFAULT 0;
```

**Model** (`model/mediafile.go`):
```go
type MediaFile struct {
    // ... existing fields ...
    EncoderDelay   int `json:"encoderDelay,omitempty"`
    EncoderPadding int `json:"encoderPadding,omitempty"`
    TotalSamples   int64 `json:"totalSamples,omitempty"`
}
```

### 4.3 Expose via API

**Subsonic API** (`server/subsonic/responses.go`):
```go
type Child struct {
    // ... existing fields ...
    EncoderDelay   int   `xml:"encoderDelay,attr,omitempty" json:"encoderDelay,omitempty"`
    EncoderPadding int   `xml:"encoderPadding,attr,omitempty" json:"encoderPadding,omitempty"`
    TotalSamples   int64 `xml:"totalSamples,attr,omitempty" json:"totalSamples,omitempty"`
}
```

### 4.4 Web UI Support

For the built-in player:

```javascript
// Pre-buffer next track
function preloadNextTrack(nextSong) {
    const audio = new Audio();
    audio.src = getStreamUrl(nextSong.id);
    audio.preload = 'auto';
    return audio;
}

// Crossfade handling
function handleTrackEnd(currentAudio, nextAudio, gaplessInfo) {
    // Calculate precise transition point
    const transitionTime = currentAudio.duration -
        (gaplessInfo.encoderPadding / gaplessInfo.sampleRate);

    // Start next track early to overlap
    if (currentAudio.currentTime >= transitionTime) {
        nextAudio.play();
        currentAudio.volume = fadeOut(currentAudio);
    }
}
```

---

## 5. Configuration

```toml
[Playback]
GaplessEnabled = true       # Enable gapless metadata extraction
CrossfadeSeconds = 0        # Optional crossfade (0 = disabled)
PreloadSeconds = 5          # How early to start loading next track
```

---

## 6. Implementation Checklist

### Phase 1: Metadata Extraction
- [x] Update TagLib wrapper to extract LAME header (MP3 encoder delay/padding)
- [x] Extract iTunSMPB from M4A/AAC files (encoder delay/padding/total samples)
- [x] Handle FLAC (frame count from stream info)
- [x] Handle Opus, Vorbis, WavPack, AIFF, WAV, DSF (total samples)
- [x] Add database columns for gapless info (encoder_delay, encoder_padding, total_samples)
- [x] Update scanner to populate fields (via metadata mapping)

### Phase 2: API Exposure
- [ ] Add fields to Subsonic `Child` response
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

## 7. Client Compatibility

### Clients That Support Gapless
| Client | Platform | Support Level |
|--------|----------|---------------|
| Navidrome Web UI | Web | Will implement |
| Symfonium | Android | Has gapless support |
| play:Sub | iOS | Has gapless support |
| DSub | Android | Limited |
| Sonos | Speakers | Native |

### What Clients Need
1. Read `encoderDelay`/`encoderPadding` from API
2. Pre-buffer next track
3. Trim audio at boundaries
4. Seamless audio context switching

---

## 8. Testing Plan

### Test Albums
Use albums known for requiring gapless:
- "Dark Side of the Moon" - Pink Floyd (crossfades)
- Any live album (continuous applause)
- Classical symphony (movement transitions)
- DJ mix compilations

### Test Cases
1. Play album start to finish - no gaps
2. Skip track - gapless from skip point
3. Shuffle mode - gapless between random tracks
4. Mixed formats - MP3 → FLAC transition
5. No gapless info - graceful fallback

---

## 9. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Not all files have gapless info | Medium | Graceful fallback to normal playback |
| Client doesn't support | Medium | API is backward compatible |
| Transcoding breaks gapless | High | Preserve timing through transcode |
| Sample rate mismatches | Low | Resample at boundary |

---

## 10. Research Links

- [LAME Gapless Info](https://wiki.hydrogenaud.io/index.php?title=LAME#VBR_header_and_target_bitrate)
- [iTunSMPB Format](https://wiki.hydrogenaud.io/index.php?title=ITunSMPB)
- [Web Audio API Scheduling](https://developer.mozilla.org/en-US/docs/Web/API/Web_Audio_API/Advanced_techniques)
- [Subsonic API Extensions](http://www.subsonic.org/pages/api.jsp)

---

## 11. Open Questions

1. **Transcoding**: How to preserve gapless info when transcoding?
2. **Fallback**: What to do when gapless info missing? Small crossfade?
3. **Client Detection**: Should server adapt response based on client capability?
4. **Performance**: Impact of storing/querying additional metadata?

---

*Last Updated: 2024-12-30*
*Status: Research/Planning*
