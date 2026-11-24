package mtls

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCertificateBinding(t *testing.T) {
	metadata := BindTokenToClientCertificate(nil, "thumbprint-1")

	require.NoError(t, ValidateCertificateBinding(metadata, "thumbprint-1"))
	require.Error(t, ValidateCertificateBinding(metadata, "thumbprint-2"))
	require.Error(t, ValidateCertificateBinding(metadata, ""))
}

func TestValidateCertificateBinding_UnboundToken(t *testing.T) {
	require.NoError(t, ValidateCertificateBinding(nil, ""))
	require.NoError(t, ValidateCertificateBinding(map[string]any{}, "ignored"))
	_, ok := map[string]any{}["cnf"]
	assert.False(t, ok)
}
