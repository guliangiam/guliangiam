package organizations

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDerefInt_Nil(t *testing.T) {
	assert.Equal(t, 0, derefInt(nil))
}

func TestDerefInt_NonNil(t *testing.T) {
	v := 42
	assert.Equal(t, 42, derefInt(&v))
}

func TestDerefBytes_Nil(t *testing.T) {
	assert.Nil(t, derefBytes(nil))
}

func TestDerefBytes_NonNil(t *testing.T) {
	v := []byte("hello")
	assert.Equal(t, []byte("hello"), derefBytes(&v))
}

func TestDerefString_Nil(t *testing.T) {
	assert.Equal(t, "", derefString(nil))
}

func TestDerefString_NonNil(t *testing.T) {
	v := "hello"
	assert.Equal(t, "hello", derefString(&v))
}

func TestMax_Organizations(t *testing.T) {
	assert.Equal(t, 20, maxInt(0, 20))
	assert.Equal(t, 20, maxInt(-1, 20))
	assert.Equal(t, 5, maxInt(5, 20))
}

func TestAggregateRow_Fields(t *testing.T) {
	row := aggregateRow{}
	assert.False(t, row.hasSet)
	assert.False(t, row.hasTheme)
	assert.False(t, row.hasSec)
}
