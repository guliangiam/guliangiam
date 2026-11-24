package organizations

import (
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateOrganizationParent(t *testing.T) {
	t.Run("root", func(t *testing.T) {
		require.NoError(t, ValidateOrganizationParent("org-1", "", nil))
	})
	t.Run("normal", func(t *testing.T) {
		require.NoError(t, ValidateOrganizationParent("org-3", "org-2", []string{"org-2", "org-1"}))
	})
	t.Run("self_parent", func(t *testing.T) {
		err := ValidateOrganizationParent("org-1", "org-1", nil)
		require.Error(t, err)
		var appErr *apperrors.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
		assert.Equal(t, "parentId", appErr.Meta["field"])
	})
	t.Run("cycle", func(t *testing.T) {
		err := ValidateOrganizationParent("org-1", "org-3", []string{"org-3", "org-2", "org-1"})
		require.Error(t, err)
		var appErr *apperrors.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
		assert.Equal(t, "org-1", appErr.Meta["cycleAt"])
	})
}

func TestValidateOrganizationParent_Depth(t *testing.T) {
	// depth = len(ancestors)+1（祖先链含 parent 到根）；上限 8 → 祖先链最多 7 个仍可挂接。
	ancestorsAtLimit := make([]string, 0, MaxOrganizationTreeDepth-1)
	for index := 0; index < MaxOrganizationTreeDepth-1; index++ {
		ancestorsAtLimit = append(ancestorsAtLimit, "ancestor")
	}
	require.NoError(t, ValidateOrganizationParent("org-new", "org-parent", ancestorsAtLimit))
	assert.Equal(t, MaxOrganizationTreeDepth, TreeDepth(ancestorsAtLimit))

	tooDeep := append(append([]string{}, ancestorsAtLimit...), "ancestor")
	err := ValidateOrganizationParent("org-new", "org-parent", tooDeep)
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, MaxOrganizationTreeDepth, appErr.Meta["maxDepth"])
}
