package enumset

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestNewSet_KeepsDeclarationOrderAndMembership(t *testing.T) {
	set := NewSet("test", "b", "a", "c")

	assert.Equal(t, "test", set.Name())
	assert.Equal(t, []string{"b", "a", "c"}, set.Values(), "declaration order is the order a caller sees")
	assert.True(t, set.Contains("a"))
	assert.False(t, set.Contains("A"), "membership is exact: folding case would hide a client/server disagreement")
	assert.False(t, set.Contains(""))
	assert.False(t, set.Contains("d"))
}

func TestSet_SortedValuesDoNotMutateTheSet(t *testing.T) {
	set := NewSet("test", "b", "a", "c")

	assert.Equal(t, []string{"a", "b", "c"}, set.SortedValues())
	assert.Equal(t, []string{"b", "a", "c"}, set.Values(), "SortedValues must not reorder the set itself")
	assert.Equal(t, "a, b, c", set.JoinedValues())
}

func TestSet_ValuesReturnsACopy(t *testing.T) {
	set := NewSet("test", "a", "b")

	values := set.Values()
	values[0] = "mutated"

	assert.Equal(t, []string{"a", "b"}, set.Values(), "a caller must not be able to widen the set through Values()")
}

func TestNewSet_PanicsOnMalformedDeclaration(t *testing.T) {
	// A malformed declaration is a programming error in a package-level
	// variable, so it must fail while the binary loads rather than per request.
	cases := map[string]func(){
		"blank name":      func() { NewSet("", "a", "b") },
		"whitespace name": func() { NewSet("  ", "a", "b") },
		"single value":    func() { NewSet("test", "a") },
		"no values":       func() { NewSet("test") },
		"blank value":     func() { NewSet("test", "a", "") },
		"padded value":    func() { NewSet("test", "a", " b") },
		"duplicate value": func() { NewSet("test", "a", "b", "a") },
	}
	for name, declare := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Panics(t, declare)
		})
	}
}

func TestSet_Validate(t *testing.T) {
	set := NewSet("test", "beta", "alpha")

	t.Run("member passes", func(t *testing.T) {
		assert.NoError(t, set.Validate("field", "alpha"))
	})

	t.Run("non-member is 422 with field, value and allowed", func(t *testing.T) {
		err := set.Validate("field", "Alpha")
		require.Error(t, err)

		var appErr *apperrors.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code,
			"the request is well formed and one field change fixes it, so 422 not 400")
		require.NotNil(t, appErr.Meta)
		assert.Equal(t, "field", appErr.Meta["field"])
		assert.Equal(t, "Alpha", appErr.Meta["value"])
		assert.Equal(t, []string{"alpha", "beta"}, appErr.Meta["allowed"])
		assert.Equal(t, "test", appErr.Meta["set"])
		assert.Contains(t, appErr.Message, "alpha, beta", "the message lists what is allowed")
	})
}

func TestSet_ValidateRequired(t *testing.T) {
	set := NewSet("test", "alpha", "beta")

	t.Run("blank is an argument error, not an unprocessable one", func(t *testing.T) {
		for _, blank := range []string{"", "   "} {
			err := set.ValidateRequired("field", blank)
			require.Error(t, err)

			var appErr *apperrors.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code,
				"absent and unknown must stay distinguishable")
		}
	})

	t.Run("unknown is 422", func(t *testing.T) {
		err := set.ValidateRequired("field", "gamma")
		require.Error(t, err)

		var appErr *apperrors.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	})

	t.Run("member passes", func(t *testing.T) {
		assert.NoError(t, set.ValidateRequired("field", "alpha"))
	})
}

func TestSet_ValidateOptional(t *testing.T) {
	set := NewSet("test", "alpha", "beta")

	t.Run("absent passes so a partial update need not mention the field", func(t *testing.T) {
		assert.NoError(t, set.ValidateOptional("field", nil))
		assert.NoError(t, set.ValidateOptionalRequired("field", nil))
	})

	t.Run("present blank is a blank error", func(t *testing.T) {
		blank := "  "
		err := set.ValidateOptional("field", &blank)
		require.Error(t, err)

		var appErr *apperrors.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
		assert.Contains(t, appErr.Message, "must not be blank")
	})

	t.Run("present unknown is 422", func(t *testing.T) {
		unknown := "gamma"
		for name, err := range map[string]error{
			"ValidateOptional":         set.ValidateOptional("field", &unknown),
			"ValidateOptionalRequired": set.ValidateOptionalRequired("field", &unknown),
		} {
			require.Error(t, err, name)
			var appErr *apperrors.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code, name)
		}
	})

	t.Run("present member passes", func(t *testing.T) {
		member := "beta"
		assert.NoError(t, set.ValidateOptional("field", &member))
		assert.NoError(t, set.ValidateOptionalRequired("field", &member))
	})
}
