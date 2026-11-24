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

func (s *SCIMSyncer) CreateGroup(ctx context.Context, config *SyncerConfig, group *scimGroup) (*scimGroup, error) {
	baseURL := s.resolveBaseURL(config)
	group.Schemas = []string{"urn:ietf:params:scim:schemas:core:2.0:Group"}

	body, err := json.Marshal(group)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "scim marshal group")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/Groups", bytes.NewReader(body))
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "scim create group request")
	}
	s.setAuthHeaders(req, config)
	req.Header.Set("Content-Type", "application/scim+json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "scim create group")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, apperrors.NewInternalf("scim create group HTTP error: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	var created scimGroup
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return nil, apperrors.WrapInternalf(err, "scim decode created group")
	}

	return &created, nil
}

// ReplaceGroup replaces a group via SCIM PUT /Groups/{id}.
func (s *SCIMSyncer) ReplaceGroup(ctx context.Context, config *SyncerConfig, group *scimGroup) (*scimGroup, error) {
	baseURL := s.resolveBaseURL(config)
	group.Schemas = []string{"urn:ietf:params:scim:schemas:core:2.0:Group"}

	body, err := json.Marshal(group)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "scim marshal group")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, baseURL+"/Groups/"+url.PathEscape(group.ID), bytes.NewReader(body))
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "scim replace group request")
	}
	s.setAuthHeaders(req, config)
	req.Header.Set("Content-Type", "application/scim+json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "scim replace group")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, apperrors.NewInternalf("scim replace group HTTP error: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	var updated scimGroup
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		return nil, apperrors.WrapInternalf(err, "scim decode updated group")
	}

	return &updated, nil
}

// PatchGroup patches a group via SCIM PATCH /Groups/{id}.
func (s *SCIMSyncer) PatchGroup(ctx context.Context, config *SyncerConfig, groupID string, operations []scimPatchOp) error {
	baseURL := s.resolveBaseURL(config)

	patchReq := scimPatchRequest{
		Schemas:    []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		Operations: operations,
	}

	body, err := json.Marshal(patchReq)
	if err != nil {
		return apperrors.WrapInternalf(err, "scim marshal group patch")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, baseURL+"/Groups/"+url.PathEscape(groupID), bytes.NewReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "scim patch group request")
	}
	s.setAuthHeaders(req, config)
	req.Header.Set("Content-Type", "application/scim+json")

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "scim patch group")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternalf("scim patch group HTTP error: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	return nil
}

// --- SCIM Filter Parser ---

// ParseSCIMFilter parses a SCIM filter string into a simple filter structure.
// Supports basic SCIM filter syntax:
//   - userName eq "john"
//   - displayName co "admin"
//   - active eq true
//   - emails.value eq "john@example.com"
//   - userName eq "john" and active eq true
