package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMarshalJSON_Nil(t *testing.T) {
	data, err := MarshalJSON(nil)
	assert.NoError(t, err)
	assert.Equal(t, "null", string(data))
}

func TestMarshalJSON_Struct(t *testing.T) {
	data, err := MarshalJSON(map[string]string{"key": "value"})
	assert.NoError(t, err)
	assert.Contains(t, string(data), `"key"`)
	assert.Contains(t, string(data), `"value"`)
}

func TestMarshalJSON_String(t *testing.T) {
	data, err := MarshalJSON("hello")
	assert.NoError(t, err)
	assert.Equal(t, `"hello"`, string(data))
}

func TestUnmarshalJSON_Empty(t *testing.T) {
	var result map[string]string
	err := UnmarshalJSON([]byte{}, &result)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestUnmarshalJSON_Null(t *testing.T) {
	var result map[string]string
	err := UnmarshalJSON([]byte("null"), &result)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestUnmarshalJSON_Valid(t *testing.T) {
	var result map[string]string
	err := UnmarshalJSON([]byte(`{"key":"value"}`), &result)
	assert.NoError(t, err)
	assert.Equal(t, "value", result["key"])
}

// The old Paginate let a caller pick the "default" size, which is how 43
// repositories ended up each choosing one. The bound is now a single constant,
// so these cases pin the range rather than a caller-supplied default.
func TestClampPageSize(t *testing.T) {
	cases := map[string]struct {
		in   int
		want int
	}{
		"zero takes the default":         {0, 20},
		"negative takes the default":     {-5, 20},
		"one passes through":             {1, 1},
		"five passes through":            {5, 5},
		"the maximum passes through":     {100, 100},
		"one over the maximum is capped": {101, 100},
		"far over the maximum is capped": {99999, 100},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, ClampPageSize(tc.in))
		})
	}
}

// pageSize=5 must yield five rows, not twenty: the previous `max(pageSize, 20)`
// was a floor, so a client asking for 5 silently received 20 while the response
// echoed 5.
func TestClampPageSize_DoesNotRaiseASmallPageSize(t *testing.T) {
	assert.Equal(t, 5, ClampPageSize(5))
	assert.NotEqual(t, 20, ClampPageSize(5))
}

// The offset and the limit must come from the same clamped size. Previously the
// offset used max(pageSize, 1) while the limit used max(pageSize, 20), so for
// pageSize in [1,19] page 2 started before page 1 ended and rows repeated.
func TestPageOffset_UsesTheSameSizeAsTheLimit(t *testing.T) {
	for pageSize := 1; pageSize <= 25; pageSize++ {
		limit := ClampPageSize(pageSize)
		firstPageEnd := 1 * limit
		secondPageStart := PageOffset(2, pageSize)
		assert.Equalf(t, firstPageEnd, secondPageStart,
			"pageSize=%d: page 2 must start where page 1 ends (limit=%d)", pageSize, limit)
	}
}

func TestPageOffset(t *testing.T) {
	cases := map[string]struct {
		page, pageSize int
		want           int
	}{
		"first page":         {1, 20, 0},
		"page zero":          {0, 20, 0},
		"negative page":      {-3, 20, 0},
		"second page":        {2, 10, 10},
		"third page":         {3, 25, 50},
		"size zero defaults": {2, 0, 20},
		"size over the max":  {2, 500, 100},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, PageOffset(tc.page, tc.pageSize))
		})
	}
}
