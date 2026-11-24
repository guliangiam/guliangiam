package auth

import (
	"testing"

	domainauth "github.com/guliangiam/guliangiam/internal/domain/auth"
	infrajwt "github.com/guliangiam/guliangiam/internal/infra/jwt"
)

func TestJWKSProviderAdapterImplementsJWKSProvider(t *testing.T) {
	var _ domainauth.JWKSProvider = (*infrajwt.JWKSProviderAdapter)(nil)
}
