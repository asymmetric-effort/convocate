package saml

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	"github.com/asymmetric-effort/convocate/src/saml-scim-agent/openbao"
)

const kvPath = "secret/data/saml-scim-agent/saml-signing-key"

// KeyPair holds the SAML signing key and certificate.
type KeyPair struct {
	PrivateKey  crypto.PrivateKey
	Certificate *x509.Certificate
	CertPEM     []byte
	Algorithm   string // "ed25519" or "rsa"
}

// LoadOrGenerateKeys loads SAML signing keys from OpenBao, generating them if they don't exist.
// The algorithm parameter selects "ed25519" or "rsa" when generating new keys.
func LoadOrGenerateKeys(client *openbao.Client, algorithm string) (*KeyPair, error) {
	data, err := client.KVRead(kvPath)
	if err != nil {
		return nil, fmt.Errorf("read keys from openbao: %w", err)
	}

	if data != nil {
		keyPEM, ok1 := data["private_key"].(string)
		certPEMStr, ok2 := data["certificate"].(string)
		if ok1 && ok2 {
			kp, err := decodeKeyPair([]byte(keyPEM), []byte(certPEMStr))
			if err == nil {
				// If algorithm field is missing in stored data, assume "rsa" (backward compat)
				if storedAlgo, ok := data["algorithm"].(string); ok {
					kp.Algorithm = storedAlgo
				} else {
					kp.Algorithm = "rsa"
				}
				return kp, nil
			}
			// If decode fails, regenerate
		}
	}

	// Generate new key pair
	kp, err := generateKeyPair(algorithm)
	if err != nil {
		return nil, fmt.Errorf("generate key pair: %w", err)
	}

	// PEM-encode private key based on algorithm
	var keyPEM []byte
	switch algorithm {
	case "ed25519":
		pkcs8Bytes, marshalErr := x509.MarshalPKCS8PrivateKey(kp.PrivateKey)
		if marshalErr != nil {
			return nil, fmt.Errorf("marshal ed25519 private key: %w", marshalErr)
		}
		keyPEM = pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: pkcs8Bytes,
		})
	case "rsa":
		rsaKey, ok := kp.PrivateKey.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("expected *rsa.PrivateKey but got %T", kp.PrivateKey)
		}
		keyPEM = pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(rsaKey),
		})
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	// Store in OpenBao
	storeData := map[string]interface{}{
		"private_key": string(keyPEM),
		"certificate": string(kp.CertPEM),
		"algorithm":   algorithm,
	}

	if err := client.KVWrite(kvPath, storeData); err != nil {
		return nil, fmt.Errorf("store keys in openbao: %w", err)
	}

	return kp, nil
}

func generateKeyPair(algorithm string) (*KeyPair, error) {
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "SAML-SCIM-Agent SAML Signing",
			Organization: []string{"Asymmetric Effort"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	var privKey crypto.PrivateKey
	var pubKey crypto.PublicKey

	switch algorithm {
	case "ed25519":
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("generate ed25519 key: %w", err)
		}
		privKey = priv
		pubKey = pub
	case "rsa":
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, fmt.Errorf("generate RSA key: %w", err)
		}
		privKey = key
		pubKey = &key.PublicKey
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, pubKey, privKey)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	return &KeyPair{
		PrivateKey:  privKey,
		Certificate: cert,
		CertPEM:     certPEM,
		Algorithm:   algorithm,
	}, nil
}

func decodeKeyPair(keyPEM, certPEM []byte) (*KeyPair, error) {
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, fmt.Errorf("failed to decode private key PEM")
	}

	var privKey crypto.PrivateKey
	var algorithm string

	switch keyBlock.Type {
	case "PRIVATE KEY":
		// PKCS#8 format — used by ed25519 (and potentially other algorithms)
		parsed, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS8 private key: %w", err)
		}
		switch parsed.(type) {
		case ed25519.PrivateKey:
			algorithm = "ed25519"
		case *rsa.PrivateKey:
			algorithm = "rsa"
		default:
			return nil, fmt.Errorf("unsupported PKCS8 key type: %T", parsed)
		}
		privKey = parsed
	case "RSA PRIVATE KEY":
		// PKCS#1 format — RSA only
		key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS1 private key: %w", err)
		}
		privKey = key
		algorithm = "rsa"
	default:
		return nil, fmt.Errorf("unsupported PEM block type: %s", keyBlock.Type)
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, fmt.Errorf("failed to decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}

	return &KeyPair{
		PrivateKey:  privKey,
		Certificate: cert,
		CertPEM:     certPEM,
		Algorithm:   algorithm,
	}, nil
}
