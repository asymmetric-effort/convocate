package mtls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"math/big"
)

// mustGenerateKey generates an ECDSA P-256 key pair using crypto/rand.
// This cannot fail for P-256 + crypto/rand; a panic here signals a
// broken runtime.
func mustGenerateKey() *ecdsa.PrivateKey {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic("mtls: generate key: " + err.Error())
	}
	return key
}

// mustSerialNumber generates a 128-bit random serial number per RFC 5280.
// This cannot fail with crypto/rand; a panic here signals a broken runtime.
func mustSerialNumber() *big.Int {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	n, err := rand.Int(rand.Reader, limit)
	if err != nil {
		panic("mtls: serial number: " + err.Error())
	}
	return n
}

// mustMarshalECKey marshals an ECDSA private key to DER form.
// This cannot fail for a valid P-256 key; a panic here signals a
// broken runtime.
func mustMarshalECKey(key *ecdsa.PrivateKey) []byte {
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		panic("mtls: marshal key: " + err.Error())
	}
	return der
}
