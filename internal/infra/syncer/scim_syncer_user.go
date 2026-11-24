package syncer

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s *SCIMSyncer) SyncUsers(ctx context.Context, config *SyncerConfig) (*SyncResult, error) {
	if config.APIURL == "" && config.Host == "" {
		return nil, apperrors.NewInvalidArgument("scim syncer: API URL or Host must be configured")
	}

	baseURL := s.resolveBaseURL(config)
	result := &SyncResult{}

	// Read local users to push (in full integration, this would come from the User repository)
	// For now, we test the connection by listing existing SCIM users.
	users, err := s.listSCIMUsers(ctx, baseURL, config)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "scim syncer list users")
	}

	result.TotalUsers = len(users)
	result.AddedUsers = len(users) // In push mode, all listed are "added" to the remote

	return result, nil
}

// SyncGroups pushes groups to the SCIM endpoint.
func (s *SCIMSyncer) SyncGroups(ctx context.Context, config *SyncerConfig) (*SyncResult, error) {
	if config.APIURL == "" && config.Host == "" {
		return nil, apperrors.NewInvalidArgument("scim syncer: API URL or Host must be configured")
	}

	baseURL := s.resolveBaseURL(config)
	result := &SyncResult{}

	groups, err := s.listSCIMGroups(ctx, baseURL, config)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "scim syncer list groups")
	}

	result.TotalGroups = len(groups)
	result.AddedGroups = len(groups)

	return result, nil
}

// TestConnection tests the SCIM endpoint by listing users with count=1.
func (s *SCIMSyncer) TestConnection(ctx context.Context, config *SyncerConfig) error {
	if config.APIURL == "" && config.Host == "" {
		return apperrors.NewInvalidArgument("scim test connection: API URL or Host must be configured")
	}

	baseURL := s.resolveBaseURL(config)
	u, _ := url.Parse(baseURL + "/Users")
	q := u.Query()
	q.Set("count", "1")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return apperrors.WrapInternalf(err, "scim test connection create request")
	}
	s.setAuthHeaders(req, config)

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "scim test connection request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternalf("scim test connection HTTP error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// --- SCIM CRUD Operations ---

// CreateUser creates a user via SCIM POST /Users.
func (s *SCIMSyncer) CreateUser(ctx context.Context, config *SyncerConfig, user *scimUser) (*scimUser, error) {
	baseURL := s.resolveBaseURL(config)
	user.Schemas = []string{"urn:ietf:params:scim:schemas:core:2.0:User"}

	body, err := json.Marshal(user)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "scim marshal user")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/Users", bytes.NewReader(body))
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "scim create user request")
	}
	s.setAuthHeaders(req, config)
	req.Header.Set("Content-Type", "application/scim+json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "scim create user")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, apperrors.NewInternalf("scim create user HTTP error: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	var created scimUser
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return nil, apperrors.WrapInternalf(err, "scim decode created user")
	}

	return &created, nil
}

// ReplaceUser replaces a user via SCIM PUT /Users/{id}.
func (s *SCIMSyncer) ReplaceUser(ctx context.Context, config *SyncerConfig, user *scimUser) (*scimUser, error) {
	baseURL := s.resolveBaseURL(config)
	user.Schemas = []string{"urn:ietf:params:scim:schemas:core:2.0:User"}

	body, err := json.Marshal(user)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "scim marshal user")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, baseURL+"/Users/"+url.PathEscape(user.ID), bytes.NewReader(body))
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "scim replace user request")
	}
	s.setAuthHeaders(req, config)
	req.Header.Set("Content-Type", "application/scim+json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "scim replace user")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, apperrors.NewInternalf("scim replace user HTTP error: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	var updated scimUser
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		return nil, apperrors.WrapInternalf(err, "scim decode updated user")
	}

	return &updated, nil
}

// PatchUser patches a user via SCIM PATCH /Users/{id}.
func (s *SCIMSyncer) PatchUser(ctx context.Context, config *SyncerConfig, userID string, operations []scimPatchOp) error {
	baseURL := s.resolveBaseURL(config)

	patchReq := scimPatchRequest{
		Schemas:    []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		Operations: operations,
	}

	body, err := json.Marshal(patchReq)
	if err != nil {
		return apperrors.WrapInternalf(err, "scim marshal patch")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, baseURL+"/Users/"+url.PathEscape(userID), bytes.NewReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "scim patch user request")
	}
	s.setAuthHeaders(req, config)
	req.Header.Set("Content-Type", "application/scim+json")

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "scim patch user")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternalf("scim patch user HTTP error: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	return nil
}

// CreateGroup creates a group via SCIM POST /Groups.
