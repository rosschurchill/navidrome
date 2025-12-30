package fingerprint

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/navidrome/navidrome/log"
	"golang.org/x/time/rate"
)

const (
	// MusicBrainz API endpoint
	musicBrainzURL = "https://musicbrainz.org/ws/2"

	// MusicBrainz rate limit: 1 request per second (be conservative)
	musicBrainzRateLimit = 1
	musicBrainzBurst     = 1

	// HTTP timeout for MusicBrainz requests
	musicBrainzTimeout = 10 * time.Second

	// User agent is required by MusicBrainz
	musicBrainzUserAgent = "Navidrome/1.0 (https://navidrome.org)"
)

// MusicBrainzClient provides access to the MusicBrainz metadata service
type MusicBrainzClient struct {
	httpClient *http.Client
	limiter    *rate.Limiter
}

// MBRecording represents a recording from MusicBrainz
type MBRecording struct {
	ID           string           `json:"id"`
	Title        string           `json:"title"`
	Length       int              `json:"length,omitempty"` // in milliseconds
	ArtistCredit []MBArtistCredit `json:"artist-credit,omitempty"`
	Releases     []MBRelease      `json:"releases,omitempty"`
	Tags         []MBTag          `json:"tags,omitempty"`
}

// MBArtistCredit represents an artist credit in MusicBrainz
type MBArtistCredit struct {
	Name    string   `json:"name"`
	JoinPhrase string `json:"joinphrase,omitempty"`
	Artist  MBArtist `json:"artist"`
}

// MBArtist represents an artist in MusicBrainz
type MBArtist struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	SortName       string `json:"sort-name,omitempty"`
	Disambiguation string `json:"disambiguation,omitempty"`
}

// MBRelease represents a release in MusicBrainz
type MBRelease struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Status      string         `json:"status,omitempty"`
	Date        string         `json:"date,omitempty"`
	Country     string         `json:"country,omitempty"`
	ReleaseGroup *MBReleaseGroup `json:"release-group,omitempty"`
}

// MBReleaseGroup represents a release group in MusicBrainz
type MBReleaseGroup struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	PrimaryType string `json:"primary-type,omitempty"`
}

// MBTag represents a tag in MusicBrainz
type MBTag struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// MBError represents an error from MusicBrainz API
type MBError struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

func (e *MBError) String() string {
	if e.Message != "" {
		return fmt.Sprintf("musicbrainz error: %s - %s", e.Error, e.Message)
	}
	return fmt.Sprintf("musicbrainz error: %s", e.Error)
}

// NewMusicBrainzClient creates a new MusicBrainz API client
func NewMusicBrainzClient() *MusicBrainzClient {
	return &MusicBrainzClient{
		httpClient: &http.Client{
			Timeout: musicBrainzTimeout,
		},
		// Rate limit: 1 request per second (slightly slower to be safe)
		limiter: rate.NewLimiter(rate.Every(1100*time.Millisecond), musicBrainzBurst),
	}
}

// GetRecording fetches detailed recording information from MusicBrainz
func (c *MusicBrainzClient) GetRecording(ctx context.Context, mbid string) (*MBRecording, error) {
	// Wait for rate limiter
	if err := c.limiter.Wait(ctx); err != nil {
		if ctx.Err() == context.DeadlineExceeded || ctx.Err() == context.Canceled {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: %s", ErrRateLimited, err)
	}

	// Build request URL with includes
	requestURL := fmt.Sprintf("%s/recording/%s?fmt=json&inc=artists+releases+release-groups+tags",
		musicBrainzURL, mbid)

	log.Debug(ctx, "MusicBrainz lookup", "mbid", mbid)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", musicBrainzUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("musicbrainz request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNoMatch
	}

	if resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("musicbrainz returned status %d", resp.StatusCode)
	}

	var recording MBRecording
	if err := json.NewDecoder(resp.Body).Decode(&recording); err != nil {
		return nil, fmt.Errorf("failed to decode musicbrainz response: %w", err)
	}

	log.Debug(ctx, "MusicBrainz lookup complete",
		"mbid", mbid,
		"title", recording.Title,
		"releases", len(recording.Releases))

	return &recording, nil
}

// GetArtist fetches detailed artist information from MusicBrainz
func (c *MusicBrainzClient) GetArtist(ctx context.Context, mbid string) (*MBArtist, error) {
	// Wait for rate limiter
	if err := c.limiter.Wait(ctx); err != nil {
		if ctx.Err() == context.DeadlineExceeded || ctx.Err() == context.Canceled {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: %s", ErrRateLimited, err)
	}

	requestURL := fmt.Sprintf("%s/artist/%s?fmt=json", musicBrainzURL, mbid)

	log.Debug(ctx, "MusicBrainz artist lookup", "mbid", mbid)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", musicBrainzUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("musicbrainz request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNoMatch
	}

	if resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("musicbrainz returned status %d", resp.StatusCode)
	}

	var artist MBArtist
	if err := json.NewDecoder(resp.Body).Decode(&artist); err != nil {
		return nil, fmt.Errorf("failed to decode musicbrainz response: %w", err)
	}

	log.Debug(ctx, "MusicBrainz artist lookup complete", "mbid", mbid, "name", artist.Name)

	return &artist, nil
}

// GetRelease fetches detailed release information from MusicBrainz
func (c *MusicBrainzClient) GetRelease(ctx context.Context, mbid string) (*MBRelease, error) {
	// Wait for rate limiter
	if err := c.limiter.Wait(ctx); err != nil {
		if ctx.Err() == context.DeadlineExceeded || ctx.Err() == context.Canceled {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: %s", ErrRateLimited, err)
	}

	requestURL := fmt.Sprintf("%s/release/%s?fmt=json&inc=release-groups", musicBrainzURL, mbid)

	log.Debug(ctx, "MusicBrainz release lookup", "mbid", mbid)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", musicBrainzUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("musicbrainz request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNoMatch
	}

	if resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("musicbrainz returned status %d", resp.StatusCode)
	}

	var release MBRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode musicbrainz response: %w", err)
	}

	log.Debug(ctx, "MusicBrainz release lookup complete", "mbid", mbid, "title", release.Title)

	return &release, nil
}
