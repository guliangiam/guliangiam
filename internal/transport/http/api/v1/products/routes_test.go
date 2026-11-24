package products

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoutes(t *testing.T) {
	routes := Routes()
	assert.NotEmpty(t, routes)
	for _, r := range routes {
		assert.NotEmpty(t, r.Pattern)
		assert.NotEmpty(t, r.Method)
	}
}
