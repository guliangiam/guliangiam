package dcr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appapplications "github.com/guliangiam/guliangiam/internal/app/applications"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// dcrSecretRepo is a read-only application repository holding one dynamically
// registered client. Only the two lookups used by GetDynamicClient are real;
// everything else is unreachable through the read path.
type dcrSecretRepo struct {
	domain.Repository
	app  *domain.Application
	hash string
}

func (r dcrSecretRepo) GetByClientID(_ context.Context, clientID string) (*domain.Application, error) {
	if r.app != nil && r.app.ClientID == clientID {
		return r.app, nil
	}
	return nil, apperrors.NewNotFound("application not found")
}

func (r dcrSecretRepo) GetRegistrationAccessTokenHash(_ context.Context, clientID string) (string, error) {
	if r.app != nil && r.app.ClientID == clientID {
		return r.hash, nil
	}
	return "", nil
}

type dcrSecretUOW struct {
	appuow.AuthUoW
	repo domain.Repository
}

func (u dcrSecretUOW) Applications() domain.Repository { return u.repo }

type dcrSecretClock struct{}

func (dcrSecretClock) Now() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }

type dcrSecretUUID struct{}

func (dcrSecretUUID) NewString() (string, error) { return "generated-id", nil }

// TestGetClient_NoSecretMaterial is the read-side regression for the DCR client
// read (RFC 7591 §2.3 / §3.2.1): the caller holds a management credential, but
// the response must not carry the client secret in any shape — neither the
// plaintext, nor the stored argon2id hash, nor the masked read projection that
// the console detail endpoint returns. RFC 7591 only returns client_secret once,
// in the registration response.
func TestGetClient_NoSecretMaterial(t *testing.T) {
	const token = "gl_rat_dcr-secret-projection"
	const appID = "app-dcr-secret"
	const clientID = "client-dcr-secret"
	hash := "$argon2id$v=19$m=65536,t=3,p=2$dcr-sentinel$dcr-sentinel"

	app := &domain.Application{
		ID:                      appID,
		OrganizationID:          "org-1",
		Slug:                    "dcr-client",
		DisplayName:             "DCR Client",
		Category:                "dcr",
		Status:                  "active",
		ClientID:                clientID,
		ClientSecretHash:        hash,
		RedirectURIs:            []string{"https://rp.example.com/cb"},
		GrantTypes:              []string{"authorization_code"},
		TokenEndpointAuthMethod: domain.TokenEndpointAuthMethodClientSecretBasic,
		CreatedAt:               time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	repo := dcrSecretRepo{app: app, hash: platformcrypto.HashTokenValue(token)}
	service := appapplications.Service{
		Clock: dcrSecretClock{},
		UUID:  dcrSecretUUID{},
		UOW:   dcrSecretUOW{repo: repo},
	}

	router := testRouter()
	RegisterChiRoutes(router, Handler{Service: service})

	req := httptest.NewRequest(http.MethodGet, "/oauth2/register/"+clientID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	for _, key := range []string{
		"client_secret",
		"clientSecret",
		"clientSecretPlain",
		"clientSecretHash",
		"clientSecretMasked",
		"clientSecretSet",
	} {
		_, exists := payload[key]
		assert.Falsef(t, exists, "DCR GET 响应不得包含字段 %q", key)
	}
	assert.NotContains(t, rec.Body.String(), "$argon2id$", "DCR GET 不得回显 secret 哈希")
	assert.NotContains(t, rec.Body.String(), platformcrypto.HashTokenValue(token), "DCR GET 不得回显注册令牌哈希")
}
