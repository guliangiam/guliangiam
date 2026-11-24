package forms

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

// TestFormByID_RequiresOrganization pins the non-HTTP fail-fast: without a
// trusted tenant the app layer rejects before touching any repository.
func TestFormByID_RequiresOrganization(t *testing.T) {
	svc := Service{}
	ctx := context.Background()

	_, err := svc.GetForm(ctx, "  ", "form-1")
	assertByIDInvalidArgument(t, err)

	err = svc.PatchForm(ctx, "", PatchFormCommand{ID: "form-1"})
	assertByIDInvalidArgument(t, err)

	err = svc.DeleteForm(ctx, "", "form-1")
	assertByIDInvalidArgument(t, err)
}
