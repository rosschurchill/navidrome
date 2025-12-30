package sonos

import (
	"context"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

// handleSearch handles content searching
func (r *Router) handleSearch(ctx context.Context, body []byte, creds *Credentials) (interface{}, error) {
	var request SearchRequest
	if err := r.extractRequest(body, "search", &request); err != nil {
		return nil, err
	}

	log.Debug(ctx, "SMAPI search", "id", request.ID, "term", request.Term, "index", request.Index, "count", request.Count)

	// Validate credentials
	_, err := r.validateCredentials(ctx, creds)
	if err != nil {
		return nil, err
	}

	// Default count if not specified
	if request.Count == 0 {
		request.Count = 50
	}

	// Search based on context
	switch request.ID {
	case ArtistsID:
		return r.searchArtists(ctx, request.Term, request.Index, request.Count)
	case AlbumsID:
		return r.searchAlbums(ctx, request.Term, request.Index, request.Count)
	case TracksID:
		return r.searchTracks(ctx, request.Term, request.Index, request.Count)
	default:
		// Search everything
		return r.searchAll(ctx, request.Term, request.Index, request.Count)
	}
}

// searchArtists searches for artists
func (r *Router) searchArtists(ctx context.Context, term string, index, count int) (*SearchResponse, error) {
	artists, err := r.ds.Artist(ctx).GetAll(model.QueryOptions{
		Offset: index,
		Max:    count,
		Sort:   "name",
		Filters: squirrel.Like{"name": "%" + term + "%"},
	})
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

	return &SearchResponse{
		Index:           index,
		Count:           len(items),
		Total:           len(items),
		MediaCollection: items,
	}, nil
}

// searchAlbums searches for albums
func (r *Router) searchAlbums(ctx context.Context, term string, index, count int) (*SearchResponse, error) {
	albums, err := r.ds.Album(ctx).GetAll(model.QueryOptions{
		Offset: index,
		Max:    count,
		Sort:   "name",
		Filters: squirrel.Like{"name": "%" + term + "%"},
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

	return &SearchResponse{
		Index:           index,
		Count:           len(items),
		Total:           len(items),
		MediaCollection: items,
	}, nil
}

// searchTracks searches for tracks
func (r *Router) searchTracks(ctx context.Context, term string, index, count int) (*SearchResponse, error) {
	songs, err := r.ds.MediaFile(ctx).GetAll(model.QueryOptions{
		Offset: index,
		Max:    count,
		Sort:   "title",
		Filters: squirrel.Like{"title": "%" + term + "%"},
	})
	if err != nil {
		return nil, err
	}

	items := make([]MediaMetadata, len(songs))
	for i, song := range songs {
		items[i] = r.songToMediaMetadata(song)
	}

	return &SearchResponse{
		Index:         index,
		Count:         len(items),
		Total:         len(items),
		MediaMetadata: items,
	}, nil
}

// searchAll performs a combined search across artists, albums, and tracks
func (r *Router) searchAll(ctx context.Context, term string, index, count int) (*SearchResponse, error) {
	// Split results: 10 artists, 20 albums, rest tracks
	artistCount := 10
	albumCount := 20
	trackCount := count - artistCount - albumCount
	if trackCount < 10 {
		trackCount = 10
	}

	var collections []MediaCollection
	var metadata []MediaMetadata

	// Search artists
	artists, err := r.ds.Artist(ctx).GetAll(model.QueryOptions{
		Max:  artistCount,
		Sort: "name",
		Filters: squirrel.Like{"name": "%" + term + "%"},
	})
	if err == nil {
		for _, artist := range artists {
			collections = append(collections, MediaCollection{
				ID:           "artist:" + artist.ID,
				ItemType:     "artist",
				Title:        artist.Name,
				AlbumArtURI:  r.artworkURL("ar", artist.ID),
				CanEnumerate: true,
				CanPlay:      true,
			})
		}
	}

	// Search albums
	albums, err := r.ds.Album(ctx).GetAll(model.QueryOptions{
		Max:  albumCount,
		Sort: "name",
		Filters: squirrel.Like{"name": "%" + term + "%"},
	})
	if err == nil {
		for _, album := range albums {
			collections = append(collections, MediaCollection{
				ID:           "album:" + album.ID,
				ItemType:     "album",
				Title:        album.Name,
				Artist:       album.AlbumArtist,
				AlbumArtURI:  r.artworkURL("al", album.ID),
				CanEnumerate: true,
				CanPlay:      true,
			})
		}
	}

	// Search tracks
	songs, err := r.ds.MediaFile(ctx).GetAll(model.QueryOptions{
		Max:  trackCount,
		Sort: "title",
		Filters: squirrel.Like{"title": "%" + term + "%"},
	})
	if err == nil {
		for _, song := range songs {
			metadata = append(metadata, r.songToMediaMetadata(song))
		}
	}

	// Also search track artist
	if len(metadata) < trackCount {
		artistSongs, err := r.ds.MediaFile(ctx).GetAll(model.QueryOptions{
			Max:  trackCount - len(metadata),
			Sort: "title",
			Filters: squirrel.Like{"artist": "%" + term + "%"},
		})
		if err == nil {
			// Dedupe by ID
			seen := make(map[string]bool)
			for _, song := range metadata {
				seen[strings.TrimPrefix(song.ID, "track:")] = true
			}
			for _, song := range artistSongs {
				if !seen[song.ID] {
					metadata = append(metadata, r.songToMediaMetadata(song))
				}
			}
		}
	}

	total := len(collections) + len(metadata)

	return &SearchResponse{
		Index:           index,
		Count:           total,
		Total:           total,
		MediaCollection: collections,
		MediaMetadata:   metadata,
	}, nil
}
