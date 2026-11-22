package auth

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/asymmetric-effort/convocate/internal/httputil"
)

// ErrMFARequired is returned when a user has MFA enrolled but did not provide a TOTP code.
var ErrMFARequired = errors.New("mfa_required")

// samlAgentURL returns the SAML/SCIM agent URL from env, or empty if not configured.
func samlAgentURL() string {
	return os.Getenv("SAML_SCIM_AGENT_URL")
}

// newSAMLClient creates an HTTP client for the SAML agent.
// TLS is verified against the system cert pool, or a custom CA if
// SAML_SCIM_AGENT_CA_CERT_FILE is set.  InsecureSkipVerify is never used.
func newSAMLClient() (*http.Client, error) {
	tlsCfg := &tls.Config{}
	if caFile := os.Getenv("SAML_SCIM_AGENT_CA_CERT_FILE"); caFile != "" {
		caPEM, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read SAML_SCIM_AGENT_CA_CERT_FILE: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("no valid certs in SAML_SCIM_AGENT_CA_CERT_FILE")
		}
		tlsCfg.RootCAs = pool
	}
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsCfg,
		},
		// Don't follow redirects — we need the HTML response
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}, nil
}

// samlLogin authenticates a user via the SAML/SCIM agent and enforces MFA TOTP
// when the user has TOTP enrolled. If MFA is required but mfaToken is empty,
// ErrMFARequired is returned so the caller can prompt for a code.
func samlLogin(agentURL, username, password, mfaToken string) (*httputil.Principal, error) {
	principal, err := samlLoginCore(agentURL, username, password)
	if err != nil {
		return nil, err
	}

	// Check MFA enrollment
	methodID := openbaoMFAMethodID()
	if methodID == "" {
		return principal, nil // MFA not configured on this deployment
	}

	enrolled, err := openbaoGetMFAStatus(principal.ID, methodID)
	if err != nil || !enrolled {
		return principal, nil // User not enrolled or status check failed
	}

	// MFA is required for this user
	if mfaToken == "" {
		return nil, ErrMFARequired
	}

	// Validate TOTP by doing a userpass login (we still have the password)
	// which triggers OpenBao MFA and gives us an mfa_request_id.
	loginResp, err := openbaoLogin(username, password)
	if err != nil {
		return nil, fmt.Errorf("MFA validation login failed: %w", err)
	}

	if loginResp.Auth.MFARequirement != nil && loginResp.Auth.MFARequirement.MFARequestID != "" {
		// Validate the TOTP code via the existing MFA validate flow
		_, err = openbaoMFAValidate(loginResp.Auth.MFARequirement.MFARequestID, methodID, mfaToken)
		if err != nil {
			return nil, fmt.Errorf("invalid MFA code: %w", err)
		}
	}

	return principal, nil
}

// samlLoginCore authenticates a user via the SAML/SCIM agent backend proxy.
// It sends credentials to the agent's /saml/login endpoint, receives a
// SAMLResponse, and extracts identity attributes from the assertion.
func samlLoginCore(agentURL, username, password string) (*httputil.Principal, error) {
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

	client, err := newSAMLClient()
	if err != nil {
		return nil, fmt.Errorf("build saml client: %w", err)
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

// SAML XML types for parsing the response —————————————————————————————————

type samlResponse struct {
	XMLName   xml.Name         `xml:"Response"`
	Assertion samlAssertion    `xml:"Assertion"`
	Signature *samlSignatureXML `xml:"Signature"`
}

type samlAssertion struct {
	Subject            samlSubject            `xml:"Subject"`
	AttributeStatement samlAttributeStatement `xml:"AttributeStatement"`
	Signature          *samlSignatureXML       `xml:"Signature"`
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

// XML-DSig types for signature verification ———————————————————————————————
// These mirror the SignatureXML family in src/saml-scim-agent/saml/assertion.go
// so that re-marshaling produces byte-for-byte identical canonical form.

type samlSignatureXML struct {
	XMLName        xml.Name          `xml:"http://www.w3.org/2000/09/xmldsig# Signature"`
	SignedInfo     samlDSigSignedInfo `xml:"SignedInfo"`
	SignatureValue samlDSigSigValue   `xml:"SignatureValue"`
	KeyInfo        samlDSigKeyInfo    `xml:"KeyInfo"`
}

type samlDSigSignedInfo struct {
	XMLName                xml.Name           `xml:"http://www.w3.org/2000/09/xmldsig# SignedInfo"`
	CanonicalizationMethod samlDSigAlgorithm  `xml:"CanonicalizationMethod"`
	SignatureMethod        samlDSigAlgorithm  `xml:"SignatureMethod"`
	Reference              samlDSigReference  `xml:"Reference"`
}

type samlDSigAlgorithm struct {
	Algorithm string `xml:"Algorithm,attr"`
}

type samlDSigReference struct {
	XMLName      xml.Name           `xml:"http://www.w3.org/2000/09/xmldsig# Reference"`
	URI          string             `xml:"URI,attr"`
	Transforms   samlDSigTransforms `xml:"Transforms"`
	DigestMethod samlDSigAlgorithm  `xml:"DigestMethod"`
	DigestValue  samlDSigDigestVal  `xml:"DigestValue"`
}

type samlDSigTransforms struct {
	Transform []samlDSigAlgorithm `xml:"Transform"`
}

type samlDSigDigestVal struct {
	Value string `xml:",chardata"`
}

type samlDSigSigValue struct {
	Value string `xml:",chardata"`
}

type samlDSigKeyInfo struct {
	XMLName  xml.Name         `xml:"http://www.w3.org/2000/09/xmldsig# KeyInfo"`
	X509Data samlDSigX509Data `xml:"X509Data"`
}

type samlDSigX509Data struct {
	X509Certificate samlDSigX509Cert `xml:"X509Certificate"`
}

type samlDSigX509Cert struct {
	Value string `xml:",chardata"`
}

// loadSAMLSigningCert returns the pinned SAML signing cert from
// SAML_SIGNING_CERT_FILE, or nil if the env var is not set.
func loadSAMLSigningCert() (*x509.Certificate, error) {
	certFile := os.Getenv("SAML_SIGNING_CERT_FILE")
	if certFile == "" {
		return nil, nil
	}
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return nil, fmt.Errorf("read SAML_SIGNING_CERT_FILE: %w", err)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("no PEM block in SAML_SIGNING_CERT_FILE")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse SAML_SIGNING_CERT_FILE: %w", err)
	}
	return cert, nil
}

// verifySAMLSignature verifies the XML-DSig signature on the SAML assertion.
// It extracts the cert from KeyInfo, optionally compares it to pinnedCert,
// then verifies the cryptographic signature over the canonical SignedInfo.
func verifySAMLSignature(sig *samlSignatureXML, pinnedCert *x509.Certificate) error {
	// Decode the cert embedded in KeyInfo
	certDER, err := base64.StdEncoding.DecodeString(strings.TrimSpace(sig.KeyInfo.X509Data.X509Certificate.Value))
	if err != nil {
		return fmt.Errorf("decode KeyInfo cert: %w", err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return fmt.Errorf("parse KeyInfo cert: %w", err)
	}

	// If a pinned cert is configured, the KeyInfo cert must match it exactly.
	if pinnedCert != nil {
		if !cert.Equal(pinnedCert) {
			return fmt.Errorf("SAML signing cert does not match pinned cert")
		}
	}

	// Re-marshal SignedInfo with the same struct definition used during signing
	// so the canonical bytes are identical.
	signedInfoXML, err := xml.Marshal(sig.SignedInfo)
	if err != nil {
		return fmt.Errorf("marshal SignedInfo for verification: %w", err)
	}
	canonical := samlCanonicalize(signedInfoXML)

	// Decode the signature value
	sigBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(sig.SignatureValue.Value))
	if err != nil {
		return fmt.Errorf("decode signature value: %w", err)
	}

	// Dispatch on the public key type embedded in the cert
	switch pub := cert.PublicKey.(type) {
	case ed25519.PublicKey:
		if !ed25519.Verify(pub, canonical, sigBytes) {
			return fmt.Errorf("ed25519 SAML signature verification failed")
		}
	case *rsa.PublicKey:
		hash := sha256.Sum256(canonical)
		if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, hash[:], sigBytes); err != nil {
			return fmt.Errorf("RSA SAML signature verification failed: %w", err)
		}
	default:
		return fmt.Errorf("unsupported key type in SAML cert: %T", cert.PublicKey)
	}

	return nil
}

// extractPrincipalFromSAML parses a SAMLResponse XML and builds a Principal.
// When SAML_SIGNING_CERT_FILE is set the assertion signature is verified;
// if the env var is absent, verification is skipped (warn-only for backward compat
// during deployments that have not yet configured the cert file).
func extractPrincipalFromSAML(samlXML []byte) (*httputil.Principal, error) {
	var resp samlResponse
	if err := xml.Unmarshal(samlXML, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal SAMLResponse: %w", err)
	}

	pinnedCert, err := loadSAMLSigningCert()
	if err != nil {
		return nil, fmt.Errorf("load SAML signing cert: %w", err)
	}

	// Prefer assertion-level signature; fall back to response-level.
	sigToVerify := resp.Assertion.Signature
	if sigToVerify == nil {
		sigToVerify = resp.Signature
	}

	if pinnedCert != nil {
		if sigToVerify == nil {
			return nil, fmt.Errorf("SAML assertion has no signature but SAML_SIGNING_CERT_FILE is configured")
		}
		if err := verifySAMLSignature(sigToVerify, pinnedCert); err != nil {
			return nil, fmt.Errorf("SAML signature verification failed: %w", err)
		}
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

// samlCanonicalize performs a simplified Exclusive XML Canonicalization,
// matching the implementation in src/saml-scim-agent/saml/assertion.go so
// that signature bytes computed during signing verify correctly here.
func samlCanonicalize(xmlData []byte) []byte {
	input := string(xmlData)

	// Remove XML declaration if present
	if strings.HasPrefix(input, "<?xml") {
		idx := strings.Index(input, "?>")
		if idx >= 0 {
			input = strings.TrimSpace(input[idx+2:])
		}
	}

	decoder := xml.NewDecoder(strings.NewReader(input))
	var result strings.Builder

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}

		switch t := tok.(type) {
		case xml.StartElement:
			result.WriteByte('<')
			if t.Name.Space != "" {
				result.WriteString(t.Name.Space)
				result.WriteByte(':')
			}
			result.WriteString(t.Name.Local)

			// Sort attributes alphabetically for canonical form
			attrs := make([]xml.Attr, len(t.Attr))
			copy(attrs, t.Attr)
			sort.Slice(attrs, func(i, j int) bool {
				ai := attrs[i]
				aj := attrs[j]
				aiIsNS := ai.Name.Space == "xmlns" || ai.Name.Local == "xmlns"
				ajIsNS := aj.Name.Space == "xmlns" || aj.Name.Local == "xmlns"
				if aiIsNS != ajIsNS {
					return aiIsNS
				}
				if ai.Name.Space != aj.Name.Space {
					return ai.Name.Space < aj.Name.Space
				}
				return ai.Name.Local < aj.Name.Local
			})

			for _, attr := range attrs {
				result.WriteByte(' ')
				if attr.Name.Space != "" {
					result.WriteString(attr.Name.Space)
					result.WriteByte(':')
				}
				result.WriteString(attr.Name.Local)
				result.WriteString(`="`)
				result.WriteString(samlXMLEscapeAttr(attr.Value))
				result.WriteByte('"')
			}
			result.WriteByte('>')

		case xml.EndElement:
			result.WriteString("</")
			if t.Name.Space != "" {
				result.WriteString(t.Name.Space)
				result.WriteByte(':')
			}
			result.WriteString(t.Name.Local)
			result.WriteByte('>')

		case xml.CharData:
			result.WriteString(samlXMLEscapeText(string(t)))
		}
	}

	return []byte(result.String())
}

func samlXMLEscapeAttr(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "\t", "&#x9;")
	s = strings.ReplaceAll(s, "\n", "&#xA;")
	s = strings.ReplaceAll(s, "\r", "&#xD;")
	return s
}

func samlXMLEscapeText(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\r", "&#xD;")
	return s
}
