package sonos

import "encoding/xml"

// SOAP Envelope structures for SMAPI
const (
	NamespaceSoap   = "http://schemas.xmlsoap.org/soap/envelope/"
	NamespaceSonos  = "http://www.sonos.com/Services/1.1"
	ContentTypeSOAP = `text/xml; charset="utf-8"`
)

// Envelope represents a SOAP envelope
type Envelope struct {
	XMLName xml.Name `xml:"http://schemas.xmlsoap.org/soap/envelope/ Envelope"`
	Header  *Header  `xml:"Header,omitempty"`
	Body    Body     `xml:"Body"`
}

// Header contains SOAP header with credentials
type Header struct {
	XMLName     xml.Name     `xml:"http://schemas.xmlsoap.org/soap/envelope/ Header"`
	Credentials *Credentials `xml:"credentials,omitempty"`
}

// Credentials contains authentication info from Sonos
type Credentials struct {
	XMLName      xml.Name `xml:"http://www.sonos.com/Services/1.1 credentials"`
	SessionID    string   `xml:"sessionId,omitempty"`
	LoginToken   *Token   `xml:"loginToken,omitempty"`
	DeviceID     string   `xml:"deviceId,omitempty"`
	DeviceCert   string   `xml:"deviceCert,omitempty"`
	ZonePlayerID string   `xml:"zonePlayerId,omitempty"`
	HouseholdID  string   `xml:"householdId,omitempty"`
}

// Token represents authentication token
type Token struct {
	Token       string `xml:"token,omitempty"`
	Key         string `xml:"key,omitempty"`
	HouseholdID string `xml:"householdId,omitempty"`
}

// Body represents SOAP body
type Body struct {
	XMLName xml.Name    `xml:"http://schemas.xmlsoap.org/soap/envelope/ Body"`
	Content interface{} `xml:",any"`
	Fault   *Fault      `xml:"Fault,omitempty"`
}

// Fault represents a SOAP fault
type Fault struct {
	XMLName     xml.Name     `xml:"http://schemas.xmlsoap.org/soap/envelope/ Fault"`
	FaultCode   string       `xml:"faultcode"`
	FaultString string       `xml:"faultstring"`
	Detail      *FaultDetail `xml:"detail,omitempty"`
}

// FaultDetail contains SMAPI-specific error details
type FaultDetail struct {
	SonosError *SonosError `xml:"SonosError,omitempty"`
}

// SonosError represents Sonos-specific error
type SonosError struct {
	XMLName   xml.Name `xml:"http://www.sonos.com/Services/1.1 SonosError"`
	ErrorCode string   `xml:"errorCode"`
	Message   string   `xml:"message,omitempty"`
}

// SMAPI Error Codes
const (
	ErrLoginInvalid      = "Client.LoginInvalid"
	ErrLoginUnauthorized = "Client.NOT_LINKED_RETRY"
	ErrSessionInvalid    = "Client.SessionIdInvalid"
	ErrItemNotFound      = "Client.ItemNotFound"
	ErrServiceUnavail    = "Server.ServiceUnknownError"
)

// =============================================================================
// SMAPI Request Types
// =============================================================================

// GetMetadataRequest - browse content
type GetMetadataRequest struct {
	XMLName   xml.Name `xml:"http://www.sonos.com/Services/1.1 getMetadata"`
	ID        string   `xml:"id"`
	Index     int      `xml:"index"`
	Count     int      `xml:"count"`
	Recursive bool     `xml:"recursive,omitempty"`
}

// GetMediaMetadataRequest - get single item details
type GetMediaMetadataRequest struct {
	XMLName xml.Name `xml:"http://www.sonos.com/Services/1.1 getMediaMetadata"`
	ID      string   `xml:"id"`
}

// GetMediaURIRequest - get streaming URL
type GetMediaURIRequest struct {
	XMLName xml.Name `xml:"http://www.sonos.com/Services/1.1 getMediaURI"`
	ID      string   `xml:"id"`
}

// SearchRequest - search content
type SearchRequest struct {
	XMLName xml.Name `xml:"http://www.sonos.com/Services/1.1 search"`
	ID      string   `xml:"id"`
	Term    string   `xml:"term"`
	Index   int      `xml:"index"`
	Count   int      `xml:"count"`
}

// GetExtendedMetadataRequest - get extended info
type GetExtendedMetadataRequest struct {
	XMLName xml.Name `xml:"http://www.sonos.com/Services/1.1 getExtendedMetadata"`
	ID      string   `xml:"id"`
}

// GetLastUpdateRequest - check for updates
type GetLastUpdateRequest struct {
	XMLName xml.Name `xml:"http://www.sonos.com/Services/1.1 getLastUpdate"`
}

// GetAppLinkRequest - start device linking
type GetAppLinkRequest struct {
	XMLName   xml.Name `xml:"http://www.sonos.com/Services/1.1 getAppLink"`
	Household string   `xml:"householdId"`
}

// GetDeviceAuthTokenRequest - exchange link code for token
type GetDeviceAuthTokenRequest struct {
	XMLName   xml.Name `xml:"http://www.sonos.com/Services/1.1 getDeviceAuthToken"`
	Household string   `xml:"householdId"`
	LinkCode  string   `xml:"linkCode"`
}

// RateItemRequest - rate a track
type RateItemRequest struct {
	XMLName xml.Name `xml:"http://www.sonos.com/Services/1.1 rateItem"`
	ID      string   `xml:"id"`
	Rating  int      `xml:"rating"`
}

// SetPlayedSecondsRequest - report playback progress
type SetPlayedSecondsRequest struct {
	XMLName       xml.Name `xml:"http://www.sonos.com/Services/1.1 setPlayedSeconds"`
	ID            string   `xml:"id"`
	Seconds       int      `xml:"seconds"`
	ContextID     string   `xml:"contextId,omitempty"`
	PrivateData   string   `xml:"privateData,omitempty"`
	OffsetMillis  int      `xml:"offsetMillis,omitempty"`
}

// =============================================================================
// SMAPI Response Types
// =============================================================================

// GetMetadataResponse - browse response
type GetMetadataResponse struct {
	XMLName           xml.Name `xml:"getMetadataResult"`
	Index             int      `xml:"index"`
	Count             int      `xml:"count"`
	Total             int      `xml:"total"`
	MediaCollection   []MediaCollection `xml:"mediaCollection,omitempty"`
	MediaMetadata     []MediaMetadata   `xml:"mediaMetadata,omitempty"`
}

// MediaCollection represents a container (album, artist, playlist, etc.)
type MediaCollection struct {
	XMLName       xml.Name `xml:"mediaCollection"`
	ID            string   `xml:"id"`
	ItemType      string   `xml:"itemType"` // collection, album, artist, playlist, etc.
	Title         string   `xml:"title"`
	Artist        string   `xml:"artist,omitempty"`
	AlbumArtURI   string   `xml:"albumArtURI,omitempty"`
	CanPlay       bool     `xml:"canPlay,omitempty"`
	CanEnumerate  bool     `xml:"canEnumerate,omitempty"`
	CanAddToFav   bool     `xml:"canAddToFavorites,omitempty"`
	ContainsText  bool     `xml:"containsText,omitempty"`
}

// MediaMetadata represents a playable item (track)
type MediaMetadata struct {
	XMLName       xml.Name       `xml:"mediaMetadata"`
	ID            string         `xml:"id"`
	ItemType      string         `xml:"itemType"` // track
	Title         string         `xml:"title"`
	MimeType      string         `xml:"mimeType,omitempty"`
	TrackMetadata *TrackMetadata `xml:"trackMetadata,omitempty"`
}

// TrackMetadata contains track-specific info
type TrackMetadata struct {
	AlbumID       string `xml:"albumId,omitempty"`
	AlbumArtURI   string `xml:"albumArtURI,omitempty"`
	Album         string `xml:"album,omitempty"`
	ArtistID      string `xml:"artistId,omitempty"`
	Artist        string `xml:"artist,omitempty"`
	Duration      int    `xml:"duration,omitempty"` // seconds
	Genre         string `xml:"genre,omitempty"`
	TrackNumber   int    `xml:"trackNumber,omitempty"`
	CanPlay       bool   `xml:"canPlay,omitempty"`
	CanAddToFav   bool   `xml:"canAddToFavorites,omitempty"`
}

// GetMediaMetadataResponse - single item response
type GetMediaMetadataResponse struct {
	XMLName       xml.Name       `xml:"getMediaMetadataResult"`
	MediaMetadata *MediaMetadata `xml:"mediaMetadata,omitempty"`
}

// GetMediaURIResponse - streaming URL response
type GetMediaURIResponse struct {
	XMLName          xml.Name `xml:"getMediaURIResult"`
	URI              string   `xml:"getMediaURIResult"`
	HTTPHeaders      *HTTPHeaders `xml:"httpHeaders,omitempty"`
	PositionInfo     *PositionInfo `xml:"positionInformation,omitempty"`
}

// HTTPHeaders for authenticated streams
type HTTPHeaders struct {
	Header []HTTPHeader `xml:"httpHeader,omitempty"`
}

// HTTPHeader single header
type HTTPHeader struct {
	Name  string `xml:"header"`
	Value string `xml:"value"`
}

// PositionInfo for resume playback
type PositionInfo struct {
	ID            string `xml:"id,omitempty"`
	Index         int    `xml:"index,omitempty"`
	OffsetMillis  int    `xml:"offsetMillis,omitempty"`
}

// SearchResponse - search results
type SearchResponse struct {
	XMLName       xml.Name `xml:"searchResult"`
	Index         int      `xml:"index"`
	Count         int      `xml:"count"`
	Total         int      `xml:"total"`
	MediaCollection []MediaCollection `xml:"mediaCollection,omitempty"`
	MediaMetadata   []MediaMetadata   `xml:"mediaMetadata,omitempty"`
}

// GetLastUpdateResponse - catalog freshness
type GetLastUpdateResponse struct {
	XMLName        xml.Name `xml:"getLastUpdateResult"`
	Catalog        string   `xml:"catalog"`
	Favorites      string   `xml:"favorites"`
	PollInterval   int      `xml:"pollInterval"` // seconds
}

// GetAppLinkResponse - device linking
type GetAppLinkResponse struct {
	XMLName     xml.Name     `xml:"getAppLinkResult"`
	AppLinkInfo *AppLinkInfo `xml:"getAppLinkResult"`
}

// AppLinkInfo for device linking
type AppLinkInfo struct {
	RegURL        string `xml:"regUrl"`
	LinkCode      string `xml:"linkCode"`
	ShowLinkCode  bool   `xml:"showLinkCode"`
}

// GetDeviceAuthTokenResponse - auth token
type GetDeviceAuthTokenResponse struct {
	XMLName   xml.Name `xml:"getDeviceAuthTokenResult"`
	AuthToken string   `xml:"authToken"`
	PrivKey   string   `xml:"privateKey,omitempty"`
}

// RateItemResponse - rating confirmation
type RateItemResponse struct {
	XMLName         xml.Name         `xml:"rateItemResult"`
	UserRating      *UserRating      `xml:"userRating,omitempty"`
	ShouldSkip      bool             `xml:"shouldSkip,omitempty"`
}

// UserRating feedback
type UserRating struct {
	Rating int `xml:"rating"`
}

// SetPlayedSecondsResponse - scrobble confirmation
type SetPlayedSecondsResponse struct {
	XMLName xml.Name `xml:"setPlayedSecondsResult"`
}

// =============================================================================
// Navigation IDs
// =============================================================================

// Navigation root IDs for Sonos browsing
const (
	RootID            = "root"
	ArtistsID         = "artists"
	AlbumsID          = "albums"
	TracksID          = "tracks"
	PlaylistsID       = "playlists"
	GenresID          = "genres"
	RecentlyAddedID   = "recent"
	RecentlyPlayedID  = "recentlyPlayed"
	FavoritesID       = "favorites"
	RandomID          = "random"
)
