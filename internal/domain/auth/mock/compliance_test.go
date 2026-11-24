package mock

import (
	"testing"

	domainauth "github.com/guliangiam/guliangiam/internal/domain/auth"
)

func TestMockJWKSProviderImplementsInterface(t *testing.T) {
	var _ domainauth.JWKSProvider = NewMockJWKSProvider(nil)
}
