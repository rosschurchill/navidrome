package dlna

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

// Browse request/response structures

// BrowseRequest represents a ContentDirectory Browse request
type BrowseRequest struct {
	XMLName        xml.Name `xml:"Browse"`
	ObjectID       string   `xml:"ObjectID"`
	BrowseFlag     string   `xml:"BrowseFlag"`
	Filter         string   `xml:"Filter"`
	StartingIndex  int      `xml:"StartingIndex"`
	RequestedCount int      `xml:"RequestedCount"`
	SortCriteria   string   `xml:"SortCriteria"`
}

// BrowseResponse represents a ContentDirectory Browse response
type BrowseResponse struct {
	XMLName        xml.Name `xml:"urn:schemas-upnp-org:service:ContentDirectory:1 BrowseResponse"`
	Result         string   `xml:"Result"`
	NumberReturned int      `xml:"NumberReturned"`
	TotalMatches   int      `xml:"TotalMatches"`
	UpdateID       uint32   `xml:"UpdateID"`
}

// GetSearchCapabilitiesResponse for GetSearchCapabilities action
type GetSearchCapabilitiesResponse struct {
	XMLName    xml.Name `xml:"urn:schemas-upnp-org:service:ContentDirectory:1 GetSearchCapabilitiesResponse"`
	SearchCaps string   `xml:"SearchCaps"`
}

// GetSortCapabilitiesResponse for GetSortCapabilities action
type GetSortCapabilitiesResponse struct {
	XMLName  xml.Name `xml:"urn:schemas-upnp-org:service:ContentDirectory:1 GetSortCapabilitiesResponse"`
	SortCaps string   `xml:"SortCaps"`
}

// GetSystemUpdateIDResponse for GetSystemUpdateID action
type GetSystemUpdateIDResponse struct {
	XMLName xml.Name `xml:"urn:schemas-upnp-org:service:ContentDirectory:1 GetSystemUpdateIDResponse"`
	Id      uint32   `xml:"Id"`
}

// DIDL-Lite content structure for Browse results

// DIDLLite is the root element for DIDL-Lite content
type DIDLLite struct {
	XMLName    xml.Name      `xml:"DIDL-Lite"`
	XmlnsDC    string        `xml:"xmlns:dc,attr"`
	XmlnsUPnP  string        `xml:"xmlns:upnp,attr"`
	Xmlns      string        `xml:"xmlns,attr"`
	Containers []Container   `xml:"container,omitempty"`
	Items      []Item        `xml:"item,omitempty"`
}

// Container represents a DIDL-Lite container (folder)
type Container struct {
	ID          string `xml:"id,attr"`
	ParentID    string `xml:"parentID,attr"`
	Restricted  string `xml:"restricted,attr"`
	Searchable  string `xml:"searchable,attr,omitempty"`
	ChildCount  int    `xml:"childCount,attr,omitempty"`
	Title       string `xml:"dc:title"`
	Class       string `xml:"upnp:class"`
	AlbumArtURI string `xml:"upnp:albumArtURI,omitempty"`
}

// Item represents a DIDL-Lite item (media file)
type Item struct {
	ID          string   `xml:"id,attr"`
	ParentID    string   `xml:"parentID,attr"`
	Restricted  string   `xml:"restricted,attr"`
	Title       string   `xml:"dc:title"`
	Creator     string   `xml:"dc:creator,omitempty"`
	Album       string   `xml:"upnp:album,omitempty"`
	Artist      string   `xml:"upnp:artist,omitempty"`
	Genre       string   `xml:"upnp:genre,omitempty"`
	Class       string   `xml:"upnp:class"`
	AlbumArtURI string   `xml:"upnp:albumArtURI,omitempty"`
	Resources   []Res    `xml:"res,omitempty"`
	TrackNumber int      `xml:"upnp:originalTrackNumber,omitempty"`
}

// Res represents a resource element
type Res struct {
	ProtocolInfo string `xml:"protocolInfo,attr"`
	Size         int64  `xml:"size,attr,omitempty"`
	Duration     string `xml:"duration,attr,omitempty"`
	Bitrate      int    `xml:"bitrate,attr,omitempty"`
	SampleFreq   int    `xml:"sampleFrequency,attr,omitempty"`
	Channels     int    `xml:"nrAudioChannels,attr,omitempty"`
	URL          string `xml:",chardata"`
}

// UPnP object classes
const (
	classContainer        = "object.container"
	classStorageFolder    = "object.container.storageFolder"
	classMusicAlbum       = "object.container.album.musicAlbum"
	classMusicArtist      = "object.container.person.musicArtist"
	classMusicGenre       = "object.container.genre.musicGenre"
	classMusicTrack       = "object.item.audioItem.musicTrack"
	classPlaylistContainer = "object.container.playlistContainer"
)

// handleBrowse handles the ContentDirectory Browse action
func (r *Router) handleBrowse(ctx context.Context, body []byte) (*BrowseResponse, error) {
	// Parse Browse request
	var req BrowseRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		// Try to extract from nested structure
		type BrowseWrapper struct {
			Browse BrowseRequest `xml:"Browse"`
		}
		var wrapper BrowseWrapper
		if err := xml.Unmarshal(body, &wrapper); err != nil {
			return nil, fmt.Errorf("failed to parse Browse request: %w", err)
		}
		req = wrapper.Browse
	}

	log.Debug(ctx, "Browse request",
		"objectID", req.ObjectID,
		"browseFlag", req.BrowseFlag,
		"startIndex", req.StartingIndex,
		"count", req.RequestedCount)

	// Handle default count
	if req.RequestedCount == 0 {
		req.RequestedCount = 100
	}

	// Build DIDL-Lite response based on ObjectID
	didl := DIDLLite{
		Xmlns:     "urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/",
		XmlnsDC:   "http://purl.org/dc/elements/1.1/",
		XmlnsUPnP: "urn:schemas-upnp-org:metadata-1-0/upnp/",
	}

	var total int

	switch req.BrowseFlag {
	case "BrowseMetadata":
		didl, total = r.browseMetadata(ctx, req.ObjectID)
	case "BrowseDirectChildren":
		didl, total = r.browseDirectChildren(ctx, req.ObjectID, req.StartingIndex, req.RequestedCount)
	default:
		return nil, fmt.Errorf("invalid BrowseFlag: %s", req.BrowseFlag)
	}

	// Marshal DIDL-Lite to XML
	didlXML, err := xml.Marshal(didl)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal DIDL-Lite: %w", err)
	}

	return &BrowseResponse{
		Result:         html.EscapeString(string(didlXML)),
		NumberReturned: len(didl.Containers) + len(didl.Items),
		TotalMatches:   total,
		UpdateID:       r.getUpdateID(),
	}, nil
}

// browseMetadata returns metadata for a single object
func (r *Router) browseMetadata(ctx context.Context, objectID string) (DIDLLite, int) {
	didl := DIDLLite{
		Xmlns:     "urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/",
		XmlnsDC:   "http://purl.org/dc/elements/1.1/",
		XmlnsUPnP: "urn:schemas-upnp-org:metadata-1-0/upnp/",
	}

	switch objectID {
	case "0":
		didl.Containers = []Container{
			{ID: "0", ParentID: "-1", Restricted: "1", Title: r.serverName, Class: classContainer},
		}
	case "music":
		didl.Containers = []Container{
			{ID: "music", ParentID: "0", Restricted: "1", Title: "Music", Class: classStorageFolder},
		}
	case "music/artists":
		didl.Containers = []Container{
			{ID: "music/artists", ParentID: "music", Restricted: "1", Title: "Artists", Class: classStorageFolder},
		}
	case "music/albums":
		didl.Containers = []Container{
			{ID: "music/albums", ParentID: "music", Restricted: "1", Title: "Albums", Class: classStorageFolder},
		}
	case "music/genres":
		didl.Containers = []Container{
			{ID: "music/genres", ParentID: "music", Restricted: "1", Title: "Genres", Class: classStorageFolder},
		}
	case "music/playlists":
		didl.Containers = []Container{
			{ID: "music/playlists", ParentID: "music", Restricted: "1", Title: "Playlists", Class: classStorageFolder},
		}
	default:
		// Handle specific artist/album/track IDs
		// This will be expanded in Phase 2
		log.Debug(ctx, "Unknown objectID for metadata", "objectID", objectID)
	}

	return didl, 1
}

// browseDirectChildren returns children of a container
func (r *Router) browseDirectChildren(ctx context.Context, objectID string, startIndex, count int) (DIDLLite, int) {
	didl := DIDLLite{
		Xmlns:     "urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/",
		XmlnsDC:   "http://purl.org/dc/elements/1.1/",
		XmlnsUPnP: "urn:schemas-upnp-org:metadata-1-0/upnp/",
	}

	switch objectID {
	case "0":
		// Root - show Music folder
		didl.Containers = []Container{
			{ID: "music", ParentID: "0", Restricted: "1", Title: "Music", Class: classStorageFolder, ChildCount: 4},
		}
		return didl, 1

	case "music":
		// Music folder - show categories
		containers := []Container{
			{ID: "music/artists", ParentID: "music", Restricted: "1", Title: "Artists", Class: classStorageFolder},
			{ID: "music/albums", ParentID: "music", Restricted: "1", Title: "Albums", Class: classStorageFolder},
			{ID: "music/genres", ParentID: "music", Restricted: "1", Title: "Genres", Class: classStorageFolder},
			{ID: "music/playlists", ParentID: "music", Restricted: "1", Title: "Playlists", Class: classStorageFolder},
		}
		// Apply pagination
		end := startIndex + count
		if end > len(containers) {
			end = len(containers)
		}
		if startIndex < len(containers) {
			didl.Containers = containers[startIndex:end]
		}
		return didl, len(containers)

	case "music/artists":
		return r.browseArtists(ctx, startIndex, count)

	case "music/albums":
		return r.browseAlbums(ctx, startIndex, count, "")

	case "music/genres":
		return r.browseGenres(ctx, startIndex, count)

	case "music/playlists":
		return r.browsePlaylists(ctx, startIndex, count)

	default:
		// Check if it's an artist, album, genre, or playlist ID
		if strings.HasPrefix(objectID, "artist/") {
			artistID := strings.TrimPrefix(objectID, "artist/")
			return r.browseAlbums(ctx, startIndex, count, artistID)
		}
		if strings.HasPrefix(objectID, "album/") {
			albumID := strings.TrimPrefix(objectID, "album/")
			return r.browseTracks(ctx, albumID, startIndex, count)
		}
		if strings.HasPrefix(objectID, "genre/") {
			genreID := strings.TrimPrefix(objectID, "genre/")
			return r.browseGenreAlbums(ctx, genreID, startIndex, count)
		}
		if strings.HasPrefix(objectID, "playlist/") {
			playlistID := strings.TrimPrefix(objectID, "playlist/")
			return r.browsePlaylistTracks(ctx, playlistID, startIndex, count)
		}
	}

	return didl, 0
}

// browseArtists returns the list of artists
func (r *Router) browseArtists(ctx context.Context, startIndex, count int) (DIDLLite, int) {
	didl := DIDLLite{
		Xmlns:     "urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/",
		XmlnsDC:   "http://purl.org/dc/elements/1.1/",
		XmlnsUPnP: "urn:schemas-upnp-org:metadata-1-0/upnp/",
	}

	// Get artists from database
	artists, err := r.ds.Artist(ctx).GetAll()
	if err != nil {
		log.Error(ctx, "Failed to get artists", err)
		return didl, 0
	}

	total := len(artists)
	end := startIndex + count
	if end > total {
		end = total
	}

	if startIndex < total {
		for _, artist := range artists[startIndex:end] {
			didl.Containers = append(didl.Containers, Container{
				ID:         "artist/" + artist.ID,
				ParentID:   "music/artists",
				Restricted: "1",
				Title:      artist.Name,
				Class:      classMusicArtist,
			})
		}
	}

	return didl, total
}

// browseAlbums returns the list of albums (optionally filtered by artist)
func (r *Router) browseAlbums(ctx context.Context, startIndex, count int, artistID string) (DIDLLite, int) {
	didl := DIDLLite{
		Xmlns:     "urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/",
		XmlnsDC:   "http://purl.org/dc/elements/1.1/",
		XmlnsUPnP: "urn:schemas-upnp-org:metadata-1-0/upnp/",
	}

	// Build query options
	opts := model.QueryOptions{
		Sort:   "name",
		Offset: startIndex,
		Max:    count,
	}

	// Filter by artist if specified
	if artistID != "" {
		opts.Filters = squirrel.Eq{"album_artist_id": artistID}
	}

	// Get albums from database
	albums, err := r.ds.Album(ctx).GetAll(opts)
	if err != nil {
		log.Error(ctx, "Failed to get albums", err)
		return didl, 0
	}

	// Get total count
	total, err := r.ds.Album(ctx).CountAll(opts)
	if err != nil {
		log.Error(ctx, "Failed to count albums", err)
		total = int64(len(albums))
	}

	parentID := "music/albums"
	if artistID != "" {
		parentID = "artist/" + artistID
	}

	for _, album := range albums {
		artURL := r.getAlbumArtURL(album.ID)
		didl.Containers = append(didl.Containers, Container{
			ID:          "album/" + album.ID,
			ParentID:    parentID,
			Restricted:  "1",
			Title:       album.Name,
			Class:       classMusicAlbum,
			AlbumArtURI: artURL,
		})
	}

	return didl, int(total)
}

// browseGenres returns the list of genres
func (r *Router) browseGenres(ctx context.Context, startIndex, count int) (DIDLLite, int) {
	didl := DIDLLite{
		Xmlns:     "urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/",
		XmlnsDC:   "http://purl.org/dc/elements/1.1/",
		XmlnsUPnP: "urn:schemas-upnp-org:metadata-1-0/upnp/",
	}

	// Get genres from database
	genres, err := r.ds.Genre(ctx).GetAll()
	if err != nil {
		log.Error(ctx, "Failed to get genres", err)
		return didl, 0
	}

	total := len(genres)
	end := startIndex + count
	if end > total {
		end = total
	}

	if startIndex < total {
		for _, genre := range genres[startIndex:end] {
			didl.Containers = append(didl.Containers, Container{
				ID:         "genre/" + genre.ID,
				ParentID:   "music/genres",
				Restricted: "1",
				Title:      genre.Name,
				Class:      classMusicGenre,
			})
		}
	}

	return didl, total
}

// browseGenreAlbums returns albums in a genre
func (r *Router) browseGenreAlbums(ctx context.Context, genreID string, startIndex, count int) (DIDLLite, int) {
	didl := DIDLLite{
		Xmlns:     "urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/",
		XmlnsDC:   "http://purl.org/dc/elements/1.1/",
		XmlnsUPnP: "urn:schemas-upnp-org:metadata-1-0/upnp/",
	}

	// Build query options with genre filter
	opts := model.QueryOptions{
		Sort:    "name",
		Offset:  startIndex,
		Max:     count,
		Filters: squirrel.Eq{"genre_id": genreID},
	}

	// Get albums from database
	albums, err := r.ds.Album(ctx).GetAll(opts)
	if err != nil {
		log.Error(ctx, "Failed to get genre albums", err)
		return didl, 0
	}

	total, err := r.ds.Album(ctx).CountAll(opts)
	if err != nil {
		log.Error(ctx, "Failed to count genre albums", err)
		total = int64(len(albums))
	}

	for _, album := range albums {
		artURL := r.getAlbumArtURL(album.ID)
		didl.Containers = append(didl.Containers, Container{
			ID:          "album/" + album.ID,
			ParentID:    "genre/" + genreID,
			Restricted:  "1",
			Title:       album.Name,
			Class:       classMusicAlbum,
			AlbumArtURI: artURL,
		})
	}

	return didl, int(total)
}

// browsePlaylists returns the list of playlists
func (r *Router) browsePlaylists(ctx context.Context, startIndex, count int) (DIDLLite, int) {
	didl := DIDLLite{
		Xmlns:     "urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/",
		XmlnsDC:   "http://purl.org/dc/elements/1.1/",
		XmlnsUPnP: "urn:schemas-upnp-org:metadata-1-0/upnp/",
	}

	// Get playlists from database
	opts := model.QueryOptions{
		Sort:   "name",
		Offset: startIndex,
		Max:    count,
	}

	playlists, err := r.ds.Playlist(ctx).GetAll(opts)
	if err != nil {
		log.Error(ctx, "Failed to get playlists", err)
		return didl, 0
	}

	total, err := r.ds.Playlist(ctx).CountAll(opts)
	if err != nil {
		log.Error(ctx, "Failed to count playlists", err)
		total = int64(len(playlists))
	}

	for _, playlist := range playlists {
		didl.Containers = append(didl.Containers, Container{
			ID:         "playlist/" + playlist.ID,
			ParentID:   "music/playlists",
			Restricted: "1",
			Title:      playlist.Name,
			Class:      classPlaylistContainer,
			ChildCount: playlist.SongCount,
		})
	}

	return didl, int(total)
}

// browsePlaylistTracks returns tracks in a playlist
func (r *Router) browsePlaylistTracks(ctx context.Context, playlistID string, startIndex, count int) (DIDLLite, int) {
	didl := DIDLLite{
		Xmlns:     "urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/",
		XmlnsDC:   "http://purl.org/dc/elements/1.1/",
		XmlnsUPnP: "urn:schemas-upnp-org:metadata-1-0/upnp/",
	}

	// Get playlist with tracks
	playlist, err := r.ds.Playlist(ctx).GetWithTracks(playlistID, true, false)
	if err != nil {
		log.Error(ctx, "Failed to get playlist tracks", err)
		return didl, 0
	}

	total := len(playlist.Tracks)
	end := startIndex + count
	if end > total {
		end = total
	}

	if startIndex < total {
		for _, track := range playlist.Tracks[startIndex:end] {
			mf := track.MediaFile
			item := r.mediaFileToItem(&mf, "playlist/"+playlistID)
			didl.Items = append(didl.Items, item)
		}
	}

	return didl, total
}

// browseTracks returns tracks in an album
func (r *Router) browseTracks(ctx context.Context, albumID string, startIndex, count int) (DIDLLite, int) {
	didl := DIDLLite{
		Xmlns:     "urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/",
		XmlnsDC:   "http://purl.org/dc/elements/1.1/",
		XmlnsUPnP: "urn:schemas-upnp-org:metadata-1-0/upnp/",
	}

	// Build query options
	opts := model.QueryOptions{
		Sort:    "disc_number, track_number",
		Offset:  startIndex,
		Max:     count,
		Filters: squirrel.Eq{"album_id": albumID},
	}

	// Get tracks from database
	tracks, err := r.ds.MediaFile(ctx).GetAll(opts)
	if err != nil {
		log.Error(ctx, "Failed to get tracks", err)
		return didl, 0
	}

	total, err := r.ds.MediaFile(ctx).CountAll(opts)
	if err != nil {
		log.Error(ctx, "Failed to count tracks", err)
		total = int64(len(tracks))
	}

	for _, track := range tracks {
		item := r.mediaFileToItem(&track, "album/"+albumID)
		didl.Items = append(didl.Items, item)
	}

	return didl, int(total)
}

// mediaFileToItem converts a MediaFile to a DIDL-Lite Item
func (r *Router) mediaFileToItem(mf *model.MediaFile, parentID string) Item {
	item := Item{
		ID:          "track/" + mf.ID,
		ParentID:    parentID,
		Restricted:  "1",
		Title:       mf.Title,
		Creator:     mf.Artist,
		Album:       mf.Album,
		Artist:      mf.Artist,
		Class:       classMusicTrack,
		AlbumArtURI: r.getAlbumArtURL(mf.AlbumID),
		TrackNumber: mf.TrackNumber,
	}

	// Add genre if available
	if mf.Genre != "" {
		item.Genre = mf.Genre
	}

	// Add resource with streaming URL
	res := Res{
		ProtocolInfo: GetProtocolInfoForMimeType(mf.ContentType()),
		Size:         mf.Size,
		Duration:     formatDuration(float64(mf.Duration)),
		Bitrate:      mf.BitRate * 125, // Convert kbps to bytes/sec
		SampleFreq:   mf.SampleRate,
		Channels:     mf.Channels,
		URL:          r.getStreamURL(mf.ID),
	}
	item.Resources = []Res{res}

	return item
}

// getStreamURL returns the streaming URL for a media file
func (r *Router) getStreamURL(mediaFileID string) string {
	baseURL := conf.Server.BaseURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://%s:%d", getLocalIP(), r.httpPort)
	}
	return fmt.Sprintf("%s/rest/stream?id=%s&f=raw", baseURL, mediaFileID)
}

// handleGetSearchCapabilities returns search capabilities
func (r *Router) handleGetSearchCapabilities(ctx context.Context) (*GetSearchCapabilitiesResponse, error) {
	return &GetSearchCapabilitiesResponse{
		SearchCaps: "dc:title,dc:creator,upnp:artist,upnp:album,upnp:genre",
	}, nil
}

// handleGetSortCapabilities returns sort capabilities
func (r *Router) handleGetSortCapabilities(ctx context.Context) (*GetSortCapabilitiesResponse, error) {
	return &GetSortCapabilitiesResponse{
		SortCaps: "dc:title,dc:creator,upnp:artist,upnp:album,upnp:originalTrackNumber",
	}, nil
}

// handleGetSystemUpdateID returns the current system update ID
func (r *Router) handleGetSystemUpdateID(ctx context.Context) (*GetSystemUpdateIDResponse, error) {
	return &GetSystemUpdateIDResponse{
		Id: r.getUpdateID(),
	}, nil
}

// getUpdateID returns a system update ID (should increment when library changes)
func (r *Router) getUpdateID() uint32 {
	// For now, return a constant. In production, this should track library changes.
	return 1
}

// formatDuration formats a duration in seconds to DLNA format (H:MM:SS.mmm)
func formatDuration(seconds float64) string {
	h := int(seconds / 3600)
	m := int(seconds/60) % 60
	s := int(seconds) % 60
	ms := int((seconds - float64(int(seconds))) * 1000)
	return fmt.Sprintf("%d:%02d:%02d.%03d", h, m, s, ms)
}

// parseDuration parses a DLNA duration string to seconds
func parseDuration(duration string) float64 {
	parts := strings.Split(duration, ":")
	if len(parts) != 3 {
		return 0
	}

	h, _ := strconv.ParseFloat(parts[0], 64)
	m, _ := strconv.ParseFloat(parts[1], 64)

	// Handle seconds with milliseconds
	secParts := strings.Split(parts[2], ".")
	s, _ := strconv.ParseFloat(secParts[0], 64)

	ms := 0.0
	if len(secParts) > 1 {
		ms, _ = strconv.ParseFloat("0."+secParts[1], 64)
	}

	return h*3600 + m*60 + s + ms
}
