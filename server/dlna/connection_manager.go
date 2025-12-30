package dlna

import (
	"context"
	"encoding/xml"
)

// ConnectionManager request/response structures

// GetProtocolInfoResponse for GetProtocolInfo action
type GetProtocolInfoResponse struct {
	XMLName xml.Name `xml:"urn:schemas-upnp-org:service:ConnectionManager:1 GetProtocolInfoResponse"`
	Source  string   `xml:"Source"`
	Sink    string   `xml:"Sink"`
}

// GetCurrentConnectionIDsResponse for GetCurrentConnectionIDs action
type GetCurrentConnectionIDsResponse struct {
	XMLName       xml.Name `xml:"urn:schemas-upnp-org:service:ConnectionManager:1 GetCurrentConnectionIDsResponse"`
	ConnectionIDs string   `xml:"ConnectionIDs"`
}

// GetCurrentConnectionInfoRequest for GetCurrentConnectionInfo action
type GetCurrentConnectionInfoRequest struct {
	XMLName      xml.Name `xml:"GetCurrentConnectionInfo"`
	ConnectionID int      `xml:"ConnectionID"`
}

// GetCurrentConnectionInfoResponse for GetCurrentConnectionInfo action
type GetCurrentConnectionInfoResponse struct {
	XMLName               xml.Name `xml:"urn:schemas-upnp-org:service:ConnectionManager:1 GetCurrentConnectionInfoResponse"`
	RcsID                 int      `xml:"RcsID"`
	AVTransportID         int      `xml:"AVTransportID"`
	ProtocolInfo          string   `xml:"ProtocolInfo"`
	PeerConnectionManager string   `xml:"PeerConnectionManager"`
	PeerConnectionID      int      `xml:"PeerConnectionID"`
	Direction             string   `xml:"Direction"`
	Status                string   `xml:"Status"`
}

// Supported audio protocol info strings for DLNA
// Format: protocol:network:contentFormat:additionalInfo
const (
	// Common audio formats
	protoInfoMP3       = "http-get:*:audio/mpeg:DLNA.ORG_PN=MP3;DLNA.ORG_OP=01;DLNA.ORG_FLAGS=01700000000000000000000000000000"
	protoInfoFLAC      = "http-get:*:audio/flac:*"
	protoInfoWAV       = "http-get:*:audio/wav:*"
	protoInfoWAVPCM    = "http-get:*:audio/L16:DLNA.ORG_PN=LPCM;DLNA.ORG_OP=01;DLNA.ORG_FLAGS=01700000000000000000000000000000"
	protoInfoAAC       = "http-get:*:audio/aac:*"
	protoInfoM4A       = "http-get:*:audio/mp4:DLNA.ORG_PN=AAC_ISO_320;DLNA.ORG_OP=01;DLNA.ORG_FLAGS=01700000000000000000000000000000"
	protoInfoOGG       = "http-get:*:audio/ogg:*"
	protoInfoOPUS      = "http-get:*:audio/opus:*"
	protoInfoWMA       = "http-get:*:audio/x-ms-wma:DLNA.ORG_PN=WMABASE;DLNA.ORG_OP=01;DLNA.ORG_FLAGS=01700000000000000000000000000000"

	// Generic audio catch-all
	protoInfoGenericAudio = "http-get:*:audio/*:*"
)

// handleGetProtocolInfo returns the supported protocols for streaming
func (r *Router) handleGetProtocolInfo(ctx context.Context) (*GetProtocolInfoResponse, error) {
	// Source protocols - what we can stream
	sourceProtocols := []string{
		protoInfoMP3,
		protoInfoFLAC,
		protoInfoWAV,
		protoInfoWAVPCM,
		protoInfoAAC,
		protoInfoM4A,
		protoInfoOGG,
		protoInfoOPUS,
		protoInfoWMA,
		protoInfoGenericAudio,
	}

	return &GetProtocolInfoResponse{
		Source: joinProtocols(sourceProtocols),
		Sink:   "", // We don't receive streams, only serve
	}, nil
}

// handleGetCurrentConnectionIDs returns active connection IDs
func (r *Router) handleGetCurrentConnectionIDs(ctx context.Context) (*GetCurrentConnectionIDsResponse, error) {
	// Return connection ID 0 (default connection)
	return &GetCurrentConnectionIDsResponse{
		ConnectionIDs: "0",
	}, nil
}

// handleGetCurrentConnectionInfo returns info about a specific connection
func (r *Router) handleGetCurrentConnectionInfo(ctx context.Context, body []byte) (*GetCurrentConnectionInfoResponse, error) {
	// For connection ID 0 (default), return standard info
	return &GetCurrentConnectionInfoResponse{
		RcsID:                 -1,
		AVTransportID:         -1,
		ProtocolInfo:          "",
		PeerConnectionManager: "",
		PeerConnectionID:      -1,
		Direction:             "Output",
		Status:                "OK",
	}, nil
}

// joinProtocols joins protocol info strings with commas
func joinProtocols(protocols []string) string {
	result := ""
	for i, p := range protocols {
		if i > 0 {
			result += ","
		}
		result += p
	}
	return result
}

// GetProtocolInfoForMimeType returns the DLNA protocol info string for a given MIME type
func GetProtocolInfoForMimeType(mimeType string) string {
	switch mimeType {
	case "audio/mpeg", "audio/mp3":
		return protoInfoMP3
	case "audio/flac", "audio/x-flac":
		return protoInfoFLAC
	case "audio/wav", "audio/x-wav", "audio/wave":
		return protoInfoWAV
	case "audio/L16":
		return protoInfoWAVPCM
	case "audio/aac", "audio/x-aac":
		return protoInfoAAC
	case "audio/mp4", "audio/x-m4a", "audio/m4a":
		return protoInfoM4A
	case "audio/ogg", "audio/x-ogg", "application/ogg":
		return protoInfoOGG
	case "audio/opus":
		return protoInfoOPUS
	case "audio/x-ms-wma", "audio/wma":
		return protoInfoWMA
	default:
		return protoInfoGenericAudio
	}
}
