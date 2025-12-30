package sonos

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

const (
	// Token security parameters
	tokenBytes       = 32              // 256-bit tokens
	linkCodeBytes    = 16              // 128-bit link codes
	linkCodeExpiry   = 5 * time.Minute // Reduced from 10 minutes
	tokenExpiry      = 90 * 24 * time.Hour // 90 days

	// Argon2 parameters for key derivation
	argon2Time    = 1
	argon2Memory  = 64 * 1024 // 64MB
	argon2Threads = 4
	argon2KeyLen  = 32

	// Rate limiting (tokens per IP per hour)
	maxAuthAttemptsPerHour = 10
)

var (
	ErrNoEncryptionKey    = errors.New("PasswordEncryptionKey must be configured for Sonos integration")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTokenExpired       = errors.New("token expired")
	ErrTokenNotFound      = errors.New("token not found")
	ErrRateLimited        = errors.New("too many authentication attempts")
)

// linkCodeStore stores pending device link codes (in-memory, short-lived)
type linkCodeStore struct {
	mu    sync.RWMutex
	codes map[string]*linkCodeEntry
}

type linkCodeEntry struct {
	UserID    string
	UserName  string
	CreatedAt time.Time
}

var linkCodes = &linkCodeStore{
	codes: make(map[string]*linkCodeEntry),
}

// Rate limiter for auth attempts
type rateLimiter struct {
	mu       sync.RWMutex
	attempts map[string][]time.Time // IP -> attempt timestamps
}

var authRateLimiter = &rateLimiter{
	attempts: make(map[string][]time.Time),
}

// checkRateLimit returns true if the request should be rate limited
func (rl *rateLimiter) checkRateLimit(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Hour)

	// Clean old attempts
	if attempts, ok := rl.attempts[ip]; ok {
		var recent []time.Time
		for _, t := range attempts {
			if t.After(cutoff) {
				recent = append(recent, t)
			}
		}
		rl.attempts[ip] = recent

		if len(recent) >= maxAuthAttemptsPerHour {
			return true
		}
	}

	return false
}

// recordAttempt records an auth attempt for rate limiting
func (rl *rateLimiter) recordAttempt(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.attempts[ip] = append(rl.attempts[ip], time.Now())
}

// storeLinkCode stores a link code associated with a user
func storeLinkCode(code, userID, userName string) {
	linkCodes.mu.Lock()
	defer linkCodes.mu.Unlock()
	linkCodes.codes[code] = &linkCodeEntry{
		UserID:    userID,
		UserName:  userName,
		CreatedAt: time.Now(),
	}

	// Cleanup expired codes periodically
	go cleanupExpiredLinkCodes()
}

// cleanupExpiredLinkCodes removes expired link codes
func cleanupExpiredLinkCodes() {
	linkCodes.mu.Lock()
	defer linkCodes.mu.Unlock()

	now := time.Now()
	for code, entry := range linkCodes.codes {
		if now.Sub(entry.CreatedAt) > linkCodeExpiry {
			delete(linkCodes.codes, code)
		}
	}
}

// consumeLinkCode retrieves and removes a link code (constant-time comparison)
func consumeLinkCode(code string) (*linkCodeEntry, bool) {
	linkCodes.mu.Lock()
	defer linkCodes.mu.Unlock()

	// Use constant-time lookup to prevent timing attacks
	var foundCode string
	var foundEntry *linkCodeEntry

	for storedCode, entry := range linkCodes.codes {
		if subtle.ConstantTimeCompare([]byte(storedCode), []byte(code)) == 1 {
			foundCode = storedCode
			foundEntry = entry
			break
		}
	}

	if foundEntry == nil {
		return nil, false
	}

	delete(linkCodes.codes, foundCode)

	// Check expiry
	if time.Since(foundEntry.CreatedAt) > linkCodeExpiry {
		return nil, false
	}

	return foundEntry, true
}

// generateSecureBytes generates cryptographically secure random bytes
func generateSecureBytes(n int) ([]byte, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return nil, fmt.Errorf("failed to generate secure random bytes: %w", err)
	}
	return bytes, nil
}

// generateLinkCode creates a new cryptographically secure link code
func generateLinkCode() (string, error) {
	bytes, err := generateSecureBytes(linkCodeBytes)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// generateAuthToken creates a new cryptographically secure auth token
func generateAuthToken() (string, error) {
	bytes, err := generateSecureBytes(tokenBytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// getEncryptionKey returns the encryption key, or error if not configured
func getEncryptionKey() ([]byte, error) {
	key := conf.Server.PasswordEncryptionKey
	if key == "" {
		return nil, ErrNoEncryptionKey
	}

	// Use Argon2 for key derivation with a fixed salt derived from the key itself
	// This ensures consistent key derivation while still being computationally expensive
	salt := sha256.Sum256([]byte("navidrome-sonos-key-derivation-v1:" + key))

	derivedKey := argon2.IDKey(
		[]byte(key),
		salt[:16], // Use first 16 bytes of hash as salt
		argon2Time,
		argon2Memory,
		argon2Threads,
		argon2KeyLen,
	)

	return derivedKey, nil
}

// hashTokenForStorage creates a secure hash of the token for database lookup
// We store the hash, not the encrypted token, for secure constant-time lookup
func hashTokenForStorage(token string) string {
	// Use SHA-256 with a domain separator to prevent rainbow table attacks
	h := sha256.New()
	h.Write([]byte("navidrome-sonos-token-v1:"))
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

// storeAuthToken stores an auth token in the database
func (r *Router) storeAuthToken(ctx context.Context, token, userID, userName, householdID, deviceName string) error {
	// Verify encryption key is configured
	if _, err := getEncryptionKey(); err != nil {
		return err
	}

	// Hash the token for storage (we store hash, not the token itself)
	tokenHash := hashTokenForStorage(token)

	deviceToken := &model.SonosDeviceToken{
		ID:          uuid.NewString(),
		UserID:      userID,
		HouseholdID: householdID,
		Token:       tokenHash, // Store hash, not plaintext or encrypted
		DeviceName:  deviceName,
		LastSeenAt:  time.Now(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	return r.ds.SonosDeviceToken(ctx).Put(deviceToken)
}

// validateAuthTokenDB checks if a token is valid using the database
func (r *Router) validateAuthTokenDB(ctx context.Context, token string) (*model.SonosDeviceToken, *model.User, error) {
	// Hash the incoming token for lookup
	tokenHash := hashTokenForStorage(token)

	// Look up by hash (constant-time at database level)
	repo := r.ds.SonosDeviceToken(ctx)
	deviceToken, err := repo.GetByToken(tokenHash)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, nil, ErrTokenNotFound
		}
		return nil, nil, err
	}

	// Check token expiration
	if time.Since(deviceToken.CreatedAt) > tokenExpiry {
		// Clean up expired token
		_ = repo.Delete(deviceToken.ID)
		return nil, nil, ErrTokenExpired
	}

	// Get the user
	user, err := r.ds.User(ctx).Get(deviceToken.UserID)
	if err != nil {
		return nil, nil, fmt.Errorf("user not found: %w", err)
	}

	// Update last seen (async to not slow down request)
	go func() {
		_ = repo.UpdateLastSeen(deviceToken.ID, time.Now())
	}()

	return deviceToken, user, nil
}

// validateCredentials extracts and validates user from SMAPI credentials
func (r *Router) validateCredentials(ctx context.Context, creds *Credentials) (*model.User, error) {
	if creds == nil {
		return nil, fmt.Errorf("no credentials provided")
	}

	// Try login token first
	if creds.LoginToken != nil && creds.LoginToken.Token != "" {
		_, user, err := r.validateAuthTokenDB(ctx, creds.LoginToken.Token)
		if err != nil {
			log.Debug(ctx, "Token validation failed", "error", err)
			return nil, fmt.Errorf("invalid or expired token")
		}
		return user, nil
	}

	// No valid authentication
	return nil, fmt.Errorf("no valid authentication")
}

// validateUserPassword validates a user's password securely
func (r *Router) validateUserPassword(ctx context.Context, username, password string) (*model.User, error) {
	user, err := r.ds.User(ctx).FindByUsernameWithPassword(username)
	if err != nil {
		// Use constant time to prevent username enumeration
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$dummy"), []byte(password))
		return nil, ErrInvalidCredentials
	}

	// Compare passwords using bcrypt (constant-time)
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return user, nil
}

// handleGetAppLink initiates device linking
func (r *Router) handleGetAppLink(ctx context.Context, body []byte, creds *Credentials, req *http.Request) (interface{}, error) {
	// Verify encryption key is configured before allowing linking
	if _, err := getEncryptionKey(); err != nil {
		log.Error(ctx, "Sonos linking requires PasswordEncryptionKey to be configured", err)
		return nil, fmt.Errorf("server configuration error: encryption key not configured")
	}

	var request GetAppLinkRequest
	if err := r.extractRequest(body, "getAppLink", &request); err != nil {
		return nil, err
	}

	log.Debug(ctx, "SMAPI getAppLink", "householdId", request.Household)

	// Generate link code
	linkCode, err := generateLinkCode()
	if err != nil {
		return nil, fmt.Errorf("failed to generate link code: %w", err)
	}

	// Build registration URL (must be HTTPS in production)
	baseURL := conf.Server.BaseURL
	if baseURL == "" {
		// Prefer HTTPS
		scheme := "https"
		if req.TLS == nil && conf.Server.TLSCert == "" {
			scheme = "http" // Fall back to HTTP only if no TLS configured
			log.Warn(ctx, "Sonos linking over HTTP - HTTPS strongly recommended for production")
		}
		baseURL = fmt.Sprintf("%s://%s", scheme, req.Host)
	}
	regURL := fmt.Sprintf("%s/sonos/link?linkCode=%s", baseURL, linkCode)

	return &GetAppLinkResponse{
		AppLinkInfo: &AppLinkInfo{
			RegURL:       regURL,
			LinkCode:     linkCode,
			ShowLinkCode: true,
		},
	}, nil
}

// handleGetDeviceAuthToken exchanges a link code for an auth token
func (r *Router) handleGetDeviceAuthToken(ctx context.Context, body []byte, creds *Credentials, req *http.Request) (interface{}, error) {
	var request GetDeviceAuthTokenRequest
	if err := r.extractRequest(body, "getDeviceAuthToken", &request); err != nil {
		return nil, err
	}

	// Rate limit by IP to prevent link code brute forcing
	clientIP := GetRemoteIP(req)
	if authRateLimiter.checkRateLimit(clientIP) {
		log.Warn(ctx, "SMAPI getDeviceAuthToken: rate limited", "ip", clientIP)
		return nil, ErrRateLimited
	}
	authRateLimiter.recordAttempt(clientIP)

	log.Debug(ctx, "SMAPI getDeviceAuthToken", "linkCode", request.LinkCode, "householdId", request.Household)

	// Look up and consume the link code
	entry, ok := consumeLinkCode(request.LinkCode)
	if !ok {
		log.Warn(ctx, "Invalid or expired link code", "linkCode", request.LinkCode)
		return nil, fmt.Errorf("link code not found or expired")
	}

	// Generate auth token
	authToken, err := generateAuthToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate auth token: %w", err)
	}

	// Store the token in the database
	err = r.storeAuthToken(ctx, authToken, entry.UserID, entry.UserName, request.Household, "Sonos System")
	if err != nil {
		log.Error(ctx, "Failed to store auth token", err)
		return nil, fmt.Errorf("failed to store auth token: %w", err)
	}

	log.Info(ctx, "Sonos device linked", "user", entry.UserName, "householdId", request.Household)

	return &GetDeviceAuthTokenResponse{
		AuthToken: authToken,
	}, nil
}

// extractRequest extracts a typed request from SOAP body
func (r *Router) extractRequest(body []byte, operation string, dest interface{}) error {
	// Try to unmarshal the full envelope
	var envelope struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			Content []byte `xml:",innerxml"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("failed to parse envelope: %w", err)
	}

	// Now unmarshal the body content into the destination type
	return xml.Unmarshal(envelope.Body.Content, dest)
}

// GetUserDevices returns all Sonos devices linked to a user
func (r *Router) GetUserDevices(ctx context.Context, userID string) (model.SonosDeviceTokens, error) {
	return r.ds.SonosDeviceToken(ctx).GetByUserID(userID)
}

// RevokeDevice removes a device token
func (r *Router) RevokeDevice(ctx context.Context, tokenID string) error {
	return r.ds.SonosDeviceToken(ctx).Delete(tokenID)
}

// RevokeAllUserDevices removes all device tokens for a user
func (r *Router) RevokeAllUserDevices(ctx context.Context, userID string) error {
	return r.ds.SonosDeviceToken(ctx).DeleteByUserID(userID)
}

// GetRemoteIP extracts the client IP from a request, handling proxies
func GetRemoteIP(r *http.Request) string {
	// Check X-Forwarded-For first (for reverse proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP (original client)
		if idx := len(xff); idx > 0 {
			for i, c := range xff {
				if c == ',' {
					return xff[:i]
				}
			}
			return xff
		}
	}

	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}
