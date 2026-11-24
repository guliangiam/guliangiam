package permissions

import (
	"errors"
	"strings"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func assertPermissionAppError(t *testing.T, err error, code apperrors.Code) *apperrors.Error {
	t.Helper()
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %T (%v)", err, err)
	}
	if appErr.Code != code {
		t.Fatalf("code = %q, want %q", appErr.Code, code)
	}
	return appErr
}

func TestValidateCreatePermissionCommand(t *testing.T) {
	assertPermissionAppError(t, validateCreatePermissionCommand(CreatePermissionCommand{}), apperrors.CodeInvalidArgument)
	validCmd := CreatePermissionCommand{
		OrganizationID: "org1",
		Name:           "perm1",
		DisplayName:    "Permission 1",
		ResourceType:   "application",
		Effect:         "allow",
		State:          "active",
	}
	if err := validateCreatePermissionCommand(validCmd); err != nil {
		t.Errorf("expected valid: %v", err)
	}
	invalidEffect := validCmd
	invalidEffect.Effect = "invalid"
	appErr := assertPermissionAppError(t, validateCreatePermissionCommand(invalidEffect), apperrors.CodeUnprocessableEntity)
	if appErr.Meta["allowed"] == nil {
		t.Fatalf("meta = %v, want allowed", appErr.Meta)
	}
	invalidState := validCmd
	invalidState.State = "invalid"
	appErr = assertPermissionAppError(t, validateCreatePermissionCommand(invalidState), apperrors.CodeUnprocessableEntity)
	if appErr.Meta["field"] != "state" || appErr.Meta["allowed"] == nil {
		t.Fatalf("meta = %v, want field=state + allowed", appErr.Meta)
	}
}

func TestValidatePatchPermissionCommand(t *testing.T) {
	if err := validatePatchPermissionCommand(PatchPermissionCommand{}); err != nil {
		t.Errorf("expected valid for empty patch: %v", err)
	}
	blank := "  "
	assertPermissionAppError(t, validatePatchPermissionCommand(PatchPermissionCommand{DisplayName: &blank}), apperrors.CodeInvalidArgument)

	badEffect := "block"
	appErr := assertPermissionAppError(t, validatePatchPermissionCommand(PatchPermissionCommand{Effect: &badEffect}), apperrors.CodeUnprocessableEntity)
	if appErr.Meta["field"] != "effect" || appErr.Meta["value"] != "block" {
		t.Fatalf("meta = %v, want field=effect value=block", appErr.Meta)
	}

	badState := "unknown"
	appErr = assertPermissionAppError(t, validatePatchPermissionCommand(PatchPermissionCommand{State: &badState}), apperrors.CodeUnprocessableEntity)
	if appErr.Meta["field"] != "state" || appErr.Meta["value"] != "unknown" {
		t.Fatalf("meta = %v, want field=state value=unknown", appErr.Meta)
	}
}

func TestValidateUniqueStrings(t *testing.T) {
	if err := validateUniqueStrings([]string{"a", "b", "c"}, "test"); err != nil {
		t.Errorf("expected valid: %v", err)
	}
	assertPermissionAppError(t, validateUniqueStrings([]string{"a", "a"}, "test"), apperrors.CodeUnprocessableEntity)
	assertPermissionAppError(t, validateUniqueStrings([]string{"a", ""}, "test"), apperrors.CodeUnprocessableEntity)
	if err := validateUniqueStrings([]string{}, "test"); err != nil {
		t.Errorf("expected valid for empty: %v", err)
	}
}

func TestSafeGetPermRow(t *testing.T) {
	row := []string{"a", "b"}
	if v := safeGetPermRow(row, 0); v != "a" {
		t.Errorf("got %q, want a", v)
	}
	if v := safeGetPermRow(row, 5); v != "" {
		t.Errorf("got %q, want empty", v)
	}
}

func TestSplitCSV(t *testing.T) {
	if v := splitCSV("a,b,c"); len(v) != 3 || v[0] != "a" {
		t.Errorf("unexpected: %v", v)
	}
	if v := splitCSV(""); v != nil {
		t.Errorf("expected nil for empty, got %v", v)
	}
	if v := splitCSV("  a , b  "); len(v) != 2 || v[0] != "a" || v[1] != "b" {
		t.Errorf("unexpected: %v", v)
	}
}

func TestBuildEnforceRvals(t *testing.T) {
	req := EnforceRequest{Sub: "alice", Dom: "org1", Obj: "resource1", Act: "read"}
	rvals := buildEnforceRvals(req)
	if len(rvals) != 4 {
		t.Fatalf("expected 4 rvals, got %d", len(rvals))
	}
	if rvals[0] != "alice" || rvals[1] != "org1" || rvals[2] != "resource1" || rvals[3] != "read" {
		t.Errorf("unexpected rvals: %v", rvals)
	}
	req2 := EnforceRequest{Sub: "bob", Obj: "data", Act: "write"}
	rvals2 := buildEnforceRvals(req2)
	if len(rvals2) != 3 {
		t.Fatalf("expected 3 rvals (no dom), got %d", len(rvals2))
	}
}

func TestNormalizeEffect_TrimSpace(t *testing.T) {
	if v := normalizeEffect(strings.TrimSpace("  allow  ")); v != "allow" {
		t.Errorf("got %q, want allow", v)
	}
}
