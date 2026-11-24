package aigateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// hashMatchingApiKeysRepo mirrors the production GetByHash contract
// (`WHERE key_hash = $1 AND deleted_at IS NULL`): exact equality on the stored
// hash. A stub that echoes the requested hash back would mask a hash-input
// mismatch between key issuance and key extraction.
type hashMatchingApiKeysRepo struct {
	keys []*domain.LlmApiKey
}

func (r *hashMatchingApiKeysRepo) GetByHash(_ context.Context, hash string) (*domain.LlmApiKey, error) {
	for _, k := range r.keys {
		if k.DeletedAt == nil && k.KeyHash == hash {
			cp := *k
			return &cp, nil
		}
	}
	return nil, apperrors.NewNotFound("api key not found")
}

func (r *hashMatchingApiKeysRepo) GetByID(_ context.Context, id string) (*domain.LlmApiKey, error) {
	for _, k := range r.keys {
		if k.ID == id {
			cp := *k
			return &cp, nil
		}
	}
	return nil, apperrors.NewNotFound("api key not found")
}

func (r *hashMatchingApiKeysRepo) List(_ context.Context, organizationID string) ([]*domain.LlmApiKey, error) {
	out := make([]*domain.LlmApiKey, 0, len(r.keys))
	for _, k := range r.keys {
		if k.OrganizationID == organizationID {
			cp := *k
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *hashMatchingApiKeysRepo) Create(_ context.Context, k *domain.LlmApiKey) error {
	r.keys = append(r.keys, k)
	return nil
}

func (r *hashMatchingApiKeysRepo) Update(_ context.Context, k *domain.LlmApiKey) error { return nil }
func (r *hashMatchingApiKeysRepo) Delete(_ context.Context, id string) error           { return nil }

func issuedKeyFixture(t *testing.T) (*domain.LlmApiKey, string) {
	t.Helper()
	k := &domain.LlmApiKey{
		ID: "k-issued", OrganizationID: "org-1", Name: "issued-key",
		Status: domain.APIKeyStatusActive, Currency: "USD",
	}
	plaintext, err := k.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	if !k.VerifyKey(plaintext) {
		t.Fatalf("VerifyKey must accept the plaintext returned by GenerateKey")
	}
	return k, plaintext
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// TestAPIKeyHash_CrossBoundary_IssuedKeyAuthenticates proves the issuance path
// (domain.GenerateKey) and the request path (app.ExtractAPIKeyHash) hash the
// same input, and that the extracted hash satisfies the repository lookup.
func TestAPIKeyHash_CrossBoundary_IssuedKeyAuthenticates(t *testing.T) {
	key, plaintext := issuedKeyFixture(t)
	repo := &hashMatchingApiKeysRepo{keys: []*domain.LlmApiKey{key}}
	svc := &GatewayService{Clock: mockClock{t: time.Now()}, ApiKeys: repo}

	cases := []struct {
		name       string
		authHeader string
		apiKey     string
	}{
		{"authorization bearer", "Bearer " + plaintext, ""},
		{"authorization raw value", plaintext, ""},
		{"x-api-key", "", plaintext},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hash, ok := ExtractAPIKeyHash(tc.authHeader, tc.apiKey)
			if !ok {
				t.Fatal("ExtractAPIKeyHash must accept the submitted key")
			}
			if hash != key.KeyHash {
				t.Fatalf("extracted hash = %s, want KeyHash = %s", hash, key.KeyHash)
			}
			got, err := repo.GetByHash(context.Background(), hash)
			if err != nil {
				t.Fatalf("GetByHash(extracted hash) must hit the issued key: %v", err)
			}
			if got.ID != key.ID {
				t.Fatalf("GetByHash returned key %s, want %s", got.ID, key.ID)
			}
			if _, err := svc.authenticate(context.Background(), hash); err != nil {
				t.Fatalf("authenticate(extracted hash) must succeed: %v", err)
			}
		})
	}
}

// TestAPIKeyHash_CrossBoundary_NoBodyOnlyAlias pins down that a body-only digest
// (the previous extraction contract) is not accepted, including the double
// "sk-" prefix that used to normalize into the issued plaintext.
func TestAPIKeyHash_CrossBoundary_NoBodyOnlyAlias(t *testing.T) {
	key, plaintext := issuedKeyFixture(t)
	repo := &hashMatchingApiKeysRepo{keys: []*domain.LlmApiKey{key}}
	svc := &GatewayService{Clock: mockClock{t: time.Now()}, ApiKeys: repo}

	doublePrefixed := "sk-" + plaintext
	cases := []struct {
		name       string
		authHeader string
		apiKey     string
	}{
		{"authorization bearer", "Bearer " + doublePrefixed, ""},
		{"authorization raw value", doublePrefixed, ""},
		{"x-api-key", "", doublePrefixed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hash, ok := ExtractAPIKeyHash(tc.authHeader, tc.apiKey)
			if !ok {
				t.Fatal("ExtractAPIKeyHash must accept a non-empty key")
			}
			if hash == key.KeyHash {
				t.Fatal("double-prefixed key must not normalize to the issued key hash")
			}
			if _, err := repo.GetByHash(context.Background(), hash); err == nil {
				t.Fatal("double-prefixed key must not resolve through GetByHash")
			}
		})
	}

	// The digest the old extractor produced (sk- stripped, body only) must miss.
	bodyOnlyHash := sha256Hex(plaintext[len("sk-"):])
	if bodyOnlyHash == key.KeyHash {
		t.Fatal("test fixture invalid: body-only hash equals the issued key hash")
	}
	if _, err := repo.GetByHash(context.Background(), bodyOnlyHash); err == nil {
		t.Fatal("body-only hash must not resolve through GetByHash")
	}
	_, err := svc.authenticate(context.Background(), bodyOnlyHash)
	if e, ok := err.(*apperrors.Error); !ok || e.Code != apperrors.CodeUnauthorized {
		t.Fatalf("body-only hash must be rejected as unauthorized, got %v", err)
	}
}
