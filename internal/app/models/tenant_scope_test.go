package models

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

// TestModelByID_RequiresOrganization pins the non-HTTP fail-fast: without a
// trusted tenant the app layer rejects before touching any repository.
func TestModelByID_RequiresOrganization(t *testing.T) {
	svc := Service{}
	ctx := context.Background()

	_, err := svc.GetModel(ctx, "  ", "model-1")
	assertByIDInvalidArgument(t, err)

	err = svc.PatchModel(ctx, "", PatchModelCommand{ID: "model-1"})
	assertByIDInvalidArgument(t, err)

	err = svc.DeleteModel(ctx, "", "model-1")
	assertByIDInvalidArgument(t, err)
}
