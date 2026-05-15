package mtls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
)

// ServerTLSConfig creates a TLS configuration for a server that requires
// mutual TLS on certain routes. TLS v1.3+ only, ECC curves only.
//
// When requireClientCert is true, the server demands a valid client
// certificate signed by the CA (mTLS). When false, client certs are
// requested but not required (used for routes that accept either a
// session cookie or a client cert).
func ServerTLSConfig(serverCert CertKeyPair, caPEM []byte, requireClientCert bool) (*tls.Config, error) {
	cert, err := tls.X509KeyPair(serverCert.CertPEM, serverCert.KeyPEM)
	if err != nil {
		return nil, fmt.Errorf("mtls: load server certificate: %w", err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("mtls: failed to parse CA certificate")
	}

	clientAuth := tls.VerifyClientCertIfGiven
	if requireClientCert {
		clientAuth = tls.RequireAndVerifyClientCert
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caPool,
		ClientAuth:   clientAuth,
		MinVersion:   tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
		},
	}, nil
}

// ClientTLSConfig creates a TLS configuration for a client that presents
// a client certificate to the server (mTLS). TLS v1.3+ only.
func ClientTLSConfig(clientCert CertKeyPair, caPEM []byte) (*tls.Config, error) {
	cert, err := tls.X509KeyPair(clientCert.CertPEM, clientCert.KeyPEM)
	if err != nil {
		return nil, fmt.Errorf("mtls: load client certificate: %w", err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("mtls: failed to parse CA certificate")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caPool,
		MinVersion:   tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
		},
	}, nil
}

// PlainTLSConfig creates a TLS configuration for a client that trusts
// the private CA but does not present a client certificate. Used by
// services that only need server-auth TLS (e.g. Redis TLS).
func PlainTLSConfig(caPEM []byte) (*tls.Config, error) {
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("mtls: failed to parse CA certificate")
	}

	return &tls.Config{
		RootCAs:    caPool,
		MinVersion: tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
		},
	}, nil
}
