package fingerprint

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/navidrome/navidrome/log"
)

const (
	// DefaultFpcalcTimeout is the default timeout for fpcalc execution
	DefaultFpcalcTimeout = 30 * time.Second
)

// ChromaprintWrapper wraps the fpcalc command-line tool
type ChromaprintWrapper struct {
	fpcalcPath string
	timeout    time.Duration
	mu         sync.RWMutex
	available  *bool // cached availability check
}

// fpcalcOutput represents the JSON output from fpcalc -json
type fpcalcOutput struct {
	Duration    float64 `json:"duration"`
	Fingerprint string  `json:"fingerprint"`
}

// NewChromaprintWrapper creates a new chromaprint wrapper
// If fpcalcPath is empty, it will attempt to find fpcalc in PATH
func NewChromaprintWrapper(fpcalcPath string) *ChromaprintWrapper {
	return &ChromaprintWrapper{
		fpcalcPath: fpcalcPath,
		timeout:    DefaultFpcalcTimeout,
	}
}

// SetTimeout sets the timeout for fpcalc execution
func (c *ChromaprintWrapper) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}

// IsAvailable checks if fpcalc is available on the system
func (c *ChromaprintWrapper) IsAvailable() bool {
	c.mu.RLock()
	if c.available != nil {
		result := *c.available
		c.mu.RUnlock()
		return result
	}
	c.mu.RUnlock()

	// Check availability
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if c.available != nil {
		return *c.available
	}

	path, err := c.getFpcalcPath()
	result := err == nil && path != ""
	c.available = &result

	if result {
		log.Info("fpcalc binary found", "path", path)
	} else {
		log.Warn("fpcalc binary not found - audio fingerprinting will be unavailable")
	}

	return result
}

// getFpcalcPath returns the path to fpcalc binary
func (c *ChromaprintWrapper) getFpcalcPath() (string, error) {
	if c.fpcalcPath != "" {
		// Check if configured path exists
		_, err := exec.LookPath(c.fpcalcPath)
		if err != nil {
			return "", fmt.Errorf("configured fpcalc path not found: %s: %w", c.fpcalcPath, err)
		}
		return c.fpcalcPath, nil
	}

	// Try to find fpcalc in PATH
	path, err := exec.LookPath("fpcalc")
	if err != nil {
		return "", ErrFpcalcNotFound
	}
	return path, nil
}

// Generate creates an audio fingerprint for the given file
func (c *ChromaprintWrapper) Generate(ctx context.Context, filePath string) (*FingerprintResult, error) {
	if !c.IsAvailable() {
		return nil, ErrFpcalcNotFound
	}

	fpcalcPath, err := c.getFpcalcPath()
	if err != nil {
		return nil, err
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Execute fpcalc with JSON output
	cmd := exec.CommandContext(ctx, fpcalcPath, "-json", filePath)

	log.Debug(ctx, "Executing fpcalc", "path", fpcalcPath, "file", filePath)

	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("%w: timeout after %v", ErrFingerprintFailed, c.timeout)
		}
		// Try to get stderr for more info
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%w: %s: %s", ErrFingerprintFailed, err, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("%w: %s", ErrFingerprintFailed, err)
	}

	// Parse JSON output
	var result fpcalcOutput
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("%w: failed to parse fpcalc output: %s", ErrFingerprintFailed, err)
	}

	if result.Fingerprint == "" {
		return nil, fmt.Errorf("%w: empty fingerprint returned", ErrFingerprintFailed)
	}

	log.Debug(ctx, "Generated fingerprint", "file", filePath, "duration", result.Duration)

	return &FingerprintResult{
		Duration:    int(result.Duration),
		Fingerprint: result.Fingerprint,
	}, nil
}
