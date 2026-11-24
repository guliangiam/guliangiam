package keys

import (
	domainauth "github.com/guliangiam/guliangiam/internal/domain/auth"
)

// JWK is the app-layer read model for a single JSON Web Key.
// Transport adapters serialize this type directly, so they never need to
// import the domain package.
type JWK struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n,omitempty"`
	E   string `json:"e,omitempty"`
	Crv string `json:"crv,omitempty"`
	X   string `json:"x,omitempty"`
	Y   string `json:"y,omitempty"`
}

// JWKS is the app-layer read model for a JSON Web Key Set.
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWKSProvider exposes the platform signing key set to transport callers
// without leaking domain types.
type JWKSProvider interface {
	// GetJWKS returns the standard-algorithm (RSA/ES) key set for the default
	// jwks_uri, kept free of the SM2 curve so strict JOSE clients can parse it.
	GetJWKS() (*JWKS, error)
	// GetGMJWKS returns the 国密 SM2-SM3 key set for the dedicated GM endpoint.
	GetGMJWKS() (*JWKS, error)
}

type jwksViewProvider struct {
	inner domainauth.JWKSProvider
}

// NewJWKSViewProvider adapts a domain JWKS port into the app-layer view used by
// the .well-known handlers.
func NewJWKSViewProvider(inner domainauth.JWKSProvider) JWKSProvider {
	return jwksViewProvider{inner: inner}
}

func (p jwksViewProvider) GetJWKS() (*JWKS, error) {
	set, err := p.inner.GetJWKS()
	if err != nil {
		return nil, err
	}
	return mapJWKSView(set), nil
}

func (p jwksViewProvider) GetGMJWKS() (*JWKS, error) {
	set, err := p.inner.GetGMJWKS()
	if err != nil {
		return nil, err
	}
	return mapJWKSView(set), nil
}

func mapJWKSView(set *domainauth.JWKS) *JWKS {
	if set == nil {
		return &JWKS{Keys: []JWK{}}
	}
	keys := make([]JWK, 0, len(set.Keys))
	for _, k := range set.Keys {
		keys = append(keys, JWK{
			Kty: k.Kty,
			Use: k.Use,
			Kid: k.Kid,
			Alg: k.Alg,
			N:   k.N,
			E:   k.E,
			Crv: k.Crv,
			X:   k.X,
			Y:   k.Y,
		})
	}
	return &JWKS{Keys: keys}
}
