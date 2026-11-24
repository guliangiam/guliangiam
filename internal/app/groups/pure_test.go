package groups

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeGetRow(t *testing.T) {
	t.Run("valid_index", func(t *testing.T) {
		row := []string{"a", "b", "c"}
		assert.Equal(t, "a", safeGetRow(row, 0))
		assert.Equal(t, "b", safeGetRow(row, 1))
		assert.Equal(t, "c", safeGetRow(row, 2))
	})

	t.Run("out_of_bounds", func(t *testing.T) {
		row := []string{"a"}
		assert.Equal(t, "a", safeGetRow(row, 0))
		assert.Equal(t, "", safeGetRow(row, 1))
		assert.Equal(t, "", safeGetRow(row, 5))
	})

	t.Run("trims_whitespace", func(t *testing.T) {
		row := []string{"  hello  ", " world "}
		assert.Equal(t, "hello", safeGetRow(row, 0))
		assert.Equal(t, "world", safeGetRow(row, 1))
	})

	t.Run("empty_row", func(t *testing.T) {
		row := []string{}
		assert.Equal(t, "", safeGetRow(row, 0))
	})
}

func TestNormalizeOptionalString(t *testing.T) {
	t.Run("spaces", func(t *testing.T) {
		assert.Equal(t, "hello", normalizeOptionalString("  hello  "))
	})

	t.Run("empty", func(t *testing.T) {
		assert.Equal(t, "", normalizeOptionalString(""))
	})

	t.Run("only_spaces", func(t *testing.T) {
		assert.Equal(t, "", normalizeOptionalString("   "))
	})

	t.Run("no_spaces", func(t *testing.T) {
		assert.Equal(t, "test", normalizeOptionalString("test"))
	})
}
