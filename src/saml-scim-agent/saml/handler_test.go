package saml

import (
	"bytes"
	"compress/flate"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/asymmetric-effort/convocate/src/saml-scim-agent/openbao"
)

func TestGenerateMetadata(t *testing.T) {
	kp, err := generateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	metadata, err := GenerateMetadata(
		"https://sso.example.com",
		"https://sso.example.com/saml/sso",
		kp.Certificate.Raw,
	)
	if err != nil {
		t.Fatalf("failed to generate metadata: %v", err)
	}

	var descriptor EntityDescriptor
	if err := xml.Unmarshal(metadata, &descriptor); err != nil {
		t.Fatalf("invalid XML: %v", err)
	}

	if descriptor.EntityID != "https://sso.example.com" {
		t.Errorf("expected entity ID https://sso.example.com, got %s", descriptor.EntityID)
	}

	if len(descriptor.IDPSSODescriptor.SingleSignOnServices) != 2 {
		t.Errorf("expected 2 SSO services, got %d", len(descriptor.IDPSSODescriptor.SingleSignOnServices))
	}

	if len(descriptor.IDPSSODescriptor.KeyDescriptors) != 1 {
		t.Errorf("expected 1 key descriptor, got %d", len(descriptor.IDPSSODescriptor.KeyDescriptors))
	}

	// Check SLO URL
	if len(descriptor.IDPSSODescriptor.SingleLogoutServices) != 1 {
		t.Fatalf("expected 1 SLO service, got %d", len(descriptor.IDPSSODescriptor.SingleLogoutServices))
	}
	sloURL := descriptor.IDPSSODescriptor.SingleLogoutServices[0].Location
	if sloURL != "https://sso.example.com/saml/slo" {
		t.Errorf("expected SLO URL ending in /slo, got %s", sloURL)
	}

	// Verify NameID formats
	if len(descriptor.IDPSSODescriptor.NameIDFormats) != 2 {
		t.Errorf("expected 2 NameID formats, got %d", len(descriptor.IDPSSODescriptor.NameIDFormats))
	}

	// Verify certificate is decodable
	certB64 := descriptor.IDPSSODescriptor.KeyDescriptors[0].KeyInfo.X509Data.X509Certificate.Value
	certDER, err := base64.StdEncoding.DecodeString(certB64)
	if err != nil {
		t.Fatalf("invalid cert base64: %v", err)
	}
	_, err = x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("invalid certificate: %v", err)
	}
}

func TestGenerateMetadataSLOURLNoSSOSuffix(t *testing.T) {
	kp, err := generateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	// SSO URL that doesn't end in /sso
	metadata, err := GenerateMetadata(
		"https://sso.example.com",
		"https://sso.example.com/auth",
		kp.Certificate.Raw,
	)
	if err != nil {
		t.Fatalf("failed to generate metadata: %v", err)
	}

	var descriptor EntityDescriptor
	xml.Unmarshal(metadata, &descriptor)

	// SLO URL should be same as SSO URL since it doesn't end in /sso
	sloURL := descriptor.IDPSSODescriptor.SingleLogoutServices[0].Location
	if sloURL != "https://sso.example.com/auth" {
		t.Errorf("expected SLO URL to be same as SSO URL, got %s", sloURL)
	}
}

func TestGenerateMetadataShortURL(t *testing.T) {
	kp, err := generateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	// Very short SSO URL (less than 4 chars)
	metadata, err := GenerateMetadata(
		"https://sso.example.com",
		"/s",
		kp.Certificate.Raw,
	)
	if err != nil {
		t.Fatalf("failed to generate metadata: %v", err)
	}

	var descriptor EntityDescriptor
	xml.Unmarshal(metadata, &descriptor)
	sloURL := descriptor.IDPSSODescriptor.SingleLogoutServices[0].Location
	if sloURL != "/s" {
		t.Errorf("expected /s, got %s", sloURL)
	}
}

func TestBuildSignedResponse(t *testing.T) {
	kp, err := generateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	params := AssertionParams{
		EntityID:  "https://sso.example.com",
		SSOURL:    "https://sso.example.com/saml/sso",
		RequestID: "_req123",
		ACSURL:    "https://sp.example.com/acs",
		Audience:  "https://sp.example.com",
		Username:  "testuser",
		Email:     "test@example.com",
		Groups:    []string{"admins", "users"},
	}

	responseXML, err := BuildSignedResponse(params, kp)
	if err != nil {
		t.Fatalf("failed to build response: %v", err)
	}

	var resp Response
	if err := xml.Unmarshal(responseXML, &resp); err != nil {
		t.Fatalf("invalid XML: %v", err)
	}

	if resp.InResponseTo != "_req123" {
		t.Errorf("expected InResponseTo _req123, got %s", resp.InResponseTo)
	}
	if resp.Destination != "https://sp.example.com/acs" {
		t.Errorf("expected destination https://sp.example.com/acs, got %s", resp.Destination)
	}
	if resp.Status.StatusCode.Value != "urn:oasis:names:tc:SAML:2.0:status:Success" {
		t.Errorf("expected success status, got %s", resp.Status.StatusCode.Value)
	}
	if resp.Assertion.Subject.NameID.Value != "testuser" {
		t.Errorf("expected NameID testuser, got %s", resp.Assertion.Subject.NameID.Value)
	}
	if resp.Assertion.Conditions.AudienceRestriction.Audience.Value != "https://sp.example.com" {
		t.Errorf("unexpected audience")
	}
	if resp.Signature == nil {
		t.Error("response missing signature")
	}
	if resp.Assertion.Signature == nil {
		t.Error("assertion missing signature")
	}
	if resp.Version != "2.0" {
		t.Errorf("expected version 2.0, got %s", resp.Version)
	}
	if resp.Issuer.Value != "https://sso.example.com" {
		t.Errorf("expected issuer https://sso.example.com, got %s", resp.Issuer.Value)
	}
}

func TestBuildSignedResponseNoGroups(t *testing.T) {
	kp, err := generateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	params := AssertionParams{
		EntityID:  "https://sso.example.com",
		SSOURL:    "https://sso.example.com/saml/sso",
		RequestID: "_req456",
		ACSURL:    "https://sp.example.com/acs",
		Audience:  "https://sp.example.com",
		Username:  "testuser",
		Email:     "test@example.com",
		Groups:    nil,
	}

	responseXML, err := BuildSignedResponse(params, kp)
	if err != nil {
		t.Fatalf("failed to build response: %v", err)
	}

	// Verify it's valid XML and contains email but not groups attribute
	var resp Response
	if err := xml.Unmarshal(responseXML, &resp); err != nil {
		t.Fatalf("invalid XML: %v", err)
	}

	xmlStr := string(responseXML)
	if !strings.Contains(xmlStr, "email") {
		t.Error("expected email attribute in response")
	}
	if strings.Contains(xmlStr, `Name="groups"`) {
		t.Error("should not have groups attribute when no groups")
	}
}

func TestBuildSignedResponseEmptyGroups(t *testing.T) {
	kp, err := generateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	params := AssertionParams{
		EntityID:  "https://sso.example.com",
		SSOURL:    "https://sso.example.com/saml/sso",
		RequestID: "_req789",
		ACSURL:    "https://sp.example.com/acs",
		Audience:  "https://sp.example.com",
		Username:  "testuser",
		Email:     "test@example.com",
		Groups:    []string{},
	}

	responseXML, err := BuildSignedResponse(params, kp)
	if err != nil {
		t.Fatalf("failed to build response: %v", err)
	}

	xmlStr := string(responseXML)
	if !strings.Contains(xmlStr, "email") {
		t.Error("expected email attribute")
	}
	if strings.Contains(xmlStr, `Name="groups"`) {
		t.Error("should not have groups attribute when groups is empty")
	}
}

func TestBuildSignedResponseBadKey(t *testing.T) {
	params := AssertionParams{
		EntityID:  "https://sso.example.com",
		SSOURL:    "https://sso.example.com/saml/sso",
		RequestID: "_req-bad",
		ACSURL:    "https://sp.example.com/acs",
		Audience:  "https://sp.example.com",
		Username:  "testuser",
		Email:     "test@example.com",
	}

	// Create a key pair with a 1-bit key that's too small for PKCS1v15 signing with SHA-256
	kp, _ := generateKeyPair()
	// Use a key where N is too small for the hash + padding
	smallKey := &rsa.PrivateKey{
		PublicKey: rsa.PublicKey{
			N: big.NewInt(15), // Ridiculously small modulus
			E: 3,
		},
		D: big.NewInt(5),
	}
	kp.PrivateKey = smallKey

	_, err := BuildSignedResponse(params, kp)
	if err == nil {
		t.Fatal("expected error for key too small for signing")
	}
}

func TestParseAuthnRequestPost(t *testing.T) {
	reqXML := `<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="_req456" Version="2.0" IssueInstant="2026-01-01T00:00:00Z" Destination="https://sso.example.com/saml/sso" AssertionConsumerServiceURL="https://sp.example.com/acs" ProtocolBinding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST"><saml:Issuer>https://sp.example.com</saml:Issuer></samlp:AuthnRequest>`

	b64 := base64.StdEncoding.EncodeToString([]byte(reqXML))

	req, err := ParseAuthnRequestPost(b64)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if req.ID != "_req456" {
		t.Errorf("expected ID _req456, got %s", req.ID)
	}
	if req.Version != "2.0" {
		t.Errorf("expected version 2.0, got %s", req.Version)
	}
	if req.AssertionConsumerServiceURL != "https://sp.example.com/acs" {
		t.Errorf("expected ACS URL https://sp.example.com/acs, got %s", req.AssertionConsumerServiceURL)
	}
	if req.Issuer.Value != "https://sp.example.com" {
		t.Errorf("expected issuer https://sp.example.com, got %s", req.Issuer.Value)
	}
}

func TestParseAuthnRequestPostInvalidBase64(t *testing.T) {
	_, err := ParseAuthnRequestPost("not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestParseAuthnRequestPostInvalidXML(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString([]byte(`<not valid xml`))
	_, err := ParseAuthnRequestPost(b64)
	if err == nil {
		t.Fatal("expected error for invalid XML")
	}
}

func TestParseAuthnRequestMissingID(t *testing.T) {
	reqXML := `<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" Version="2.0" IssueInstant="2026-01-01T00:00:00Z"><saml:Issuer xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion">https://sp.example.com</saml:Issuer></samlp:AuthnRequest>`
	b64 := base64.StdEncoding.EncodeToString([]byte(reqXML))

	_, err := ParseAuthnRequestPost(b64)
	if err == nil {
		t.Fatal("expected error for missing ID")
	}
	if !strings.Contains(err.Error(), "missing ID") {
		t.Errorf("expected missing ID error, got: %v", err)
	}
}

func TestParseAuthnRequestBadVersion(t *testing.T) {
	reqXML := `<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" ID="_test" Version="1.1" IssueInstant="2026-01-01T00:00:00Z"><saml:Issuer xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion">https://sp.example.com</saml:Issuer></samlp:AuthnRequest>`
	b64 := base64.StdEncoding.EncodeToString([]byte(reqXML))

	_, err := ParseAuthnRequestPost(b64)
	if err == nil {
		t.Fatal("expected error for bad version")
	}
	if !strings.Contains(err.Error(), "unsupported SAML version") {
		t.Errorf("expected unsupported version error, got: %v", err)
	}
}

func TestParseAuthnRequestRedirect(t *testing.T) {
	reqXML := `<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="_req789" Version="2.0" IssueInstant="2026-01-01T00:00:00Z" Destination="https://sso.example.com/saml/sso" AssertionConsumerServiceURL="https://sp.example.com/acs"><saml:Issuer>https://sp.example.com</saml:Issuer></samlp:AuthnRequest>`

	// DEFLATE compress
	var buf bytes.Buffer
	w, _ := flate.NewWriter(&buf, flate.DefaultCompression)
	w.Write([]byte(reqXML))
	w.Close()

	// Base64 encode then URL-encode (as it would appear in a real query string)
	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())
	urlEncoded := url.QueryEscape(b64)

	req, err := ParseAuthnRequestRedirect(urlEncoded)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if req.ID != "_req789" {
		t.Errorf("expected ID _req789, got %s", req.ID)
	}
	if req.AssertionConsumerServiceURL != "https://sp.example.com/acs" {
		t.Errorf("unexpected ACS URL: %s", req.AssertionConsumerServiceURL)
	}
}

func TestParseAuthnRequestRedirectInvalidBase64(t *testing.T) {
	_, err := ParseAuthnRequestRedirect("!!!not-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestParseAuthnRequestRedirectBadURLEncoding(t *testing.T) {
	// A % without valid hex digits - url.QueryUnescape will fail, code falls through to use raw value
	// But the raw value is also not valid base64, so should still fail
	_, err := ParseAuthnRequestRedirect("%ZZ")
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
}

func TestParseAuthnRequestRedirectUncompressed(t *testing.T) {
	// Some SPs send uncompressed data that fails DEFLATE - the code falls back
	reqXML := `<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="_reqUncomp" Version="2.0" IssueInstant="2026-01-01T00:00:00Z" AssertionConsumerServiceURL="https://sp.example.com/acs"><saml:Issuer>https://sp.example.com</saml:Issuer></samlp:AuthnRequest>`

	// Just base64 encode without compression, then URL-encode
	b64 := base64.StdEncoding.EncodeToString([]byte(reqXML))
	urlEncoded := url.QueryEscape(b64)

	req, err := ParseAuthnRequestRedirect(urlEncoded)
	if err != nil {
		t.Fatalf("failed to parse uncompressed: %v", err)
	}
	if req.ID != "_reqUncomp" {
		t.Errorf("expected ID _reqUncomp, got %s", req.ID)
	}
}

func TestKeyGeneration(t *testing.T) {
	kp, err := generateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}

	if kp.PrivateKey == nil {
		t.Fatal("private key is nil")
	}
	if kp.Certificate == nil {
		t.Fatal("certificate is nil")
	}
	if len(kp.CertPEM) == 0 {
		t.Fatal("cert PEM is empty")
	}

	block, _ := pem.Decode(kp.CertPEM)
	if block == nil {
		t.Fatal("failed to decode cert PEM")
	}
	if block.Type != "CERTIFICATE" {
		t.Errorf("expected CERTIFICATE block, got %s", block.Type)
	}

	if kp.PrivateKey.N.BitLen() != 2048 {
		t.Errorf("expected 2048 bit key, got %d", kp.PrivateKey.N.BitLen())
	}

	// Verify certificate subject
	if kp.Certificate.Subject.CommonName != "SAML-SCIM-Agent SAML Signing" {
		t.Errorf("expected CN 'SAML-SCIM-Agent SAML Signing', got %s", kp.Certificate.Subject.CommonName)
	}
	if len(kp.Certificate.Subject.Organization) != 1 || kp.Certificate.Subject.Organization[0] != "Asymmetric Effort" {
		t.Errorf("unexpected organization: %v", kp.Certificate.Subject.Organization)
	}
}

func TestKeyPairEncodeDecode(t *testing.T) {
	kp, err := generateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(kp.PrivateKey),
	})

	decoded, err := decodeKeyPair(keyPEM, kp.CertPEM)
	if err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if decoded.PrivateKey.N.Cmp(kp.PrivateKey.N) != 0 {
		t.Error("private keys don't match")
	}
}

func TestDecodeKeyPairInvalidKeyPEM(t *testing.T) {
	kp, _ := generateKeyPair()
	_, err := decodeKeyPair([]byte("not a pem"), kp.CertPEM)
	if err == nil {
		t.Fatal("expected error for invalid key PEM")
	}
}

func TestDecodeKeyPairInvalidCertPEM(t *testing.T) {
	kp, _ := generateKeyPair()
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(kp.PrivateKey),
	})
	_, err := decodeKeyPair(keyPEM, []byte("not a pem"))
	if err == nil {
		t.Fatal("expected error for invalid cert PEM")
	}
}

func TestDecodeKeyPairInvalidKeyBytes(t *testing.T) {
	kp, _ := generateKeyPair()
	badKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: []byte("not a real key"),
	})
	_, err := decodeKeyPair(badKeyPEM, kp.CertPEM)
	if err == nil {
		t.Fatal("expected error for invalid key bytes")
	}
}

func TestDecodeKeyPairInvalidCertBytes(t *testing.T) {
	kp, _ := generateKeyPair()
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(kp.PrivateKey),
	})
	badCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: []byte("not a real cert"),
	})
	_, err := decodeKeyPair(keyPEM, badCertPEM)
	if err == nil {
		t.Fatal("expected error for invalid cert bytes")
	}
}

func TestCanonicalize(t *testing.T) {
	input := `<?xml version="1.0"?><Root b="2" a="1"><Child>text</Child></Root>`
	result := string(canonicalize([]byte(input)))

	if strings.Contains(result, "<?xml") {
		t.Error("canonical form should not have XML declaration")
	}

	if !strings.Contains(result, `a="1"`) || !strings.Contains(result, `b="2"`) {
		t.Error("attributes should be preserved")
	}

	// Verify attributes are sorted (a before b)
	aIdx := strings.Index(result, `a="1"`)
	bIdx := strings.Index(result, `b="2"`)
	if aIdx > bIdx {
		t.Error("attributes should be sorted alphabetically")
	}
}

func TestCanonicalizeNoDeclaration(t *testing.T) {
	input := `<Root><Child>text</Child></Root>`
	result := string(canonicalize([]byte(input)))

	if !strings.Contains(result, "<Root>") {
		t.Error("expected Root element")
	}
	if !strings.Contains(result, "<Child>text</Child>") {
		t.Error("expected Child element with text")
	}
}

func TestCanonicalizeXmlnsFirst(t *testing.T) {
	input := `<Root z="1" xmlns:foo="http://foo" a="2"><Child/></Root>`
	result := string(canonicalize([]byte(input)))

	// xmlns attributes should come first
	if !strings.Contains(result, "foo") {
		t.Error("expected xmlns attribute")
	}
}

func TestXmlEscapeAttr(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"a&b", "a&amp;b"},
		{"a<b", "a&lt;b"},
		{"a>b", "a&gt;b"},
		{`a"b`, "a&quot;b"},
		{"a\tb", "a&#x9;b"},
		{"a\nb", "a&#xA;b"},
		{"a\rb", "a&#xD;b"},
	}

	for _, tc := range tests {
		result := xmlEscapeAttr(tc.input)
		if result != tc.expected {
			t.Errorf("xmlEscapeAttr(%q): expected %q, got %q", tc.input, tc.expected, result)
		}
	}
}

func TestXmlEscapeText(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"a&b", "a&amp;b"},
		{"a<b", "a&lt;b"},
		{"a>b", "a&gt;b"},
		{"a\rb", "a&#xD;b"},
		{"a\nb", "a\nb"}, // newlines not escaped in text
	}

	for _, tc := range tests {
		result := xmlEscapeText(tc.input)
		if result != tc.expected {
			t.Errorf("xmlEscapeText(%q): expected %q, got %q", tc.input, tc.expected, result)
		}
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	if id1 == id2 {
		t.Error("generated IDs should be unique")
	}
	if len(id1) != 32 {
		t.Errorf("expected 32 char hex, got %d chars: %s", len(id1), id1)
	}
}

func TestLoadOrGenerateKeysNew(t *testing.T) {
	// Mock OpenBao that returns 404 for read, then accepts write
	mux := http.NewServeMux()
	readCount := 0
	mux.HandleFunc("/v1/secret/data/saml-scim-agent/saml-signing-key", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			readCount++
			w.WriteHeader(http.StatusNotFound)
		case http.MethodPost:
			w.WriteHeader(http.StatusOK)
		}
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := openbao.NewClient(ts.URL, "test-token", true)
	kp, err := LoadOrGenerateKeys(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kp == nil {
		t.Fatal("expected key pair")
	}
	if kp.PrivateKey == nil {
		t.Fatal("private key is nil")
	}
	if kp.Certificate == nil {
		t.Fatal("certificate is nil")
	}
}

func TestLoadOrGenerateKeysExisting(t *testing.T) {
	// Generate a key pair to store
	kp, _ := generateKeyPair()
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(kp.PrivateKey),
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/secret/data/saml-scim-agent/saml-signing-key", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"data": map[string]interface{}{
					"private_key": string(keyPEM),
					"certificate": string(kp.CertPEM),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := openbao.NewClient(ts.URL, "test-token", true)
	loaded, err := LoadOrGenerateKeys(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected key pair")
	}
	if loaded.PrivateKey.N.Cmp(kp.PrivateKey.N) != 0 {
		t.Error("loaded key doesn't match stored key")
	}
}

func TestLoadOrGenerateKeysNonStringValues(t *testing.T) {
	// Stored data has non-string values for keys
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/secret/data/saml-scim-agent/saml-signing-key", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"private_key": 12345,
						"certificate": true,
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		case http.MethodPost:
			w.WriteHeader(http.StatusOK)
		}
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := openbao.NewClient(ts.URL, "test-token", true)
	kp, err := LoadOrGenerateKeys(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kp == nil {
		t.Fatal("expected newly generated key pair")
	}
}

func TestLoadOrGenerateKeysReadError(t *testing.T) {
	client := openbao.NewClient("http://127.0.0.1:1", "test-token", true)
	_, err := LoadOrGenerateKeys(client)
	if err == nil {
		t.Fatal("expected error for connection failure")
	}
}

func TestLoadOrGenerateKeysWriteError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/secret/data/saml-scim-agent/saml-signing-key", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
		case http.MethodPost:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := openbao.NewClient(ts.URL, "test-token", true)
	_, err := LoadOrGenerateKeys(client)
	if err == nil {
		t.Fatal("expected error when write fails")
	}
}

func TestLoadOrGenerateKeysInvalidStoredKey(t *testing.T) {
	// Stored data has invalid PEM - should regenerate
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/secret/data/saml-scim-agent/saml-signing-key", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"private_key": "invalid-pem",
						"certificate": "invalid-pem",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		case http.MethodPost:
			w.WriteHeader(http.StatusOK)
		}
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := openbao.NewClient(ts.URL, "test-token", true)
	kp, err := LoadOrGenerateKeys(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kp == nil {
		t.Fatal("expected newly generated key pair")
	}
}

func TestLoadOrGenerateKeysMissingFields(t *testing.T) {
	// Stored data is missing required fields
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/secret/data/saml-scim-agent/saml-signing-key", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"other_field": "something",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		case http.MethodPost:
			w.WriteHeader(http.StatusOK)
		}
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := openbao.NewClient(ts.URL, "test-token", true)
	kp, err := LoadOrGenerateKeys(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kp == nil {
		t.Fatal("expected newly generated key pair")
	}
}

// Helper: ServeMetadata tests

func makeTestHandler(t *testing.T) (*Handler, *httptest.Server) {
	t.Helper()
	kp, err := generateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	baoMux := http.NewServeMux()
	baoMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	baoServer := httptest.NewServer(baoMux)

	client := openbao.NewClient(baoServer.URL, "test-token", true)
	handler := &Handler{
		Client:   client,
		Keys:     kp,
		EntityID: "https://sso.example.com",
		SSOURL:   "https://sso.example.com/saml/sso",
	}

	return handler, baoServer
}

func TestServeMetadataGET(t *testing.T) {
	handler, baoServer := makeTestHandler(t)
	defer baoServer.Close()

	req := httptest.NewRequest(http.MethodGet, "/saml/metadata", nil)
	w := httptest.NewRecorder()
	handler.ServeMetadata(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/xml" {
		t.Errorf("expected Content-Type application/xml, got %s", ct)
	}

	// Verify valid XML
	var descriptor EntityDescriptor
	if err := xml.Unmarshal(w.Body.Bytes(), &descriptor); err != nil {
		t.Fatalf("invalid XML: %v", err)
	}
	if descriptor.EntityID != "https://sso.example.com" {
		t.Errorf("wrong entity ID: %s", descriptor.EntityID)
	}
}

func TestServeMetadataEmptyCertRaw(t *testing.T) {
	// Test with empty certificate raw bytes - GenerateMetadata still won't error
	// since it just base64-encodes whatever it gets
	kp, _ := generateKeyPair()
	origRaw := kp.Certificate.Raw
	kp.Certificate.Raw = []byte{} // empty

	baoMux := http.NewServeMux()
	baoServer := httptest.NewServer(baoMux)
	defer baoServer.Close()

	client := openbao.NewClient(baoServer.URL, "test-token", true)
	handler := &Handler{
		Client:   client,
		Keys:     kp,
		EntityID: "https://sso.example.com",
		SSOURL:   "https://sso.example.com/saml/sso",
	}

	req := httptest.NewRequest(http.MethodGet, "/saml/metadata", nil)
	w := httptest.NewRecorder()
	handler.ServeMetadata(w, req)

	// Should still succeed (empty cert is valid for base64 encoding)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	// Restore for other tests
	kp.Certificate.Raw = origRaw
}

func TestServeMetadataMethodNotAllowed(t *testing.T) {
	handler, baoServer := makeTestHandler(t)
	defer baoServer.Close()

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		req := httptest.NewRequest(method, "/saml/metadata", nil)
		w := httptest.NewRecorder()
		handler.ServeMetadata(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: expected 405, got %d", method, w.Code)
		}
	}
}

func TestServeSSOGetMissingSAMLRequest(t *testing.T) {
	handler, baoServer := makeTestHandler(t)
	defer baoServer.Close()

	req := httptest.NewRequest(http.MethodGet, "/saml/sso", nil)
	w := httptest.NewRecorder()
	handler.ServeSSO(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestServeSSOGetInvalidSAMLRequest(t *testing.T) {
	handler, baoServer := makeTestHandler(t)
	defer baoServer.Close()

	req := httptest.NewRequest(http.MethodGet, "/saml/sso?SAMLRequest=invalid!!!", nil)
	w := httptest.NewRecorder()
	handler.ServeSSO(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestServeSSOGetValid(t *testing.T) {
	handler, baoServer := makeTestHandler(t)
	defer baoServer.Close()

	reqXML := `<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="_req1" Version="2.0" IssueInstant="2026-01-01T00:00:00Z" AssertionConsumerServiceURL="https://sp.example.com/acs"><saml:Issuer>https://sp.example.com</saml:Issuer></samlp:AuthnRequest>`

	var buf bytes.Buffer
	w2, _ := flate.NewWriter(&buf, flate.DefaultCompression)
	w2.Write([]byte(reqXML))
	w2.Close()
	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())
	// Double URL-encode: once for the query string parse, once for ParseAuthnRequestRedirect's QueryUnescape
	doubleEncoded := url.QueryEscape(url.QueryEscape(b64))

	req := httptest.NewRequest(http.MethodGet, "/saml/sso?SAMLRequest="+doubleEncoded+"&RelayState=test", nil)
	w := httptest.NewRecorder()
	handler.ServeSSO(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	// Should show login form
	body := w.Body.String()
	if !strings.Contains(body, "Sign In") {
		t.Error("expected login form")
	}
	if !strings.Contains(body, "RelayState") {
		t.Error("expected RelayState in form")
	}
}

func TestServeSSOPostMissingSAMLRequest(t *testing.T) {
	handler, baoServer := makeTestHandler(t)
	defer baoServer.Close()

	form := url.Values{}
	form.Set("RelayState", "test")
	req := httptest.NewRequest(http.MethodPost, "/saml/sso", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handler.ServeSSO(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestServeSSOPostInvalidSAMLRequest(t *testing.T) {
	handler, baoServer := makeTestHandler(t)
	defer baoServer.Close()

	form := url.Values{}
	form.Set("SAMLRequest", "notvalidbase64!!!")
	req := httptest.NewRequest(http.MethodPost, "/saml/sso", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handler.ServeSSO(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestServeSSOPostValid(t *testing.T) {
	handler, baoServer := makeTestHandler(t)
	defer baoServer.Close()

	reqXML := `<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="_req2" Version="2.0" IssueInstant="2026-01-01T00:00:00Z" AssertionConsumerServiceURL="https://sp.example.com/acs"><saml:Issuer>https://sp.example.com</saml:Issuer></samlp:AuthnRequest>`
	b64 := base64.StdEncoding.EncodeToString([]byte(reqXML))

	form := url.Values{}
	form.Set("SAMLRequest", b64)
	form.Set("RelayState", "test")
	req := httptest.NewRequest(http.MethodPost, "/saml/sso", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handler.ServeSSO(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Sign In") {
		t.Error("expected login form")
	}
}

func TestServeSSOMethodNotAllowed(t *testing.T) {
	handler, baoServer := makeTestHandler(t)
	defer baoServer.Close()

	req := httptest.NewRequest(http.MethodPut, "/saml/sso", nil)
	w := httptest.NewRecorder()
	handler.ServeSSO(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestServeLoginMethodNotAllowed(t *testing.T) {
	handler, baoServer := makeTestHandler(t)
	defer baoServer.Close()

	req := httptest.NewRequest(http.MethodGet, "/saml/login", nil)
	w := httptest.NewRecorder()
	handler.ServeLogin(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestServeLoginMissingCredentials(t *testing.T) {
	handler, baoServer := makeTestHandler(t)
	defer baoServer.Close()

	reqXML := `<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="_req3" Version="2.0" IssueInstant="2026-01-01T00:00:00Z" AssertionConsumerServiceURL="https://sp.example.com/acs"><saml:Issuer>https://sp.example.com</saml:Issuer></samlp:AuthnRequest>`
	b64 := base64.StdEncoding.EncodeToString([]byte(reqXML))

	form := url.Values{}
	form.Set("SAMLRequest", b64)
	form.Set("RelayState", "test")
	// No username or password
	req := httptest.NewRequest(http.MethodPost, "/saml/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handler.ServeLogin(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (login form with error), got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Username and password are required") {
		t.Error("expected error message about missing credentials")
	}
}

func TestServeLoginMissingSAMLRequest(t *testing.T) {
	handler, baoServer := makeTestHandler(t)
	defer baoServer.Close()

	form := url.Values{}
	form.Set("username", "user")
	form.Set("password", "pass")
	// No SAMLRequest
	req := httptest.NewRequest(http.MethodPost, "/saml/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handler.ServeLogin(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestServeLoginAuthFailure(t *testing.T) {
	kp, _ := generateKeyPair()

	baoMux := http.NewServeMux()
	baoMux.HandleFunc("/v1/auth/userpass/login/baduser", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	baoServer := httptest.NewServer(baoMux)
	defer baoServer.Close()

	client := openbao.NewClient(baoServer.URL, "test-token", true)
	handler := &Handler{
		Client:   client,
		Keys:     kp,
		EntityID: "https://sso.example.com",
		SSOURL:   "https://sso.example.com/saml/sso",
	}

	reqXML := `<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="_req4" Version="2.0" IssueInstant="2026-01-01T00:00:00Z" AssertionConsumerServiceURL="https://sp.example.com/acs"><saml:Issuer>https://sp.example.com</saml:Issuer></samlp:AuthnRequest>`
	b64 := base64.StdEncoding.EncodeToString([]byte(reqXML))

	form := url.Values{}
	form.Set("SAMLRequest", b64)
	form.Set("RelayState", "test")
	form.Set("username", "baduser")
	form.Set("password", "badpass")
	req := httptest.NewRequest(http.MethodPost, "/saml/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handler.ServeLogin(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (login form with error), got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Invalid credentials") {
		t.Error("expected Invalid credentials error")
	}
}

func TestServeLoginSuccess(t *testing.T) {
	kp, _ := generateKeyPair()

	baoMux := http.NewServeMux()
	baoMux.HandleFunc("/v1/auth/userpass/login/testuser", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"auth": map[string]interface{}{
				"client_token": "s.user-token",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	baoMux.HandleFunc("/v1/identity/entity/name/testuser", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"id":        "entity-1",
				"name":      "testuser",
				"metadata":  map[string]string{"email": "test@example.com"},
				"group_ids": []string{"group-1"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	baoServer := httptest.NewServer(baoMux)
	defer baoServer.Close()

	client := openbao.NewClient(baoServer.URL, "test-token", true)
	handler := &Handler{
		Client:   client,
		Keys:     kp,
		EntityID: "https://sso.example.com",
		SSOURL:   "https://sso.example.com/saml/sso",
	}

	// Use redirect-format SAMLRequest (deflate + base64 + URL-encode for ParseAuthnRequestRedirect)
	reqXML := `<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="_req5" Version="2.0" IssueInstant="2026-01-01T00:00:00Z" AssertionConsumerServiceURL="https://sp.example.com/acs"><saml:Issuer>https://sp.example.com</saml:Issuer></samlp:AuthnRequest>`

	var buf bytes.Buffer
	fw, _ := flate.NewWriter(&buf, flate.DefaultCompression)
	fw.Write([]byte(reqXML))
	fw.Close()
	b64 := url.QueryEscape(base64.StdEncoding.EncodeToString(buf.Bytes()))

	form := url.Values{}
	form.Set("SAMLRequest", b64)
	form.Set("RelayState", "test-relay")
	form.Set("username", "testuser")
	form.Set("password", "password")
	req := httptest.NewRequest(http.MethodPost, "/saml/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handler.ServeLogin(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	// Should render POST binding form
	if !strings.Contains(body, "SAMLResponse") {
		t.Error("expected SAMLResponse in response")
	}
	if !strings.Contains(body, "test-relay") {
		t.Error("expected RelayState in response")
	}
	if !strings.Contains(body, "https://sp.example.com/acs") {
		t.Error("expected ACS URL in form action")
	}
}

func TestServeLoginSuccessWithEntityNoEmail(t *testing.T) {
	kp, _ := generateKeyPair()

	baoMux := http.NewServeMux()
	baoMux.HandleFunc("/v1/auth/userpass/login/testuser", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"auth": map[string]interface{}{
				"client_token": "s.user-token",
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	baoMux.HandleFunc("/v1/identity/entity/name/testuser", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"id":       "entity-1",
				"name":     "testuser",
				"metadata": map[string]string{}, // no email
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	baoServer := httptest.NewServer(baoMux)
	defer baoServer.Close()

	client := openbao.NewClient(baoServer.URL, "test-token", true)
	handler := &Handler{
		Client:   client,
		Keys:     kp,
		EntityID: "https://sso.example.com",
		SSOURL:   "https://sso.example.com/saml/sso",
	}

	reqXML := `<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="_reqNoEmail" Version="2.0" IssueInstant="2026-01-01T00:00:00Z" AssertionConsumerServiceURL="https://sp.example.com/acs"><saml:Issuer>https://sp.example.com</saml:Issuer></samlp:AuthnRequest>`
	b64 := base64.StdEncoding.EncodeToString([]byte(reqXML))

	form := url.Values{}
	form.Set("SAMLRequest", b64)
	form.Set("username", "testuser")
	form.Set("password", "password")
	req := httptest.NewRequest(http.MethodPost, "/saml/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handler.ServeLogin(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "SAMLResponse") {
		t.Error("expected SAMLResponse")
	}
}

func TestServeLoginSuccessPostFormat(t *testing.T) {
	kp, _ := generateKeyPair()

	baoMux := http.NewServeMux()
	baoMux.HandleFunc("/v1/auth/userpass/login/testuser", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"auth": map[string]interface{}{
				"client_token": "s.user-token",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	baoMux.HandleFunc("/v1/identity/entity/name/testuser", func(w http.ResponseWriter, r *http.Request) {
		// Return 404 to test fallback to basic info
		w.WriteHeader(http.StatusNotFound)
	})
	baoServer := httptest.NewServer(baoMux)
	defer baoServer.Close()

	client := openbao.NewClient(baoServer.URL, "test-token", true)
	handler := &Handler{
		Client:   client,
		Keys:     kp,
		EntityID: "https://sso.example.com",
		SSOURL:   "https://sso.example.com/saml/sso",
	}

	// Use POST-format SAMLRequest (base64 only, no deflate)
	reqXML := `<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="_reqPost" Version="2.0" IssueInstant="2026-01-01T00:00:00Z" AssertionConsumerServiceURL="https://sp.example.com/acs"><saml:Issuer>https://sp.example.com</saml:Issuer></samlp:AuthnRequest>`
	b64 := base64.StdEncoding.EncodeToString([]byte(reqXML))

	form := url.Values{}
	form.Set("SAMLRequest", b64)
	form.Set("RelayState", "")
	form.Set("username", "testuser")
	form.Set("password", "password")
	req := httptest.NewRequest(http.MethodPost, "/saml/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handler.ServeLogin(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "SAMLResponse") {
		t.Error("expected SAMLResponse")
	}
}

func TestServeLoginInvalidSAMLRequest(t *testing.T) {
	kp, _ := generateKeyPair()

	baoMux := http.NewServeMux()
	baoMux.HandleFunc("/v1/auth/userpass/login/testuser", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"auth": map[string]interface{}{
				"client_token": "s.user-token",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	baoServer := httptest.NewServer(baoMux)
	defer baoServer.Close()

	client := openbao.NewClient(baoServer.URL, "test-token", true)
	handler := &Handler{
		Client:   client,
		Keys:     kp,
		EntityID: "https://sso.example.com",
		SSOURL:   "https://sso.example.com/saml/sso",
	}

	// SAMLRequest that is valid base64 but invalid XML for both formats
	invalidXML := base64.StdEncoding.EncodeToString([]byte("not xml at all"))

	form := url.Values{}
	form.Set("SAMLRequest", invalidXML)
	form.Set("username", "testuser")
	form.Set("password", "password")
	req := httptest.NewRequest(http.MethodPost, "/saml/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handler.ServeLogin(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestServeLoginBuildResponseError(t *testing.T) {
	// Use a key that's too small to sign, triggering BuildSignedResponse error
	smallKey := &rsa.PrivateKey{
		PublicKey: rsa.PublicKey{
			N: big.NewInt(15),
			E: 3,
		},
		D: big.NewInt(5),
	}
	kp, _ := generateKeyPair()
	kp.PrivateKey = smallKey

	baoMux := http.NewServeMux()
	baoMux.HandleFunc("/v1/auth/userpass/login/testuser", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"auth": map[string]interface{}{
				"client_token": "s.token",
			},
		})
	})
	baoMux.HandleFunc("/v1/identity/entity/name/testuser", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	baoServer := httptest.NewServer(baoMux)
	defer baoServer.Close()

	client := openbao.NewClient(baoServer.URL, "test-token", true)
	handler := &Handler{
		Client:   client,
		Keys:     kp,
		EntityID: "https://sso.example.com",
		SSOURL:   "https://sso.example.com/saml/sso",
	}

	reqXML := `<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="_reqBad" Version="2.0" IssueInstant="2026-01-01T00:00:00Z" AssertionConsumerServiceURL="https://sp.example.com/acs"><saml:Issuer>https://sp.example.com</saml:Issuer></samlp:AuthnRequest>`
	b64 := base64.StdEncoding.EncodeToString([]byte(reqXML))

	form := url.Values{}
	form.Set("SAMLRequest", b64)
	form.Set("username", "testuser")
	form.Set("password", "password")
	req := httptest.NewRequest(http.MethodPost, "/saml/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handler.ServeLogin(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for signing error, got %d: %s", w.Code, w.Body.String())
	}
}

func TestServeLoginNoIssuer(t *testing.T) {
	kp, _ := generateKeyPair()

	baoMux := http.NewServeMux()
	baoMux.HandleFunc("/v1/auth/userpass/login/testuser", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"auth": map[string]interface{}{
				"client_token": "s.user-token",
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	baoMux.HandleFunc("/v1/identity/entity/name/testuser", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	baoServer := httptest.NewServer(baoMux)
	defer baoServer.Close()

	client := openbao.NewClient(baoServer.URL, "test-token", true)
	handler := &Handler{
		Client:   client,
		Keys:     kp,
		EntityID: "https://sso.example.com",
		SSOURL:   "https://sso.example.com/saml/sso",
	}

	// AuthnRequest with ACS URL but no Issuer - audience should fall back to ACS URL
	reqXML := `<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="_reqNoIss" Version="2.0" IssueInstant="2026-01-01T00:00:00Z" AssertionConsumerServiceURL="https://sp.example.com/acs"></samlp:AuthnRequest>`
	b64 := base64.StdEncoding.EncodeToString([]byte(reqXML))

	form := url.Values{}
	form.Set("SAMLRequest", b64)
	form.Set("username", "testuser")
	form.Set("password", "password")
	req := httptest.NewRequest(http.MethodPost, "/saml/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handler.ServeLogin(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "SAMLResponse") {
		t.Error("expected SAMLResponse in response")
	}
}

func TestServeLoginNoACSURL(t *testing.T) {
	kp, _ := generateKeyPair()

	baoMux := http.NewServeMux()
	baoMux.HandleFunc("/v1/auth/userpass/login/testuser", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"auth": map[string]interface{}{
				"client_token": "s.user-token",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	baoMux.HandleFunc("/v1/identity/entity/name/testuser", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	baoServer := httptest.NewServer(baoMux)
	defer baoServer.Close()

	client := openbao.NewClient(baoServer.URL, "test-token", true)
	handler := &Handler{
		Client:   client,
		Keys:     kp,
		EntityID: "https://sso.example.com",
		SSOURL:   "https://sso.example.com/saml/sso",
	}

	// AuthnRequest without ACS URL
	reqXML := `<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="_reqNoACS" Version="2.0" IssueInstant="2026-01-01T00:00:00Z"><saml:Issuer>https://sp.example.com</saml:Issuer></samlp:AuthnRequest>`
	b64 := base64.StdEncoding.EncodeToString([]byte(reqXML))

	form := url.Values{}
	form.Set("SAMLRequest", b64)
	form.Set("username", "testuser")
	form.Set("password", "password")
	req := httptest.NewRequest(http.MethodPost, "/saml/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handler.ServeLogin(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing ACS URL, got %d: %s", w.Code, w.Body.String())
	}
}

func TestServeSSOPostParseFormError(t *testing.T) {
	handler, baoServer := makeTestHandler(t)
	defer baoServer.Close()

	// Body with invalid encoding that causes ParseForm to fail
	req := httptest.NewRequest(http.MethodPost, "/saml/sso", &errorReader{})
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.ContentLength = 100
	w := httptest.NewRecorder()
	handler.ServeSSO(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

type errorReader struct{}

func (r *errorReader) Read(p []byte) (int, error) {
	return 0, fmt.Errorf("read error")
}

func TestServeLoginParseFormError(t *testing.T) {
	handler, baoServer := makeTestHandler(t)
	defer baoServer.Close()

	// Use errorReader to cause ParseForm to fail
	req := httptest.NewRequest(http.MethodPost, "/saml/login", &errorReader{})
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.ContentLength = 100
	w := httptest.NewRecorder()
	handler.ServeLogin(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestServeSLO(t *testing.T) {
	handler, baoServer := makeTestHandler(t)
	defer baoServer.Close()

	req := httptest.NewRequest(http.MethodGet, "/saml/slo", nil)
	w := httptest.NewRecorder()
	handler.ServeSLO(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Logged Out") {
		t.Error("expected logout message")
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("expected text/html, got %s", ct)
	}
}

func TestServeSLOMethodNotAllowed(t *testing.T) {
	handler, baoServer := makeTestHandler(t)
	defer baoServer.Close()

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		req := httptest.NewRequest(method, "/saml/slo", nil)
		w := httptest.NewRecorder()
		handler.ServeSLO(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: expected 405, got %d", method, w.Code)
		}
	}
}

func TestShowLoginForm(t *testing.T) {
	handler, baoServer := makeTestHandler(t)
	defer baoServer.Close()

	w := httptest.NewRecorder()
	handler.showLoginForm(w, "saml-req-value", "relay-state-value", "Error message")

	body := w.Body.String()
	if !strings.Contains(body, "saml-req-value") {
		t.Error("expected SAMLRequest value in form")
	}
	if !strings.Contains(body, "relay-state-value") {
		t.Error("expected RelayState value in form")
	}
	if !strings.Contains(body, "Error message") {
		t.Error("expected error message in form")
	}
}

func TestShowLoginFormNoError(t *testing.T) {
	handler, baoServer := makeTestHandler(t)
	defer baoServer.Close()

	w := httptest.NewRecorder()
	handler.showLoginForm(w, "req", "relay", "")

	body := w.Body.String()
	if strings.Contains(body, `style="color:red"`) {
		t.Error("should not show error paragraph when no error")
	}
}

