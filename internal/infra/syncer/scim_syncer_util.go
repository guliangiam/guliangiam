package syncer

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s *SCIMSyncer) resolveBaseURL(config *SyncerConfig) string {
	if config.APIURL != "" {
		return strings.TrimRight(config.APIURL, "/")
	}
	scheme := urlSchemeHTTPS
	if len(config.Host) > 8 && config.Host[:8] == urlSchemeHTTPS {
		scheme = ""
	}
	return scheme + strings.TrimRight(config.Host, "/")
}

func (s *SCIMSyncer) setAuthHeaders(req *http.Request, config *SyncerConfig) {
	if config.ClientSecret != "" {
		// Bearer token authentication
		req.Header.Set("Authorization", "Bearer "+config.ClientSecret)
	} else if config.ClientID != "" && config.Password != "" {
		// Basic authentication
		req.SetBasicAuth(config.ClientID, config.Password)
	}
}

func (s *SCIMSyncer) listSCIMUsers(ctx context.Context, baseURL string, config *SyncerConfig) ([]scimUser, error) {
	u, _ := url.Parse(baseURL + "/Users")
	q := u.Query()
	q.Set("count", "100")
	if config.Filter != "" {
		q.Set("filter", config.Filter)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return nil, err
	}
	s.setAuthHeaders(req, config)
	req.Header.Set("Content-Type", "application/scim+json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "scim list users")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperrors.NewInternalf("scim list users HTTP error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var listResp scimListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, apperrors.WrapInternalf(err, "scim decode users list")
	}

	var users []scimUser
	if err := json.Unmarshal(listResp.Resources, &users); err != nil {
		return nil, apperrors.WrapInternalf(err, "scim decode user resources")
	}

	return users, nil
}

func (s *SCIMSyncer) listSCIMGroups(ctx context.Context, baseURL string, config *SyncerConfig) ([]scimGroup, error) {
	u, _ := url.Parse(baseURL + "/Groups")
	q := u.Query()
	q.Set("count", "100")
	if config.Filter != "" {
		q.Set("filter", config.Filter)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return nil, err
	}
	s.setAuthHeaders(req, config)
	req.Header.Set("Content-Type", "application/scim+json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "scim list groups")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, apperrors.NewInternalf("scim list groups HTTP error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var listResp scimListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, apperrors.WrapInternalf(err, "scim decode groups list")
	}

	var groups []scimGroup
	if err := json.Unmarshal(listResp.Resources, &groups); err != nil {
		return nil, apperrors.WrapInternalf(err, "scim decode group resources")
	}

	return groups, nil
}

// Ensure SCIMSyncer implements Engine.
var _ Engine = (*SCIMSyncer)(nil)
