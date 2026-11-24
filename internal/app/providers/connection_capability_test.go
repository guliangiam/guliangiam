package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
)

func TestIdentityConnectionProbeModeIDV(t *testing.T) {
	for _, kind := range []string{"jumio", "aliyun_idv", "tencent_idv"} {
		p := &domain.Provider{
			Category: domain.CategoryIDVerification,
			Kind:     domain.Kind(kind),
		}
		assert.Equal(t, connectionProbeIDV, identityConnectionProbeMode(p), kind)
	}

	unwired := &domain.Provider{Category: domain.CategoryIDVerification, Kind: domain.Kind("custom")}
	assert.Equal(t, connectionProbeStructuralOnly, identityConnectionProbeMode(unwired))
	assert.Equal(t, connectionProbeStructuralOnly, identityConnectionProbeMode(nil))
}
