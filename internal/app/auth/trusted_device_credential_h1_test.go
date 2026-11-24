package auth

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// allowedCredentialWritePrefixes are the only right-hand sides allowed when a
// persistence-layer TrustedDevice is constructed:
//   - trustedDeviceCredentialDigest(...) — persistence and verification form.
//   - trustedDeviceCredentialMask(...)   — client-facing views and audit payloads.
//   - credentialDigest                   — local variable already derived above
//     via trustedDeviceCredentialDigest (only in RegisterTrustedDevice).
var allowedCredentialWritePrefixes = []string{
	"trustedDeviceCredentialDigest(",
	"trustedDeviceCredentialMask(",
	"credentialDigest",
}

// TestTrustedDeviceCredentialWriteSitesUseDigest is the H1 structural guard.
// It walks every non-test Go file under internal/ that constructs the persistence
// entity (i.e. imports the trusteddevices domain package) and rejects any
// assignment of a literal to TrustedDevice.DeviceCredentialID that does not
// read from the single conversion functions. A new caller that writes a
// plaintext credential (or an ad-hoc hash) therefore fails this test instead of
// silently shipping.
//
// Transport DTOs (LoginCommand / TrustedDeviceRegisterCommand) are excluded on
// purpose: they must carry the client-supplied plaintext until the app layer
// hashes it; they never construct or persist the entity.
func TestTrustedDeviceCredentialWriteSitesUseDigest(t *testing.T) {
	// internal/app/auth -> repo root.
	internalRoot := filepath.Join("..", "..", "..", "internal")
	err := filepath.WalkDir(internalRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		// Only files that construct the persistence entity are in scope.
		if !strings.Contains(string(data), `"github.com/guliangiam/guliangiam/internal/domain/trusteddevices"`) {
			return nil
		}
		for lineNo, line := range strings.Split(string(data), "\n") {
			rhs, ok := assignedCredentialValue(line)
			if !ok {
				continue
			}
			if !allowedCredentialWrite(rhs) {
				t.Errorf("%s:%d DeviceCredentialID assignment must call trustedDeviceCredentialDigest/mask, got %q",
					path, lineNo+1, rhs)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan internal/: %v", err)
	}
}

// TestTrustedDeviceCredentialPlaintextWriteIsRejected is the guard's
// negative control: a plaintext right-hand side must not be accepted. Without
// this, the structural test could pass merely because no write sites exist.
func TestTrustedDeviceCredentialPlaintextWriteIsRejected(t *testing.T) {
	rhs, ok := assignedCredentialValue(`DeviceCredentialID: "plaintext-credential"`)
	if !ok {
		t.Fatal("assignedCredentialValue did not detect a composite-literal assignment")
	}
	if allowedCredentialWrite(rhs) {
		t.Fatalf("plaintext RHS %q was accepted; the structural guard cannot fail on a new plaintext caller", rhs)
	}

	rhs, ok = assignedCredentialValue(`device.DeviceCredentialID = "plaintext-credential"`)
	if !ok {
		t.Fatal("assignedCredentialValue did not detect a field assignment")
	}
	if allowedCredentialWrite(rhs) {
		t.Fatalf("plaintext field RHS %q was accepted; the structural guard cannot fail on a new plaintext caller", rhs)
	}
}

func allowedCredentialWrite(rhs string) bool {
	for _, prefix := range allowedCredentialWritePrefixes {
		if strings.HasPrefix(rhs, prefix) {
			return true
		}
	}
	return false
}

// assignedCredentialValue returns the right-hand side when the line assigns a
// literal to DeviceCredentialID, either in a composite literal
// ("DeviceCredentialID: x") or as a field assignment ("x.DeviceCredentialID = y").
func assignedCredentialValue(line string) (string, bool) {
	if idx := strings.Index(line, "DeviceCredentialID:"); idx >= 0 {
		return strings.TrimSpace(line[idx+len("DeviceCredentialID:"):]), true
	}
	if idx := strings.Index(line, "DeviceCredentialID ="); idx >= 0 {
		return strings.TrimSpace(line[idx+len("DeviceCredentialID ="):]), true
	}
	return "", false
}
