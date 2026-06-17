package orchestrator

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

func TestValidatePasswordStrength(t *testing.T) {
	valid := []string{
		"Str0ng!Passw0rd",
		"aB3$aB3$aB3$",
		"correct horse Battery9",
	}
	for _, pw := range valid {
		if err := validatePasswordStrength("admin_password", pw); err != nil {
			t.Errorf("expected %q valid, got %v", pw, err)
		}
	}

	invalid := []string{
		"",                // empty
		"short1A",         // too short
		"alllowercase1",   // only 2 classes (lower+digit)
		"ALLUPPER12345",   // only 2 classes (upper+digit)
		"with\nnewline12", // control char
		"NoSymbolsButOk",  // only 2 classes (upper+lower) - 11 chars also short
	}
	for _, pw := range invalid {
		if err := validatePasswordStrength("admin_password", pw); err == nil {
			t.Errorf("expected %q invalid", pw)
		}
	}
}

// generateTestKeypair returns a freshly generated self-signed certificate and
// its private key in PEM form, valid for the given duration from now.
func generateTestKeypair(t *testing.T, validFor time.Duration) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "novus-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(validFor),
		DNSNames:     []string{"panel.example.com"},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))

	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	return certPEM, keyPEM
}

func TestValidateCustomTLSMaterialValid(t *testing.T) {
	cert, key := generateTestKeypair(t, 365*24*time.Hour)
	if err := validateCustomTLSMaterial(cert, key); err != nil {
		t.Errorf("expected valid keypair, got %v", err)
	}
}

func TestValidateCustomTLSMaterialMalformedCert(t *testing.T) {
	_, key := generateTestKeypair(t, time.Hour)
	if err := validateCustomTLSMaterial("not a pem", key); err == nil {
		t.Error("expected error for malformed certificate")
	}
}

func TestValidateCustomTLSMaterialMalformedKey(t *testing.T) {
	cert, _ := generateTestKeypair(t, time.Hour)
	if err := validateCustomTLSMaterial(cert, "not a pem"); err == nil {
		t.Error("expected error for malformed private key")
	}
}

func TestValidateCustomTLSMaterialMismatch(t *testing.T) {
	cert, _ := generateTestKeypair(t, time.Hour)
	_, otherKey := generateTestKeypair(t, time.Hour)
	if err := validateCustomTLSMaterial(cert, otherKey); err == nil {
		t.Error("expected error for cert/key mismatch")
	}
}

func TestValidateCustomTLSMaterialExpired(t *testing.T) {
	cert, key := generateTestKeypair(t, -time.Hour)
	if err := validateCustomTLSMaterial(cert, key); err == nil {
		t.Error("expected error for expired certificate")
	}
}
