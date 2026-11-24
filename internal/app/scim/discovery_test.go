package scim

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleServiceProviderConfig(t *testing.T) {
	req := httptest.NewRequest("GET", "/scim/v2/ServiceProviderConfig", http.NoBody)
	w := httptest.NewRecorder()
	HandleServiceProviderConfig(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/scim+json", w.Header().Get("Content-Type"))

	var config ServiceProviderConfig
	err := json.NewDecoder(w.Body).Decode(&config)
	require.NoError(t, err)
	assert.True(t, config.Patch.Supported)
	assert.True(t, config.Filter.Supported)
	assert.False(t, config.Bulk.Supported)
	// 宣告必须与实际能力一致：SCIM 面不改密、中间件只接受 Bearer。
	assert.False(t, config.ChangePassword.Supported)
	// sortBy/sortOrder 已被查询编排消费，sort 宣告支持（OPEN-API-87 已关闭）。
	assert.True(t, config.Sort.Supported)
	require.Len(t, config.AuthenticationSchemes, 1)
	assert.Equal(t, "oauthbearertoken", config.AuthenticationSchemes[0].Type)
}

func TestHandleResourceTypes(t *testing.T) {
	req := httptest.NewRequest("GET", "/scim/v2/ResourceTypes", http.NoBody)
	w := httptest.NewRecorder()
	HandleResourceTypes(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/scim+json", w.Header().Get("Content-Type"))

	var types []ResourceType
	err := json.NewDecoder(w.Body).Decode(&types)
	require.NoError(t, err)
	assert.Len(t, types, 2)
	assert.Equal(t, "User", types[0].ID)
	assert.Equal(t, "Group", types[1].ID)
}

func TestHandleSchemas(t *testing.T) {
	req := httptest.NewRequest("GET", "/scim/v2/Schemas", http.NoBody)
	w := httptest.NewRecorder()
	HandleSchemas(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/scim+json", w.Header().Get("Content-Type"))

	var schemas []Schema
	err := json.NewDecoder(w.Body).Decode(&schemas)
	require.NoError(t, err)
	assert.Len(t, schemas, 2)
	assert.Equal(t, "urn:ietf:params:scim:schemas:core:2.0:User", schemas[0].ID)
	assert.Equal(t, "urn:ietf:params:scim:schemas:core:2.0:Group", schemas[1].ID)
}
