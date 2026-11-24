package idv

import (
	"strings"
	"testing"

	"github.com/guliangiam/guliangiam/internal/platform/crypto/cryptotest"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestSubjectRef_EmptyInputYieldsNoReference(t *testing.T) {
	ref, err := subjectRef("   ")
	if err != nil || ref != "" {
		t.Fatalf("empty ID number must yield no reference, got %q err=%v", ref, err)
	}
}

func TestSubjectRef_FailClosedWithoutKey(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", "")
	t.Setenv("ENCRYPTION_KEY", "")
	_, err := subjectRef("110101199001011234")
	if appErr, ok := err.(*apperrors.Error); !ok || appErr.Code != apperrors.CodeUnavailable {
		t.Fatalf("missing KEK must fail closed (Unavailable), got %v", err)
	}
}

func TestSubjectRef_DeterministicDeidentifiedHex(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)
	const idNumber = "110101199001011234"
	first, err := subjectRef(idNumber)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := subjectRef("  " + idNumber + "  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first != second {
		t.Error("trimming must not change the derived reference")
	}
	if len(first) != 64 {
		t.Errorf("SM3 hex must be 64 chars, got %d", len(first))
	}
	if strings.Contains(first, idNumber) {
		t.Fatal("reference must not contain the plaintext ID number")
	}
	other, err := subjectRef("110101199001011235")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if other == first {
		t.Error("different ID numbers must derive different references")
	}
}

func TestNewBizSeq_Unique32Hex(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		seq, err := newBizSeq()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(seq) != 32 {
			t.Fatalf("bizSeq must be 32 hex chars, got %d", len(seq))
		}
		if seen[seq] {
			t.Fatal("bizSeq must be unique per request")
		}
		seen[seq] = true
	}
}
