package crypto

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emmansun/gmsm/sm2"
	"github.com/emmansun/gmsm/smx509"
)

const (
	testGMTLSSignCertFile = "GM_TLS_SIGN_CERT_FILE"
	testGMTLSSignKeyFile  = "GM_TLS_SIGN_KEY_FILE"
	testGMTLSEncCertFile  = "GM_TLS_ENC_CERT_FILE"
	testGMTLSEncKeyFile   = "GM_TLS_ENC_KEY_FILE"
	testGMTLSCAFile       = "GM_TLS_CA_FILE"
)

type gmTLSTestFiles struct {
	signCert string
	signKey  string
	encCert  string
	encKey   string
	ca       string
}

func TestGMTLSCapabilityDisabledWithoutMaterial(t *testing.T) {
	clearGMTLSEnv(t)

	status := gmTLSCapability()
	if status.Available {
		t.Fatal("gm_tls must not be available without GM_TLS_ENABLED and certificate material")
	}
	if !strings.Contains(status.Detail, "GM_TLS_ENABLED") {
		t.Fatalf("detail = %q, want GM_TLS_ENABLED reason", status.Detail)
	}
}

func TestGMTLSCapabilityEnabledWithoutMaterial(t *testing.T) {
	clearGMTLSEnv(t)
	t.Setenv("GM_TLS_ENABLED", "true")

	status := gmTLSCapability()
	if status.Available {
		t.Fatal("GM_TLS_ENABLED=true alone must not make gm_tls available")
	}
	for _, env := range []string{
		testGMTLSSignCertFile, testGMTLSSignKeyFile,
		testGMTLSEncCertFile, testGMTLSEncKeyFile, testGMTLSCAFile,
	} {
		if !strings.Contains(status.Detail, env) {
			t.Fatalf("detail = %q, want missing %s", status.Detail, env)
		}
	}
}

func TestGMTLSCapabilityAcceptsValidSM2DualCertificate(t *testing.T) {
	files := newGMTLSValidFiles(t)
	setGMTLSEnv(t, files, true)

	status := gmTLSCapability()
	if !status.Available {
		t.Fatalf("gm_tls unavailable for a valid SM2 dual-certificate set: %s", status.Detail)
	}
	if !strings.Contains(status.Detail, "verified") {
		t.Fatalf("detail = %q, want verification summary", status.Detail)
	}
	for _, capability := range DetectCapabilities() {
		if capability.ID == CapabilityGMTLS && !capability.Available {
			t.Fatalf("DetectCapabilities reported gm_tls unavailable: %s", capability.Detail)
		}
	}
}

func TestGMTLSCapabilityDisabledWithValidMaterialIsUnavailable(t *testing.T) {
	files := newGMTLSValidFiles(t)
	setGMTLSEnv(t, files, false)

	status := gmTLSCapability()
	if status.Available {
		t.Fatal("GM_TLS_ENABLED=false must keep gm_tls unavailable even with valid material")
	}
	if !strings.Contains(status.Detail, "disabled") {
		t.Fatalf("detail = %q, want disabled reason", status.Detail)
	}
}

func TestGMTLSCapabilityRejectsInvalidCertificate(t *testing.T) {
	files := newGMTLSValidFiles(t)
	writeFile(t, files.signCert, []byte("this is not a certificate"))
	setGMTLSEnv(t, files, true)

	status := gmTLSCapability()
	if status.Available {
		t.Fatal("gm_tls accepted an invalid certificate file")
	}
	if !strings.Contains(status.Detail, testGMTLSSignCertFile) {
		t.Fatalf("detail = %q, want %s reference", status.Detail, testGMTLSSignCertFile)
	}
}

func TestGMTLSCapabilityRejectsNonSM2Certificate(t *testing.T) {
	files := newGMTLSValidFiles(t)
	writeStandardECDSACertificate(t, files.signCert)
	setGMTLSEnv(t, files, true)

	status := gmTLSCapability()
	if status.Available {
		t.Fatal("gm_tls accepted a non-SM2 certificate")
	}
	if !strings.Contains(status.Detail, "not SM2") {
		t.Fatalf("detail = %q, want non-SM2 reason", status.Detail)
	}
}

func TestGMTLSCapabilityRejectsExpiredCertificate(t *testing.T) {
	files := newGMTLSValidFiles(t)
	rootCert, rootKey := newGMTLSTestCA(t)
	expiredAt := time.Now().Add(-24 * time.Hour)
	_, certDER := issueGMTLSCertificate(t, rootCert, rootKey,
		"guliangiam-gm-tls-expired", smx509.KeyUsageDigitalSignature,
		expiredAt.Add(-24*time.Hour), expiredAt)
	writePEMFile(t, files.signCert, "CERTIFICATE", certDER)
	setGMTLSEnv(t, files, true)

	status := gmTLSCapability()
	if status.Available {
		t.Fatal("gm_tls accepted an expired certificate")
	}
	if !strings.Contains(status.Detail, "expired") {
		t.Fatalf("detail = %q, want expiry reason", status.Detail)
	}
}

func TestGMTLSCapabilityRejectsMismatchedPrivateKey(t *testing.T) {
	files := newGMTLSValidFiles(t)
	encKey, err := os.ReadFile(files.encKey)
	if err != nil {
		t.Fatalf("read enc key: %v", err)
	}
	writeFile(t, files.signKey, encKey)
	setGMTLSEnv(t, files, true)

	status := gmTLSCapability()
	if status.Available {
		t.Fatal("gm_tls accepted a private key that does not match the sign certificate")
	}
	if !strings.Contains(status.Detail, "does not match") {
		t.Fatalf("detail = %q, want mismatch reason", status.Detail)
	}
}

func TestGMTLSCapabilityRejectsUntrustedChain(t *testing.T) {
	files := newGMTLSValidFiles(t)
	otherRootCert, otherRootKey := newGMTLSTestCA(t)
	signKey, signDER := issueGMTLSCertificate(t, otherRootCert, otherRootKey,
		"guliangiam-gm-tls-untrusted", smx509.KeyUsageDigitalSignature,
		time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
	writePEMFile(t, files.signCert, "CERTIFICATE", signDER)
	writePEMFile(t, files.signKey, "PRIVATE KEY", mustMarshalSM2PrivateKey(t, signKey))
	setGMTLSEnv(t, files, true)

	status := gmTLSCapability()
	if status.Available {
		t.Fatal("gm_tls accepted a sign certificate outside the configured CA")
	}
	if !strings.Contains(status.Detail, "chain verification") {
		t.Fatalf("detail = %q, want chain verification reason", status.Detail)
	}
}

func TestValidateRuntimeProfileGMTLSMissingOnlyDegrades(t *testing.T) {
	t.Cleanup(func() {
		SetRuntimeReport(ProfileReport{Profile: string(ProfileDefault), Available: true})
	})
	clearGMTLSEnv(t)
	t.Setenv("CRYPTO_PROFILE", "gm")
	t.Setenv("GM_TLS_ENABLED", "true")

	report, err := ValidateRuntimeProfile()
	if err != nil {
		t.Fatalf("missing gm_tls material must not refuse startup: %v", err)
	}
	if !report.Available {
		t.Fatalf("report = %+v, want available", report)
	}
	if len(report.Degraded) != 1 || report.Degraded[0] != CapabilityGMTLS {
		t.Fatalf("Degraded = %v, want [gm_tls]", report.Degraded)
	}
	SetRuntimeReport(report)
	if health := HealthReport(); health.Status != HealthStatusDegraded {
		t.Fatalf("health = %+v, want degraded", health)
	}
}

func TestValidateRuntimeProfileGMTLSFailureOnlyDegrades(t *testing.T) {
	t.Cleanup(func() {
		SetRuntimeReport(ProfileReport{Profile: string(ProfileDefault), Available: true})
	})
	clearGMTLSEnv(t)
	t.Setenv("CRYPTO_PROFILE", "gm")
	t.Setenv("GM_TLS_ENABLED", "true")
	t.Setenv(testGMTLSSignCertFile, filepath.Join(t.TempDir(), "missing.pem"))

	report, err := ValidateRuntimeProfile()
	if err != nil {
		t.Fatalf("gm_tls material failure must not refuse startup: %v", err)
	}
	if !report.Available {
		t.Fatalf("report = %+v, want available", report)
	}
	if len(report.Degraded) != 1 || report.Degraded[0] != CapabilityGMTLS {
		t.Fatalf("Degraded = %v, want [gm_tls]", report.Degraded)
	}
	for _, capability := range report.Capabilities {
		if capability.ID == CapabilityGMTLS {
			if capability.Available {
				t.Fatal("ProfileReport marked gm_tls available after a material failure")
			}
			if capability.Detail == "" {
				t.Fatal("ProfileReport gm_tls reason is empty")
			}
		}
	}

	SetRuntimeReport(report)
	health := HealthReport()
	if health.Status != HealthStatusDegraded {
		t.Fatalf("health = %+v, want degraded", health)
	}
	if health.DegradedReasons[CapabilityGMTLS] == "" {
		t.Fatalf("health = %+v, want a concrete gm_tls degrade reason", health)
	}
}

func setGMTLSEnv(t *testing.T, files gmTLSTestFiles, enabled bool) {
	t.Helper()
	value := "false"
	if enabled {
		value = "true"
	}
	t.Setenv("GM_TLS_ENABLED", value)
	t.Setenv(testGMTLSSignCertFile, files.signCert)
	t.Setenv(testGMTLSSignKeyFile, files.signKey)
	t.Setenv(testGMTLSEncCertFile, files.encCert)
	t.Setenv(testGMTLSEncKeyFile, files.encKey)
	t.Setenv(testGMTLSCAFile, files.ca)
}

func clearGMTLSEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GM_TLS_ENABLED", "")
	t.Setenv(testGMTLSSignCertFile, "")
	t.Setenv(testGMTLSSignKeyFile, "")
	t.Setenv(testGMTLSEncCertFile, "")
	t.Setenv(testGMTLSEncKeyFile, "")
	t.Setenv(testGMTLSCAFile, "")
}

func newGMTLSValidFiles(t *testing.T) gmTLSTestFiles {
	t.Helper()
	dir := t.TempDir()
	rootCert, rootKey := newGMTLSTestCA(t)
	signKey, signDER := issueGMTLSCertificate(t, rootCert, rootKey,
		"guliangiam-gm-tls-sign", smx509.KeyUsageDigitalSignature,
		time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
	encKey, encDER := issueGMTLSCertificate(t, rootCert, rootKey,
		"guliangiam-gm-tls-encrypt", smx509.KeyUsageKeyEncipherment,
		time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))

	files := gmTLSTestFiles{
		signCert: filepath.Join(dir, "sign.crt"),
		signKey:  filepath.Join(dir, "sign.key"),
		encCert:  filepath.Join(dir, "enc.crt"),
		encKey:   filepath.Join(dir, "enc.key"),
		ca:       filepath.Join(dir, "ca.crt"),
	}
	writePEMFile(t, files.signCert, "CERTIFICATE", signDER)
	writePEMFile(t, files.signKey, "PRIVATE KEY", mustMarshalSM2PrivateKey(t, signKey))
	writePEMFile(t, files.encCert, "CERTIFICATE", encDER)
	writePEMFile(t, files.encKey, "PRIVATE KEY", mustMarshalSM2PrivateKey(t, encKey))
	writePEMFile(t, files.ca, "CERTIFICATE", rootCert.Raw)
	return files
}

func newGMTLSTestCA(t *testing.T) (*smx509.Certificate, *sm2.PrivateKey) {
	t.Helper()
	key := mustGenerateSM2Key(t)
	template := &smx509.Certificate{
		SerialNumber:          randomSerial(t),
		Subject:               pkix.Name{CommonName: "GuliangIAM GM TLS Test Root CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              smx509.KeyUsageCertSign | smx509.KeyUsageCRLSign,
	}
	der := mustCreateCertificate(t, template, template, &key.PublicKey, key)
	cert, err := smx509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse test root certificate: %v", err)
	}
	return cert, key
}

func issueGMTLSCertificate(t *testing.T, parent *smx509.Certificate, parentKey *sm2.PrivateKey, commonName string, usage smx509.KeyUsage, notBefore, notAfter time.Time) (*sm2.PrivateKey, []byte) {
	t.Helper()
	key := mustGenerateSM2Key(t)
	template := &smx509.Certificate{
		SerialNumber: randomSerial(t),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     usage,
	}
	return key, mustCreateCertificate(t, template, parent, &key.PublicKey, parentKey)
}

func mustGenerateSM2Key(t *testing.T) *sm2.PrivateKey {
	t.Helper()
	key, err := sm2.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate SM2 key: %v", err)
	}
	return key
}

func mustCreateCertificate(t *testing.T, template, parent *smx509.Certificate, publicKey, signer any) []byte {
	t.Helper()
	der, err := smx509.CreateCertificate(rand.Reader, template, parent, publicKey, signer)
	if err != nil {
		t.Fatalf("create SM2 certificate: %v", err)
	}
	return der
}

func mustMarshalSM2PrivateKey(t *testing.T, key *sm2.PrivateKey) []byte {
	t.Helper()
	der, err := smx509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal SM2 private key: %v", err)
	}
	return der
}

func randomSerial(t *testing.T) *big.Int {
	t.Helper()
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 62))
	if err != nil {
		t.Fatalf("generate serial: %v", err)
	}
	return serial
}

func writeStandardECDSACertificate(t *testing.T, path string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ECDSA key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: randomSerial(t),
		Subject:      pkix.Name{CommonName: "guliangiam-standard-ecdsa"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create standard ECDSA certificate: %v", err)
	}
	writePEMFile(t, path, "CERTIFICATE", der)
}

func writePEMFile(t *testing.T, path, blockType string, der []byte) {
	t.Helper()
	var buf bytes.Buffer
	if err := pem.Encode(&buf, &pem.Block{Type: blockType, Bytes: der}); err != nil {
		t.Fatalf("encode PEM: %v", err)
	}
	writeFile(t, path, buf.Bytes())
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
