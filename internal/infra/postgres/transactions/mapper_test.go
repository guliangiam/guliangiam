package transactions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNilIfEmpty(t *testing.T) {
	t.Run("empty_string_returns_nil", func(t *testing.T) {
		result := nilIfEmpty("")
		assert.Nil(t, result)
	})

	t.Run("non_empty_string_returns_pointer", func(t *testing.T) {
		result := nilIfEmpty("hello")
		assert.NotNil(t, result)
		assert.Equal(t, "hello", *result)
	})

	t.Run("whitespace_string_returns_pointer", func(t *testing.T) {
		result := nilIfEmpty("  ")
		assert.NotNil(t, result)
		assert.Equal(t, "  ", *result)
	})
}
