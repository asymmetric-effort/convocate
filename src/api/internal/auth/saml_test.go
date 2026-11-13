package auth

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestExtractSAMLResponseFromHTML(t *testing.T) {
	html := `<form method="post"><input type="hidden" name="SAMLResponse" value="dGVzdA=="/></form>`
	got, err := extractSAMLResponseFromHTML(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "dGVzdA==" {
		t.Errorf("got %q, want %q", got, "dGVzdA==")
	}
}

func TestExtractSAMLResponseFromHTML_NotFound(t *testing.T) {
	html := `<form><input type="hidden" name="other" value="test"/></form>`
	_, err := extractSAMLResponseFromHTML(html)
	if err == nil {
		t.Fatal("expected error when SAMLResponse not present")
	}
}

func TestExtractSAMLResponseFromHTML_Unterminated(t *testing.T) {
	html := `name="SAMLResponse" value="abc123`
	_, err := extractSAMLResponseFromHTML(html)
	if err == nil {
		t.Fatal("expected error for unterminated value")
	}
}

func buildTestSAMLResponseXML(nameID, email string, groups []string) string {
	groupAttrs := ""
	for _, g := range groups {
		groupAttrs += fmt.Sprintf(`<saml:AttributeValue>%s</saml:AttributeValue>`, g)
	}
	return fmt.Sprintf(`<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol">
  <saml:Assertion xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion">
    <saml:Subject>
      <saml:NameID>%s</saml:NameID>
    </saml:Subject>
    <saml:AttributeStatement>
      <saml:Attribute Name="email">
        <saml:AttributeValue>%s</saml:AttributeValue>
      </saml:Attribute>
      <saml:Attribute Name="groups">
        %s
      </saml:Attribute>
    </saml:AttributeStatement>
  </saml:Assertion>
</samlp:Response>`, nameID, email, groupAttrs)
}

func TestExtractPrincipalFromSAML(t *testing.T) {
	xml := buildTestSAMLResponseXML("testuser", "test@example.com", []string{"admin", "node-ops"})
	principal, err := extractPrincipalFromSAML([]byte(xml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if principal.Username != "testuser" {
		t.Errorf("Username = %q, want %q", principal.Username, "testuser")
	}
	if principal.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", principal.Email, "test@example.com")
	}
	if principal.IDP != "saml" {
		t.Errorf("IDP = %q, want %q", principal.IDP, "saml")
	}
	if principal.ID != "testuser" {
		t.Errorf("ID = %q, want %q", principal.ID, "testuser")
	}
	if len(principal.Roles) != 2 {
		t.Errorf("Roles = %v, want 2 items", principal.Roles)
	}
	if len(principal.Groups) != 2 {
		t.Errorf("Groups = %v, want 2 items", principal.Groups)
	}
	// admin role gives all applets
	if len(principal.AuthorizedApplets) != len(allApplets) {
		t.Errorf("AuthorizedApplets = %v, want all applets", principal.AuthorizedApplets)
	}
}

func TestExtractPrincipalFromSAML_EmptyNameID(t *testing.T) {
	xml := buildTestSAMLResponseXML("", "test@example.com", []string{"admin"})
	_, err := extractPrincipalFromSAML([]byte(xml))
	if err == nil {
		t.Fatal("expected error for empty NameID")
	}
}

func TestExtractPrincipalFromSAML_InvalidXML(t *testing.T) {
	_, err := extractPrincipalFromSAML([]byte("not valid xml"))
	if err == nil {
		t.Fatal("expected error for invalid XML")
	}
}

func TestExtractPrincipalFromSAML_NoGroups(t *testing.T) {
	xml := `<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol">
  <saml:Assertion xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion">
    <saml:Subject>
      <saml:NameID>testuser</saml:NameID>
    </saml:Subject>
    <saml:AttributeStatement>
      <saml:Attribute Name="email">
        <saml:AttributeValue>test@example.com</saml:AttributeValue>
      </saml:Attribute>
    </saml:AttributeStatement>
  </saml:Assertion>
</samlp:Response>`
	principal, err := extractPrincipalFromSAML([]byte(xml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(principal.Roles) != 0 {
		t.Errorf("Roles = %v, want empty", principal.Roles)
	}
	if len(principal.AuthorizedApplets) != 0 {
		t.Errorf("AuthorizedApplets = %v, want empty", principal.AuthorizedApplets)
	}
}

func TestSamlAgentURL(t *testing.T) {
	os.Setenv("SAML_SCIM_AGENT_URL", "https://test:443")
	t.Cleanup(func() { os.Unsetenv("SAML_SCIM_AGENT_URL") })
	got := samlAgentURL()
	if got != "https://test:443" {
		t.Errorf("got %q, want %q", got, "https://test:443")
	}
}

func TestSamlAgentURL_NotSet(t *testing.T) {
	os.Unsetenv("SAML_SCIM_AGENT_URL")
	got := samlAgentURL()
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestSamlLogin_Integration(t *testing.T) {
	// Build a valid SAMLResponse XML
	samlXML := buildTestSAMLResponseXML("integrationuser", "int@example.com", []string{"node-ops"})
	samlB64 := base64.StdEncoding.EncodeToString([]byte(samlXML))

	// Create a mock SAML agent server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/saml/login" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Return HTML with SAMLResponse
		html := fmt.Sprintf(`<html><body><form method="post">
			<input type="hidden" name="SAMLResponse" value="%s"/>
			<input type="hidden" name="RelayState" value="convocate"/>
		</form></body></html>`, samlB64)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	}))
	defer server.Close()

	principal, err := samlLogin(server.URL, "integrationuser", "password123", "")
	if err != nil {
		t.Fatalf("samlLogin failed: %v", err)
	}
	if principal.Username != "integrationuser" {
		t.Errorf("Username = %q, want %q", principal.Username, "integrationuser")
	}
	if principal.Email != "int@example.com" {
		t.Errorf("Email = %q, want %q", principal.Email, "int@example.com")
	}
	if principal.IDP != "saml" {
		t.Errorf("IDP = %q, want %q", principal.IDP, "saml")
	}
	if len(principal.Roles) != 1 || principal.Roles[0] != "node-ops" {
		t.Errorf("Roles = %v, want [node-ops]", principal.Roles)
	}
}

func TestSamlLogin_BadPassword(t *testing.T) {
	// Mock server returns login form without SAMLResponse (authentication failure)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `<html><body><form method="post">
			<input type="text" name="username"/>
			<input type="password" name="password"/>
			<button type="submit">Login</button>
		</form></body></html>`
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	}))
	defer server.Close()

	_, err := samlLogin(server.URL, "baduser", "wrongpassword", "")
	if err == nil {
		t.Fatal("expected error for bad credentials")
	}
}

func TestSamlLogin_ServerError(t *testing.T) {
	// Mock server returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := samlLogin(server.URL, "user", "pass", "")
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestSamlLogin_ConnectionRefused(t *testing.T) {
	_, err := samlLogin("http://127.0.0.1:1", "user", "pass", "")
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestSamlLogin_InvalidSAMLResponse(t *testing.T) {
	// Return SAMLResponse that is not valid base64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `<form><input type="hidden" name="SAMLResponse" value="%%%notbase64%%%"/></form>`
		w.Write([]byte(html))
	}))
	defer server.Close()

	_, err := samlLogin(server.URL, "user", "pass", "")
	if err == nil {
		t.Fatal("expected error for invalid base64 SAMLResponse")
	}
}

func TestSamlLogin_MalformedXML(t *testing.T) {
	// Return SAMLResponse with valid base64 but invalid XML
	badXML := base64.StdEncoding.EncodeToString([]byte("not xml at all"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := fmt.Sprintf(`<form><input type="hidden" name="SAMLResponse" value="%s"/></form>`, badXML)
		w.Write([]byte(html))
	}))
	defer server.Close()

	_, err := samlLogin(server.URL, "user", "pass", "")
	if err == nil {
		t.Fatal("expected error for malformed XML in SAMLResponse")
	}
}

// buildMockSAMLServer creates a test SAML agent server that returns a valid SAMLResponse.
func buildMockSAMLServer(t *testing.T, nameID, email string, groups []string) *httptest.Server {
	t.Helper()
	samlXML := buildTestSAMLResponseXML(nameID, email, groups)
	samlB64 := base64.StdEncoding.EncodeToString([]byte(samlXML))

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/saml/login" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		html := fmt.Sprintf(`<html><body><form method="post">
			<input type="hidden" name="SAMLResponse" value="%s"/>
			<input type="hidden" name="RelayState" value="convocate"/>
		</form></body></html>`, samlB64)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	}))
}

// buildMockBaoForMFA creates a mock OpenBao server for MFA tests.
// If enrolled is true, the entity will have mfa_secrets for the given method ID.
// If mfaTriggered is true, the userpass login will return mfa_requirement.
func buildMockBaoForMFA(t *testing.T, entityID, methodID string, enrolled, mfaTriggered bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/identity/entity/id/"+entityID && r.Method == http.MethodGet:
			mfaSecrets := map[string]any{}
			if enrolled {
				mfaSecrets[methodID] = map[string]any{"type": "totp"}
			}
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":          entityID,
					"name":        "testuser",
					"metadata":    map[string]string{"name": "Test User", "email": "test@example.com"},
					"policies":    []string{"default"},
					"group_ids":   []string{},
					"mfa_secrets": mfaSecrets,
				},
			})

		case r.URL.Path == "/v1/auth/userpass/login/testuser" && r.Method == http.MethodPost:
			if mfaTriggered {
				json.NewEncoder(w).Encode(map[string]any{
					"auth": map[string]any{
						"client_token":   "",
						"entity_id":      "",
						"policies":       nil,
						"metadata":       nil,
						"lease_duration": 0,
						"mfa_requirement": map[string]any{
							"mfa_request_id": "mfa-req-saml-001",
						},
					},
				})
			} else {
				json.NewEncoder(w).Encode(map[string]any{
					"auth": map[string]any{
						"client_token":   "tok-saml-valid",
						"entity_id":      entityID,
						"policies":       []string{"default"},
						"metadata":       map[string]string{"name": "Test User"},
						"lease_duration": 3600,
					},
				})
			}

		case r.URL.Path == "/v1/sys/mfa/validate" && r.Method == http.MethodPost:
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			payload, _ := body["mfa_payload"].(map[string]any)
			codes, _ := payload[methodID].([]any)
			if len(codes) == 1 && codes[0] == "123456" {
				json.NewEncoder(w).Encode(map[string]any{
					"auth": map[string]any{
						"client_token":   "tok-mfa-saml-ok",
						"entity_id":      entityID,
						"policies":       []string{"default"},
						"metadata":       map[string]string{"name": "Test User"},
						"lease_duration": 3600,
					},
				})
			} else {
				w.WriteHeader(http.StatusForbidden)
			}

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestSamlLogin_MFANotConfigured(t *testing.T) {
	// No MFA method ID set — should pass without MFA
	os.Unsetenv("OPENBAO_MFA_METHOD_ID")

	samlServer := buildMockSAMLServer(t, "testuser", "test@example.com", []string{"node-ops"})
	defer samlServer.Close()

	principal, err := samlLogin(samlServer.URL, "testuser", "password123", "")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if principal.Username != "testuser" {
		t.Errorf("Username = %q, want %q", principal.Username, "testuser")
	}
}

func TestSamlLogin_MFANotEnrolled(t *testing.T) {
	entityID := "testuser"
	methodID := "test-mfa-method"

	bao := buildMockBaoForMFA(t, entityID, methodID, false, false)
	defer bao.Close()

	os.Setenv("OPENBAO_ADDR", bao.URL)
	os.Setenv("OPENBAO_MFA_METHOD_ID", methodID)
	os.Setenv("OPENBAO_TOKEN", "test-service-token")
	t.Cleanup(func() {
		os.Unsetenv("OPENBAO_ADDR")
		os.Unsetenv("OPENBAO_MFA_METHOD_ID")
		os.Unsetenv("OPENBAO_TOKEN")
	})

	samlServer := buildMockSAMLServer(t, entityID, "test@example.com", []string{"node-ops"})
	defer samlServer.Close()

	principal, err := samlLogin(samlServer.URL, "testuser", "password123", "")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if principal.Username != entityID {
		t.Errorf("Username = %q, want %q", principal.Username, entityID)
	}
}

func TestSamlLogin_MFARequired_NoCode(t *testing.T) {
	entityID := "testuser"
	methodID := "test-mfa-method"

	bao := buildMockBaoForMFA(t, entityID, methodID, true, true)
	defer bao.Close()

	os.Setenv("OPENBAO_ADDR", bao.URL)
	os.Setenv("OPENBAO_MFA_METHOD_ID", methodID)
	os.Setenv("OPENBAO_TOKEN", "test-service-token")
	t.Cleanup(func() {
		os.Unsetenv("OPENBAO_ADDR")
		os.Unsetenv("OPENBAO_MFA_METHOD_ID")
		os.Unsetenv("OPENBAO_TOKEN")
	})

	samlServer := buildMockSAMLServer(t, entityID, "test@example.com", []string{"node-ops"})
	defer samlServer.Close()

	_, err := samlLogin(samlServer.URL, "testuser", "password123", "")
	if err == nil {
		t.Fatal("expected ErrMFARequired, got nil")
	}
	if !errors.Is(err, ErrMFARequired) {
		t.Errorf("expected ErrMFARequired, got: %v", err)
	}
}

func TestSamlLogin_MFASuccess(t *testing.T) {
	entityID := "testuser"
	methodID := "test-mfa-method"

	bao := buildMockBaoForMFA(t, entityID, methodID, true, true)
	defer bao.Close()

	os.Setenv("OPENBAO_ADDR", bao.URL)
	os.Setenv("OPENBAO_MFA_METHOD_ID", methodID)
	os.Setenv("OPENBAO_TOKEN", "test-service-token")
	t.Cleanup(func() {
		os.Unsetenv("OPENBAO_ADDR")
		os.Unsetenv("OPENBAO_MFA_METHOD_ID")
		os.Unsetenv("OPENBAO_TOKEN")
	})

	samlServer := buildMockSAMLServer(t, entityID, "test@example.com", []string{"node-ops"})
	defer samlServer.Close()

	principal, err := samlLogin(samlServer.URL, "testuser", "password123", "123456")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if principal.Username != entityID {
		t.Errorf("Username = %q, want %q", principal.Username, entityID)
	}
}

func TestSamlLogin_MFAInvalidCode(t *testing.T) {
	entityID := "testuser"
	methodID := "test-mfa-method"

	bao := buildMockBaoForMFA(t, entityID, methodID, true, true)
	defer bao.Close()

	os.Setenv("OPENBAO_ADDR", bao.URL)
	os.Setenv("OPENBAO_MFA_METHOD_ID", methodID)
	os.Setenv("OPENBAO_TOKEN", "test-service-token")
	t.Cleanup(func() {
		os.Unsetenv("OPENBAO_ADDR")
		os.Unsetenv("OPENBAO_MFA_METHOD_ID")
		os.Unsetenv("OPENBAO_TOKEN")
	})

	samlServer := buildMockSAMLServer(t, entityID, "test@example.com", []string{"node-ops"})
	defer samlServer.Close()

	_, err := samlLogin(samlServer.URL, "testuser", "password123", "000000")
	if err == nil {
		t.Fatal("expected error for invalid MFA code")
	}
	if errors.Is(err, ErrMFARequired) {
		t.Error("should not be ErrMFARequired, should be invalid code error")
	}
}

func TestSamlLogin_MFALoginFails(t *testing.T) {
	entityID := "testuser"
	methodID := "test-mfa-method"

	// Create a bao that has MFA enrolled but userpass login fails
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/identity/entity/id/"+entityID && r.Method == http.MethodGet:
			mfaSecrets := map[string]any{methodID: map[string]any{"type": "totp"}}
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":          entityID,
					"name":        "testuser",
					"metadata":    map[string]string{},
					"policies":    []string{"default"},
					"group_ids":   []string{},
					"mfa_secrets": mfaSecrets,
				},
			})
		case r.URL.Path == "/v1/auth/userpass/login/testuser" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusUnauthorized)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer bao.Close()

	os.Setenv("OPENBAO_ADDR", bao.URL)
	os.Setenv("OPENBAO_MFA_METHOD_ID", methodID)
	os.Setenv("OPENBAO_TOKEN", "test-service-token")
	t.Cleanup(func() {
		os.Unsetenv("OPENBAO_ADDR")
		os.Unsetenv("OPENBAO_MFA_METHOD_ID")
		os.Unsetenv("OPENBAO_TOKEN")
	})

	samlServer := buildMockSAMLServer(t, entityID, "test@example.com", []string{"node-ops"})
	defer samlServer.Close()

	_, err := samlLogin(samlServer.URL, "testuser", "password123", "123456")
	if err == nil {
		t.Fatal("expected error when userpass login fails")
	}
}

func TestSamlLoginCore(t *testing.T) {
	samlServer := buildMockSAMLServer(t, "coreuser", "core@example.com", []string{"admin"})
	defer samlServer.Close()

	principal, err := samlLoginCore(samlServer.URL, "coreuser", "pass123")
	if err != nil {
		t.Fatalf("samlLoginCore failed: %v", err)
	}
	if principal.Username != "coreuser" {
		t.Errorf("Username = %q, want %q", principal.Username, "coreuser")
	}
	if principal.Email != "core@example.com" {
		t.Errorf("Email = %q, want %q", principal.Email, "core@example.com")
	}
}

// ——— Signature verification tests ———————————————————————————————————————

// testKeyPair generates a signing key pair and self-signed cert for tests.
func testKeyPair(t *testing.T, algo string) (crypto.PrivateKey, *x509.Certificate, []byte) {
	t.Helper()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-saml"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	var privKey crypto.PrivateKey
	var pubKey crypto.PublicKey
	switch algo {
	case "ed25519":
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatalf("generate ed25519: %v", err)
		}
		privKey, pubKey = priv, pub
	case "rsa":
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("generate rsa: %v", err)
		}
		privKey, pubKey = key, &key.PublicKey
	default:
		t.Fatalf("unknown algo %q", algo)
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, pubKey, privKey)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	return privKey, cert, certPEM
}

// buildSignedSAMLResponse builds a signed SAMLResponse using the same logic as
// the saml-scim-agent (re-marshal SignedInfo → samlCanonicalize → sign).
func buildSignedSAMLResponse(t *testing.T, nameID, email string, groups []string, privKey crypto.PrivateKey, cert *x509.Certificate) []byte {
	t.Helper()
	assertionID := "assert001"
	certB64 := base64.StdEncoding.EncodeToString(cert.Raw)

	// Build SignedInfo
	now := time.Now().UTC()
	var sigAlgo string
	switch privKey.(type) {
	case ed25519.PrivateKey:
		sigAlgo = "http://www.w3.org/2021/04/xmldsig-more#eddsa-ed25519"
	default:
		sigAlgo = "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"
	}

	// Minimal assertion XML for digest computation
	groupAttrs := ""
	for _, g := range groups {
		groupAttrs += fmt.Sprintf(`<saml:AttributeValue>%s</saml:AttributeValue>`, g)
	}
	assertionBody := fmt.Sprintf(
		`<saml:Assertion xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="%s" Version="2.0" IssueInstant="%s">`+
			`<saml:Subject><saml:NameID>%s</saml:NameID></saml:Subject>`+
			`<saml:AttributeStatement>`+
			`<saml:Attribute Name="email"><saml:AttributeValue>%s</saml:AttributeValue></saml:Attribute>`+
			`<saml:Attribute Name="groups">%s</saml:Attribute>`+
			`</saml:AttributeStatement>`+
			`</saml:Assertion>`,
		assertionID, now.Format(time.RFC3339), nameID, email, groupAttrs,
	)
	digest := sha256.Sum256(samlCanonicalize([]byte(assertionBody)))
	digestB64 := base64.StdEncoding.EncodeToString(digest[:])

	si := samlDSigSignedInfo{
		CanonicalizationMethod: samlDSigAlgorithm{Algorithm: "http://www.w3.org/2001/10/xml-exc-c14n#"},
		SignatureMethod:        samlDSigAlgorithm{Algorithm: sigAlgo},
		Reference: samlDSigReference{
			URI: "#" + assertionID,
			Transforms: samlDSigTransforms{
				Transform: []samlDSigAlgorithm{
					{Algorithm: "http://www.w3.org/2000/09/xmldsig#enveloped-signature"},
					{Algorithm: "http://www.w3.org/2001/10/xml-exc-c14n#"},
				},
			},
			DigestMethod: samlDSigAlgorithm{Algorithm: "http://www.w3.org/2001/04/xmlenc#sha256"},
			DigestValue:  samlDSigDigestVal{Value: digestB64},
		},
	}
	siXML, _ := xml.Marshal(si)
	canonical := samlCanonicalize(siXML)

	var sigBytes []byte
	switch k := privKey.(type) {
	case ed25519.PrivateKey:
		sigBytes = ed25519.Sign(k, canonical)
	case *rsa.PrivateKey:
		h := sha256.Sum256(canonical)
		var err error
		sigBytes, err = rsa.SignPKCS1v15(rand.Reader, k, crypto.SHA256, h[:])
		if err != nil {
			t.Fatalf("rsa sign: %v", err)
		}
	}
	sigB64 := base64.StdEncoding.EncodeToString(sigBytes)

	sigXML := samlSignatureXML{
		SignedInfo:     si,
		SignatureValue: samlDSigSigValue{Value: sigB64},
		KeyInfo: samlDSigKeyInfo{
			X509Data: samlDSigX509Data{
				X509Certificate: samlDSigX509Cert{Value: certB64},
			},
		},
	}
	sigXMLBytes, _ := xml.Marshal(sigXML)

	resp := fmt.Sprintf(
		`<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol">`+
			`<saml:Assertion xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="%s" Version="2.0" IssueInstant="%s">`+
			`<saml:Subject><saml:NameID>%s</saml:NameID></saml:Subject>`+
			`<saml:AttributeStatement>`+
			`<saml:Attribute Name="email"><saml:AttributeValue>%s</saml:AttributeValue></saml:Attribute>`+
			`<saml:Attribute Name="groups">%s</saml:Attribute>`+
			`</saml:AttributeStatement>`+
			`%s`+
			`</saml:Assertion>`+
			`</samlp:Response>`,
		assertionID, now.Format(time.RFC3339), nameID, email, groupAttrs, string(sigXMLBytes),
	)
	return []byte(resp)
}

func TestNewSAMLClient_NoCAFile(t *testing.T) {
	os.Unsetenv("SAML_SCIM_AGENT_CA_CERT_FILE")
	client, err := newSAMLClient()
	if err != nil {
		t.Fatalf("newSAMLClient: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewSAMLClient_MissingCAFile(t *testing.T) {
	t.Setenv("SAML_SCIM_AGENT_CA_CERT_FILE", "/nonexistent/ca.pem")
	_, err := newSAMLClient()
	if err == nil {
		t.Fatal("expected error for missing CA file")
	}
}

func TestVerifySAMLSignature_Ed25519(t *testing.T) {
	privKey, cert, certPEM := testKeyPair(t, "ed25519")

	f, _ := os.CreateTemp(t.TempDir(), "saml-cert-*.pem")
	f.Write(certPEM)
	f.Close()
	t.Setenv("SAML_SIGNING_CERT_FILE", f.Name())
	t.Cleanup(func() { os.Unsetenv("SAML_SIGNING_CERT_FILE") })

	samlXML := buildSignedSAMLResponse(t, "signeduser", "s@example.com", []string{"admin"}, privKey, cert)
	principal, err := extractPrincipalFromSAML(samlXML)
	if err != nil {
		t.Fatalf("extractPrincipalFromSAML: %v", err)
	}
	if principal.Username != "signeduser" {
		t.Errorf("Username = %q, want %q", principal.Username, "signeduser")
	}
}

func TestVerifySAMLSignature_RSA(t *testing.T) {
	privKey, cert, certPEM := testKeyPair(t, "rsa")

	f, _ := os.CreateTemp(t.TempDir(), "saml-cert-*.pem")
	f.Write(certPEM)
	f.Close()
	t.Setenv("SAML_SIGNING_CERT_FILE", f.Name())
	t.Cleanup(func() { os.Unsetenv("SAML_SIGNING_CERT_FILE") })

	samlXML := buildSignedSAMLResponse(t, "rsauser", "r@example.com", []string{"node-ops"}, privKey, cert)
	principal, err := extractPrincipalFromSAML(samlXML)
	if err != nil {
		t.Fatalf("extractPrincipalFromSAML: %v", err)
	}
	if principal.Username != "rsauser" {
		t.Errorf("Username = %q, want %q", principal.Username, "rsauser")
	}
}

func TestVerifySAMLSignature_WrongCert(t *testing.T) {
	privKey, cert, _ := testKeyPair(t, "ed25519")
	_, _, wrongCertPEM := testKeyPair(t, "ed25519") // different cert

	f, _ := os.CreateTemp(t.TempDir(), "saml-cert-*.pem")
	f.Write(wrongCertPEM)
	f.Close()
	t.Setenv("SAML_SIGNING_CERT_FILE", f.Name())
	t.Cleanup(func() { os.Unsetenv("SAML_SIGNING_CERT_FILE") })

	samlXML := buildSignedSAMLResponse(t, "user", "u@example.com", []string{}, privKey, cert)
	_, err := extractPrincipalFromSAML(samlXML)
	if err == nil {
		t.Fatal("expected error when cert does not match pinned cert")
	}
}

func TestVerifySAMLSignature_NoCertFile_NoPanic(t *testing.T) {
	os.Unsetenv("SAML_SIGNING_CERT_FILE")
	// Without SAML_SIGNING_CERT_FILE, unsigned responses are still accepted.
	xmlData := buildTestSAMLResponseXML("user", "u@example.com", []string{"admin"})
	principal, err := extractPrincipalFromSAML([]byte(xmlData))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if principal.Username != "user" {
		t.Errorf("Username = %q, want %q", principal.Username, "user")
	}
}

func TestVerifySAMLSignature_CertFileSet_NoSignature(t *testing.T) {
	_, _, certPEM := testKeyPair(t, "ed25519")
	f, _ := os.CreateTemp(t.TempDir(), "saml-cert-*.pem")
	f.Write(certPEM)
	f.Close()
	t.Setenv("SAML_SIGNING_CERT_FILE", f.Name())
	t.Cleanup(func() { os.Unsetenv("SAML_SIGNING_CERT_FILE") })

	// Unsigned XML — should fail when cert file is configured
	xmlData := buildTestSAMLResponseXML("user", "u@example.com", []string{"admin"})
	_, err := extractPrincipalFromSAML([]byte(xmlData))
	if err == nil {
		t.Fatal("expected error for unsigned assertion when SAML_SIGNING_CERT_FILE is set")
	}
}
