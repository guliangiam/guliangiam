package uow

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	uow := New(nil)
	assert.NotNil(t, uow)
}
