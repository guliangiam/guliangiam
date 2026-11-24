package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func sampleManifest() manifest {
	return manifest{
		Schema:        manifestSchema,
		ChainID:       "org-1",
		FromSeq:       1,
		ToSeq:         3,
		Count:         3,
		PayloadSHA256: "abc123",
		PayloadBytes:  42,
		HeadEntryHash: "deadbeef",
		GeneratedAt:   time.Unix(0, 0).UTC(),
	}
}

func TestSignManifestIsDeterministicAndKeyBound(t *testing.T) {
	m := sampleManifest()
	a := signManifest(m, "key-a")
	b := signManifest(m, "key-a")
	c := signManifest(m, "key-b")
	if a != b {
		t.Fatalf("same input must sign identically: %s != %s", a, b)
	}
	if a == c {
		t.Fatal("different keys must produce different signatures")
	}
	if len(a) != 64 {
		t.Fatalf("signature length = %d, want 64 hex chars", len(a))
	}
}

func TestVerifyArchiveDetectsTamperedPayload(t *testing.T) {
	dir := t.TempDir()
	payload := filepath.Join(dir, "seg.ndjson")
	manifestPath := filepath.Join(dir, "seg.ndjson.manifest.json")
	m := sampleManifest()
	m.PayloadSHA256 = sha256Hex(t, "line1\nline2\nline3\n")
	m.PayloadBytes = int64(len("line1\nline2\nline3\n"))
	m.Signature = signManifest(m, "k")
	if err := os.WriteFile(payload, []byte("line1\nline2\nline3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeManifestForTest(manifestPath, m); err != nil {
		t.Fatal(err)
	}
	if err := verifyArchive(payload, manifestPath, "k"); err != nil {
		t.Fatalf("healthy archive must verify: %v", err)
	}

	// 篡改第 2 行：哈希/字节数不匹配必须报错。
	if err := os.WriteFile(payload, []byte("line1\nHACKED\nline3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyArchive(payload, manifestPath, "k"); err == nil {
		t.Fatal("tampered payload must fail verification")
	}
}

func sha256Hex(t *testing.T, s string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func writeManifestForTest(path string, m manifest) error {
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}
