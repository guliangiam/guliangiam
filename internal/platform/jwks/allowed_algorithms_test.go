package jwks

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateSigningAlgorithm_Allowed(t *testing.T) {
	allowed := []string{"RS256", "SM2-SM3", "RS384", "RS512", "ES256", "ES384", "ES512", "PS256", "PS384", "PS512"}
	for _, alg := range allowed {
		t.Run(alg, func(t *testing.T) {
			err := ValidateSigningAlgorithm(alg)
			assert.NoError(t, err)
		})
	}
}

func TestValidateSigningAlgorithm_Forbidden(t *testing.T) {
	forbidden := []string{"none", "HS256", "HS384", "HS512"}
	for _, alg := range forbidden {
		t.Run(alg, func(t *testing.T) {
			err := ValidateSigningAlgorithm(alg)
			assert.Error(t, err)
		})
	}
}

func TestValidateSigningAlgorithm_Unknown(t *testing.T) {
	err := ValidateSigningAlgorithm("UNKNOWN")
	assert.Error(t, err)
}

func TestIsAlgorithmAllowed(t *testing.T) {
	assert.True(t, IsAlgorithmAllowed("RS256"))
	assert.True(t, IsAlgorithmAllowed("SM2-SM3"))
	assert.True(t, IsAlgorithmAllowed("ES256"))
	assert.False(t, IsAlgorithmAllowed("HS256"))
	assert.False(t, IsAlgorithmAllowed("none"))
	assert.False(t, IsAlgorithmAllowed("UNKNOWN"))
}

func TestFormatAllowedAlgorithms(t *testing.T) {
	result := FormatAllowedAlgorithms()
	assert.Contains(t, result, "RS256")
	assert.Contains(t, result, "ES256")
}
