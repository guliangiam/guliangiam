package bootstrap

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func bootstrapTestRSAKeyPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal RSA PKCS#8: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func bootstrapTestKeyRingJSON(t *testing.T, kid string) []byte {
	t.Helper()
	document := map[string]any{
		"keys": []map[string]any{{
			"kid":             kid,
			"algorithm":       "RS256",
			"private_key_pem": bootstrapTestRSAKeyPEM(t),
			"created_at":      time.Now().UTC().Add(-time.Hour),
			"retired_at":      nil,
		}},
	}
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("marshal key ring JSON: %v", err)
	}
	return data
}

func TestNewJWKSManagerWithoutKeyRingWarnsEphemeral(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("ALLOW_EPHEMERAL_SIGNING_KEYS", "")

	var sink bytes.Buffer
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })
	slog.SetDefault(slog.New(slog.NewTextHandler(&sink, &slog.HandlerOptions{Level: slog.LevelDebug})))

	manager, err := newJWKSManager("")
	if err != nil {
		t.Fatalf("newJWKSManager() error = %v", err)
	}
	if manager == nil {
		t.Fatal("expected an in-memory manager")
	}
	if _, err := manager.GetKeyPair(); err != nil {
		t.Fatalf("GetKeyPair() error = %v", err)
	}
	if !strings.Contains(strings.ToLower(sink.String()), "ephemeral signing keys") {
		t.Fatalf("startup log does not warn about ephemeral signing keys: %q", sink.String())
	}
}

func TestNewJWKSManagerProductionWithoutKeyRingFailsClosed(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("ALLOW_EPHEMERAL_SIGNING_KEYS", "")

	_, err := newJWKSManager("")
	if err == nil {
		t.Fatal("production without a key ring must refuse to start")
	}
	for _, want := range []string{"JWT_SIGNING_KEYS_FILE", "ALLOW_EPHEMERAL_SIGNING_KEYS", "replicas", "restart"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not explain %q", err, want)
		}
	}
}

func TestNewJWKSManagerProductionEscapeHatchAllowsEphemeralWithWarning(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("ALLOW_EPHEMERAL_SIGNING_KEYS", "on")

	var sink bytes.Buffer
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })
	slog.SetDefault(slog.New(slog.NewTextHandler(&sink, &slog.HandlerOptions{Level: slog.LevelDebug})))

	manager, err := newJWKSManager("")
	if err != nil {
		t.Fatalf("newJWKSManager() with ALLOW_EPHEMERAL_SIGNING_KEYS=on error = %v", err)
	}
	if manager == nil {
		t.Fatal("expected an in-memory manager")
	}
	if _, err := manager.GetKeyPair(); err != nil {
		t.Fatalf("GetKeyPair() error = %v", err)
	}

	logged := strings.ToLower(sink.String())
	if !strings.Contains(logged, "level=warn") || !strings.Contains(logged, "ephemeral signing keys") {
		t.Fatalf("startup log must warn about ephemeral signing keys: %q", sink.String())
	}
	for _, want := range []string{"replicas", "restart", "kid"} {
		if !strings.Contains(logged, want) {
			t.Fatalf("escape-hatch warning does not explain %q: %q", want, sink.String())
		}
	}
}

func TestNewJWKSManagerLoadsKeyRingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jwt-signing-keys.json")
	if err := os.WriteFile(path, bootstrapTestKeyRingJSON(t, "rsa_file"), 0o600); err != nil {
		t.Fatalf("write key ring: %v", err)
	}

	manager, err := newJWKSManager(path)
	if err != nil {
		t.Fatalf("newJWKSManager() error = %v", err)
	}
	key, err := manager.GetKeyPair()
	if err != nil {
		t.Fatalf("GetKeyPair() error = %v", err)
	}
	if key.Kid != "rsa_file" {
		t.Fatalf("signing kid = %q, want rsa_file", key.Kid)
	}
	set, err := manager.GetJWKS()
	if err != nil {
		t.Fatalf("GetJWKS() error = %v", err)
	}
	if len(set.Keys) != 1 || set.Keys[0].Kid != "rsa_file" {
		t.Fatalf("JWKS = %+v, want only rsa_file", set.Keys)
	}
}

func TestNewJWKSManagerRejectsInvalidKeyRingFile(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		content string
		wantErr string
	}{
		{
			name:    "missing file",
			path:    filepath.Join(t.TempDir(), "missing.json"),
			wantErr: "read JWT signing key ring file",
		},
		{
			name:    "empty keys",
			content: `{"keys":[]}`,
			wantErr: "at least one key",
		},
		{
			name: "bad pem",
			content: `{"keys":[{"kid":"rsa_bad","algorithm":"RS256","private_key_pem":"not a key",` +
				`"created_at":"2026-11-01T00:00:00Z"}]}`,
			wantErr: "parse RSA private key",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := tc.path
			if path == "" {
				path = filepath.Join(t.TempDir(), "jwt-signing-keys.json")
				if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
					t.Fatalf("write key ring: %v", err)
				}
			}
			_, err := newJWKSManager(path)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}
