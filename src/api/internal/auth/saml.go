package auth

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/asymmetric-effort/convocate/internal/httputil"
)

// samlAgentURL returns the SAML/SCIM agent URL from env, or empty if not configured.
func samlAgentURL() string {
	return os.Getenv("SAML_SCIM_AGENT_URL")
}

// samlLogin authenticates a user via the SAML/SCIM agent backend proxy.
// It sends credentials to the agent's /saml/login endpoint, receives a
// SAMLResponse, and extracts identity attributes from the assertion.
func samlLogin(agentURL, username, password string) (*httputil.Principal, error) {
	// Build a minimal SAMLRequest
	samlReqXML := fmt.Sprintf(
		`<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" `+
			`ID="_%x" Version="2.0" IssueInstant="%s" `+
			`AssertionConsumerServiceURL="%s/api/v1/auth/login" `+
			`Destination="%s/saml/sso">`+
			`<saml:Issuer xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion">convocate-api</saml:Issuer>`+
			`</samlp:AuthnRequest>`,
		time.Now().UnixNano(), time.Now().UTC().Format(time.RFC3339), agentURL, agentURL,
	)
	samlReqB64 := base64.StdEncoding.EncodeToString([]byte(samlReqXML))

	// POST to SAML agent /saml/login
	formData := url.Values{
		"SAMLRequest": {samlReqB64},
		"RelayState":  {"convocate"},
		"username":    {username},
		"password":    {password},
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		// Don't follow redirects — we need the HTML response
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.PostForm(agentURL+"/saml/login", formData)
	if err != nil {
		return nil, fmt.Errorf("saml agent request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read saml response: %w", err)
	}

	html := string(body)

	// Check if response contains SAMLResponse (success) or login form (failure)
	if !strings.Contains(html, "SAMLResponse") {
		return nil, fmt.Errorf("authentication failed")
	}

	// Extract base64 SAMLResponse from HTML form
	samlRespB64, err := extractSAMLResponseFromHTML(html)
	if err != nil {
		return nil, fmt.Errorf("extract SAMLResponse: %w", err)
	}

	// Decode and parse assertions
	samlRespXML, err := base64.StdEncoding.DecodeString(samlRespB64)
	if err != nil {
		return nil, fmt.Errorf("decode SAMLResponse: %w", err)
	}

	principal, err := extractPrincipalFromSAML(samlRespXML)
	if err != nil {
		return nil, fmt.Errorf("extract assertions: %w", err)
	}

	return principal, nil
}

// extractSAMLResponseFromHTML extracts the base64-encoded SAMLResponse value
// from the auto-submit HTML form returned by the SAML agent.
func extractSAMLResponseFromHTML(html string) (string, error) {
	// Look for: name="SAMLResponse" value="..."
	marker := `name="SAMLResponse" value="`
	idx := strings.Index(html, marker)
	if idx < 0 {
		return "", fmt.Errorf("SAMLResponse not found in response")
	}
	start := idx + len(marker)
	end := strings.Index(html[start:], `"`)
	if end < 0 {
		return "", fmt.Errorf("SAMLResponse value not terminated")
	}
	return html[start : start+end], nil
}

// SAML XML types for parsing the response
type samlResponse struct {
	XMLName   xml.Name      `xml:"Response"`
	Assertion samlAssertion `xml:"Assertion"`
}

type samlAssertion struct {
	Subject            samlSubject            `xml:"Subject"`
	AttributeStatement samlAttributeStatement `xml:"AttributeStatement"`
}

type samlSubject struct {
	NameID struct {
		Value string `xml:",chardata"`
	} `xml:"NameID"`
}

type samlAttributeStatement struct {
	Attributes []samlAttribute `xml:"Attribute"`
}

type samlAttribute struct {
	Name   string               `xml:"Name,attr"`
	Values []samlAttributeValue `xml:"AttributeValue"`
}

type samlAttributeValue struct {
	Value string `xml:",chardata"`
}

// extractPrincipalFromSAML parses a SAMLResponse XML and builds a Principal.
func extractPrincipalFromSAML(samlXML []byte) (*httputil.Principal, error) {
	var resp samlResponse
	if err := xml.Unmarshal(samlXML, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal SAMLResponse: %w", err)
	}

	username := resp.Assertion.Subject.NameID.Value
	if username == "" {
		return nil, fmt.Errorf("NameID is empty")
	}

	var email string
	var groups []string
	for _, attr := range resp.Assertion.AttributeStatement.Attributes {
		switch attr.Name {
		case "email":
			if len(attr.Values) > 0 {
				email = attr.Values[0].Value
			}
		case "groups":
			for _, v := range attr.Values {
				groups = append(groups, v.Value)
			}
		}
	}

	// Map groups to roles and applets (reuse existing rolesToApplets)
	applets := rolesToApplets(groups)

	return &httputil.Principal{
		ID:                username,
		Username:          username,
		Name:              username,
		Email:             email,
		Groups:            groups,
		Roles:             groups,
		IDP:               "saml",
		AuthorizedApplets: applets,
	}, nil
}
