package sonos

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/artwork"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

// Router handles Sonos SMAPI requests
type Router struct {
	ds          model.DataStore
	artwork     artwork.Artwork
	baseURL     string
	serviceName string
}

// New creates a new Sonos SMAPI router
func New(ds model.DataStore, artwork artwork.Artwork) *Router {
	serviceName := conf.Server.Sonos.ServiceName
	if serviceName == "" {
		serviceName = "Navidrome"
	}
	return &Router{
		ds:          ds,
		artwork:     artwork,
		serviceName: serviceName,
	}
}

// Routes returns the chi router for Sonos SMAPI
func (r *Router) Routes() chi.Router {
	router := chi.NewRouter()

	// SMAPI SOAP endpoint
	router.Post("/ws/sonos", r.handleSOAP)
	router.Get("/ws/sonos", r.handleWSDL)

	// Strings (localization) endpoint
	router.Get("/strings.xml", r.handleStrings)

	// Presentation map (UI customization)
	router.Get("/presentationMap.xml", r.handlePresentationMap)

	// Device registration endpoint
	router.Get("/register", r.handleRegisterPage)
	router.Post("/register", r.handleRegister)

	// Link code callback (user authentication)
	router.Get("/link", r.handleLinkPage)
	router.Post("/link", r.handleLink)

	return router
}

// handleSOAP processes SOAP requests
func (r *Router) handleSOAP(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	// Read request body
	body, err := io.ReadAll(req.Body)
	if err != nil {
		log.Error(ctx, "Failed to read SOAP request body", err)
		r.writeSOAPFault(w, ErrServiceUnavail, "Failed to read request")
		return
	}

	// Parse SOAP envelope
	var envelope Envelope
	if err := xml.Unmarshal(body, &envelope); err != nil {
		log.Error(ctx, "Failed to parse SOAP envelope", err, "body", string(body))
		r.writeSOAPFault(w, ErrServiceUnavail, "Invalid SOAP envelope")
		return
	}

	// Extract credentials from header
	var creds *Credentials
	if envelope.Header != nil {
		creds = envelope.Header.Credentials
	}

	// Determine operation from SOAPAction header or body content
	soapAction := strings.Trim(req.Header.Get("SOAPAction"), `"`)
	if soapAction == "" {
		// Try to detect from body
		soapAction = r.detectOperation(body)
	}

	log.Debug(ctx, "SMAPI request", "action", soapAction, "hasCredentials", creds != nil)

	// Route to appropriate handler
	var response interface{}
	switch {
	case strings.Contains(soapAction, "getMetadata"):
		response, err = r.handleGetMetadata(ctx, body, creds)
	case strings.Contains(soapAction, "getMediaMetadata"):
		response, err = r.handleGetMediaMetadata(ctx, body, creds)
	case strings.Contains(soapAction, "getMediaURI"):
		response, err = r.handleGetMediaURI(ctx, body, creds, req)
	case strings.Contains(soapAction, "search"):
		response, err = r.handleSearch(ctx, body, creds)
	case strings.Contains(soapAction, "getExtendedMetadata"):
		response, err = r.handleGetExtendedMetadata(ctx, body, creds)
	case strings.Contains(soapAction, "getLastUpdate"):
		response, err = r.handleGetLastUpdate(ctx, body, creds)
	case strings.Contains(soapAction, "getAppLink"):
		response, err = r.handleGetAppLink(ctx, body, creds, req)
	case strings.Contains(soapAction, "getDeviceAuthToken"):
		response, err = r.handleGetDeviceAuthToken(ctx, body, creds, req)
	case strings.Contains(soapAction, "rateItem"):
		response, err = r.handleRateItem(ctx, body, creds)
	case strings.Contains(soapAction, "setPlayedSeconds"):
		response, err = r.handleSetPlayedSeconds(ctx, body, creds)
	default:
		log.Warn(ctx, "Unknown SMAPI operation", "action", soapAction)
		r.writeSOAPFault(w, ErrServiceUnavail, fmt.Sprintf("Unknown operation: %s", soapAction))
		return
	}

	if err != nil {
		log.Error(ctx, "SMAPI operation failed", err, "action", soapAction)
		r.writeSOAPFault(w, ErrServiceUnavail, err.Error())
		return
	}

	r.writeSOAPResponse(w, response)
}

// detectOperation tries to detect SOAP operation from body
func (r *Router) detectOperation(body []byte) string {
	operations := []string{
		"getMetadata", "getMediaMetadata", "getMediaURI", "search",
		"getExtendedMetadata", "getLastUpdate", "getAppLink",
		"getDeviceAuthToken", "rateItem", "setPlayedSeconds",
		"createContainer", "deleteContainer", "addToContainer", "removeFromContainer",
	}
	for _, op := range operations {
		if bytes.Contains(body, []byte(op)) {
			return op
		}
	}
	return ""
}

// writeSOAPResponse writes a successful SOAP response
func (r *Router) writeSOAPResponse(w http.ResponseWriter, result interface{}) {
	envelope := Envelope{
		Body: Body{Content: result},
	}

	w.Header().Set("Content-Type", ContentTypeSOAP)
	w.WriteHeader(http.StatusOK)

	// Write XML declaration
	w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(envelope); err != nil {
		log.Error("Failed to encode SOAP response", err)
	}
}

// writeSOAPFault writes a SOAP fault response
func (r *Router) writeSOAPFault(w http.ResponseWriter, code, message string) {
	fault := &Fault{
		FaultCode:   code,
		FaultString: message,
	}

	envelope := Envelope{
		Body: Body{Fault: fault},
	}

	w.Header().Set("Content-Type", ContentTypeSOAP)
	w.WriteHeader(http.StatusInternalServerError)

	w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	enc.Encode(envelope)
}

// handleWSDL serves the SMAPI WSDL file
func (r *Router) handleWSDL(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Write([]byte(smapiWSDL))
}

// handleStrings serves localization strings
func (r *Router) handleStrings(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	// Escape serviceName for XML output to prevent injection
	escapedName := html.EscapeString(r.serviceName)
	fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?>
<stringtables xmlns="http://sonos.com/sonosapi">
  <stringtable rev="1" xml:lang="en-US">
    <string stringId="ServiceName">%s</string>
    <string stringId="AppLinkMessage">Please log in to %s to link your Sonos system.</string>
  </stringtable>
</stringtables>`, escapedName, escapedName)
}

// handlePresentationMap serves UI customization
func (r *Router) handlePresentationMap(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	fmt.Fprint(w, `<?xml version="1.0" encoding="utf-8"?>
<Presentation xmlns="http://sonos.com/sonosapi">
  <PresentationMap type="BrowseIconSizeMap">
    <Match>
      <browseIconSizeMap>
        <sizeEntry size="square" substitution="_size_180"/>
        <sizeEntry size="legacy" substitution=""/>
      </browseIconSizeMap>
    </Match>
  </PresentationMap>
</Presentation>`)
}

// handleRegisterPage shows device registration page
func (r *Router) handleRegisterPage(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Escape all user-configurable values for HTML output
	escapedName := html.EscapeString(r.serviceName)
	escapedURL := html.EscapeString(conf.Server.BaseURL)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Register %s with Sonos</title></head>
<body>
<h1>Register %s with Sonos</h1>
<p>Use the Sonos app to add this music service.</p>
<p>Service URL: %s/sonos</p>
</body>
</html>`, escapedName, escapedName, escapedURL)
}

// handleRegister processes device registration (not typically used directly)
func (r *Router) handleRegister(w http.ResponseWriter, req *http.Request) {
	// Registration is typically done via Sonos app's customsd endpoint
	w.WriteHeader(http.StatusOK)
}

// handleLinkPage shows device linking page
func (r *Router) handleLinkPage(w http.ResponseWriter, req *http.Request) {
	linkCode := req.URL.Query().Get("linkCode")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Escape all values for HTML output - linkCode is user-supplied!
	escapedName := html.EscapeString(r.serviceName)
	escapedLinkCode := html.EscapeString(linkCode)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Link %s to Sonos</title></head>
<body>
<h1>Link %s to Sonos</h1>
<form method="POST">
  <input type="hidden" name="linkCode" value="%s">
  <p>Enter your Navidrome credentials to link this Sonos device:</p>
  <p><label>Username: <input type="text" name="username" required></label></p>
  <p><label>Password: <input type="password" name="password" required></label></p>
  <p><button type="submit">Link Account</button></p>
</form>
</body>
</html>`, escapedName, escapedName, escapedLinkCode)
}

// handleLink processes device linking with security hardening
func (r *Router) handleLink(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	// Rate limiting check
	clientIP := GetRemoteIP(req)
	if authRateLimiter.checkRateLimit(clientIP) {
		log.Warn(ctx, "Sonos link: rate limited", "ip", clientIP)
		http.Error(w, "Too many attempts. Please try again later.", http.StatusTooManyRequests)
		return
	}

	// Parse form data
	if err := req.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	linkCode := req.FormValue("linkCode")
	username := req.FormValue("username")
	password := req.FormValue("password")

	// Record this attempt for rate limiting
	authRateLimiter.recordAttempt(clientIP)

	// Validate credentials using secure bcrypt comparison
	user, err := r.validateUserPassword(ctx, username, password)
	if err != nil {
		log.Warn(ctx, "Sonos link: authentication failed", "username", username, "ip", clientIP)
		// Use generic error message to prevent username enumeration
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Store link code -> user mapping
	storeLinkCode(linkCode, user.ID, user.UserName)

	log.Info(ctx, "Sonos link code created", "username", username, "ip", clientIP)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Use html.EscapeString to prevent XSS
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Success</title></head>
<body>
<h1>Successfully Linked!</h1>
<p>Your Sonos system is now linked to %s as user "%s".</p>
<p>You can close this window and return to your Sonos app.</p>
</body>
</html>`, html.EscapeString(r.serviceName), html.EscapeString(username))
}

// Minimal WSDL for SMAPI (version 1.1)
var smapiWSDL = `<?xml version="1.0" encoding="utf-8"?>
<definitions xmlns="http://schemas.xmlsoap.org/wsdl/"
             xmlns:soap="http://schemas.xmlsoap.org/wsdl/soap/"
             xmlns:tns="http://www.sonos.com/Services/1.1"
             xmlns:xsd="http://www.w3.org/2001/XMLSchema"
             name="Sonos"
             targetNamespace="http://www.sonos.com/Services/1.1">
  <types>
    <xsd:schema targetNamespace="http://www.sonos.com/Services/1.1"/>
  </types>
  <message name="getMetadataIn"><part name="parameters" element="tns:getMetadata"/></message>
  <message name="getMetadataOut"><part name="parameters" element="tns:getMetadataResponse"/></message>
  <message name="searchIn"><part name="parameters" element="tns:search"/></message>
  <message name="searchOut"><part name="parameters" element="tns:searchResponse"/></message>
  <message name="getMediaMetadataIn"><part name="parameters" element="tns:getMediaMetadata"/></message>
  <message name="getMediaMetadataOut"><part name="parameters" element="tns:getMediaMetadataResponse"/></message>
  <message name="getMediaURIIn"><part name="parameters" element="tns:getMediaURI"/></message>
  <message name="getMediaURIOut"><part name="parameters" element="tns:getMediaURIResponse"/></message>
  <portType name="SonosSoap">
    <operation name="getMetadata"><input message="tns:getMetadataIn"/><output message="tns:getMetadataOut"/></operation>
    <operation name="search"><input message="tns:searchIn"/><output message="tns:searchOut"/></operation>
    <operation name="getMediaMetadata"><input message="tns:getMediaMetadataIn"/><output message="tns:getMediaMetadataOut"/></operation>
    <operation name="getMediaURI"><input message="tns:getMediaURIIn"/><output message="tns:getMediaURIOut"/></operation>
  </portType>
  <binding name="SonosSoap" type="tns:SonosSoap">
    <soap:binding style="document" transport="http://schemas.xmlsoap.org/soap/http"/>
  </binding>
  <service name="Sonos"><port name="SonosSoap" binding="tns:SonosSoap"/></service>
</definitions>`
