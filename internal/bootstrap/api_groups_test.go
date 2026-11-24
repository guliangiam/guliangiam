package bootstrap

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
)

func TestV1GroupsRoutes(t *testing.T) {
	router, _, _, _, _, _, _, _ := newTestV1Router(t)

	// Create
	createBody := map[string]any{
		"organizationId": "org-1",
		"name":           "engineering",
		"displayName":    "Engineering",
		"type":           "department",
		"isTopGroup":     true,
		"isEnabled":      true,
	}
	createRecorder := performJSONRequest(t, router, http.MethodPost, "/api/v1/groups", createBody)
	assertStatus(t, createRecorder, http.StatusCreated)
	var createResponse struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &createResponse); err != nil {
		t.Fatalf("json.Unmarshal(group create response) error = %v", err)
	}
	if createResponse.Status != "created" {
		t.Fatalf("group create response status = %q, want %q", createResponse.Status, "created")
	}
	groupID := createResponse.ID

	// Get
	getRecorder := httptest.NewRecorder()
	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/groups/"+groupID, nil)
	getRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(getRecorder, getRequest)
	assertStatus(t, getRecorder, http.StatusOK)
	var getResponse struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
	}
	if err := json.Unmarshal(getRecorder.Body.Bytes(), &getResponse); err != nil {
		t.Fatalf("json.Unmarshal(group get response) error = %v", err)
	}
	if getResponse.Name != "engineering" {
		t.Fatalf("group get response name = %q, want %q", getResponse.Name, "engineering")
	}

	// List
	listRecorder := performRawJSONRequest(router, http.MethodGet, "/api/v1/groups?page=1&pageSize=10&organizationId=org-1", nil)
	assertStatus(t, listRecorder, http.StatusOK)
	var listResponse struct {
		Items []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
		Page struct {
			Total int64 `json:"total"`
		} `json:"page"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("json.Unmarshal(group list response) error = %v", err)
	}
	if listResponse.Page.Total < 1 {
		t.Fatalf("group list total = %d, want >= 1", listResponse.Page.Total)
	}

	// Patch
	patchBody := map[string]any{
		"displayName": "Engineering Team",
		"revision":    1,
	}
	patchRecorder := performJSONRequest(t, router, http.MethodPatch, "/api/v1/groups/"+groupID, patchBody)
	assertStatus(t, patchRecorder, http.StatusOK)
	var patchResponse struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(patchRecorder.Body.Bytes(), &patchResponse); err != nil {
		t.Fatalf("json.Unmarshal(group patch response) error = %v", err)
	}
	if patchResponse.Status != "updated" {
		t.Fatalf("group patch response status = %q, want %q", patchResponse.Status, "updated")
	}

	// Delete
	deleteRecorder := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/groups/"+groupID, nil)
	deleteRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
	router.ServeHTTP(deleteRecorder, deleteRequest)
	assertStatus(t, deleteRecorder, http.StatusOK)
	var deleteResponse struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(deleteRecorder.Body.Bytes(), &deleteResponse); err != nil {
		t.Fatalf("json.Unmarshal(group delete response) error = %v", err)
	}
	if deleteResponse.Status != "deleted" {
		t.Fatalf("group delete response status = %q, want %q", deleteResponse.Status, "deleted")
	}
}
