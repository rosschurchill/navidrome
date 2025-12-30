package dlna

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/navidrome/navidrome/log"
)

// SOAP envelope structures

// SOAPEnvelope represents a SOAP envelope
type SOAPEnvelope struct {
	XMLName xml.Name `xml:"http://schemas.xmlsoap.org/soap/envelope/ Envelope"`
	Body    SOAPBody
}

// SOAPBody represents the SOAP body
type SOAPBody struct {
	XMLName xml.Name `xml:"http://schemas.xmlsoap.org/soap/envelope/ Body"`
	Content []byte   `xml:",innerxml"`
}

// SOAPFault represents a SOAP fault
type SOAPFault struct {
	XMLName     xml.Name `xml:"http://schemas.xmlsoap.org/soap/envelope/ Fault"`
	FaultCode   string   `xml:"faultcode"`
	FaultString string   `xml:"faultstring"`
	Detail      string   `xml:"detail,omitempty"`
}

// UPnP error codes
const (
	upnpErrorInvalidAction     = 401
	upnpErrorInvalidArgs       = 402
	upnpErrorActionFailed      = 501
	upnpErrorNoSuchObject      = 701
	upnpErrorInvalidSort       = 709
	upnpErrorInvalidContainer  = 710
	upnpErrorRestrictedObject  = 711
	upnpErrorBadMetadata       = 712
	upnpErrorRestrictedParent  = 713
	upnpErrorNoSuchSourceRes   = 714
	upnpErrorSourceResourceAcc = 715
	upnpErrorTransferBusy      = 716
	upnpErrorNoSuchFileTransf  = 717
	upnpErrorNoSuchDestRes     = 718
	upnpErrorDestResourceAcc   = 719
	upnpErrorCannotProcess     = 720
)

// handleContentDirectoryControl handles SOAP requests for ContentDirectory service
func (r *Router) handleContentDirectoryControl(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	// Read request body
	body, err := io.ReadAll(req.Body)
	if err != nil {
		log.Error(ctx, "Failed to read SOAP request", err)
		r.writeSOAPFault(w, upnpErrorActionFailed, "Failed to read request")
		return
	}

	// Parse SOAP envelope
	var envelope SOAPEnvelope
	if err := xml.Unmarshal(body, &envelope); err != nil {
		log.Error(ctx, "Failed to parse SOAP envelope", err, "body", string(body))
		r.writeSOAPFault(w, upnpErrorActionFailed, "Invalid SOAP envelope")
		return
	}

	// Determine action from SOAPAction header
	soapAction := strings.Trim(req.Header.Get("SOAPAction"), `"`)
	action := extractActionName(soapAction)

	log.Debug(ctx, "ContentDirectory request", "action", action)

	// Route to appropriate handler
	var response interface{}
	switch action {
	case "Browse":
		response, err = r.handleBrowse(ctx, envelope.Body.Content)
	case "GetSearchCapabilities":
		response, err = r.handleGetSearchCapabilities(ctx)
	case "GetSortCapabilities":
		response, err = r.handleGetSortCapabilities(ctx)
	case "GetSystemUpdateID":
		response, err = r.handleGetSystemUpdateID(ctx)
	default:
		log.Warn(ctx, "Unknown ContentDirectory action", "action", action)
		r.writeSOAPFault(w, upnpErrorInvalidAction, fmt.Sprintf("Unknown action: %s", action))
		return
	}

	if err != nil {
		log.Error(ctx, "ContentDirectory action failed", err, "action", action)
		r.writeSOAPFault(w, upnpErrorActionFailed, err.Error())
		return
	}

	r.writeSOAPResponse(w, response)
}

// handleConnectionManagerControl handles SOAP requests for ConnectionManager service
func (r *Router) handleConnectionManagerControl(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	// Read request body
	body, err := io.ReadAll(req.Body)
	if err != nil {
		log.Error(ctx, "Failed to read SOAP request", err)
		r.writeSOAPFault(w, upnpErrorActionFailed, "Failed to read request")
		return
	}

	// Parse SOAP envelope
	var envelope SOAPEnvelope
	if err := xml.Unmarshal(body, &envelope); err != nil {
		log.Error(ctx, "Failed to parse SOAP envelope", err, "body", string(body))
		r.writeSOAPFault(w, upnpErrorActionFailed, "Invalid SOAP envelope")
		return
	}

	// Determine action from SOAPAction header
	soapAction := strings.Trim(req.Header.Get("SOAPAction"), `"`)
	action := extractActionName(soapAction)

	log.Debug(ctx, "ConnectionManager request", "action", action)

	// Route to appropriate handler
	var response interface{}
	switch action {
	case "GetProtocolInfo":
		response, err = r.handleGetProtocolInfo(ctx)
	case "GetCurrentConnectionIDs":
		response, err = r.handleGetCurrentConnectionIDs(ctx)
	case "GetCurrentConnectionInfo":
		response, err = r.handleGetCurrentConnectionInfo(ctx, envelope.Body.Content)
	default:
		log.Warn(ctx, "Unknown ConnectionManager action", "action", action)
		r.writeSOAPFault(w, upnpErrorInvalidAction, fmt.Sprintf("Unknown action: %s", action))
		return
	}

	if err != nil {
		log.Error(ctx, "ConnectionManager action failed", err, "action", action)
		r.writeSOAPFault(w, upnpErrorActionFailed, err.Error())
		return
	}

	r.writeSOAPResponse(w, response)
}

// writeSOAPResponse writes a successful SOAP response
func (r *Router) writeSOAPResponse(w http.ResponseWriter, result interface{}) {
	// Wrap in SOAP envelope
	respBody, err := xml.Marshal(result)
	if err != nil {
		r.writeSOAPFault(w, upnpErrorActionFailed, "Failed to marshal response")
		return
	}

	envelope := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    %s
  </s:Body>
</s:Envelope>`, string(respBody))

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("Ext", "")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(envelope))
}

// writeSOAPFault writes a SOAP fault response
func (r *Router) writeSOAPFault(w http.ResponseWriter, code int, message string) {
	fault := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <s:Fault>
      <faultcode>s:Client</faultcode>
      <faultstring>UPnPError</faultstring>
      <detail>
        <UPnPError xmlns="urn:schemas-upnp-org:control-1-0">
          <errorCode>%d</errorCode>
          <errorDescription>%s</errorDescription>
        </UPnPError>
      </detail>
    </s:Fault>
  </s:Body>
</s:Envelope>`, code, message)

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(fault))
}

// extractActionName extracts the action name from a SOAPAction header
func extractActionName(soapAction string) string {
	// SOAPAction format: "urn:schemas-upnp-org:service:ContentDirectory:1#Browse"
	if idx := strings.LastIndex(soapAction, "#"); idx >= 0 {
		return soapAction[idx+1:]
	}
	return soapAction
}
