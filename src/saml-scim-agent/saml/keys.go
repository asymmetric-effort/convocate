package saml

import (
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
	PrivateKey  *rsa.PrivateKey
	Certificate *x509.Certificate
	CertPEM     []byte
}

// LoadOrGenerateKeys loads SAML signing keys from OpenBao, generating them if they don't exist.
func LoadOrGenerateKeys(client *openbao.Client) (*KeyPair, error) {
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
				return kp, nil
			}
			// If decode fails, regenerate
		}
	}

	// Generate new key pair
	kp, err := generateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("generate key pair: %w", err)
	}

	// Store in OpenBao
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(kp.PrivateKey),
	})

	storeData := map[string]interface{}{
		"private_key": string(keyPEM),
		"certificate": string(kp.CertPEM),
	}

	if err := client.KVWrite(kvPath, storeData); err != nil {
		return nil, fmt.Errorf("store keys in openbao: %w", err)
	}

	return kp, nil
}

func generateKeyPair() (*KeyPair, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate RSA key: %w", err)
	}

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

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
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
		PrivateKey:  key,
		Certificate: cert,
		CertPEM:     certPEM,
	}, nil
}

func decodeKeyPair(keyPEM, certPEM []byte) (*KeyPair, error) {
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, fmt.Errorf("failed to decode private key PEM")
	}

	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
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
		PrivateKey:  key,
		Certificate: cert,
		CertPEM:     certPEM,
	}, nil
}
