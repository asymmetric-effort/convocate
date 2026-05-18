package mtls

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net"
	"time"
)

// CertKeyPair holds a PEM-encoded certificate and private key.
type CertKeyPair struct {
	CertPEM []byte
	KeyPEM  []byte
}

// IssueServerCert issues a server certificate signed by the CA. The cert
// includes the given DNS names and IP addresses as SANs.
func (ca *CA) IssueServerCert(commonName string, dnsNames []string, ips []net.IP, validity time.Duration) (*CertKeyPair, error) {
	privateKey := mustGenerateKey()
	serialNumber := mustSerialNumber()

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{orgName},
		},
		NotBefore:             now,
		NotAfter:              now.Add(validity),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
		IPAddresses:           ips,
	}

	return ca.signCert(template, &privateKey.PublicKey, privateKey)
}

// IssueClientCert issues a client certificate signed by the CA for mTLS
// authentication. The commonName identifies the client (e.g. a host ID).
func (ca *CA) IssueClientCert(commonName string, validity time.Duration) (*CertKeyPair, error) {
	privateKey := mustGenerateKey()
	serialNumber := mustSerialNumber()

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{orgName},
		},
		NotBefore:             now,
		NotAfter:              now.Add(validity),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	return ca.signCert(template, &privateKey.PublicKey, privateKey)
}

// IssueCombinedCert issues a certificate valid for both server and client
// authentication. Used by the Router API which both serves TLS and
// authenticates to other services.
func (ca *CA) IssueCombinedCert(commonName string, dnsNames []string, ips []net.IP, validity time.Duration) (*CertKeyPair, error) {
	privateKey := mustGenerateKey()
	serialNumber := mustSerialNumber()

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{orgName},
		},
		NotBefore:             now,
		NotAfter:              now.Add(validity),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
		IPAddresses:           ips,
	}

	return ca.signCert(template, &privateKey.PublicKey, privateKey)
}

func (ca *CA) signCert(template *x509.Certificate, pub *ecdsa.PublicKey, priv *ecdsa.PrivateKey) (*CertKeyPair, error) {
	certDER, err := x509.CreateCertificate(rand.Reader, template, ca.Certificate, pub, ca.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("mtls: sign certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: pemCertificate, Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: pemECKey, Bytes: mustMarshalECKey(priv)})

	return &CertKeyPair{
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
	}, nil
}
