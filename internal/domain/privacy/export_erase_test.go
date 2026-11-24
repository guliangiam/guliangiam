package privacy

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildEvidencePackage_Deterministic(t *testing.T) {
	now := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	recordsA := []ExportRecord{
		{Category: "profile", SubjectID: "user-1", Payload: map[string]any{"name": "alice"}},
		{Category: "consent", SubjectID: "user-1", Payload: map[string]any{"version": 2}},
	}
	recordsB := []ExportRecord{recordsA[1], recordsA[0]}

	pkgA, err := BuildEvidencePackage(recordsA, "cn", now)
	require.NoError(t, err)
	pkgB, err := BuildEvidencePackage(recordsB, "cn", now)
	require.NoError(t, err)

	assert.Equal(t, pkgA.Checksum, pkgB.Checksum, "same set must hash identically regardless of input order")
	assert.Equal(t, 2, pkgA.LineCount)
	assert.Equal(t, "cn", pkgA.Region)

	lines, err := pkgA.JSONLines()
	require.NoError(t, err)
	sum := sha256.Sum256(lines)
	assert.Equal(t, hex.EncodeToString(sum[:]), pkgA.Checksum, "manifest checksum must match the downloadable payload")

	manifest := pkgA.Manifest()
	assert.Equal(t, pkgA.Checksum, manifest.Checksum)
	assert.Equal(t, now, manifest.GeneratedAt)
}

func TestBuildEvidencePackage_Validation(t *testing.T) {
	now := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	_, err := BuildEvidencePackage(nil, "", now)
	require.Error(t, err)

	_, err = BuildEvidencePackage([]ExportRecord{{SubjectID: "user-1"}}, "cn", now)
	require.Error(t, err)

	_, err = BuildEvidencePackage([]ExportRecord{{Category: "profile"}}, "cn", now)
	require.Error(t, err)
}

func TestPlanErase_RetainedCategoryExcluded(t *testing.T) {
	scope, err := PlanErase(
		[]EraseTarget{{Category: "profile", TargetID: "user-1"}, {Category: "audit", TargetID: "user-1"}},
		[]RetentionItem{{Category: "audit", LegalBasis: "compliance-record-retention", Blocks: false}},
	)
	require.NoError(t, err)
	assert.Len(t, scope.Erase, 1)
	assert.Equal(t, "profile", scope.Erase[0].Category)
	require.Len(t, scope.Retain, 1)
	assert.Equal(t, "compliance-record-retention", scope.Retain[0].LegalBasis)
}

func TestPlanErase_LegalHoldBlocks(t *testing.T) {
	_, err := PlanErase(
		[]EraseTarget{{Category: "audit", TargetID: "user-1"}},
		[]RetentionItem{{Category: "audit", LegalBasis: "litigation-hold", Blocks: true}},
	)
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeConflict, appErr.Code)
	assert.Equal(t, "litigation-hold", appErr.Meta["legalBasis"])
}

func TestPlanErase_NothingLeft(t *testing.T) {
	_, err := PlanErase(
		[]EraseTarget{{Category: "audit", TargetID: "user-1"}},
		[]RetentionItem{{Category: "audit", LegalBasis: "retention", Blocks: false}},
	)
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeConflict, appErr.Code)
}

func TestPlanErase_Validation(t *testing.T) {
	_, err := PlanErase(nil, nil)
	require.Error(t, err)

	_, err = PlanErase([]EraseTarget{{Category: "profile", TargetID: "user-1"}},
		[]RetentionItem{{Category: "audit"}})
	require.Error(t, err)

	_, err = PlanErase([]EraseTarget{{Category: "profile"}}, nil)
	require.Error(t, err)
}
