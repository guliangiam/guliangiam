package rules

import (
	"context"
	"errors"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/require"
)

func assertByIDInvalidArgument(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

// TestRuleByID_RequiresOrganization pins the non-HTTP fail-fast: without a
// trusted tenant the app layer rejects before touching any repository.
func TestRuleByID_RequiresOrganization(t *testing.T) {
	svc := Service{}
	ctx := context.Background()

	_, err := svc.GetRule(ctx, "  ", "rule-1")
	assertByIDInvalidArgument(t, err)

	err = svc.PatchRule(ctx, "", PatchRuleCommand{ID: "rule-1"})
	assertByIDInvalidArgument(t, err)

	err = svc.DeleteRule(ctx, "", "rule-1")
	assertByIDInvalidArgument(t, err)
}
