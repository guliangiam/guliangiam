package groups

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildGroupOrderBy(t *testing.T) {
	clause, err := buildGroupOrderBy("", "")
	require.NoError(t, err)
	require.Equal(t, "ORDER BY created_at DESC, id ASC", clause, "无参数保留历史默认 DESC")

	clause, err = buildGroupOrderBy("displayName", "asc")
	require.NoError(t, err)
	require.Equal(t, "ORDER BY display_name ASC, id ASC", clause)

	_, err = buildGroupOrderBy("password", "")
	require.Error(t, err, "白名单外字段必须报错而非静默回落")

	_, err = buildGroupOrderBy("", "sideways")
	require.Error(t, err)
}
