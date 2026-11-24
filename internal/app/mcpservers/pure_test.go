package mcpservers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExpandCIDR_InvalidCIDR(t *testing.T) {
	_, err := expandCIDR("not-a-cidr")
	assert.Error(t, err)
}

func TestExpandCIDR_Slash32(t *testing.T) {
	ips, err := expandCIDR("192.168.1.1/32")
	assert.NoError(t, err)
	assert.Len(t, ips, 1)
	assert.Equal(t, "192.168.1.1", ips[0])
}

func TestExpandCIDR_Slash30(t *testing.T) {
	ips, err := expandCIDR("192.168.1.0/30")
	assert.NoError(t, err)
	// /30 has 4 addresses, but network and broadcast are skipped
	assert.True(t, len(ips) >= 2)
}

func TestExpandCIDR_InvalidBits(t *testing.T) {
	_, err := expandCIDR("192.168.1.0/33")
	assert.Error(t, err)
}
