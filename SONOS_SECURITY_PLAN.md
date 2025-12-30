# Sonos SMAPI Security Hardening Plan

## Overview
This document outlines the security measures for the Native Sonos SMAPI integration in Navidrome.

---

## 1. Token Security

### 1.1 Token Generation
| Aspect | Current | Proposed | Rationale |
|--------|---------|----------|-----------|
| Auth Token Length | 32 bytes (256-bit) | ✅ Keep | Industry standard for session tokens |
| Link Code Length | 8 bytes (64-bit) | 16 bytes (128-bit) | Stronger against brute force |
| RNG Source | `crypto/rand` | ✅ Keep | Cryptographically secure |
| Encoding | Hex | ✅ Keep (auth), Base64URL (link) | URL-safe for link codes |

### 1.2 Token Storage
| Aspect | Current | Proposed | Rationale |
|--------|---------|----------|-----------|
| Storage Method | Encrypted token | **Hash of token** | Hashing is one-way, more secure |
| Hash Algorithm | N/A | SHA-256 with domain separator | Prevents rainbow tables |
| Domain Separator | N/A | `"navidrome-sonos-token-v1:"` | Version allows future upgrades |

**Why hash instead of encrypt?**
- We never need to retrieve the original token
- Hashing is one-way - if DB is compromised, tokens can't be recovered
- Enables constant-time lookup via database index

### 1.3 Token Validation
| Aspect | Current | Proposed | Rationale |
|--------|---------|----------|-----------|
| Lookup Method | Encrypt-then-compare | Hash-then-lookup | AES-GCM nonce makes encrypt non-deterministic |
| Timing Safety | Not addressed | Database index lookup | Constant-time at DB level |

---

## 2. Key Derivation

### 2.1 Encryption Key Management
| Aspect | Current | Proposed | Rationale |
|--------|---------|----------|-----------|
| Key Source | `PasswordEncryptionKey` | ✅ Keep | Reuse existing config |
| Default Key | Hardcoded fallback | **ERROR if not set** | No defaults for security |
| Key Derivation | SHA-256 | **Argon2id** | Memory-hard, brute-force resistant |
| Salt | None | Derived from key + version | Consistent but unique per installation |

### 2.2 Argon2 Parameters (Proposed)
```go
const (
    argon2Time    = 1        // Iterations
    argon2Memory  = 64 * 1024 // 64MB memory
    argon2Threads = 4        // Parallelism
    argon2KeyLen  = 32       // 256-bit output
)
```

**Trade-offs:**
- Higher memory = more brute-force resistant
- Too high = slow startup on low-memory systems
- 64MB is reasonable for server environments

---

## 3. Authentication Flow

### 3.1 Password Validation (Link Page)
| Aspect | Current | Proposed | Rationale |
|--------|---------|----------|-----------|
| Comparison | Direct string `==` | **bcrypt.CompareHashAndPassword** | Constant-time, uses existing hash |
| Username Enumeration | Timing leaks | Dummy bcrypt on user not found | Constant time regardless of user existence |

### 3.2 Link Code Security
| Aspect | Current | Proposed | Rationale |
|--------|---------|----------|-----------|
| Expiry | 10 minutes | **5 minutes** | Shorter window reduces attack surface |
| Lookup | Map iteration | **Constant-time comparison** | Prevents timing attacks |
| Cleanup | On access | **Periodic goroutine** | Remove expired codes proactively |

---

## 4. Rate Limiting

### 4.1 Authentication Endpoints
| Endpoint | Limit | Window | Action |
|----------|-------|--------|--------|
| `/sonos/link` (POST) | 10 attempts | 1 hour | Per IP |
| `/ws/sonos` (getDeviceAuthToken) | 10 attempts | 1 hour | Per link code |

### 4.2 Implementation
- In-memory sliding window counter
- Per-IP tracking (respecting X-Forwarded-For for proxies)
- 429 Too Many Requests response when limited

---

## 5. Token Expiration

### 5.1 Token Lifecycle
| Token Type | Lifetime | Renewal | Rationale |
|------------|----------|---------|-----------|
| Link Code | 5 minutes | None | Short-lived by design |
| Auth Token | 90 days | On use (extend) | Balance UX vs security |

### 5.2 Expiration Handling
- Check `created_at` on every validation
- Auto-delete expired tokens on access
- Optional: Background job to clean old tokens

---

## 6. Transport Security

### 6.1 HTTPS Requirements
| Scenario | Behavior | Rationale |
|----------|----------|-----------|
| TLS configured | Use HTTPS | Secure by default |
| No TLS, BaseURL set | Use BaseURL scheme | User configured |
| No TLS, no BaseURL | HTTP with **warning log** | Development only |

### 6.2 Sonos Requirements
- Sonos devices require HTTPS for production service registration
- Self-signed certificates may work for local testing
- Recommend Traefik/reverse proxy with Let's Encrypt

---

## 7. Input Validation & XSS Prevention

### 7.1 Output Encoding
| Output | Method | Applied To |
|--------|--------|------------|
| HTML pages | `html.EscapeString()` | Username, service name |
| JSON responses | `json.Encoder` | All API responses |
| XML/SOAP | `xml.Encoder` | All SMAPI responses |

### 7.2 Input Validation
- Link codes: Alphanumeric only
- Household IDs: UUID format validation
- Usernames: Existing Navidrome validation

---

## 8. Logging & Audit

### 8.1 Security Events to Log
| Event | Level | Data Logged |
|-------|-------|-------------|
| Successful link | INFO | Username, household ID, IP |
| Failed auth | WARN | Username (not password), IP |
| Rate limited | WARN | IP, endpoint |
| Token expired | DEBUG | Token ID (not token value) |
| Device revoked | INFO | Token ID, user, IP |

### 8.2 Sensitive Data Handling
- **NEVER log:** Passwords, tokens, link codes
- **Redact in logs:** Full IPs in production (last octet)

---

## 9. Configuration Requirements

### 9.1 Required Settings
```toml
# REQUIRED for Sonos integration
PasswordEncryptionKey = "your-secure-random-key-here"

[Sonos]
Enabled = true
```

### 9.2 Startup Validation
- If `Sonos.Enabled = true` and `PasswordEncryptionKey = ""`:
  - Log ERROR
  - Refuse to start Sonos endpoints
  - Return clear error to users attempting to link

---

## 10. Attack Vectors & Mitigations

| Attack | Mitigation |
|--------|------------|
| **Brute force tokens** | 256-bit tokens (2^256 combinations) |
| **Brute force link codes** | 128-bit + 5min expiry + rate limiting |
| **Timing attacks on auth** | bcrypt + constant-time comparison |
| **Token theft from DB** | Hashed storage (can't recover original) |
| **Session hijacking** | HTTPS required for production |
| **XSS** | HTML escaping on all outputs |
| **Username enumeration** | Constant-time regardless of user existence |
| **CSRF on link page** | Same-origin + link code validation |

---

## 11. Implementation Checklist

### Phase 1: Core Security (Must Have)
- [ ] Remove default encryption key fallback
- [ ] Implement Argon2id key derivation
- [ ] Store token hash instead of encrypted token
- [ ] Use bcrypt for password validation
- [ ] Add rate limiting to auth endpoints
- [ ] Implement token expiration (90 days)
- [ ] Add constant-time link code comparison
- [ ] HTML escape all outputs

### Phase 2: Hardening (Should Have)
- [ ] Reduce link code expiry to 5 minutes
- [ ] Increase link code to 128-bit
- [ ] Add security event logging
- [ ] Implement IP-based rate limiting
- [ ] Add startup validation for required config

### Phase 3: Operational (Nice to Have)
- [ ] Background job to clean expired tokens
- [ ] Admin UI to view/revoke devices
- [ ] Token activity monitoring
- [ ] Rate limit metrics for Prometheus

---

## 12. Testing Plan

### Security Tests
1. **Token entropy test**: Verify randomness distribution
2. **Timing attack test**: Measure response times for valid vs invalid
3. **Rate limit test**: Verify 429 after threshold
4. **Expiration test**: Verify tokens expire correctly
5. **Hash collision test**: Verify no false positives

### Integration Tests
1. Full linking flow with valid credentials
2. Linking flow with invalid credentials
3. Token validation after server restart
4. Device revocation and re-linking

---

## 13. Dependencies Added

```go
import (
    "golang.org/x/crypto/argon2"  // Key derivation
    "golang.org/x/crypto/bcrypt"  // Password comparison
    "crypto/subtle"               // Constant-time comparison
)
```

These are standard Go crypto libraries, already used elsewhere in Navidrome.

---

## 14. Rollback Plan

If issues arise:
1. Keep old auth.go backup
2. Migration adds columns, doesn't remove
3. Token hash format versioned (`v1:`) for future changes
4. Config flags to enable/disable new security features during transition

---

## Questions for Review

1. **Token expiry**: 90 days appropriate? Could be configurable.
2. **Rate limits**: 10/hour too strict or lenient?
3. **Argon2 memory**: 64MB ok for all deployments?
4. **HTTPS enforcement**: Warn only or block in production?

---

*Created: 2024-12-30*
*Status: DRAFT - Pending Review*
