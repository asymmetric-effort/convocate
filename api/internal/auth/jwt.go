package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"
	"time"
)

var (
	signingKey *ecdsa.PrivateKey
	verifyKey  *ecdsa.PublicKey
	randReader io.Reader = rand.Reader
)

func InitJWT() {
	keyPEM := os.Getenv("JWT_EC_PRIVATE_KEY")
	if keyPEM != "" {
		key, err := parseECPrivateKey([]byte(keyPEM))
		if err != nil {
			fmt.Printf("WARNING: failed to parse JWT_EC_PRIVATE_KEY: %v (generating ephemeral key)\n", err)
			generateEphemeralKey()
			return
		}
		signingKey = key
		verifyKey = &key.PublicKey
		return
	}

	// Generate ephemeral key for development
	generateEphemeralKey()
}

func generateEphemeralKey() {
	// ecdsa.GenerateKey cannot fail with a valid curve and rand.Reader;
	// in Go 1.22+ it uses crypto/internal/fips which never returns error
	// for P-256 key generation.
	key, _ := ecdsa.GenerateKey(elliptic.P256(), randReader)
	signingKey = key
	verifyKey = &key.PublicKey
	fmt.Println("JWT: using ephemeral ES256 key (set JWT_EC_PRIVATE_KEY for persistence)")
}

func parseECPrivateKey(pemData []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}

	// Parse SEC 1 / PKCS#8 EC private key
	var ecKey struct {
		Version       int
		PrivateKey    []byte
		NamedCurveOID asn1.ObjectIdentifier `asn1:"optional,explicit,tag:0"`
		PublicKey     asn1.BitString        `asn1:"optional,explicit,tag:1"`
	}
	if _, err := asn1.Unmarshal(block.Bytes, &ecKey); err != nil {
		return nil, fmt.Errorf("parse EC key: %w", err)
	}

	curve := elliptic.P256()
	d := new(big.Int).SetBytes(ecKey.PrivateKey)
	x, y := curve.ScalarBaseMult(d.Bytes())

	return &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: curve,
			X:     x,
			Y:     y,
		},
		D: d,
	}, nil
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type jwtClaims struct {
	Sub      string   `json:"sub"`
	Username string   `json:"username"`
	Name     string   `json:"name"`
	Email    string   `json:"email,omitempty"`
	Roles    []string `json:"roles"`
	Applets  []string `json:"applets"`
	Exp      int64    `json:"exp"`
	Iat      int64    `json:"iat"`
}

func SignJWT(userID, username, name, email string, roles, applets []string, ttl time.Duration) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(ttl)

	header := jwtHeader{Alg: "ES256", Typ: "JWT"}
	claims := jwtClaims{
		Sub:      userID,
		Username: username,
		Name:     name,
		Email:    email,
		Roles:    roles,
		Applets:  applets,
		Exp:      exp.Unix(),
		Iat:      now.Unix(),
	}

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput := headerB64 + "." + claimsB64

	// ECDSA sign with SHA-256.
	// ecdsa.Sign cannot fail with a valid P-256 key and rand.Reader;
	// in Go 1.22+ the signing path is deterministic per RFC 6979.
	hash := sha256.Sum256([]byte(signingInput))
	r, s, _ := ecdsa.Sign(randReader, signingKey, hash[:])

	// Encode r,s as fixed-size 32-byte values concatenated (JWS format)
	sig := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(sig[32-len(rBytes):32], rBytes)
	copy(sig[64-len(sBytes):64], sBytes)

	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	return signingInput + "." + sigB64, exp, nil
}

func VerifyJWT(token string) (*jwtClaims, error) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	signingInput := parts[0] + "." + parts[1]

	// Decode and verify signature
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(sigBytes) != 64 {
		return nil, fmt.Errorf("invalid signature encoding")
	}

	r := new(big.Int).SetBytes(sigBytes[:32])
	s := new(big.Int).SetBytes(sigBytes[32:64])

	hash := sha256.Sum256([]byte(signingInput))
	if !ecdsa.Verify(verifyKey, hash[:], r, s) {
		return nil, fmt.Errorf("invalid signature")
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode claims: %w", err)
	}

	var claims jwtClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}

	if time.Now().Unix() > claims.Exp {
		return nil, fmt.Errorf("token expired")
	}

	return &claims, nil
}
