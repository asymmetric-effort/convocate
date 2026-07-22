package saml

import (
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"

	"github.com/asymmetric-effort/convocate/src/saml-scim-agent/openbao"
)

// Handler handles SAML IdP endpoints.
type Handler struct {
	Client   *openbao.Client
	Keys     *KeyPair
	EntityID string
	SSOURL   string
}

// loginFormHTML is a simple HTML login form.
const loginFormHTML = `<!DOCTYPE html>
<html>
<head><title>SAML/SCIM Agent SSO Login</title></head>
<body>
<h1>Sign In</h1>
{{if .Error}}<p style="color:red">{{.Error}}</p>{{end}}
<form method="POST" action="/saml/login">
<input type="hidden" name="SAMLRequest" value="{{.SAMLRequest}}">
<input type="hidden" name="RelayState" value="{{.RelayState}}">
<label>Username: <input type="text" name="username" required></label><br>
<label>Password: <input type="password" name="password" required></label><br>
<button type="submit">Sign In</button>
</form>
</body>
</html>`

// postBindingHTML is the auto-submit form for POST binding response.
const postBindingHTML = `<!DOCTYPE html>
<html>
<head><title>SSO Redirect</title></head>
<body onload="document.forms[0].submit()">
<form method="POST" action="{{.ACSURL}}">
<input type="hidden" name="SAMLResponse" value="{{.SAMLResponse}}">
<input type="hidden" name="RelayState" value="{{.RelayState}}">
<noscript><button type="submit">Continue</button></noscript>
</form>
</body>
</html>`

var loginTmpl = template.Must(template.New("login").Parse(loginFormHTML))
var postTmpl = template.Must(template.New("post").Parse(postBindingHTML))

// ServeMetadata handles GET /saml/metadata.
func (h *Handler) ServeMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metadata, err := GenerateMetadata(h.EntityID, h.SSOURL, h.Keys.Certificate.Raw)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.Write(metadata)
}

// ServeSSO handles GET and POST /saml/sso.
func (h *Handler) ServeSSO(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleSSORedirect(w, r)
	case http.MethodPost:
		h.handleSSOPost(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleSSORedirect(w http.ResponseWriter, r *http.Request) {
	samlReq := r.URL.Query().Get("SAMLRequest")
	relayState := r.URL.Query().Get("RelayState")

	if samlReq == "" {
		http.Error(w, "missing SAMLRequest parameter", http.StatusBadRequest)
		return
	}

	// Validate the request can be parsed
	_, err := ParseAuthnRequestRedirect(samlReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid SAMLRequest: %v", err), http.StatusBadRequest)
		return
	}

	// Show login form
	h.showLoginForm(w, samlReq, relayState, "")
}

func (h *Handler) handleSSOPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	samlReq := r.FormValue("SAMLRequest")
	relayState := r.FormValue("RelayState")

	if samlReq == "" {
		http.Error(w, "missing SAMLRequest", http.StatusBadRequest)
		return
	}

	// Validate the request
	_, err := ParseAuthnRequestPost(samlReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid SAMLRequest: %v", err), http.StatusBadRequest)
		return
	}

	// Show login form
	h.showLoginForm(w, samlReq, relayState, "")
}

// ServeLogin handles POST /saml/login (form submission).
func (h *Handler) ServeLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")
	samlReq := r.FormValue("SAMLRequest")
	relayState := r.FormValue("RelayState")

	if username == "" || password == "" {
		h.showLoginForm(w, samlReq, relayState, "Username and password are required")
		return
	}

	if samlReq == "" {
		http.Error(w, "missing SAMLRequest", http.StatusBadRequest)
		return
	}

	// Authenticate against OpenBao
	token, err := h.Client.Authenticate(username, password)
	if err != nil {
		h.showLoginForm(w, samlReq, relayState, "Invalid credentials")
		return
	}

	// Parse the original AuthnRequest
	// Try redirect format first, then POST format
	authnReq, err := ParseAuthnRequestRedirect(samlReq)
	if err != nil {
		authnReq, err = ParseAuthnRequestPost(samlReq)
		if err != nil {
			http.Error(w, "invalid SAMLRequest", http.StatusBadRequest)
			return
		}
	}

	// Get user entity for attributes
	entity, err := h.Client.GetEntityByNameWithToken(username, token)
	if err != nil || entity == nil {
		// Fall back to basic info
		entity = &openbao.Entity{
			Name:     username,
			Metadata: map[string]string{},
		}
	}

	email := entity.Metadata["email"]
	if email == "" {
		email = username
	}

	// Determine audience (SP entity ID)
	audience := authnReq.Issuer.Value
	if audience == "" {
		audience = authnReq.AssertionConsumerServiceURL
	}

	acsURL := authnReq.AssertionConsumerServiceURL
	if acsURL == "" {
		http.Error(w, "no AssertionConsumerServiceURL in request", http.StatusBadRequest)
		return
	}

	// Build groups list from entity group IDs
	groups := make([]string, 0, len(entity.GroupIDs))
	for i := 0; i < len(entity.GroupIDs); i++ {
		groups = append(groups, entity.GroupIDs[i])
	}

	// Build signed SAML Response
	params := AssertionParams{
		EntityID:  h.EntityID,
		SSOURL:    h.SSOURL,
		RequestID: authnReq.ID,
		ACSURL:    acsURL,
		Audience:  audience,
		Username:  username,
		Email:     email,
		Groups:    groups,
	}

	responseXML, err := BuildSignedResponse(params, h.Keys)
	if err != nil {
		http.Error(w, "internal error building response", http.StatusInternalServerError)
		return
	}

	// Base64 encode the response for POST binding
	responseB64 := base64.StdEncoding.EncodeToString(responseXML)

	// Render POST binding form
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	postTmpl.Execute(w, map[string]string{
		"ACSURL":       acsURL,
		"SAMLResponse": responseB64,
		"RelayState":   relayState,
	})
}

// ServeSLO handles GET /saml/slo (single logout).
func (h *Handler) ServeSLO(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Simple SLO: just respond with a success page
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html><html><body><h1>Logged Out</h1><p>You have been logged out.</p></body></html>`))
}

func (h *Handler) showLoginForm(w http.ResponseWriter, samlReq, relayState, errorMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	loginTmpl.Execute(w, map[string]string{
		"SAMLRequest": samlReq,
		"RelayState":  relayState,
		"Error":       errorMsg,
	})
}
