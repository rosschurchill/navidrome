package fingerprint

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/navidrome/navidrome/log"
	"golang.org/x/time/rate"
)

const (
	// AcoustID API endpoint
	acoustIDURL = "https://api.acoustid.org/v2/lookup"

	// AcoustID rate limit: 3 requests per second
	acoustIDRateLimit = 3
	acoustIDBurst     = 1

	// HTTP timeout for AcoustID requests
	acoustIDTimeout = 10 * time.Second
)

// AcoustIDClient provides access to the AcoustID fingerprint lookup service
type AcoustIDClient struct {
	apiKey     string
	httpClient *http.Client
	limiter    *rate.Limiter
}

// AcoustIDResponse represents the response from AcoustID API
type AcoustIDResponse struct {
	Status  string            `json:"status"`
	Results []AcoustIDResult  `json:"results,omitempty"`
	Error   *AcoustIDAPIError `json:"error,omitempty"`
}

// AcoustIDResult represents a single result from AcoustID lookup
type AcoustIDResult struct {
	ID         string              `json:"id"`
	Score      float64             `json:"score"`
	Recordings []AcoustIDRecording `json:"recordings,omitempty"`
}

// AcoustIDRecording represents a recording match from AcoustID
type AcoustIDRecording struct {
	ID      string            `json:"id"` // MusicBrainz Recording ID
	Title   string            `json:"title,omitempty"`
	Artists []AcoustIDArtist  `json:"artists,omitempty"`
	Releases []AcoustIDRelease `json:"releasegroups,omitempty"`
}

// AcoustIDArtist represents an artist in AcoustID response
type AcoustIDArtist struct {
	ID   string `json:"id"` // MusicBrainz Artist ID
	Name string `json:"name"`
}

// AcoustIDRelease represents a release in AcoustID response
type AcoustIDRelease struct {
	ID    string `json:"id"`
	Title string `json:"title,omitempty"`
	Type  string `json:"type,omitempty"`
}

// AcoustIDAPIError represents an error from the AcoustID API
type AcoustIDAPIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *AcoustIDAPIError) Error() string {
	return fmt.Sprintf("acoustid error(%d): %s", e.Code, e.Message)
}

// NewAcoustIDClient creates a new AcoustID API client
func NewAcoustIDClient(apiKey string) *AcoustIDClient {
	return &AcoustIDClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: acoustIDTimeout,
		},
		// Rate limit: 3 requests per second
		limiter: rate.NewLimiter(rate.Every(time.Second/acoustIDRateLimit), acoustIDBurst),
	}
}

// IsConfigured returns true if the client has an API key configured
func (c *AcoustIDClient) IsConfigured() bool {
	return c.apiKey != ""
}

// Lookup queries AcoustID for recordings matching the given fingerprint
func (c *AcoustIDClient) Lookup(ctx context.Context, fingerprint string, duration int) (*AcoustIDResponse, error) {
	if !c.IsConfigured() {
		return nil, fmt.Errorf("acoustid API key not configured")
	}

	// Wait for rate limiter
	if err := c.limiter.Wait(ctx); err != nil {
		if ctx.Err() == context.DeadlineExceeded || ctx.Err() == context.Canceled {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: %s", ErrRateLimited, err)
	}

	// Build request URL
	params := url.Values{
		"client":      {c.apiKey},
		"fingerprint": {fingerprint},
		"duration":    {strconv.Itoa(duration)},
		"meta":        {"recordings releasegroups"},
	}

	requestURL := acoustIDURL + "?" + params.Encode()

	log.Debug(ctx, "AcoustID lookup", "duration", duration, "fingerprintLen", len(fingerprint))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Navidrome/1.0 (https://navidrome.org)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("acoustid request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("acoustid returned status %d", resp.StatusCode)
	}

	var response AcoustIDResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode acoustid response: %w", err)
	}

	if response.Status != "ok" {
		if response.Error != nil {
			return nil, response.Error
		}
		return nil, fmt.Errorf("acoustid returned status: %s", response.Status)
	}

	log.Debug(ctx, "AcoustID lookup complete", "results", len(response.Results))

	return &response, nil
}
