package sonos_cast

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	"github.com/navidrome/navidrome/server"
)

// API handles REST API endpoints for Sonos Cast
type API struct {
	sonosCast *SonosCast
	ds        model.DataStore
}

// NewAPI creates a new Sonos Cast API handler
func NewAPI(sonosCast *SonosCast, ds model.DataStore) *API {
	return &API{
		sonosCast: sonosCast,
		ds:        ds,
	}
}

// Router returns the chi router with all Sonos Cast endpoints
func (a *API) Router() http.Handler {
	r := chi.NewRouter()

	// Apply authentication middleware - user must be logged in
	log.Info("Setting up Sonos Cast router with authentication middleware")
	r.Use(server.Authenticator(a.ds))
	r.Use(server.JWTRefresher)

	// Device endpoints
	r.Get("/devices", a.getDevices)
	r.Post("/devices/refresh", a.refreshDevices)
	r.Get("/devices/{id}", a.getDevice)
	r.Get("/devices/{id}/state", a.getDeviceState)

	// Playback control
	r.Post("/devices/{id}/play", a.play)
	r.Post("/devices/{id}/pause", a.pause)
	r.Post("/devices/{id}/stop", a.stop)
	r.Post("/devices/{id}/seek", a.seek)
	r.Post("/devices/{id}/next", a.next)
	r.Post("/devices/{id}/previous", a.previous)

	// Volume control
	r.Get("/devices/{id}/volume", a.getVolume)
	r.Post("/devices/{id}/volume", a.setVolume)
	r.Post("/devices/{id}/mute", a.setMute)

	// Cast media
	r.Post("/devices/{id}/cast", a.castMedia)

	return r
}

// getDevices returns all discovered Sonos devices
func (a *API) getDevices(w http.ResponseWriter, r *http.Request) {
	devices := a.sonosCast.GetDevices()
	a.sendJSON(w, http.StatusOK, devices)
}

// refreshDevices forces a new SSDP discovery
func (a *API) refreshDevices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := a.sonosCast.RefreshDevices(ctx); err != nil {
		a.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	devices := a.sonosCast.GetDevices()
	a.sendJSON(w, http.StatusOK, devices)
}

// getDevice returns a specific device by UUID
func (a *API) getDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "id")
	device, ok := a.sonosCast.GetDevice(deviceID)
	if !ok {
		a.sendError(w, http.StatusNotFound, "device not found")
		return
	}
	a.sendJSON(w, http.StatusOK, device)
}

// getDeviceState returns the current playback state of a device
func (a *API) getDeviceState(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	deviceID := chi.URLParam(r, "id")

	state, err := a.sonosCast.GetPlaybackState(ctx, deviceID)
	if err != nil {
		if err == ErrDeviceNotFound {
			a.sendError(w, http.StatusNotFound, "device not found")
		} else {
			a.sendError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	// Enrich track with quality info from database
	if state.CurrentTrack != nil && state.CurrentTrack.URI != "" {
		a.enrichTrackQuality(ctx, state.CurrentTrack)
	}

	a.sendJSON(w, http.StatusOK, state)
}

// enrichTrackQuality looks up track in database and adds quality info
func (a *API) enrichTrackQuality(ctx context.Context, track *Track) {
	// Extract track ID from stream URI
	// URI format: http://host:port/rest/stream?id=TRACKID&u=...
	trackID := extractTrackIDFromURI(track.URI)
	if trackID == "" {
		return
	}

	// Look up track in database
	mfRepo := a.ds.MediaFile(ctx)
	mf, err := mfRepo.Get(trackID)
	if err != nil {
		log.Debug(ctx, "Could not look up track for quality info", "trackID", trackID, "error", err)
		return
	}

	// Populate quality fields
	track.Format = strings.ToUpper(mf.Suffix)
	track.BitRate = mf.BitRate
	track.SampleRate = mf.SampleRate
	track.BitDepth = mf.BitDepth

	// Check if transcoding is likely happening
	// Sonos can't handle >48kHz, so hi-res audio gets transcoded
	track.Transcoding = mf.SampleRate > 48000

	log.Debug(ctx, "Enriched track with quality info",
		"trackID", trackID,
		"format", track.Format,
		"bitRate", track.BitRate,
		"sampleRate", track.SampleRate,
		"bitDepth", track.BitDepth,
		"transcoding", track.Transcoding)
}

// extractTrackIDFromURI extracts the track ID from a Subsonic stream URL
func extractTrackIDFromURI(uri string) string {
	// Parse the URL
	parsed, err := url.Parse(uri)
	if err != nil {
		return ""
	}

	// Get the "id" query parameter
	return parsed.Query().Get("id")
}

// play starts playback on a device
func (a *API) play(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	deviceID := chi.URLParam(r, "id")

	if err := a.sonosCast.Play(ctx, deviceID); err != nil {
		if err == ErrDeviceNotFound {
			a.sendError(w, http.StatusNotFound, "device not found")
		} else {
			a.sendError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	a.sendJSON(w, http.StatusOK, map[string]string{"status": "playing"})
}

// pause pauses playback on a device
func (a *API) pause(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	deviceID := chi.URLParam(r, "id")

	if err := a.sonosCast.Pause(ctx, deviceID); err != nil {
		if err == ErrDeviceNotFound {
			a.sendError(w, http.StatusNotFound, "device not found")
		} else {
			a.sendError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	a.sendJSON(w, http.StatusOK, map[string]string{"status": "paused"})
}

// stop stops playback on a device
func (a *API) stop(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	deviceID := chi.URLParam(r, "id")

	if err := a.sonosCast.Stop(ctx, deviceID); err != nil {
		if err == ErrDeviceNotFound {
			a.sendError(w, http.StatusNotFound, "device not found")
		} else {
			a.sendError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	a.sendJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

// seekRequest is the request body for seek
type seekRequest struct {
	Position int `json:"position"` // seconds
}

// seek seeks to a position on a device
func (a *API) seek(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	deviceID := chi.URLParam(r, "id")

	var req seekRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	position := time.Duration(req.Position) * time.Second
	if err := a.sonosCast.Seek(ctx, deviceID, position); err != nil {
		if err == ErrDeviceNotFound {
			a.sendError(w, http.StatusNotFound, "device not found")
		} else {
			a.sendError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	a.sendJSON(w, http.StatusOK, map[string]string{"status": "seeked"})
}

// next skips to the next track on a device
func (a *API) next(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	deviceID := chi.URLParam(r, "id")

	if err := a.sonosCast.Next(ctx, deviceID); err != nil {
		if err == ErrDeviceNotFound {
			a.sendError(w, http.StatusNotFound, "device not found")
		} else {
			a.sendError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	a.sendJSON(w, http.StatusOK, map[string]string{"status": "next"})
}

// previous goes to the previous track on a device
func (a *API) previous(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	deviceID := chi.URLParam(r, "id")

	if err := a.sonosCast.Previous(ctx, deviceID); err != nil {
		if err == ErrDeviceNotFound {
			a.sendError(w, http.StatusNotFound, "device not found")
		} else {
			a.sendError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	a.sendJSON(w, http.StatusOK, map[string]string{"status": "previous"})
}

// getVolume returns the current volume of a device
func (a *API) getVolume(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	deviceID := chi.URLParam(r, "id")

	volume, err := a.sonosCast.GetVolume(ctx, deviceID)
	if err != nil {
		if err == ErrDeviceNotFound {
			a.sendError(w, http.StatusNotFound, "device not found")
		} else {
			a.sendError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	a.sendJSON(w, http.StatusOK, map[string]int{"volume": volume})
}

// setVolume sets the volume on a device
func (a *API) setVolume(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	deviceID := chi.URLParam(r, "id")

	var req VolumeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Volume < 0 || req.Volume > 100 {
		a.sendError(w, http.StatusBadRequest, "volume must be between 0 and 100")
		return
	}

	if err := a.sonosCast.SetVolume(ctx, deviceID, req.Volume); err != nil {
		if err == ErrDeviceNotFound {
			a.sendError(w, http.StatusNotFound, "device not found")
		} else {
			a.sendError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	a.sendJSON(w, http.StatusOK, map[string]int{"volume": req.Volume})
}

// muteRequest is the request body for mute
type muteRequest struct {
	Muted bool `json:"muted"`
}

// setMute sets the mute state on a device
func (a *API) setMute(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	deviceID := chi.URLParam(r, "id")

	var req muteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := a.sonosCast.SetMute(ctx, deviceID, req.Muted); err != nil {
		if err == ErrDeviceNotFound {
			a.sendError(w, http.StatusNotFound, "device not found")
		} else {
			a.sendError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	a.sendJSON(w, http.StatusOK, map[string]bool{"muted": req.Muted})
}

// castRequest is the request body for casting media
type castRequest struct {
	// New format from UI
	TrackIds []string `json:"trackIds"` // array of track IDs
	Resource string   `json:"resource"` // album, playlist, song

	// Legacy format
	Type       string `json:"type"`       // track, album, playlist
	ID         string `json:"id"`         // single media ID
	StartIndex int    `json:"startIndex"` // for albums/playlists
}

// castMedia casts media to a Sonos device
func (a *API) castMedia(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	deviceID := chi.URLParam(r, "id")

	// Debug auth headers - use Info level to ensure visibility
	authHeader := r.Header.Get("X-ND-Authorization")
	log.Info(ctx, "Cast request received", "deviceID", deviceID, "hasAuthHeader", authHeader != "", "authHeaderLen", len(authHeader))

	var req castRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error(ctx, "Failed to decode cast request", err)
		a.sendError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	log.Debug(ctx, "Cast request parsed", "trackIds", req.TrackIds, "resource", req.Resource, "type", req.Type, "id", req.ID)

	// Get user from context for stream authentication
	user, ok := request.UserFrom(ctx)
	if !ok {
		log.Warn(ctx, "No user in context for Sonos cast")
	} else {
		log.Debug(ctx, "User for cast", "username", user.UserName)
	}

	// Handle new format from UI (trackIds + resource)
	if len(req.TrackIds) > 0 {
		log.Info(ctx, "Casting tracks to Sonos", "count", len(req.TrackIds), "resource", req.Resource, "deviceID", deviceID)
		// For now, cast just the first track (queue support TODO)
		if err := a.castTrack(ctx, deviceID, req.TrackIds[0], user); err != nil {
			log.Error(ctx, "Failed to cast track", err, "trackID", req.TrackIds[0], "deviceID", deviceID)
			if err == ErrDeviceNotFound {
				a.sendError(w, http.StatusNotFound, "device not found")
			} else {
				a.sendError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
		a.sendJSON(w, http.StatusOK, map[string]string{"status": "casting"})
		return
	}

	// Handle legacy format (type + id)
	switch req.Type {
	case "track":
		log.Info(ctx, "Casting single track (legacy)", "trackID", req.ID, "deviceID", deviceID)
		if err := a.castTrack(ctx, deviceID, req.ID, user); err != nil {
			log.Error(ctx, "Failed to cast track", err, "trackID", req.ID, "deviceID", deviceID)
			if err == ErrDeviceNotFound {
				a.sendError(w, http.StatusNotFound, "device not found")
			} else {
				a.sendError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
	case "album":
		a.sendError(w, http.StatusNotImplemented, "album casting not yet implemented")
		return
	case "playlist":
		a.sendError(w, http.StatusNotImplemented, "playlist casting not yet implemented")
		return
	default:
		log.Warn(ctx, "Invalid cast request - no trackIds and no valid type", "type", req.Type)
		a.sendError(w, http.StatusBadRequest, "invalid media type or missing trackIds")
		return
	}

	a.sendJSON(w, http.StatusOK, map[string]string{"status": "casting"})
}

// castTrack casts a single track to a device
func (a *API) castTrack(ctx context.Context, deviceID, trackID string, user model.User) error {
	log.Debug(ctx, "Looking up track for cast", "trackID", trackID)

	// Get track from database
	mfRepo := a.ds.MediaFile(ctx)
	track, err := mfRepo.Get(trackID)
	if err != nil {
		log.Error(ctx, "Failed to get track from database", err, "trackID", trackID)
		return fmt.Errorf("track not found: %w", err)
	}

	log.Debug(ctx, "Found track", "title", track.Title, "artist", track.Artist, "album", track.Album,
		"format", track.Suffix, "sampleRate", track.SampleRate, "bitDepth", track.BitDepth)

	// Get full user with password for Subsonic auth
	userRepo := a.ds.User(ctx)
	fullUser, err := userRepo.FindByUsernameWithPassword(user.UserName)
	if err != nil {
		log.Error(ctx, "Failed to get user for Subsonic auth", err, "username", user.UserName)
		return fmt.Errorf("user not found: %w", err)
	}

	// Get the base URL for streaming - Sonos needs an absolute URL it can reach
	// We use the internal IP since Sonos is on the same network
	baseURL := a.sonosCast.GetStreamBaseURL()
	log.Debug(ctx, "Using stream base URL", "baseURL", baseURL)

	// Check for hi-res audio that Sonos doesn't support
	// Sonos FLAC limit: 48kHz sample rate, 24-bit depth
	needsTranscode := false
	if track.SampleRate > 48000 {
		log.Warn(ctx, "Hi-res audio detected - will transcode for Sonos compatibility",
			"track", track.Title, "sampleRate", track.SampleRate, "limit", 48000)
		needsTranscode = true
	}

	// Build stream URL with Subsonic token auth
	streamURL := buildStreamURL(baseURL, trackID, fullUser, needsTranscode)
	log.Debug(ctx, "Built stream URL", "streamURL", streamURL, "transcoding", needsTranscode)

	// Build album art URL
	artURL := ""
	if track.HasCoverArt {
		artURL = buildCoverArtURL(baseURL, track.AlbumID, fullUser)
		log.Debug(ctx, "Built cover art URL", "artURL", artURL)
	}

	// Get MIME type for the stream
	mimeType := track.ContentType()
	if mimeType == "" {
		mimeType = "audio/flac" // Default fallback
	}

	// Build DIDL metadata with stream URL and MIME type
	// The <res> element with protocolInfo is REQUIRED by Sonos
	metadata := a.sonosCast.BuildTrackMetadata(
		track.ID,
		track.Title,
		track.Artist,
		track.Album,
		artURL,
		streamURL,
		mimeType,
	)
	log.Debug(ctx, "Built DIDL metadata", "metadataLen", len(metadata), "mimeType", mimeType)

	// Cast to device
	log.Info(ctx, "Sending PlayURI to Sonos", "deviceID", deviceID, "track", track.Title)
	err = a.sonosCast.PlayURI(ctx, deviceID, streamURL, metadata)
	if err != nil {
		log.Error(ctx, "PlayURI failed", err, "deviceID", deviceID, "streamURL", streamURL)
		return err
	}

	log.Info(ctx, "Successfully sent cast command", "deviceID", deviceID, "track", track.Title)
	return nil
}

// generateSubsonicToken generates a Subsonic API token (MD5 of password+salt)
func generateSubsonicToken(password string) (token, salt string) {
	// Generate random salt
	saltBytes := make([]byte, 8)
	rand.Read(saltBytes)
	salt = hex.EncodeToString(saltBytes)

	// Token is MD5(password + salt)
	hash := md5.Sum([]byte(password + salt))
	token = hex.EncodeToString(hash[:])

	return token, salt
}

// buildStreamURL builds a Subsonic stream URL for a track with token auth
// If needsTranscode is true, it will request FLAC transcoding at 48kHz for hi-res compatibility
func buildStreamURL(baseURL, trackID string, user *model.User, needsTranscode bool) string {
	// Generate Subsonic token auth
	token, salt := generateSubsonicToken(user.Password)

	if needsTranscode {
		// Hi-res audio needs transcoding to 48kHz FLAC for Sonos compatibility
		// We use FLAC to maintain quality, and estimateContentLength for seeking
		// Note: Seeking may be limited with transcoded streams
		return fmt.Sprintf("%s/rest/stream?id=%s&u=%s&t=%s&s=%s&c=SonosCast&v=1.16.1&format=flac&maxBitRate=0&estimateContentLength=true",
			baseURL, trackID, user.UserName, token, salt)
	}

	// Build HTTP URL with Subsonic token authentication
	// Use format=raw to serve original file without transcoding - this ensures:
	//   1. Proper Content-Length header (required by Sonos for seeking)
	//   2. Range request support (206 Partial Content responses)
	//   3. Seek/scrub functionality works correctly
	// Note: Transcoded streams set Accept-Ranges: none which breaks seeking
	return fmt.Sprintf("%s/rest/stream?id=%s&u=%s&t=%s&s=%s&c=SonosCast&v=1.16.1&format=raw",
		baseURL, trackID, user.UserName, token, salt)
}

// buildCoverArtURL builds a Subsonic cover art URL with token auth
func buildCoverArtURL(baseURL, albumID string, user *model.User) string {
	// Generate Subsonic token auth
	token, salt := generateSubsonicToken(user.Password)

	return fmt.Sprintf("%s/rest/getCoverArt?id=%s&u=%s&t=%s&s=%s&c=SonosCast&v=1.16.1",
		baseURL, albumID, user.UserName, token, salt)
}

// sendJSON sends a JSON response
func (a *API) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Error("Failed to encode JSON response", err)
	}
}

// sendError sends an error response
func (a *API) sendError(w http.ResponseWriter, status int, message string) {
	a.sendJSON(w, status, map[string]string{"error": message})
}
