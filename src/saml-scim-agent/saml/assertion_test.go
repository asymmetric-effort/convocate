package saml

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"strings"
	"testing"
)

func TestBuildSignedResponse_Ed25519(t *testing.T) {
	kp, err := generateKeyPair("ed25519")
	if err != nil {
		t.Fatalf("generate keys: %v", err)
	}
	params := AssertionParams{
		EntityID:  "https://test-idp",
		SSOURL:    "https://test-idp/sso",
		RequestID: "_test123",
		ACSURL:    "https://sp/acs",
		Audience:  "https://sp",
		Username:  "testuser",
		Email:     "test@example.com",
		Groups:    []string{"admin"},
	}
	resp, err := BuildSignedResponse(params, kp)
	if err != nil {
		t.Fatalf("BuildSignedResponse: %v", err)
	}
	xmlStr := string(resp)
	if !strings.Contains(xmlStr, "eddsa-ed25519") {
		t.Error("expected ed25519 algorithm URI in signature")
	}
	if !strings.Contains(xmlStr, "SignatureValue") {
		t.Error("expected SignatureValue in response")
	}
}

func TestBuildSignedResponse_RSA(t *testing.T) {
	kp, err := generateKeyPair("rsa")
	if err != nil {
		t.Fatalf("generate keys: %v", err)
	}
	params := AssertionParams{
		EntityID:  "https://test-idp",
		SSOURL:    "https://test-idp/sso",
		RequestID: "_test456",
		ACSURL:    "https://sp/acs",
		Audience:  "https://sp",
		Username:  "rsauser",
		Email:     "rsa@example.com",
	}
	resp, err := BuildSignedResponse(params, kp)
	if err != nil {
		t.Fatalf("BuildSignedResponse: %v", err)
	}
	xmlStr := string(resp)
	if !strings.Contains(xmlStr, "rsa-sha256") {
		t.Error("expected rsa-sha256 algorithm URI in signature")
	}
}

func TestBuildSignedResponse_Ed25519_NoGroups(t *testing.T) {
	kp, err := generateKeyPair("ed25519")
	if err != nil {
		t.Fatalf("generate keys: %v", err)
	}
	params := AssertionParams{
		EntityID:  "https://test-idp",
		SSOURL:    "https://test-idp/sso",
		RequestID: "_testng",
		ACSURL:    "https://sp/acs",
		Audience:  "https://sp",
		Username:  "testuser",
		Email:     "test@example.com",
		Groups:    nil,
	}
	responseXML, err := BuildSignedResponse(params, kp)
	if err != nil {
		t.Fatalf("BuildSignedResponse: %v", err)
	}
	xmlStr := string(responseXML)
	if !strings.Contains(xmlStr, "email") {
		t.Error("expected email attribute in response")
	}
	if strings.Contains(xmlStr, `Name="groups"`) {
		t.Error("should not have groups attribute when no groups")
	}
}

func TestSignature_Ed25519_AlgorithmURI(t *testing.T) {
	kp, err := generateKeyPair("ed25519")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	params := AssertionParams{
		EntityID: "https://test", SSOURL: "https://test/sso",
		RequestID: "_v1", ACSURL: "https://sp/acs", Audience: "https://sp",
		Username: "u", Email: "u@e.com",
	}
	respXML, err := BuildSignedResponse(params, kp)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var resp Response
	if err := xml.Unmarshal(respXML, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Signature == nil {
		t.Fatal("response has no signature")
	}
	if resp.Signature.SignedInfo.SignatureMethod.Algorithm != "http://www.w3.org/2021/04/xmldsig-more#eddsa-ed25519" {
		t.Errorf("algorithm = %q, want eddsa-ed25519 URI", resp.Signature.SignedInfo.SignatureMethod.Algorithm)
	}
}

func TestSignature_RSA_AlgorithmURI(t *testing.T) {
	kp, err := generateKeyPair("rsa")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	params := AssertionParams{
		EntityID: "https://test", SSOURL: "https://test/sso",
		RequestID: "_v2", ACSURL: "https://sp/acs", Audience: "https://sp",
		Username: "u", Email: "u@e.com",
	}
	respXML, err := BuildSignedResponse(params, kp)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var resp Response
	if err := xml.Unmarshal(respXML, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Signature == nil {
		t.Fatal("response has no signature")
	}
	if resp.Signature.SignedInfo.SignatureMethod.Algorithm != "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256" {
		t.Errorf("algorithm = %q, want rsa-sha256 URI", resp.Signature.SignedInfo.SignatureMethod.Algorithm)
	}
}

func TestSignature_RSA_CryptoVerify(t *testing.T) {
	kp, err := generateKeyPair("rsa")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	params := AssertionParams{
		EntityID: "https://test", SSOURL: "https://test/sso",
		RequestID: "_v2rsa", ACSURL: "https://sp/acs", Audience: "https://sp",
		Username: "u", Email: "u@e.com",
	}
	respXML, err := BuildSignedResponse(params, kp)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var resp Response
	if err := xml.Unmarshal(respXML, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Signature == nil {
		t.Fatal("response has no signature")
	}
	sigBytes, err := base64.StdEncoding.DecodeString(resp.Signature.SignatureValue.Value)
	if err != nil {
		t.Fatalf("decode sig: %v", err)
	}
	signedInfoXML, err := xml.Marshal(resp.Signature.SignedInfo)
	if err != nil {
		t.Fatalf("marshal signed info: %v", err)
	}
	canonical := canonicalize(signedInfoXML)
	hash := sha256.Sum256(canonical)
	rsaPub := kp.Certificate.PublicKey.(*rsa.PublicKey)
	if err := rsa.VerifyPKCS1v15(rsaPub, crypto.SHA256, hash[:], sigBytes); err != nil {
		t.Errorf("RSA signature verification failed: %v", err)
	}
}

func TestSignature_Ed25519_CryptoVerify(t *testing.T) {
	kp, err := generateKeyPair("ed25519")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	params := AssertionParams{
		EntityID: "https://test", SSOURL: "https://test/sso",
		RequestID: "_v3", ACSURL: "https://sp/acs", Audience: "https://sp",
		Username: "u", Email: "u@e.com",
	}
	respXML, err := BuildSignedResponse(params, kp)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var resp Response
	if err := xml.Unmarshal(respXML, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Signature == nil {
		t.Fatal("response has no signature")
	}
	sigBytes, err := base64.StdEncoding.DecodeString(resp.Signature.SignatureValue.Value)
	if err != nil {
		t.Fatalf("decode sig: %v", err)
	}
	signedInfoXML, err := xml.Marshal(resp.Signature.SignedInfo)
	if err != nil {
		t.Fatalf("marshal signed info: %v", err)
	}
	canonical := canonicalize(signedInfoXML)
	pub := kp.Certificate.PublicKey.(ed25519.PublicKey)
	if !ed25519.Verify(pub, canonical, sigBytes) {
		t.Error("ed25519 signature verification failed")
	}
}

func TestBuildSignedResponse_BothAlgorithms_StructuralParity(t *testing.T) {
	// Both algorithms should produce structurally identical responses
	// differing only in signature method and values
	algorithms := []string{"ed25519", "rsa"}
	for _, algo := range algorithms {
		kp, err := generateKeyPair(algo)
		if err != nil {
			t.Fatalf("generate(%s): %v", algo, err)
		}
		params := AssertionParams{
			EntityID:  "https://test-idp",
			SSOURL:    "https://test-idp/sso",
			RequestID: "_struct" + algo,
			ACSURL:    "https://sp/acs",
			Audience:  "https://sp",
			Username:  "testuser",
			Email:     "test@example.com",
			Groups:    []string{"group1"},
		}
		respXML, err := BuildSignedResponse(params, kp)
		if err != nil {
			t.Fatalf("BuildSignedResponse(%s): %v", algo, err)
		}
		var resp Response
		if err := xml.Unmarshal(respXML, &resp); err != nil {
			t.Fatalf("unmarshal(%s): %v", algo, err)
		}
		// Both should have response and assertion signatures
		if resp.Signature == nil {
			t.Errorf("%s: response missing signature", algo)
		}
		if resp.Assertion.Signature == nil {
			t.Errorf("%s: assertion missing signature", algo)
		}
		if resp.Version != "2.0" {
			t.Errorf("%s: version = %q, want 2.0", algo, resp.Version)
		}
		if resp.Issuer.Value != "https://test-idp" {
			t.Errorf("%s: issuer = %q, want https://test-idp", algo, resp.Issuer.Value)
		}
		if resp.Status.StatusCode.Value != "urn:oasis:names:tc:SAML:2.0:status:Success" {
			t.Errorf("%s: status = %q", algo, resp.Status.StatusCode.Value)
		}
		if resp.Assertion.Subject.NameID.Value != "testuser" {
			t.Errorf("%s: NameID = %q, want testuser", algo, resp.Assertion.Subject.NameID.Value)
		}
	}
}

func TestBuildSignedResponse_AssertionSignature_Ed25519(t *testing.T) {
	kp, err := generateKeyPair("ed25519")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	params := AssertionParams{
		EntityID: "https://test", SSOURL: "https://test/sso",
		RequestID: "_asrt", ACSURL: "https://sp/acs", Audience: "https://sp",
		Username: "u", Email: "u@e.com",
	}
	respXML, err := BuildSignedResponse(params, kp)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var resp Response
	if err := xml.Unmarshal(respXML, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Verify the assertion signature also uses ed25519
	if resp.Assertion.Signature == nil {
		t.Fatal("assertion has no signature")
	}
	if resp.Assertion.Signature.SignedInfo.SignatureMethod.Algorithm != "http://www.w3.org/2021/04/xmldsig-more#eddsa-ed25519" {
		t.Errorf("assertion signature algorithm = %q, want eddsa-ed25519 URI",
			resp.Assertion.Signature.SignedInfo.SignatureMethod.Algorithm)
	}
}
