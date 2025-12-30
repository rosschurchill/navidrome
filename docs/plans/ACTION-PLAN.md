# Action Plan: Implementation & Testing

> **Created**: 2024-12-30
> **Purpose**: Consolidate all handovers into a prioritized action plan

---

## Current State Summary

| Feature | Phase 1 | Phase 2 | Phase 3 | Phase 4 | Deployed |
|---------|---------|---------|---------|---------|----------|
| **Sonos Security** | ✅ | Testing | - | - | ✅ Port 4535 |
| **DLNA/UPnP** | ✅ | ✅ | Streaming | Polish | ⚠️ Config needed |
| **Gapless Playback** | ✅ | API | Web Player | Testing | ⚠️ Rescan needed |
| **Audio Fingerprinting** | ✅ | DB/Cache | API | UI | ⚠️ Config needed |

---

## Immediate Actions (Test Container)

### 1. Enable All Features in Test Container

```bash
# Stop current container
ssh ross@tvsp01 "/share/ZFS530_DATA/.qpkg/container-station/bin/docker stop navidrome-test"

# Recreate with all features enabled
ssh ross@tvsp01 "/share/ZFS530_DATA/.qpkg/container-station/bin/docker rm navidrome-test"

ssh ross@tvsp01 "/share/ZFS530_DATA/.qpkg/container-station/bin/docker run -d \
  --name navidrome-test \
  -p 4535:4533 \
  -v /share/Music:/music:ro \
  -v /share/Container/navidrome-test/data:/data \
  -e ND_LOGLEVEL=debug \
  -e ND_PASSWORDENCRYPTIONKEY='test-encryption-key-32chars!!' \
  -e ND_SONOS_ENABLED=true \
  -e ND_SONOS_SERVICENAME='Navidrome Test' \
  -e ND_DLNA_ENABLED=true \
  -e ND_DLNA_SERVERNAME='Navidrome Test' \
  -e ND_FINGERPRINT_ENABLED=true \
  -e ND_FINGERPRINT_ACOUSTIDAPIKEY='YOUR_ACOUSTID_KEY' \
  navidrome-fork:latest"
```

### 2. Trigger Library Rescan
Required for gapless metadata to be populated:
```bash
# Via web UI or API
curl -X POST "http://192.168.1.200:4535/api/scan" -H "Authorization: Bearer TOKEN"
```

---

## Testing Checklist

### Sonos SMAPI (Phase 2 Testing)
| Test | Command/Method | Expected |
|------|----------------|----------|
| Strings endpoint | `curl http://192.168.1.200:4535/sonos/strings.xml` | XML with service name |
| Link page | `curl http://192.168.1.200:4535/sonos/link` | HTML login form |
| WSDL | `curl http://192.168.1.200:4535/sonos/ws/sonos` | WSDL XML |
| Rate limiting | 11 failed POSTs to `/sonos/link` | 429 on 11th attempt |
| Invalid token | SOAP call with bad token | Auth error |
| **Actual Sonos device** | Add service in Sonos app | Device appears, can browse |

### DLNA/UPnP (Verification)
| Test | Method | Expected |
|------|--------|----------|
| SSDP discovery | Use DLNA client (VLC, BubbleUPnP) | Server appears in device list |
| Device description | `curl http://192.168.1.200:4535/dlna/device.xml` | UPnP device XML |
| Browse root | SOAP Browse ObjectID="0" | Root containers (Music) |
| Browse artists | SOAP Browse ObjectID="music/artists" | Artist list |
| Play track | Select track in DLNA client | Audio streams |
| Album art | Check if album art displays | Art visible |

### Gapless Playback (Metadata Verification)
| Test | Command | Expected |
|------|---------|----------|
| Check DB after rescan | `SELECT id, title, total_samples FROM media_file WHERE total_samples > 0 LIMIT 5;` | Sample counts populated |
| FLAC files | Query FLAC tracks | total_samples from stream info |
| M4A files | Query M4A tracks | encoder_delay, encoder_padding from iTunSMPB |
| MP3 files | Query MP3 tracks | total_samples calculated |

### Audio Fingerprinting (Manual Testing)
| Test | Method | Expected |
|------|--------|----------|
| fpcalc available | `docker exec navidrome-test fpcalc -version` | Version output |
| Config loaded | Check logs for fingerprint config | Config values logged |
| Generate fingerprint | Via Go code/test | Fingerprint string returned |

---

## Phase 2 Implementation Tasks

### Priority 1: Complete API Exposure

#### Gapless API (2-3 hours)
- [ ] Add `encoderDelay`, `encoderPadding`, `totalSamples` to Subsonic `Child` response
  - File: `server/subsonic/responses/responses.go`
- [ ] Add fields to native API song response
  - File: `server/nativeapi/media_files.go`
- [ ] Test with Subsonic client (DSub, Symfonium)

#### Fingerprinting Database (2-3 hours)
- [ ] Create migration `db/migrations/20251230140000_create_fingerprint_cache.go`
- [ ] Create model `model/fingerprint_cache.go`
- [ ] Create repository `persistence/fingerprint_cache_repository.go`
- [ ] Add cache layer to fingerprint service

### Priority 2: DLNA Polish

#### Search Implementation (2-3 hours)
- [ ] Implement Search action in ContentDirectory
- [ ] Support basic search criteria parsing

#### Icons (1 hour)
- [ ] Add actual icon resources
- [ ] Serve from `/dlna/icon/{size}.png`

### Priority 3: Fingerprinting API

#### REST Endpoints (2-3 hours)
- [ ] `POST /api/fingerprint/{id}` - Single track identification
- [ ] Wire into nativeapi router
- [ ] Test with curl

---

## Phase 3 Implementation Tasks

### Gapless Web Player (Complex - 4-6 hours)
- [ ] Track preloading in player
- [ ] Calculate transition points from gapless info
- [ ] Web Audio API scheduling for seamless transitions
- [ ] Optional crossfade settings

### DLNA Streaming Enhancements
- [ ] Multiple `<res>` elements per track
- [ ] Transcoding profile per device
- [ ] LPCM fallback

### Fingerprinting UI
- [ ] "Identify" context menu action
- [ ] Metadata suggestions dialog
- [ ] Accept/reject workflow

---

## Testing Devices

### Sonos
- [ ] Sonos speaker/soundbar for actual linking test
- [ ] Sonos S2 app for service registration

### DLNA
- [ ] VLC (any platform)
- [ ] BubbleUPnP (Android)
- [ ] Smart TV (Samsung/LG if available)

### Gapless
- [ ] Web browser (Chrome, Firefox)
- [ ] Symfonium (if it supports gapless with API data)

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Sonos requires HTTPS | Use reverse proxy (Traefik) with Let's Encrypt |
| DLNA no auth | Accept as limitation, document in user docs |
| fpcalc not in container | Add chromaprint to Dockerfile |
| Gapless web player complex | Start with basic preloading, iterate |

---

## Success Criteria

### MVP (Minimum Viable)
- [ ] Sonos can link and browse library
- [ ] DLNA devices discover and stream
- [ ] Gapless metadata in database
- [ ] Fingerprint generates and looks up

### Full Feature
- [ ] Sonos full playback, search, ratings
- [ ] DLNA search, transcoding
- [ ] Gapless seamless web playback
- [ ] Fingerprint batch processing, UI

---

## Session Assignments (if parallelizing)

| Task | Complexity | Suggested Session |
|------|------------|-------------------|
| Gapless API exposure | Low | Any |
| Fingerprinting DB/cache | Medium | Session C continues |
| DLNA Search | Medium | Session B continues |
| Gapless Web Player | High | Dedicated session |
| Fingerprinting UI | Medium | After API complete |

---

*Created: 2024-12-30*
*Status: Ready for Implementation*
