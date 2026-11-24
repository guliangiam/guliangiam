package syncer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// OktaSyncer syncs users and groups from Okta.
type OktaSyncer struct{}

// oktaTokenResponse represents the response from Okta OAuth2 token endpoint.
type oktaTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

// oktaUserResponse represents a single Okta user from the API.
type oktaUserResponse struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Created   string `json:"created"`
	Activated string `json:"activated"`
	Profile   struct {
		Login     string `json:"login"`
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Email     string `json:"email"`
	} `json:"profile"`
}

// oktaGroupResponse represents a single Okta group from the API.
type oktaGroupResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Profile struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"profile"`
}

// getOktaAccessToken obtains an access token using client credentials flow.
func (s *OktaSyncer) getOktaAccessToken(ctx context.Context, config *SyncerConfig) (string, error) {
	// Okta supports SSWS (API token) or OAuth2 client credentials.
	// If TokenURL is provided, use OAuth2; otherwise use SSWS token directly.
	if config.TokenURL == "" && config.ClientSecret != "" {
		// Use SSWS token: ClientSecret holds the API token
		return config.ClientSecret, nil
	}

	tokenURL := config.TokenURL
	if tokenURL == "" {
		return "", apperrors.NewInvalidArgument("okta syncer: either TokenURL or API token (ClientSecret) must be provided")
	}

	u, _ := url.Parse(tokenURL)
	q := u.Query()
	q.Set("grant_type", "client_credentials")
	q.Set("scope", "okta.users.read okta.groups.read")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), http.NoBody)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "okta syncer create token request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(config.ClientID, config.ClientSecret)

	client := ssrf.SafeHTTPClient(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "okta syncer get token")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "okta syncer read token response")
	}

	var tokenResp oktaTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", apperrors.WrapInternalf(err, "okta syncer parse token response")
	}

	if tokenResp.AccessToken == "" {
		return "", apperrors.NewNotFound("okta syncer token error: empty access token received")
	}

	return tokenResp.AccessToken, nil
}

// oktaBaseURL returns the Okta API base URL from config.
func (s *OktaSyncer) oktaBaseURL(config *SyncerConfig) string {
	if config.APIURL != "" {
		return config.APIURL
	}
	// Construct from Host: https://{domain}
	if config.Host != "" {
		scheme := urlSchemeHTTPS
		if len(config.Host) > 8 && config.Host[:8] == urlSchemeHTTPS {
			scheme = ""
		}
		return scheme + config.Host
	}
	return ""
}

// SyncUsers synchronizes users from Okta.
func (s *OktaSyncer) SyncUsers(ctx context.Context, config *SyncerConfig) (*SyncResult, error) {
	token, err := s.getOktaAccessToken(ctx, config)
	if err != nil {
		return nil, err
	}

	baseURL := s.oktaBaseURL(config)
	if baseURL == "" {
		return nil, apperrors.NewInvalidArgument("okta syncer: API URL or Host must be configured")
	}

	result := &SyncResult{}

	users, errs := s.listUsers(ctx, token, baseURL, config)
	result.TotalUsers = len(users)
	result.Errors = append(result.Errors, errs...)

	// Map users and persist via write-back callback
	isIncremental := config.LastSyncAt != nil
	for i := range users {
		u := &users[i]
		if u.Profile.Login == "" {
			result.FailedUsers++
			result.Errors = append(result.Errors, fmt.Sprintf("okta user missing profile.login: id=%s", u.ID))
			continue
		}

		mapped := s.mapUser(u, config.AttributeMap)

		if config.OnUserSynced != nil {
			created, err := config.OnUserSynced(ctx, "okta", u.ID, mapped)
			if err != nil {
				result.FailedUsers++
				result.Errors = append(result.Errors, fmt.Sprintf("okta user write-back failed: id=%s, err=%v", u.ID, err))
				continue
			}
			if created {
				result.AddedUsers++
			} else {
				result.UpdatedUsers++
			}
		} else {
			// Read-only mode: count without persisting
			if isIncremental {
				result.UpdatedUsers++
			} else {
				result.AddedUsers++
			}
		}
	}

	return result, nil
}

// SyncGroups synchronizes groups from Okta.
func (s *OktaSyncer) SyncGroups(ctx context.Context, config *SyncerConfig) (*SyncResult, error) {
	token, err := s.getOktaAccessToken(ctx, config)
	if err != nil {
		return nil, err
	}

	baseURL := s.oktaBaseURL(config)
	if baseURL == "" {
		return nil, apperrors.NewInvalidArgument("okta syncer: API URL or Host must be configured")
	}

	result := &SyncResult{}

	groups, errs := s.listGroups(ctx, token, baseURL, config)
	result.TotalGroups = len(groups)
	result.Errors = append(result.Errors, errs...)

	// Map groups and persist via write-back callback
	isIncremental := config.LastSyncAt != nil
	for _, g := range groups {
		if g.Profile.Name == "" {
			result.FailedGroups++
			result.Errors = append(result.Errors, fmt.Sprintf("okta group missing profile.name: id=%s", g.ID))
			continue
		}

		mapped := s.mapGroup(g, config.AttributeMap)

		if config.OnGroupSynced != nil {
			created, err := config.OnGroupSynced(ctx, config.OrganizationID, g.ID, mapped)
			if err != nil {
				result.FailedGroups++
				result.Errors = append(result.Errors, fmt.Sprintf("okta group write-back failed: id=%s, err=%v", g.ID, err))
				continue
			}
			if created {
				result.AddedGroups++
			} else {
				result.UpdatedGroups++
			}
		} else {
			// Read-only mode: count without persisting
			if isIncremental {
				result.UpdatedGroups++
			} else {
				result.AddedGroups++
			}
		}
	}

	return result, nil
}

// TestConnection tests the connection to Okta by listing users with a limit of 1.
func (s *OktaSyncer) TestConnection(ctx context.Context, config *SyncerConfig) error {
	token, err := s.getOktaAccessToken(ctx, config)
	if err != nil {
		return apperrors.WrapInternalf(err, "okta test connection failed")
	}

	baseURL := s.oktaBaseURL(config)
	if baseURL == "" {
		return apperrors.NewInvalidArgument("okta test connection: API URL or Host must be configured")
	}

	u, _ := url.Parse(baseURL + "/api/v1/users")
	q := u.Query()
	q.Set("limit", "1")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return apperrors.WrapInternalf(err, "okta test connection create request")
	}
	req.Header.Set("Authorization", "SSWS "+token)
	req.Header.Set("Content-Type", "application/json")

	client := ssrf.SafeHTTPClient(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "okta test connection request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternalf("okta test connection HTTP error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// listUsers lists all users from Okta using pagination.
func (s *OktaSyncer) listUsers(ctx context.Context, token, baseURL string, config *SyncerConfig) (users []oktaUserResponse, errors []string) {
	u, _ := url.Parse(baseURL + "/api/v1/users")
	q := u.Query()
	q.Set("limit", "200")
	if config.Filter != "" {
		q.Set("search", config.Filter) // Okta filter expression, e.g., "status eq \"ACTIVE\""
	}
	// Add incremental sync search filter when LastSyncAt is set
	if config.LastSyncAt != nil {
		incrementalSearch := fmt.Sprintf("lastUpdated gt %q", config.LastSyncAt.Format(time.RFC3339))
		existingSearch := q.Get("search")
		if existingSearch != "" {
			q.Set("search", existingSearch+" and "+incrementalSearch)
		} else {
			q.Set("search", incrementalSearch)
		}
	}
	u.RawQuery = q.Encode()
	return fetchOktaPages[oktaUserResponse](ctx, token, u.String(), "user")
}

// listGroups lists all groups from Okta using pagination.
func (s *OktaSyncer) listGroups(ctx context.Context, token, baseURL string, config *SyncerConfig) (groups []oktaGroupResponse, errors []string) {
	u, _ := url.Parse(baseURL + "/api/v1/groups")
	q := u.Query()
	q.Set("limit", "200")
	if config.Filter != "" {
		q.Set("search", config.Filter) // e.g., "type eq \"OKTA_GROUP\""
	}
	// Add incremental sync search filter when LastSyncAt is set
	if config.LastSyncAt != nil {
		incrementalSearch := fmt.Sprintf("lastUpdated gt %q", config.LastSyncAt.Format(time.RFC3339))
		existingSearch := q.Get("search")
		if existingSearch != "" {
			q.Set("search", existingSearch+" and "+incrementalSearch)
		} else {
			q.Set("search", incrementalSearch)
		}
	}
	u.RawQuery = q.Encode()
	return fetchOktaPages[oktaGroupResponse](ctx, token, u.String(), "group")
}

// fetchOktaPages follows Okta's Link-header pagination and returns the
// accumulated values plus any per-page errors.
func fetchOktaPages[T any](ctx context.Context, token, nextURL, kind string) (items []T, errors []string) {
	for nextURL != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, nextURL, http.NoBody)
		if err != nil {
			errors = append(errors, fmt.Sprintf("okta create %s list request: %v", kind, err))
			break
		}
		req.Header.Set("Authorization", "SSWS "+token)
		req.Header.Set("Content-Type", "application/json")

		client := ssrf.SafeHTTPClient(30 * time.Second)
		resp, err := client.Do(req)
		if err != nil {
			errors = append(errors, fmt.Sprintf("okta %s list request: %v", kind, err))
			break
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			errors = append(errors, fmt.Sprintf("okta read %s list response: %v", kind, err))
			break
		}

		if resp.StatusCode != http.StatusOK {
			errors = append(errors, fmt.Sprintf("okta %s list HTTP error: status=%d, body=%s", kind, resp.StatusCode, string(body)))
			break
		}

		var page []T
		if err := json.Unmarshal(body, &page); err != nil {
			errors = append(errors, fmt.Sprintf("okta parse %s list response: %v", kind, err))
			break
		}

		items = append(items, page...)

		// Okta uses Link header for pagination
		nextURL = parseOktaNextLink(resp.Header.Get("Link"))
	}

	return items, errors
}

// parseOktaNextLink extracts the "next" URL from the Okta Link header.
// Okta Link header format: <https://domain/api/v1/users?after=xxx>; rel="next"
func parseOktaNextLink(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}

	// Simple parser for Link header
	const relNext = `rel="next"`
	start := 0
	for start < len(linkHeader) {
		// Find the start of a URL (after <)
		lt := indexByte(linkHeader[start:], '<')
		if lt == -1 {
			break
		}
		gt := indexByte(linkHeader[start+lt:], '>')
		if gt == -1 {
			break
		}

		urlPart := linkHeader[start+lt+1 : start+lt+gt]
		rest := linkHeader[start+lt+gt+1:]

		// Check if this segment has rel="next"
		if containsStr(rest, relNext) {
			return urlPart
		}

		start += lt + gt + 1
	}

	return ""
}

// indexByte finds the first occurrence of byte c in s, returning -1 if not found.
func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// containsStr checks if substr is in s.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// mapUser maps an Okta user to a field map using the configured attribute mapping.
// Default mapping (when AttributeMap is empty):
//   - profile.login -> username, profile.firstName + profile.lastName -> displayName, profile.email -> email
func (s *OktaSyncer) mapUser(item *oktaUserResponse, attributeMap map[string]string) map[string]interface{} {
	result := make(map[string]interface{})

	// Compute display name from firstName + lastName
	displayName := item.Profile.FirstName
	if displayName != "" && item.Profile.LastName != "" {
		displayName += " "
	}
	displayName += item.Profile.LastName

	if len(attributeMap) == 0 {
		// Default Okta -> GuliangIAM field mapping
		result["id"] = item.ID
		result["username"] = item.Profile.Login
		result["displayName"] = displayName
		result["email"] = item.Profile.Email
		result["status"] = item.Status
		return result
	}

	// Apply custom mapping: AttributeMap key is the Okta source field name
	if target, ok := attributeMap["id"]; ok {
		result[target] = item.ID
	}
	if target, ok := attributeMap["status"]; ok {
		result[target] = item.Status
	}
	if target, ok := attributeMap["profile.login"]; ok {
		result[target] = item.Profile.Login
	}
	if target, ok := attributeMap["profile.displayName"]; ok {
		result[target] = displayName
	}
	if target, ok := attributeMap["profile.firstName"]; ok {
		result[target] = item.Profile.FirstName
	}
	if target, ok := attributeMap["profile.lastName"]; ok {
		result[target] = item.Profile.LastName
	}
	if target, ok := attributeMap["profile.email"]; ok {
		result[target] = item.Profile.Email
	}

	return result
}

// mapGroup maps an Okta group to a field map using the configured attribute mapping.
// Default mapping (when AttributeMap is empty):
//   - profile.name -> name, profile.description -> description
func (s *OktaSyncer) mapGroup(item oktaGroupResponse, attributeMap map[string]string) map[string]interface{} {
	result := make(map[string]interface{})

	if len(attributeMap) == 0 {
		result["id"] = item.ID
		result["type"] = item.Type
		result["name"] = item.Profile.Name
		result["description"] = item.Profile.Description
		return result
	}

	if target, ok := attributeMap["id"]; ok {
		result[target] = item.ID
	}
	if target, ok := attributeMap["type"]; ok {
		result[target] = item.Type
	}
	if target, ok := attributeMap["profile.name"]; ok {
		result[target] = item.Profile.Name
	}
	if target, ok := attributeMap["profile.description"]; ok {
		result[target] = item.Profile.Description
	}

	return result
}

// Ensure OktaSyncer implements Engine.
var _ Engine = (*OktaSyncer)(nil)
