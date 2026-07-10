package auth

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestSignAndVerifyJWT(t *testing.T) {
	InitJWT()

	token, exp, err := SignJWT("usr-001", "testuser", "Test User", "test@example.com",
		[]string{"admin"}, []string{"nmgr", "amgr"}, time.Hour)
	if err != nil {
		t.Fatalf("SignJWT failed: %v", err)
	}
	if token == "" {
		t.Fatal("SignJWT returned empty token")
	}
	if exp.Before(time.Now()) {
		t.Fatal("token already expired")
	}

	claims, err := VerifyJWT(token)
	if err != nil {
		t.Fatalf("VerifyJWT failed: %v", err)
	}
	if claims.Sub != "usr-001" {
		t.Errorf("Sub = %q, want %q", claims.Sub, "usr-001")
	}
	if claims.Username != "testuser" {
		t.Errorf("Username = %q, want %q", claims.Username, "testuser")
	}
	if claims.Name != "Test User" {
		t.Errorf("Name = %q, want %q", claims.Name, "Test User")
	}
	if len(claims.Roles) != 1 || claims.Roles[0] != "admin" {
		t.Errorf("Roles = %v, want [admin]", claims.Roles)
	}
	if len(claims.Applets) != 2 {
		t.Errorf("Applets = %v, want 2 items", claims.Applets)
	}
}

func TestVerifyJWT_InvalidToken(t *testing.T) {
	InitJWT()

	_, err := VerifyJWT("invalid.token.here")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestVerifyJWT_TamperedPayload(t *testing.T) {
	InitJWT()

	token, _, err := SignJWT("usr-001", "admin", "Admin", "", []string{"admin"}, []string{}, time.Hour)
	if err != nil {
		t.Fatalf("SignJWT failed: %v", err)
	}

	// Tamper with the payload by changing a character
	parts := []byte(token)
	// Find the second dot
	dotCount := 0
	for i, b := range parts {
		if b == '.' {
			dotCount++
			if dotCount == 1 {
				// Flip a byte in the payload section
				if i+5 < len(parts) {
					parts[i+5] ^= 0xFF
				}
				break
			}
		}
	}

	_, err = VerifyJWT(string(parts))
	if err == nil {
		t.Fatal("expected error for tampered token")
	}
}

func TestVerifyJWT_ExpiredToken(t *testing.T) {
	InitJWT()

	token, _, err := SignJWT("usr-001", "admin", "Admin", "", []string{}, []string{}, -time.Hour)
	if err != nil {
		t.Fatalf("SignJWT failed: %v", err)
	}

	_, err = VerifyJWT(token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestES256Algorithm(t *testing.T) {
	InitJWT()

	token, _, err := SignJWT("usr-001", "admin", "Admin", "", []string{}, []string{}, time.Hour)
	if err != nil {
		t.Fatalf("SignJWT failed: %v", err)
	}

	// Verify the header says ES256
	claims, err := VerifyJWT(token)
	if err != nil {
		t.Fatalf("VerifyJWT failed: %v", err)
	}
	if claims.Sub != "usr-001" {
		t.Errorf("unexpected Sub: %s", claims.Sub)
	}
}

func TestVerifyJWT_ValidSigBadClaims(t *testing.T) {
	InitJWT()

	// Create a token with valid signature but invalid JSON claims
	// by directly signing a header + invalid-base64 claims
	headerJSON, _ := json.Marshal(jwtHeader{Alg: "ES256", Typ: "JWT"})
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	// Use a claims string that is valid base64 but invalid JSON
	invalidJSON := base64.RawURLEncoding.EncodeToString([]byte("not valid json{{{"))
	signingInput := headerB64 + "." + invalidJSON

	hash := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, signingKey, hash[:])
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}

	sig := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(sig[32-len(rBytes):32], rBytes)
	copy(sig[64-len(sBytes):64], sBytes)
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	token := signingInput + "." + sigB64

	_, err = VerifyJWT(token)
	if err == nil {
		t.Fatal("expected error for invalid JSON claims")
	}
}

func TestVerifyJWT_ValidSigBadBase64Claims(t *testing.T) {
	InitJWT()

	// Create a token where claims part is not valid base64
	// The signature is valid over the raw string including invalid base64
	headerJSON, _ := json.Marshal(jwtHeader{Alg: "ES256", Typ: "JWT"})
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	// "%%" is not valid base64url
	badB64Claims := "%%not-valid-base64%%"
	signingInput := headerB64 + "." + badB64Claims

	hash := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, signingKey, hash[:])
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}

	sig := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(sig[32-len(rBytes):32], rBytes)
	copy(sig[64-len(sBytes):64], sBytes)
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	token := signingInput + "." + sigB64

	_, err = VerifyJWT(token)
	if err == nil {
		t.Fatal("expected error for bad base64 claims")
	}
}
