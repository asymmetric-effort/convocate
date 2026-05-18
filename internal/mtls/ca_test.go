package mtls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"
)

func TestGenerateCA(t *testing.T) {
	ca, err := GenerateCA("convocate-test-ca", 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateCA error: %v", err)
	}

	t.Run("certificate is CA", func(t *testing.T) {
		if !ca.Certificate.IsCA {
			t.Error("certificate is not a CA")
		}
	})

	t.Run("certificate is self-signed", func(t *testing.T) {
		err := ca.Certificate.CheckSignatureFrom(ca.Certificate)
		if err != nil {
			t.Errorf("certificate is not self-signed: %v", err)
		}
	})

	t.Run("common name", func(t *testing.T) {
		if ca.Certificate.Subject.CommonName != "convocate-test-ca" {
			t.Errorf("CommonName: got %q, want %q", ca.Certificate.Subject.CommonName, "convocate-test-ca")
		}
	})

	t.Run("organization", func(t *testing.T) {
		if len(ca.Certificate.Subject.Organization) != 1 || ca.Certificate.Subject.Organization[0] != orgName {
			t.Errorf("Organization: got %v, want [convocate]", ca.Certificate.Subject.Organization)
		}
	})

	t.Run("key usage", func(t *testing.T) {
		if ca.Certificate.KeyUsage&x509.KeyUsageCertSign == 0 {
			t.Error("missing KeyUsageCertSign")
		}
		if ca.Certificate.KeyUsage&x509.KeyUsageCRLSign == 0 {
			t.Error("missing KeyUsageCRLSign")
		}
	})

	t.Run("validity period", func(t *testing.T) {
		if ca.Certificate.NotBefore.After(time.Now()) {
			t.Error("NotBefore is in the future")
		}
		expectedExpiry := time.Now().Add(24 * time.Hour)
		if ca.Certificate.NotAfter.Before(expectedExpiry.Add(-1 * time.Minute)) {
			t.Error("NotAfter is too early")
		}
	})

	t.Run("ECC P-256 key", func(t *testing.T) {
		pubKey, ok := ca.Certificate.PublicKey.(*ecdsa.PublicKey)
		if !ok {
			t.Fatalf("expected *ecdsa.PublicKey, got %T", ca.Certificate.PublicKey)
		}
		if pubKey.Curve != elliptic.P256() {
			t.Errorf("curve: got %v, want P-256", pubKey.Curve.Params().Name)
		}
	})

	t.Run("PEM encoding valid", func(t *testing.T) {
		block, _ := pem.Decode(ca.CertPEM)
		if block == nil {
			t.Fatal("failed to decode CertPEM")
		}
		if block.Type != pemCertificate {
			t.Errorf("CertPEM type: got %q, want CERTIFICATE", block.Type)
		}

		keyBlock, _ := pem.Decode(ca.KeyPEM)
		if keyBlock == nil {
			t.Fatal("failed to decode KeyPEM")
		}
		if keyBlock.Type != pemECKey {
			t.Errorf("KeyPEM type: got %q, want EC PRIVATE KEY", keyBlock.Type)
		}
	})

	t.Run("private key matches certificate", func(t *testing.T) {
		pubKey := ca.Certificate.PublicKey.(*ecdsa.PublicKey)
		if pubKey.X.Cmp(ca.PrivateKey.PublicKey.X) != 0 || pubKey.Y.Cmp(ca.PrivateKey.PublicKey.Y) != 0 {
			t.Error("private key does not match certificate public key")
		}
	})
}

func TestLoadCA(t *testing.T) {
	original, err := GenerateCA("convocate-test-ca", 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateCA error: %v", err)
	}

	loaded, err := LoadCA(original.CertPEM, original.KeyPEM)
	if err != nil {
		t.Fatalf("LoadCA error: %v", err)
	}

	if loaded.Certificate.Subject.CommonName != original.Certificate.Subject.CommonName {
		t.Errorf("CommonName mismatch: got %q, want %q",
			loaded.Certificate.Subject.CommonName, original.Certificate.Subject.CommonName)
	}

	if loaded.PrivateKey.PublicKey.X.Cmp(original.PrivateKey.PublicKey.X) != 0 {
		t.Error("loaded private key does not match original")
	}
}

func TestLoadCAInvalidCert(t *testing.T) {
	_, err := LoadCA([]byte("not a PEM"), []byte("not a PEM"))
	if err == nil {
		t.Error("expected error for invalid cert PEM, got nil")
	}
}

func TestLoadCAInvalidKey(t *testing.T) {
	ca, err := GenerateCA("test", 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateCA error: %v", err)
	}
	_, err = LoadCA(ca.CertPEM, []byte("not a PEM"))
	if err == nil {
		t.Error("expected error for invalid key PEM, got nil")
	}
}

func TestLoadCAWrongKeyType(t *testing.T) {
	ca, err := GenerateCA("test", 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateCA error: %v", err)
	}
	// Use the cert PEM as the key PEM — wrong type.
	_, err = LoadCA(ca.CertPEM, ca.CertPEM)
	if err == nil {
		t.Error("expected error for wrong key PEM type, got nil")
	}
}

func TestTrustBundle(t *testing.T) {
	ca, err := GenerateCA("test", 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateCA error: %v", err)
	}
	bundle := ca.TrustBundle()
	if len(bundle) == 0 {
		t.Error("TrustBundle returned empty bytes")
	}
	block, _ := pem.Decode(bundle)
	if block == nil {
		t.Fatal("TrustBundle is not valid PEM")
	}
	if block.Type != pemCertificate {
		t.Errorf("TrustBundle PEM type: got %q, want CERTIFICATE", block.Type)
	}
}

func TestGenerateCAUniqueness(t *testing.T) {
	ca1, err := GenerateCA("test1", 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateCA error: %v", err)
	}
	ca2, err := GenerateCA("test2", 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateCA error: %v", err)
	}
	if ca1.Certificate.SerialNumber.Cmp(ca2.Certificate.SerialNumber) == 0 {
		t.Error("two CAs have the same serial number")
	}
}
