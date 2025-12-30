# Handover: Sonos SMAPI Security Hardening

> **Session**: Sonos Security (Session A)
> **Date**: 2024-12-30
> **Status**: Phase 1 Complete, Deployed to Test Container

---

## What Was Done

### Phase 1 Security Hardening - COMPLETE

All items from `02-SONOS-SMAPI.md` Section 3 implemented:

| Item | Status | Implementation |
|------|--------|----------------|
| Token hashing | ✅ | SHA-256 with domain separator `navidrome-sonos-token-v1:` |
| bcrypt password validation | ✅ | Constant-time comparison + dummy hash on user not found |
| Rate limiting | ✅ | 10 attempts/hour per IP on `/link` POST and `getDeviceAuthToken` |
| Token expiration | ✅ | 90-day expiry, auto-delete on validation |
| Encryption key requirement | ✅ | Errors if `PasswordEncryptionKey` not set |
| Argon2id key derivation | ✅ | 64MB memory, 4 threads, 32-byte output |
| XSS prevention | ✅ | `html.EscapeString` on serviceName, linkCode, username, baseURL |

### Files Modified

| File | Changes |
|------|---------|
| `server/sonos/smapi.go` | Added `html` import, XSS escaping in 4 handlers, rate limiting on getDeviceAuthToken |
| `server/sonos/auth.go` | Added `*http.Request` param to `handleGetDeviceAuthToken` for IP-based rate limiting |

### Build Fixes (From Other Sessions)

Fixed issues from parallel sessions to get a working build:

| Issue | File | Fix |
|-------|------|-----|
| TagLib API unavailable | `adapters/taglib/taglib_wrapper.cpp` | Removed `encoderDelay()`/`encoderPadding()` calls, using sample count fallback |
| Unused import | `server/dlna/content_directory.go` | Removed `conf`, added `squirrel` |
| Missing method | `server/dlna/dlna.go` | Added `getAlbumArtURL()` |
| Wrong filter type | `server/dlna/content_directory.go` | Changed `map[string]interface{}` to `squirrel.Eq{}` |

---

## Deployment

### Test Container Created
```
Name: navidrome-test
URL: http://192.168.1.200:4535
Image: navidrome-fork:latest
Sonos Enabled: Yes ("Navidrome Test")
Log Level: debug
```

### Verified Working
- Web UI: ✅
- `/sonos/strings.xml`: ✅
- `/sonos/link`: ✅
- `/sonos/ws/sonos` (WSDL): ✅

---

## What's Next

### Phase 2: Testing (from `02-SONOS-SMAPI.md`)
- [ ] Test full linking flow with actual Sonos device
- [ ] Test token expiration (modify `created_at`, verify rejection)
- [ ] Test rate limiting (11 failed attempts, verify 429)
- [ ] Test device revocation via API

### Phase 3: Documentation
- [ ] Update CLAUDE.md with final implementation
- [ ] Add user-facing docs for Sonos setup
- [ ] Document HTTPS requirements

---

## Key Code Locations

| Purpose | File | Lines |
|---------|------|-------|
| Token hashing | `server/sonos/auth.go` | 217-225 |
| bcrypt validation | `server/sonos/auth.go` | 308-322 |
| Rate limiter | `server/sonos/auth.go` | 67-107 |
| Rate limit check (link) | `server/sonos/smapi.go` | 276-288 |
| Rate limit check (token) | `server/sonos/auth.go` | 374-380 |
| XSS escaping | `server/sonos/smapi.go` | 202, 232-233, 256-257, 314 |
| Argon2id key derivation | `server/sonos/auth.go` | 194-215 |

---

## Testing Commands

```bash
# View container logs
ssh ross@tvsp01 "/share/ZFS530_DATA/.qpkg/container-station/bin/docker logs navidrome-test --tail 50"

# Restart container
ssh ross@tvsp01 "/share/ZFS530_DATA/.qpkg/container-station/bin/docker restart navidrome-test"

# Test Sonos strings
curl http://192.168.1.200:4535/sonos/strings.xml

# Test link page
curl http://192.168.1.200:4535/sonos/link
```

---

## Notes for Next Session

1. **Rate limiting is IP-based** - If testing from same IP, you'll hit the limit after 10 attempts. Rate limiter resets after 1 hour.

2. **Token hashing is one-way** - Can't recover tokens from DB. To "see" a token, you'd need to intercept the `getDeviceAuthToken` response.

3. **Sonos requires HTTPS for production** - Current test is HTTP. For real Sonos linking, need reverse proxy with TLS.

4. **PasswordEncryptionKey required** - If not set, Sonos endpoints will error. Set via `ND_PASSWORDENCRYPTIONKEY` env var.

---

*Written by: Sonos Security Session (Session A)*
*Date: 2024-12-30*
