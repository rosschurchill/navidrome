# Feature Plan: Native Sonos SMAPI Integration

> **Status**: 🔄 In Progress
> **Priority**: High
> **Complexity**: High
> **Dependencies**: None (replaces external bonob service)

---

## 1. Overview

### What Is This?
Native implementation of Sonos Music API (SMAPI) in Navidrome, allowing Sonos speakers to stream directly without the external [bonob](https://github.com/simojenki/bonob) service.

### Why Do We Need It?
- **Eliminate dependency** on external bonob service
- **Better integration** with Navidrome's auth system
- **Lossless streaming** support (FLAC, ALAC, WAV)
- **Full control** over features and updates

### What Is SMAPI?
- SOAP 1.1-based web services API
- Sonos devices call your server to browse/search/stream music
- Requires device "linking" (OAuth-like flow)
- XML request/response format

---

## 2. Current Implementation Status

### Completed ✅
| Component | File(s) | Notes |
|-----------|---------|-------|
| SOAP Router | `server/sonos/smapi.go` | Handles all SMAPI operations |
| Type Definitions | `server/sonos/types.go` | Request/response structs |
| Content Browsing | `server/sonos/browsing.go` | Artists, albums, tracks, playlists, genres |
| Search | `server/sonos/search.go` | Search across all content types |
| Device Discovery | `server/sonos/discovery.go` | SSDP-based discovery |
| Basic Auth | `server/sonos/auth.go` | Link codes, tokens (needs hardening) |
| Config Options | `conf/configuration.go` | `Sonos.Enabled`, `ServiceName`, `AutoRegister` |
| DB Migration | `db/migrations/20251230*` | `sonos_device_token` table |
| Device Management API | `server/nativeapi/sonos_devices.go` | List/revoke devices |

### Needs Work 🔄
| Component | Issue | Plan Section |
|-----------|-------|--------------|
| Token Security | Encryption approach flawed | §3.1 |
| Password Validation | Direct comparison (timing attack) | §3.2 |
| Rate Limiting | Not implemented | §3.3 |
| Token Expiration | Not enforced | §3.4 |
| HTTPS Enforcement | Only warns | §3.5 |

---

## 3. Security Hardening Plan

### 3.1 Token Storage (CRITICAL)

**Problem**: Current implementation encrypts tokens with AES-GCM, but GCM uses random nonces, making the same token encrypt differently each time. Can't do lookups.

**Solution**: Store a **hash** of the token, not the encrypted token.

```go
// Hash token for storage (one-way, secure lookup)
func hashTokenForStorage(token string) string {
    h := sha256.New()
    h.Write([]byte("navidrome-sonos-token-v1:"))  // Domain separator
    h.Write([]byte(token))
    return hex.EncodeToString(h.Sum(nil))
}
```

**Why This Is Better**:
- One-way: If DB is compromised, can't recover tokens
- Deterministic: Same token → same hash → can do DB lookup
- Domain separator prevents rainbow table attacks

### 3.2 Password Validation (CRITICAL)

**Problem**: Direct string comparison leaks timing information.

```go
// BAD - timing attack vulnerable
if user.Password != password { ... }
```

**Solution**: Use bcrypt's constant-time comparison.

```go
// GOOD - constant time
func (r *Router) validateUserPassword(ctx context.Context, username, password string) (*model.User, error) {
    user, err := r.ds.User(ctx).FindByUsernameWithPassword(username)
    if err != nil {
        // Prevent username enumeration - do dummy bcrypt
        _ = bcrypt.CompareHashAndPassword([]byte("$2a$10$dummy"), []byte(password))
        return nil, ErrInvalidCredentials
    }

    if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
        return nil, ErrInvalidCredentials
    }
    return user, nil
}
```

### 3.3 Rate Limiting (HIGH)

**Implementation**:
```go
const maxAuthAttemptsPerHour = 10

type rateLimiter struct {
    mu       sync.RWMutex
    attempts map[string][]time.Time  // IP -> timestamps
}

func (rl *rateLimiter) checkRateLimit(ip string) bool {
    // Sliding window: count attempts in last hour
    // Return true if >= maxAuthAttemptsPerHour
}
```

**Apply To**:
- `POST /sonos/link` (login form)
- `getDeviceAuthToken` SOAP call

### 3.4 Token Expiration (HIGH)

**Policy**:
| Token Type | Lifetime | Action on Expiry |
|------------|----------|------------------|
| Link Code | 5 minutes | Auto-delete from memory |
| Auth Token | 90 days | Delete from DB, require re-link |

**Implementation**:
```go
const tokenExpiry = 90 * 24 * time.Hour

func (r *Router) validateAuthTokenDB(ctx context.Context, token string) (...) {
    // ... lookup token ...

    if time.Since(deviceToken.CreatedAt) > tokenExpiry {
        _ = repo.Delete(deviceToken.ID)  // Clean up
        return nil, nil, ErrTokenExpired
    }
    // ...
}
```

### 3.5 HTTPS Enforcement (MEDIUM)

**Current**: Logs warning if HTTP.

**Options**:
1. **Warn only** (current) - Development friendly
2. **Block linking over HTTP** - More secure
3. **Configurable** - `Sonos.RequireHTTPS` option

**Recommendation**: Option 3 - default to warn, allow strict mode.

### 3.6 Key Derivation (MEDIUM)

**Problem**: Simple SHA-256 of `PasswordEncryptionKey` is weak.

**Solution**: Argon2id key derivation.

```go
func getEncryptionKey() ([]byte, error) {
    key := conf.Server.PasswordEncryptionKey
    if key == "" {
        return nil, ErrNoEncryptionKey  // NO DEFAULTS
    }

    // Argon2id: memory-hard, brute-force resistant
    salt := sha256.Sum256([]byte("navidrome-sonos-key-derivation-v1:" + key))
    return argon2.IDKey([]byte(key), salt[:16], 1, 64*1024, 4, 32), nil
}
```

### 3.7 XSS Prevention (LOW)

**All HTML outputs must use**:
```go
html.EscapeString(userProvidedValue)
```

**Applied to**:
- Service name in link page
- Username in success page

---

## 4. Configuration

### 4.1 Config Options
```toml
[Sonos]
Enabled = false          # Must explicitly enable
ServiceName = "Navidrome" # Shown in Sonos app
AutoRegister = false     # SSDP auto-discovery (future)
# RequireHTTPS = false   # Future: enforce HTTPS for linking
```

### 4.2 Environment Variables
```bash
ND_SONOS_ENABLED=true
ND_SONOS_SERVICENAME="My Music Server"
```

### 4.3 Required Settings
```toml
# REQUIRED when Sonos.Enabled = true
PasswordEncryptionKey = "your-32-char-random-string"
```

---

## 5. API Reference

### 5.1 SMAPI Endpoints (SOAP)
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/sonos/ws/sonos` | POST | All SMAPI SOAP calls |
| `/sonos/ws/sonos` | GET | WSDL file |
| `/sonos/strings.xml` | GET | Localization strings |
| `/sonos/presentationMap.xml` | GET | UI customization |

### 5.2 Device Linking Endpoints (HTTP)
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/sonos/link` | GET | Show login form |
| `/sonos/link` | POST | Process login, store link code |

### 5.3 Management API (REST)
| Endpoint | Method | Purpose | Auth |
|----------|--------|---------|------|
| `/api/sonos/devices` | GET | List user's linked devices | User |
| `/api/sonos/devices/{id}` | DELETE | Revoke device | User/Admin |

---

## 6. Database Schema

### Table: `sonos_device_token`
```sql
CREATE TABLE sonos_device_token (
    id            VARCHAR(255) PRIMARY KEY,
    user_id       VARCHAR(255) NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    household_id  VARCHAR(255) NOT NULL,
    token         VARCHAR(255) NOT NULL UNIQUE,  -- SHA-256 hash of token
    device_name   VARCHAR(255) DEFAULT '',
    last_seen_at  DATETIME,
    created_at    DATETIME NOT NULL,
    updated_at    DATETIME NOT NULL
);

CREATE INDEX idx_sonos_token_user ON sonos_device_token(user_id);
CREATE INDEX idx_sonos_token_household ON sonos_device_token(household_id);
```

---

## 7. Implementation Checklist

### Phase 1: Security Hardening (Do First) ✅ COMPLETE
- [x] Replace token encryption with hashing (SHA-256 with domain separator)
- [x] Implement bcrypt password validation (constant-time, anti-enumeration)
- [x] Add rate limiting to auth endpoints (handleLink + getDeviceAuthToken)
- [x] Implement token expiration (90 days)
- [x] Remove default encryption key fallback (errors if not set)
- [x] Add Argon2id key derivation (64MB memory, 4 threads)
- [x] HTML escape all outputs (serviceName, linkCode, username, baseURL)

### Phase 2: Testing
- [ ] Test full linking flow
- [ ] Test token expiration
- [ ] Test rate limiting
- [ ] Test device revocation
- [ ] Test streaming (lossy and lossless)

### Phase 3: Documentation
- [ ] Update CLAUDE.md with final implementation
- [ ] Add user-facing docs for Sonos setup
- [ ] Document HTTPS requirements

---

## 8. Testing Plan

### Manual Tests
1. **Link Flow**: Enable Sonos, try linking from Sonos app
2. **Browse**: Navigate artists → albums → tracks
3. **Search**: Search for known content
4. **Play**: Play track, verify audio quality
5. **Revoke**: Revoke device, verify can't stream

### Security Tests
1. **Rate Limit**: Try 11 failed logins, verify 429
2. **Token Expiry**: Modify created_at, verify rejection
3. **Bad Token**: Send random token, verify rejection
4. **Timing**: Measure response time for valid vs invalid user

---

## 9. Open Questions

1. **Token Lifetime**: Is 90 days appropriate? Make configurable?
2. **Rate Limit Threshold**: 10/hour too strict for shared IPs?
3. **HTTPS Policy**: Warn vs block vs configurable?
4. **Household Sharing**: Allow multiple users per household?

---

## 10. Files Reference

| Purpose | File |
|---------|------|
| SOAP Router | `server/sonos/smapi.go` |
| Types | `server/sonos/types.go` |
| Auth & Security | `server/sonos/auth.go` |
| Browsing | `server/sonos/browsing.go` |
| Search | `server/sonos/search.go` |
| Discovery | `server/sonos/discovery.go` |
| Config | `conf/configuration.go` (sonosOptions) |
| Migration | `db/migrations/20251230120000_create_sonos_device_token.go` |
| Model | `model/sonos_device_token.go` |
| Repository | `persistence/sonos_device_token_repository.go` |
| Management API | `server/nativeapi/sonos_devices.go` |

---

*Last Updated: 2024-12-30*
