package applications

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
)

func TestProjectClientJWKSEmitsOnlyWhitelistedPublicMembers(t *testing.T) {
	resetFakeJWKStores()
	repo := &mockApplicationRepo{}
	// 人为构造一行“库中存在越界数据”的 JWK：私钥成员也写在 JWK 里。
	require.NoError(t, repo.ReplaceClientJWKs(context.Background(), "org-1", "app-1", []*domain.ClientJWK{{
		ID:             "jwk-private-leak",
		ApplicationID:  "app-1",
		OrganizationID: "org-1",
		Kid:            "kid-leak",
		Alg:            "RS256",
		JWK: `{"kty":"RSA","kid":"kid-leak","alg":"RS256","n":"modulus","e":"AQAB",` +
			`"d":"private-exponent","p":"prime-p","q":"prime-q","dp":"dp","dq":"dq","qi":"qi"}`,
	}}, time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)))

	keys, err := repo.ListActiveClientJWKs(context.Background(), "org-1", "app-1")
	require.NoError(t, err)
	view := projectClientJWKS(keys)

	assert.Equal(t, []string{"kid-leak"}, view.Kids)
	require.Len(t, view.Keys, 1)
	assert.Equal(t, "RSA", view.Keys[0].Kty)
	assert.Equal(t, "kid-leak", view.Keys[0].Kid)

	raw, err := json.Marshal(view)
	require.NoError(t, err)
	for _, forbidden := range []string{`"d"`, `"p"`, `"q"`, `"dp"`, `"dq"`, `"qi"`, "private-exponent"} {
		assert.NotContains(t, string(raw), forbidden, "私钥成员 %s 不得出现在投影中", forbidden)
	}
	assert.Contains(t, string(raw), `"n":"modulus"`)
}

func TestProjectClientJWKSReturnsEmptyArraysWhenNoKeys(t *testing.T) {
	view := projectClientJWKS(nil)
	raw, err := json.Marshal(view)
	require.NoError(t, err)
	assert.JSONEq(t, `{"kids":[],"keys":[]}`, string(raw))
}

func TestProjectClientJWKSSkipsUnusableRows(t *testing.T) {
	view := projectClientJWKS([]*domain.ClientJWK{
		nil,
		{ID: "broken", Kid: "broken", Status: domain.ClientJWKStatusActive, JWK: `{`},
		{ID: "oct", Kid: "oct", Status: domain.ClientJWKStatusActive, JWK: `{"kty":"oct","kid":"oct"}`},
		{ID: "rsa-no-n", Kid: "rsa-no-n", Status: domain.ClientJWKStatusActive, JWK: `{"kty":"RSA","kid":"rsa-no-n","e":"AQAB"}`},
	})
	assert.Equal(t, []string{}, view.Kids)
	assert.Equal(t, []ClientJWKPublicView{}, view.Keys)
}

func TestGetApplicationProjectsClientJWKS(t *testing.T) {
	resetFakeJWKStores()
	svc, repo, _, _ := newJWKService(t)
	repo.apps = append(repo.apps, &domain.Application{
		ID:                      "jwk-app-id",
		OrganizationID:          "org-1",
		ClientID:                "gl_cid_jwks",
		TokenEndpointAuthMethod: domain.TokenEndpointAuthMethodPrivateKeyJWT,
		Revision:                1,
	})
	require.NoError(t, repo.ReplaceClientJWKs(context.Background(), "org-1", "jwk-app-id", []*domain.ClientJWK{{
		ID:             "jwk-1",
		ApplicationID:  "jwk-app-id",
		OrganizationID: "org-1",
		Kid:            "kid-1",
		Alg:            "RS256",
		JWK:            `{"kty":"RSA","kid":"kid-1","alg":"RS256","n":"modulus","e":"AQAB","d":"leak"}`,
	}}, time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)))

	detail, err := svc.GetApplication(context.Background(), "org-1", "jwk-app-id")
	require.NoError(t, err)
	assert.Equal(t, []string{"kid-1"}, detail.ClientJWKS.Kids)
	require.Len(t, detail.ClientJWKS.Keys, 1)
	assert.Equal(t, "modulus", detail.ClientJWKS.Keys[0].N)

	raw, err := json.Marshal(detail)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), `"d":"leak"`, "详情响应不得回显私钥成员")
}

func TestGetApplicationEchoesClientJWKSURI(t *testing.T) {
	resetFakeJWKStores()
	svc, repo, _, _ := newJWKService(t)
	repo.apps = append(repo.apps, &domain.Application{
		ID:                      "uri-app-id",
		OrganizationID:          "org-1",
		ClientID:                "gl_cid_uri",
		TokenEndpointAuthMethod: domain.TokenEndpointAuthMethodPrivateKeyJWT,
		Revision:                1,
	})
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	require.NoError(t, repo.SetClientJWKSURI(context.Background(), "org-1", "uri-app-id", "https://client.example.com/jwks.json", now))

	detail, err := svc.GetApplication(context.Background(), "org-1", "uri-app-id")
	require.NoError(t, err)
	assert.Equal(t, "https://client.example.com/jwks.json", detail.JWKSURI)

	defaultDetail, err := svc.GetDefaultApplication(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Equal(t, "https://client.example.com/jwks.json", defaultDetail.JWKSURI)

	// NotFound 是“仅内嵌 JWKS”的正常状态：读回为空且 JSON 省略字段，而非报错。
	require.NoError(t, repo.DeleteClientJWKSURI(context.Background(), "org-1", "uri-app-id"))
	detail, err = svc.GetApplication(context.Background(), "org-1", "uri-app-id")
	require.NoError(t, err)
	assert.Empty(t, detail.JWKSURI)
	raw, err := json.Marshal(detail)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), `"jwksUri"`)
}
