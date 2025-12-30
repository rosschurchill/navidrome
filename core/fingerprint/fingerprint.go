// Package fingerprint provides audio fingerprinting integration with AcoustID/MusicBrainz
// for automatic track identification and metadata enrichment.
package fingerprint

import (
	"context"
	"errors"

	"github.com/navidrome/navidrome/conf"
)

var (
	// ErrFpcalcNotFound is returned when fpcalc binary cannot be found
	ErrFpcalcNotFound = errors.New("fpcalc binary not found")
	// ErrFingerprintFailed is returned when fingerprint generation fails
	ErrFingerprintFailed = errors.New("fingerprint generation failed")
	// ErrNoMatch is returned when no matching recordings are found
	ErrNoMatch = errors.New("no matching recordings found")
	// ErrRateLimited is returned when API rate limit is exceeded
	ErrRateLimited = errors.New("rate limit exceeded")
	// ErrDisabled is returned when fingerprinting is disabled in config
	ErrDisabled = errors.New("fingerprinting is disabled")
)

// FingerprintResult contains the result of fingerprint generation
type FingerprintResult struct {
	Duration    int    `json:"duration"`
	Fingerprint string `json:"fingerprint"`
}

// MatchResult represents a single match from fingerprint lookup
type MatchResult struct {
	AcoustID    string  `json:"acoustid"`
	MusicBrainzID string  `json:"musicbrainz_id"`
	Score       float64 `json:"score"`
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	Album       string  `json:"album"`
	ReleaseDate string  `json:"release_date,omitempty"`
}

// Service provides audio fingerprinting functionality
type Service interface {
	// IsEnabled returns true if fingerprinting is configured and available
	IsEnabled() bool

	// Generate creates an audio fingerprint for the given file
	Generate(ctx context.Context, filePath string) (*FingerprintResult, error)

	// Lookup searches for matches using the given fingerprint
	Lookup(ctx context.Context, fingerprint string, duration int) ([]MatchResult, error)

	// Identify generates a fingerprint and looks up matches in one call
	Identify(ctx context.Context, filePath string) ([]MatchResult, error)
}

// service implements the Service interface
type service struct {
	chromaprint *ChromaprintWrapper
	acoustid    *AcoustIDClient
	musicbrainz *MusicBrainzClient
}

// NewService creates a new fingerprint service
func NewService() Service {
	if !conf.Server.Fingerprint.Enabled {
		return &disabledService{}
	}

	chromaprint := NewChromaprintWrapper(conf.Server.Fingerprint.FpcalcPath)
	acoustid := NewAcoustIDClient(conf.Server.Fingerprint.AcoustIDApiKey)
	musicbrainz := NewMusicBrainzClient()

	return &service{
		chromaprint: chromaprint,
		acoustid:    acoustid,
		musicbrainz: musicbrainz,
	}
}

func (s *service) IsEnabled() bool {
	return conf.Server.Fingerprint.Enabled && s.chromaprint.IsAvailable()
}

func (s *service) Generate(ctx context.Context, filePath string) (*FingerprintResult, error) {
	if !s.IsEnabled() {
		return nil, ErrDisabled
	}
	return s.chromaprint.Generate(ctx, filePath)
}

func (s *service) Lookup(ctx context.Context, fingerprint string, duration int) ([]MatchResult, error) {
	if !s.IsEnabled() {
		return nil, ErrDisabled
	}

	// First lookup in AcoustID
	acoustidResults, err := s.acoustid.Lookup(ctx, fingerprint, duration)
	if err != nil {
		return nil, err
	}

	if len(acoustidResults.Results) == 0 {
		return nil, ErrNoMatch
	}

	// Convert to MatchResults, optionally enriching with MusicBrainz data
	var matches []MatchResult
	for _, result := range acoustidResults.Results {
		for _, recording := range result.Recordings {
			match := MatchResult{
				AcoustID:      result.ID,
				MusicBrainzID: recording.ID,
				Score:         result.Score,
				Title:         recording.Title,
			}
			// Get artist name from the first artist
			if len(recording.Artists) > 0 {
				match.Artist = recording.Artists[0].Name
			}
			matches = append(matches, match)
		}
	}

	return matches, nil
}

func (s *service) Identify(ctx context.Context, filePath string) ([]MatchResult, error) {
	if !s.IsEnabled() {
		return nil, ErrDisabled
	}

	fp, err := s.Generate(ctx, filePath)
	if err != nil {
		return nil, err
	}

	return s.Lookup(ctx, fp.Fingerprint, fp.Duration)
}

// disabledService is a no-op implementation when fingerprinting is disabled
type disabledService struct{}

func (d *disabledService) IsEnabled() bool { return false }
func (d *disabledService) Generate(ctx context.Context, filePath string) (*FingerprintResult, error) {
	return nil, ErrDisabled
}
func (d *disabledService) Lookup(ctx context.Context, fingerprint string, duration int) ([]MatchResult, error) {
	return nil, ErrDisabled
}
func (d *disabledService) Identify(ctx context.Context, filePath string) ([]MatchResult, error) {
	return nil, ErrDisabled
}
