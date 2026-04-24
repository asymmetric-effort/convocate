// Package tlsutil generates the X.509 CA + leaf certificates the claude-shell
// rsyslog listener uses to authenticate agents over TLS. It's kept separate
// from sshutil because the key material has a different shape (x509 chains
// vs. OpenSSH keys) and a different lifetime profile (10y CA, 1y leaves).
//
// ECDSA P-256 is used for both the CA and issued certs: broadly compatible
// with rsyslog's GnuTLS + OpenSSL drivers on Ubuntu 22.04, smaller than RSA,
// and modern enough to stay relevant over the CA's lifetime.
package tlsutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

// KeyMaterial bundles a parsed certificate + private key and their PEM
// encodings. The PEM forms are what init-shell / init-agent write to disk;
// the typed fields let callers (e.g. agent signing) sign new leaves.
type KeyMaterial struct {
	Cert    *x509.Certificate
	Key     *ecdsa.PrivateKey
	CertPEM []byte
	KeyPEM  []byte
}

// GenerateCA mints a self-signed ECDSA P-256 CA with the given common name
// and validity in years. The returned KeyMaterial can be used directly by
// SignCert to issue leaves.
func GenerateCA(commonName string, validYears int) (*KeyMaterial, error) {
	if commonName == "" {
		return nil, fmt.Errorf("tlsutil: CA common name required")
	}
	if validYears <= 0 {
		validYears = 10
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate CA key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("serial: %w", err)
	}
	now := time.Now().UTC()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{"claude-shell"},
		},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.AddDate(validYears, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("sign CA: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("parse CA: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM, err := marshalECKey(key)
	if err != nil {
		return nil, err
	}
	return &KeyMaterial{Cert: cert, Key: key, CertPEM: certPEM, KeyPEM: keyPEM}, nil
}

// SignOptions configures a leaf-cert issuance.
type SignOptions struct {
	// CommonName ends up in Subject.CN — for client certs we use the
	// agent-id here so the rsyslog receiver can route logs per agent.
	CommonName string
	// DNSNames populate SAN. For a server cert that's the listener's
	// hostname(s). For a client cert, leave empty.
	DNSNames []string
	// ValidYears caps the leaf's lifetime. Defaults to 1 when zero.
	ValidYears int
	// IsServer toggles ExtKeyUsage between ServerAuth (true) and
	// ClientAuth (false).
	IsServer bool
}

// SignCert issues a leaf certificate under ca with the parameters in opts.
// The returned KeyMaterial contains a fresh ECDSA P-256 key and the signed
// cert; the CA itself is not mutated.
func SignCert(ca *KeyMaterial, opts SignOptions) (*KeyMaterial, error) {
	if ca == nil || ca.Cert == nil || ca.Key == nil {
		return nil, fmt.Errorf("tlsutil: CA material is nil")
	}
	if opts.CommonName == "" {
		return nil, fmt.Errorf("tlsutil: SignOptions.CommonName required")
	}
	if opts.ValidYears <= 0 {
		opts.ValidYears = 1
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate leaf key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("serial: %w", err)
	}
	usage := x509.ExtKeyUsageClientAuth
	if opts.IsServer {
		usage = x509.ExtKeyUsageServerAuth
	}
	now := time.Now().UTC()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   opts.CommonName,
			Organization: []string{"claude-shell"},
		},
		NotBefore:   now.Add(-5 * time.Minute),
		NotAfter:    now.AddDate(opts.ValidYears, 0, 0),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{usage},
		DNSNames:    opts.DNSNames,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.Cert, &key.PublicKey, ca.Key)
	if err != nil {
		return nil, fmt.Errorf("sign leaf: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("parse leaf: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM, err := marshalECKey(key)
	if err != nil {
		return nil, err
	}
	return &KeyMaterial{Cert: cert, Key: key, CertPEM: certPEM, KeyPEM: keyPEM}, nil
}

// ParseKeyMaterial loads a CertPEM+KeyPEM pair back into a KeyMaterial.
// Used by init-agent to reuse the CA produced by an earlier init-shell
// run when signing new client certs.
func ParseKeyMaterial(certPEM, keyPEM []byte) (*KeyMaterial, error) {
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, fmt.Errorf("no PEM block in cert")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse cert: %w", err)
	}
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, fmt.Errorf("no PEM block in key")
	}
	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		// Fall back to PKCS8 in case the key was written in that form.
		generic, perr := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
		if perr != nil {
			return nil, fmt.Errorf("parse key: %w / %w", err, perr)
		}
		ec, ok := generic.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("unsupported key type %T (expected ECDSA)", generic)
		}
		key = ec
	}
	return &KeyMaterial{Cert: cert, Key: key, CertPEM: certPEM, KeyPEM: keyPEM}, nil
}

func marshalECKey(k *ecdsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalECPrivateKey(k)
	if err != nil {
		return nil, fmt.Errorf("marshal key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), nil
}
