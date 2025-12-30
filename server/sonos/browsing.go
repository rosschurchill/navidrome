package sonos

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

// handleGetMetadata handles content browsing
func (r *Router) handleGetMetadata(ctx context.Context, body []byte, creds *Credentials) (interface{}, error) {
	var request GetMetadataRequest
	if err := r.extractRequest(body, "getMetadata", &request); err != nil {
		return nil, err
	}

	log.Debug(ctx, "SMAPI getMetadata", "id", request.ID, "index", request.Index, "count", request.Count)

	// Validate credentials
	user, err := r.validateCredentials(ctx, creds)
	if err != nil {
		log.Warn(ctx, "SMAPI auth failed", err)
		// For browsing, we might allow guest access or return limited content
		// For now, require authentication
		return nil, err
	}

	// Default count if not specified
	if request.Count == 0 {
		request.Count = 100
	}

	switch request.ID {
	case RootID:
		return r.getRootMenu(ctx, user)
	case ArtistsID:
		return r.getArtists(ctx, user, request.Index, request.Count)
	case AlbumsID:
		return r.getAlbums(ctx, user, request.Index, request.Count)
	case PlaylistsID:
		return r.getPlaylists(ctx, user, request.Index, request.Count)
	case GenresID:
		return r.getGenres(ctx, user, request.Index, request.Count)
	case RecentlyAddedID:
		return r.getRecentlyAdded(ctx, user, request.Index, request.Count)
	case RecentlyPlayedID:
		return r.getRecentlyPlayed(ctx, user, request.Index, request.Count)
	case FavoritesID:
		return r.getFavorites(ctx, user, request.Index, request.Count)
	case RandomID:
		return r.getRandom(ctx, user, request.Index, request.Count)
	default:
		// Check if it's an artist, album, genre, or playlist ID
		return r.getContainerContents(ctx, user, request.ID, request.Index, request.Count)
	}
}

// getRootMenu returns the root navigation menu
func (r *Router) getRootMenu(ctx context.Context, user *model.User) (*GetMetadataResponse, error) {
	items := []MediaCollection{
		{ID: ArtistsID, ItemType: "collection", Title: "Artists", CanEnumerate: true},
		{ID: AlbumsID, ItemType: "collection", Title: "Albums", CanEnumerate: true},
		{ID: GenresID, ItemType: "collection", Title: "Genres", CanEnumerate: true},
		{ID: PlaylistsID, ItemType: "collection", Title: "Playlists", CanEnumerate: true},
		{ID: RecentlyAddedID, ItemType: "collection", Title: "Recently Added", CanEnumerate: true},
		{ID: RecentlyPlayedID, ItemType: "collection", Title: "Recently Played", CanEnumerate: true},
		{ID: FavoritesID, ItemType: "collection", Title: "Favorites", CanEnumerate: true},
		{ID: RandomID, ItemType: "collection", Title: "Random", CanEnumerate: true},
	}

	return &GetMetadataResponse{
		Index:           0,
		Count:           len(items),
		Total:           len(items),
		MediaCollection: items,
	}, nil
}

// getArtists returns list of artists
func (r *Router) getArtists(ctx context.Context, user *model.User, index, count int) (*GetMetadataResponse, error) {
	artists, err := r.ds.Artist(ctx).GetAll(model.QueryOptions{
		Offset: index,
		Max:    count,
		Sort:   "name",
	})
	if err != nil {
		return nil, err
	}

	total, err := r.ds.Artist(ctx).CountAll()
	if err != nil {
		return nil, err
	}

	items := make([]MediaCollection, len(artists))
	for i, artist := range artists {
		items[i] = MediaCollection{
			ID:           "artist:" + artist.ID,
			ItemType:     "artist",
			Title:        artist.Name,
			AlbumArtURI:  r.artworkURL("ar", artist.ID),
			CanEnumerate: true,
			CanPlay:      true,
		}
	}

	return &GetMetadataResponse{
		Index:           index,
		Count:           len(items),
		Total:           int(total),
		MediaCollection: items,
	}, nil
}

// getAlbums returns list of albums
func (r *Router) getAlbums(ctx context.Context, user *model.User, index, count int) (*GetMetadataResponse, error) {
	albums, err := r.ds.Album(ctx).GetAll(model.QueryOptions{
		Offset: index,
		Max:    count,
		Sort:   "name",
	})
	if err != nil {
		return nil, err
	}

	total, err := r.ds.Album(ctx).CountAll()
	if err != nil {
		return nil, err
	}

	items := make([]MediaCollection, len(albums))
	for i, album := range albums {
		items[i] = MediaCollection{
			ID:           "album:" + album.ID,
			ItemType:     "album",
			Title:        album.Name,
			Artist:       album.AlbumArtist,
			AlbumArtURI:  r.artworkURL("al", album.ID),
			CanEnumerate: true,
			CanPlay:      true,
		}
	}

	return &GetMetadataResponse{
		Index:           index,
		Count:           len(items),
		Total:           int(total),
		MediaCollection: items,
	}, nil
}

// getPlaylists returns user playlists
func (r *Router) getPlaylists(ctx context.Context, user *model.User, index, count int) (*GetMetadataResponse, error) {
	playlists, err := r.ds.Playlist(ctx).GetAll(model.QueryOptions{
		Offset: index,
		Max:    count,
		Sort:   "name",
		Filters: squirrel.Eq{"owner_id": user.ID},
	})
	if err != nil {
		return nil, err
	}

	// Count total (approximate)
	total := len(playlists)
	if len(playlists) == count {
		// There might be more
		total = index + count + 1
	}

	items := make([]MediaCollection, len(playlists))
	for i, pl := range playlists {
		items[i] = MediaCollection{
			ID:           "playlist:" + pl.ID,
			ItemType:     "playlist",
			Title:        pl.Name,
			CanEnumerate: true,
			CanPlay:      true,
		}
	}

	return &GetMetadataResponse{
		Index:           index,
		Count:           len(items),
		Total:           total,
		MediaCollection: items,
	}, nil
}

// getGenres returns list of genres
func (r *Router) getGenres(ctx context.Context, user *model.User, index, count int) (*GetMetadataResponse, error) {
	genres, err := r.ds.Genre(ctx).GetAll(model.QueryOptions{
		Offset: index,
		Max:    count,
		Sort:   "name",
	})
	if err != nil {
		return nil, err
	}

	items := make([]MediaCollection, len(genres))
	for i, genre := range genres {
		items[i] = MediaCollection{
			ID:           "genre:" + genre.ID,
			ItemType:     "collection",
			Title:        genre.Name,
			CanEnumerate: true,
			CanPlay:      true,
		}
	}

	return &GetMetadataResponse{
		Index:           index,
		Count:           len(items),
		Total:           len(items),
		MediaCollection: items,
	}, nil
}

// getRecentlyAdded returns recently added albums
func (r *Router) getRecentlyAdded(ctx context.Context, user *model.User, index, count int) (*GetMetadataResponse, error) {
	albums, err := r.ds.Album(ctx).GetAll(model.QueryOptions{
		Offset: index,
		Max:    count,
		Sort:   "recently_added",
		Order:  "desc",
	})
	if err != nil {
		return nil, err
	}

	items := make([]MediaCollection, len(albums))
	for i, album := range albums {
		items[i] = MediaCollection{
			ID:           "album:" + album.ID,
			ItemType:     "album",
			Title:        album.Name,
			Artist:       album.AlbumArtist,
			AlbumArtURI:  r.artworkURL("al", album.ID),
			CanEnumerate: true,
			CanPlay:      true,
		}
	}

	return &GetMetadataResponse{
		Index:           index,
		Count:           len(items),
		Total:           100, // Cap at 100 for "recent"
		MediaCollection: items,
	}, nil
}

// getRecentlyPlayed returns recently played tracks
func (r *Router) getRecentlyPlayed(ctx context.Context, user *model.User, index, count int) (*GetMetadataResponse, error) {
	songs, err := r.ds.MediaFile(ctx).GetAll(model.QueryOptions{
		Offset: index,
		Max:    count,
		Sort:   "play_date",
		Order:  "desc",
		Filters: squirrel.Gt{"play_count": 0},
	})
	if err != nil {
		return nil, err
	}

	items := make([]MediaMetadata, len(songs))
	for i, song := range songs {
		items[i] = r.songToMediaMetadata(song)
	}

	return &GetMetadataResponse{
		Index:         index,
		Count:         len(items),
		Total:         100,
		MediaMetadata: items,
	}, nil
}

// getFavorites returns starred items
func (r *Router) getFavorites(ctx context.Context, user *model.User, index, count int) (*GetMetadataResponse, error) {
	songs, err := r.ds.MediaFile(ctx).GetAll(model.QueryOptions{
		Offset: index,
		Max:    count,
		Sort:   "title",
		Filters: squirrel.Eq{"starred": true},
	})
	if err != nil {
		return nil, err
	}

	items := make([]MediaMetadata, len(songs))
	for i, song := range songs {
		items[i] = r.songToMediaMetadata(song)
	}

	total, _ := r.ds.MediaFile(ctx).CountAll(model.QueryOptions{
		Filters: squirrel.Eq{"starred": true},
	})

	return &GetMetadataResponse{
		Index:         index,
		Count:         len(items),
		Total:         int(total),
		MediaMetadata: items,
	}, nil
}

// getRandom returns random tracks
func (r *Router) getRandom(ctx context.Context, user *model.User, index, count int) (*GetMetadataResponse, error) {
	songs, err := r.ds.MediaFile(ctx).GetAll(model.QueryOptions{
		Max:  count,
		Sort: "random",
	})
	if err != nil {
		return nil, err
	}

	items := make([]MediaMetadata, len(songs))
	for i, song := range songs {
		items[i] = r.songToMediaMetadata(song)
	}

	return &GetMetadataResponse{
		Index:         0,
		Count:         len(items),
		Total:         len(items),
		MediaMetadata: items,
	}, nil
}

// getContainerContents returns contents of an artist, album, genre, or playlist
func (r *Router) getContainerContents(ctx context.Context, user *model.User, id string, index, count int) (*GetMetadataResponse, error) {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid container ID: %s", id)
	}

	containerType := parts[0]
	containerID := parts[1]

	switch containerType {
	case "artist":
		return r.getArtistAlbums(ctx, user, containerID, index, count)
	case "album":
		return r.getAlbumTracks(ctx, user, containerID, index, count)
	case "genre":
		return r.getGenreAlbums(ctx, user, containerID, index, count)
	case "playlist":
		return r.getPlaylistTracks(ctx, user, containerID, index, count)
	default:
		return nil, fmt.Errorf("unknown container type: %s", containerType)
	}
}

// getArtistAlbums returns albums by an artist
func (r *Router) getArtistAlbums(ctx context.Context, user *model.User, artistID string, index, count int) (*GetMetadataResponse, error) {
	albums, err := r.ds.Album(ctx).GetAll(model.QueryOptions{
		Offset: index,
		Max:    count,
		Sort:   "max_year",
		Order:  "desc",
		Filters: squirrel.Eq{"album_artist_id": artistID},
	})
	if err != nil {
		return nil, err
	}

	items := make([]MediaCollection, len(albums))
	for i, album := range albums {
		items[i] = MediaCollection{
			ID:           "album:" + album.ID,
			ItemType:     "album",
			Title:        album.Name,
			Artist:       album.AlbumArtist,
			AlbumArtURI:  r.artworkURL("al", album.ID),
			CanEnumerate: true,
			CanPlay:      true,
		}
	}

	return &GetMetadataResponse{
		Index:           index,
		Count:           len(items),
		Total:           len(items),
		MediaCollection: items,
	}, nil
}

// getAlbumTracks returns tracks in an album
func (r *Router) getAlbumTracks(ctx context.Context, user *model.User, albumID string, index, count int) (*GetMetadataResponse, error) {
	songs, err := r.ds.MediaFile(ctx).GetAll(model.QueryOptions{
		Offset: index,
		Max:    count,
		Sort:   "album",
		Filters: squirrel.Eq{"album_id": albumID},
	})
	if err != nil {
		return nil, err
	}

	items := make([]MediaMetadata, len(songs))
	for i, song := range songs {
		items[i] = r.songToMediaMetadata(song)
	}

	return &GetMetadataResponse{
		Index:         index,
		Count:         len(items),
		Total:         len(items),
		MediaMetadata: items,
	}, nil
}

// getGenreAlbums returns albums in a genre
func (r *Router) getGenreAlbums(ctx context.Context, user *model.User, genreID string, index, count int) (*GetMetadataResponse, error) {
	// Get genre name first - GenreRepository only has GetAll, so filter by ID
	genres, err := r.ds.Genre(ctx).GetAll(model.QueryOptions{
		Filters: squirrel.Eq{"id": genreID},
		Max:     1,
	})
	if err != nil || len(genres) == 0 {
		return nil, fmt.Errorf("genre not found: %s", genreID)
	}
	genre := genres[0]

	albums, err := r.ds.Album(ctx).GetAll(model.QueryOptions{
		Offset: index,
		Max:    count,
		Sort:   "name",
		Filters: squirrel.Like{"genre": "%" + genre.Name + "%"},
	})
	if err != nil {
		return nil, err
	}

	items := make([]MediaCollection, len(albums))
	for i, album := range albums {
		items[i] = MediaCollection{
			ID:           "album:" + album.ID,
			ItemType:     "album",
			Title:        album.Name,
			Artist:       album.AlbumArtist,
			AlbumArtURI:  r.artworkURL("al", album.ID),
			CanEnumerate: true,
			CanPlay:      true,
		}
	}

	return &GetMetadataResponse{
		Index:           index,
		Count:           len(items),
		Total:           len(items),
		MediaCollection: items,
	}, nil
}

// getPlaylistTracks returns tracks in a playlist
func (r *Router) getPlaylistTracks(ctx context.Context, user *model.User, playlistID string, index, count int) (*GetMetadataResponse, error) {
	playlist, err := r.ds.Playlist(ctx).GetWithTracks(playlistID, true, false)
	if err != nil {
		return nil, err
	}

	// Apply pagination
	tracks := playlist.Tracks
	total := len(tracks)
	if index >= len(tracks) {
		tracks = nil
	} else {
		end := index + count
		if end > len(tracks) {
			end = len(tracks)
		}
		tracks = tracks[index:end]
	}

	items := make([]MediaMetadata, len(tracks))
	for i, entry := range tracks {
		items[i] = r.songToMediaMetadata(entry.MediaFile)
	}

	return &GetMetadataResponse{
		Index:         index,
		Count:         len(items),
		Total:         total,
		MediaMetadata: items,
	}, nil
}

// handleGetMediaMetadata returns metadata for a single item
func (r *Router) handleGetMediaMetadata(ctx context.Context, body []byte, creds *Credentials) (interface{}, error) {
	var request GetMediaMetadataRequest
	if err := r.extractRequest(body, "getMediaMetadata", &request); err != nil {
		return nil, err
	}

	log.Debug(ctx, "SMAPI getMediaMetadata", "id", request.ID)

	// Validate credentials
	_, err := r.validateCredentials(ctx, creds)
	if err != nil {
		return nil, err
	}

	parts := strings.SplitN(request.ID, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid media ID: %s", request.ID)
	}

	itemType := parts[0]
	itemID := parts[1]

	switch itemType {
	case "track":
		song, err := r.ds.MediaFile(ctx).Get(itemID)
		if err != nil {
			return nil, err
		}
		metadata := r.songToMediaMetadata(*song)
		return &GetMediaMetadataResponse{
			MediaMetadata: &metadata,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported item type: %s", itemType)
	}
}

// handleGetMediaURI returns the streaming URL for a track
// This implementation prioritizes LOSSLESS streaming when possible
func (r *Router) handleGetMediaURI(ctx context.Context, body []byte, creds *Credentials, req *http.Request) (interface{}, error) {
	var request GetMediaURIRequest
	if err := r.extractRequest(body, "getMediaURI", &request); err != nil {
		return nil, err
	}

	log.Debug(ctx, "SMAPI getMediaURI", "id", request.ID)

	// Validate credentials
	user, err := r.validateCredentials(ctx, creds)
	if err != nil {
		return nil, err
	}

	parts := strings.SplitN(request.ID, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid media ID: %s", request.ID)
	}

	trackID := parts[1]

	// Get track info to determine format and provide metadata
	song, err := r.ds.MediaFile(ctx).Get(trackID)
	if err != nil {
		return nil, err
	}

	// Build streaming URL
	baseURL := conf.Server.BaseURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://%s", req.Host)
	}

	// Determine optimal streaming strategy based on source format
	// Sonos natively supports: FLAC, ALAC, WAV, AIFF, MP3, AAC, OGG, WMA
	format := "raw" // Default to raw (no transcoding) for lossless

	sonosNativeFormats := map[string]bool{
		"flac": true, "alac": true, "wav": true, "aiff": true,
		"mp3": true, "aac": true, "m4a": true, "ogg": true, "wma": true,
	}

	suffix := strings.ToLower(song.Suffix)
	if !sonosNativeFormats[suffix] {
		// Non-native format (e.g., DSD, APE, WV) - transcode to FLAC for lossless
		format = "flac"
		log.Debug(ctx, "Sonos: transcoding non-native format to FLAC",
			"original", suffix, "track", song.Title)
	}

	// Use Subsonic stream endpoint with LOSSLESS settings:
	// - format=raw: No transcoding for native formats
	// - maxBitRate=0: No bitrate limiting
	// - estimateContentLength=true: Better seeking support
	streamURL := fmt.Sprintf(
		"%s/rest/stream.view?id=%s&u=%s&s=sonos&t=%s&c=sonos&v=1.16.1&format=%s&maxBitRate=0&estimateContentLength=true",
		baseURL, trackID, user.UserName, user.Password, format)

	log.Debug(ctx, "Sonos stream URL generated",
		"track", song.Title,
		"format", song.Suffix,
		"bitRate", song.BitRate,
		"bitDepth", song.BitDepth,
		"sampleRate", song.SampleRate,
		"lossless", format == "raw" || format == "flac")

	return &GetMediaURIResponse{
		URI: streamURL,
	}, nil
}

// handleGetExtendedMetadata returns extended metadata
func (r *Router) handleGetExtendedMetadata(ctx context.Context, body []byte, creds *Credentials) (interface{}, error) {
	var request GetExtendedMetadataRequest
	if err := r.extractRequest(body, "getExtendedMetadata", &request); err != nil {
		return nil, err
	}

	log.Debug(ctx, "SMAPI getExtendedMetadata", "id", request.ID)

	// For now, just return the regular metadata
	return r.handleGetMediaMetadata(ctx, body, creds)
}

// handleGetLastUpdate returns catalog freshness timestamps
func (r *Router) handleGetLastUpdate(ctx context.Context, body []byte, creds *Credentials) (interface{}, error) {
	// Return current timestamp as catalog version
	ts := strconv.FormatInt(time.Now().Unix(), 10)

	return &GetLastUpdateResponse{
		Catalog:      ts,
		Favorites:    ts,
		PollInterval: 3600, // 1 hour
	}, nil
}

// handleRateItem handles star rating
func (r *Router) handleRateItem(ctx context.Context, body []byte, creds *Credentials) (interface{}, error) {
	var request RateItemRequest
	if err := r.extractRequest(body, "rateItem", &request); err != nil {
		return nil, err
	}

	log.Debug(ctx, "SMAPI rateItem", "id", request.ID, "rating", request.Rating)

	// Validate credentials
	_, err := r.validateCredentials(ctx, creds)
	if err != nil {
		return nil, err
	}

	parts := strings.SplitN(request.ID, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid media ID: %s", request.ID)
	}

	trackID := parts[1]

	// Rating > 0 means starred, 0 means unstarred
	starred := request.Rating > 0

	err = r.ds.MediaFile(ctx).SetStar(starred, trackID)
	if err != nil {
		return nil, err
	}

	return &RateItemResponse{
		UserRating: &UserRating{Rating: request.Rating},
	}, nil
}

// handleSetPlayedSeconds handles scrobbling
func (r *Router) handleSetPlayedSeconds(ctx context.Context, body []byte, creds *Credentials) (interface{}, error) {
	var request SetPlayedSecondsRequest
	if err := r.extractRequest(body, "setPlayedSeconds", &request); err != nil {
		return nil, err
	}

	log.Debug(ctx, "SMAPI setPlayedSeconds", "id", request.ID, "seconds", request.Seconds)

	// Validate credentials
	_, err := r.validateCredentials(ctx, creds)
	if err != nil {
		return nil, err
	}

	parts := strings.SplitN(request.ID, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid media ID: %s", request.ID)
	}

	trackID := parts[1]

	// Get track to check duration
	song, err := r.ds.MediaFile(ctx).Get(trackID)
	if err != nil {
		return nil, err
	}

	// Scrobble if played more than 50% or 4 minutes
	threshold := song.Duration / 2
	if threshold > 240 {
		threshold = 240
	}

	if float32(request.Seconds) >= threshold {
		// Record scrobble
		if err := r.ds.MediaFile(ctx).IncPlayCount(trackID, time.Now()); err != nil {
			log.Error(ctx, "Failed to record play", err, "trackId", trackID)
		} else {
			log.Info(ctx, "Recorded play via Sonos", "track", song.Title)
		}
	}

	return &SetPlayedSecondsResponse{}, nil
}

// songToMediaMetadata converts a MediaFile to MediaMetadata
func (r *Router) songToMediaMetadata(song model.MediaFile) MediaMetadata {
	// Determine MIME type - use original format for Sonos-native formats
	mimeType := song.ContentType()

	// Build quality indicator for display
	// Sonos can show quality badges if we provide proper metadata
	_ = "" // qualityInfo placeholder - could be used for display
	if song.BitDepth >= 24 || song.SampleRate > 48000 {
		// Hi-Res audio
		_ = "Hi-Res"
	} else if isLosslessFormat(song.Suffix) {
		// Lossless audio
		_ = "Lossless"
	}

	title := song.Title

	return MediaMetadata{
		ID:       "track:" + song.ID,
		ItemType: "track",
		Title:    title,
		MimeType: mimeType,
		TrackMetadata: &TrackMetadata{
			AlbumID:     "album:" + song.AlbumID,
			Album:       song.Album,
			AlbumArtURI: r.artworkURL("al", song.AlbumID),
			ArtistID:    "artist:" + song.ArtistID,
			Artist:      song.Artist,
			Duration:    int(song.Duration),
			Genre:       song.Genre,
			TrackNumber: song.TrackNumber,
			CanPlay:     true,
			CanAddToFav: true,
		},
	}
}

// isLosslessFormat checks if the format is lossless
func isLosslessFormat(suffix string) bool {
	lossless := map[string]bool{
		"flac": true, "alac": true, "wav": true, "aiff": true,
		"ape": true, "wv": true, "dsd": true, "dsf": true, "dff": true,
	}
	return lossless[strings.ToLower(suffix)]
}

// artworkURL builds artwork URL
func (r *Router) artworkURL(prefix, id string) string {
	baseURL := conf.Server.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:4533"
	}
	return fmt.Sprintf("%s/rest/getCoverArt.view?id=%s-%s&size=300", baseURL, prefix, id)
}
