package sonos_cast

import (
	"errors"
	"fmt"
)

var (
	// ErrDeviceNotFound is returned when a device UUID is not in the cache
	ErrDeviceNotFound = errors.New("sonos device not found")

	// ErrNoDevices is returned when no Sonos devices are available
	ErrNoDevices = errors.New("no sonos devices discovered")

	// ErrNotCoordinator is returned when trying to control a non-coordinator speaker
	ErrNotCoordinator = errors.New("device is not a group coordinator")

	// ErrInvalidVolume is returned when volume is out of range
	ErrInvalidVolume = errors.New("volume must be between 0 and 100")

	// ErrPlaybackFailed is returned when playback control fails
	ErrPlaybackFailed = errors.New("playback control failed")
)

// UPnP error codes from Sonos/AVTransport specification
const (
	UPnPErrorTransitionNotAvailable = 701
	UPnPErrorNoContents             = 702
	UPnPErrorReadError              = 703
	UPnPErrorFormatNotSupported     = 704
	UPnPErrorTransportLocked        = 705
	UPnPErrorWriteError             = 706
	UPnPErrorProtectedContent       = 707
	UPnPErrorFormatMismatch         = 708
	UPnPErrorIllegalMIMEType        = 714
	UPnPErrorContentBusy            = 715
	UPnPErrorResourceNotFound       = 716
	UPnPErrorInvalidInstanceID      = 718
	UPnPErrorNotCoordinator         = 800
)

// UPnPError represents a SOAP fault from a Sonos device
type UPnPError struct {
	Code        int
	Description string
}

func (e *UPnPError) Error() string {
	return fmt.Sprintf("UPnP error %d: %s", e.Code, e.Description)
}

// upnpErrorDescription returns a human-readable description for UPnP error codes
func upnpErrorDescription(code int) string {
	switch code {
	case UPnPErrorTransitionNotAvailable:
		return "Speaker is grouped - ungroup it in Sonos app or select the group coordinator"
	case UPnPErrorNoContents:
		return "No contents"
	case UPnPErrorReadError:
		return "Read error"
	case UPnPErrorFormatNotSupported:
		return "Format not supported by device"
	case UPnPErrorTransportLocked:
		return "Transport is locked"
	case UPnPErrorWriteError:
		return "Write error"
	case UPnPErrorProtectedContent:
		return "Content is protected"
	case UPnPErrorFormatMismatch:
		return "Format mismatch"
	case UPnPErrorIllegalMIMEType:
		return "Illegal MIME type (check Content-Type header from stream server)"
	case UPnPErrorContentBusy:
		return "Content is busy"
	case UPnPErrorResourceNotFound:
		return "Resource not found (URL unreachable from Sonos network)"
	case UPnPErrorInvalidInstanceID:
		return "Invalid InstanceID (must be 0)"
	case UPnPErrorNotCoordinator:
		return "Not a coordinator (send command to group coordinator)"
	default:
		return "Unknown error"
	}
}
