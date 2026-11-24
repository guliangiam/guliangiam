package syncer

import (
	"bytes"
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

// KeycloakSyncer syncs users and groups from Keycloak.
type KeycloakSyncer struct{}

// keycloakTokenResponse represents the response from Keycloak token endpoint.
type keycloakTokenResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	TokenType        string `json:"token_type"`
	Scope            string `json:"scope"`
}

// keycloakUserResponse represents a single Keycloak user from the admin API.
type keycloakUserResponse struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Enabled       bool   `json:"enabled"`
	Email         string `json:"email"`
	FirstName     string `json:"firstName"`
	LastName      string `json:"lastName"`
	EmailVerified bool   `json:"emailVerified"`
}

// keycloakGroupResponse represents a single Keycloak group from the admin API.
type keycloakGroupResponse struct {
	ID        string                  `json:"id"`
	Name      string                  `json:"name"`
	Path      string                  `json:"path"`
	SubGroups []keycloakGroupResponse `json:"subGroups"`
}

// getKeycloakAccessToken obtains an access token using client credentials from Keycloak.
func (s *KeycloakSyncer) getKeycloakAccessToken(ctx context.Context, config *SyncerConfig) (string, error) {
	// Build token URL: {host}/realms/{realm}/protocol/openid-connect/token
	// config.Host = Keycloak base URL (e.g., https://keycloak.example.com)
	// config.BaseDN = realm name (e.g., master)
	realm := config.BaseDN
	if realm == "" {
		realm = "master"
	}

	tokenURL := config.TokenURL
	if tokenURL == "" {
		tokenURL = fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", config.Host, realm)
	}

	formData := url.Values{}
	formData.Set("client_id", config.ClientID)
	formData.Set("client_secret", config.ClientSecret)
	formData.Set("grant_type", "client_credentials")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewReader([]byte(formData.Encode())))
	if err != nil {
		return "", apperrors.WrapInternalf(err, "keycloak syncer create token request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := ssrf.SafeHTTPClient(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "keycloak syncer get token")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "keycloak syncer read token response")
	}

	var tokenResp keycloakTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", apperrors.WrapInternalf(err, "keycloak syncer parse token response")
	}

	if tokenResp.AccessToken == "" {
		return "", apperrors.NewNotFound("keycloak syncer token error: empty access token received")
	}

	return tokenResp.AccessToken, nil
}

// keycloakAdminBaseURL builds the Keycloak admin REST API base URL.
func (s *KeycloakSyncer) keycloakAdminBaseURL(config *SyncerConfig) string {
	if config.APIURL != "" {
		return config.APIURL
	}

	realm := config.BaseDN
	if realm == "" {
		realm = "master"
	}

	return fmt.Sprintf("%s/admin/realms/%s", config.Host, realm)
}

// SyncUsers synchronizes users from Keycloak.
func (s *KeycloakSyncer) SyncUsers(ctx context.Context, config *SyncerConfig) (*SyncResult, error) {
	token, err := s.getKeycloakAccessToken(ctx, config)
	if err != nil {
		return nil, err
	}

	adminURL := s.keycloakAdminBaseURL(config)

	result := &SyncResult{}

	users, errs := s.listUsers(ctx, token, adminURL, config)
	result.TotalUsers = len(users)
	result.Errors = append(result.Errors, errs...)

	// Map users and persist via write-back callback
	isIncremental := config.LastSyncAt != nil
	for _, u := range users {
		if u.Username == "" {
			result.FailedUsers++
			result.Errors = append(result.Errors, fmt.Sprintf("keycloak user missing username: id=%s", u.ID))
			continue
		}

		mapped := s.mapUser(u, config.AttributeMap)

		if config.OnUserSynced != nil {
			created, err := config.OnUserSynced(ctx, "keycloak", u.ID, mapped)
			if err != nil {
				result.FailedUsers++
				result.Errors = append(result.Errors, fmt.Sprintf("keycloak user write-back failed: id=%s, err=%v", u.ID, err))
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

// SyncGroups synchronizes groups from Keycloak.
func (s *KeycloakSyncer) SyncGroups(ctx context.Context, config *SyncerConfig) (*SyncResult, error) {
	token, err := s.getKeycloakAccessToken(ctx, config)
	if err != nil {
		return nil, err
	}

	adminURL := s.keycloakAdminBaseURL(config)

	result := &SyncResult{}

	groups, errs := s.listGroups(ctx, token, adminURL, config)
	result.TotalGroups = len(groups)
	result.Errors = append(result.Errors, errs...)

	// Map groups and persist via write-back callback
	isIncremental := config.LastSyncAt != nil
	for _, g := range groups {
		if g.Name == "" {
			result.FailedGroups++
			result.Errors = append(result.Errors, fmt.Sprintf("keycloak group missing name: id=%s", g.ID))
			continue
		}

		mapped := s.mapGroup(g, config.AttributeMap)

		if config.OnGroupSynced != nil {
			created, err := config.OnGroupSynced(ctx, config.OrganizationID, g.ID, mapped)
			if err != nil {
				result.FailedGroups++
				result.Errors = append(result.Errors, fmt.Sprintf("keycloak group write-back failed: id=%s, err=%v", g.ID, err))
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

// TestConnection tests the connection to Keycloak by obtaining an access token.
func (s *KeycloakSyncer) TestConnection(ctx context.Context, config *SyncerConfig) error {
	_, err := s.getKeycloakAccessToken(ctx, config)
	if err != nil {
		return apperrors.WrapInternalf(err, "keycloak test connection failed")
	}
	return nil
}

// listUsers lists all users from Keycloak Admin REST API.
func (s *KeycloakSyncer) listUsers(ctx context.Context, token, adminURL string, config *SyncerConfig) (users []keycloakUserResponse, errors []string) {
	var allUsers []keycloakUserResponse

	u, _ := url.Parse(adminURL + "/users")
	q := u.Query()
	q.Set("max", "100")
	if config.Filter != "" {
		q.Set("search", config.Filter) // Keycloak search filter
	}
	// Note: Keycloak Admin REST API does not support a direct timestamp filter
	// for incremental sync. Full sync is performed regardless of LastSyncAt setting.
	u.RawQuery = q.Encode()

	first := 0
	nextURL := u.String()

	for nextURL != "" {
		// Update the "first" parameter for pagination
		parsedURL, _ := url.Parse(nextURL)
		q := parsedURL.Query()
		q.Set("first", fmt.Sprintf("%d", first))
		parsedURL.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), http.NoBody)
		if err != nil {
			errors = append(errors, fmt.Sprintf("keycloak create user list request: %v", err))
			break
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		client := ssrf.SafeHTTPClient(30 * time.Second)
		resp, err := client.Do(req)
		if err != nil {
			errors = append(errors, fmt.Sprintf("keycloak user list request: %v", err))
			break
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			errors = append(errors, fmt.Sprintf("keycloak read user list response: %v", err))
			break
		}

		if resp.StatusCode != http.StatusOK {
			errors = append(errors, fmt.Sprintf("keycloak user list HTTP error: status=%d, body=%s", resp.StatusCode, string(body)))
			break
		}

		var users []keycloakUserResponse
		if err := json.Unmarshal(body, &users); err != nil {
			errors = append(errors, fmt.Sprintf("keycloak parse user list response: %v", err))
			break
		}

		if len(users) == 0 {
			break
		}

		allUsers = append(allUsers, users...)
		first += len(users)

		// If fewer than max results returned, we've reached the end
		if len(users) < 100 {
			break
		}
	}

	return allUsers, errors
}

// listGroups lists all groups from Keycloak Admin REST API.
func (s *KeycloakSyncer) listGroups(ctx context.Context, token, adminURL string, config *SyncerConfig) (groups []keycloakGroupResponse, errors []string) {
	var allGroups []keycloakGroupResponse

	u, _ := url.Parse(adminURL + "/groups")
	q := u.Query()
	q.Set("max", "100")
	if config.Filter != "" {
		q.Set("search", config.Filter)
	}
	// Note: Keycloak Admin REST API does not support a direct timestamp filter
	// for incremental sync. Full sync is performed regardless of LastSyncAt setting.
	u.RawQuery = q.Encode()

	first := 0
	nextURL := u.String()

	for nextURL != "" {
		parsedURL, _ := url.Parse(nextURL)
		q := parsedURL.Query()
		q.Set("first", fmt.Sprintf("%d", first))
		parsedURL.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), http.NoBody)
		if err != nil {
			errors = append(errors, fmt.Sprintf("keycloak create group list request: %v", err))
			break
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		client := ssrf.SafeHTTPClient(30 * time.Second)
		resp, err := client.Do(req)
		if err != nil {
			errors = append(errors, fmt.Sprintf("keycloak group list request: %v", err))
			break
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			errors = append(errors, fmt.Sprintf("keycloak read group list response: %v", err))
			break
		}

		if resp.StatusCode != http.StatusOK {
			errors = append(errors, fmt.Sprintf("keycloak group list HTTP error: status=%d, body=%s", resp.StatusCode, string(body)))
			break
		}

		var groups []keycloakGroupResponse
		if err := json.Unmarshal(body, &groups); err != nil {
			errors = append(errors, fmt.Sprintf("keycloak parse group list response: %v", err))
			break
		}

		if len(groups) == 0 {
			break
		}

		// Flatten sub-groups
		flattened := flattenKeycloakGroups(groups)
		allGroups = append(allGroups, flattened...)

		first += len(groups)
		if len(groups) < 100 {
			break
		}
	}

	return allGroups, errors
}

// flattenKeycloakGroups recursively flattens Keycloak groups with sub-groups.
func flattenKeycloakGroups(groups []keycloakGroupResponse) []keycloakGroupResponse {
	var result []keycloakGroupResponse
	for _, g := range groups {
		result = append(result, g)
		if len(g.SubGroups) > 0 {
			result = append(result, flattenKeycloakGroups(g.SubGroups)...)
		}
	}
	return result
}

// mapUser maps a Keycloak user to a field map using the configured attribute mapping.
// Default mapping (when AttributeMap is empty):
//   - username -> username, firstName + lastName -> displayName, email -> email, enabled -> enabled
func (s *KeycloakSyncer) mapUser(item keycloakUserResponse, attributeMap map[string]string) map[string]interface{} {
	result := make(map[string]interface{})

	// Compute display name from firstName + lastName
	displayName := item.FirstName
	if displayName != "" && item.LastName != "" {
		displayName += " "
	}
	displayName += item.LastName

	if len(attributeMap) == 0 {
		// Default Keycloak -> GuliangIAM field mapping
		result["id"] = item.ID
		result["username"] = item.Username
		result["displayName"] = displayName
		result["email"] = item.Email
		result["enabled"] = item.Enabled
		result["emailVerified"] = item.EmailVerified
		return result
	}

	// Apply custom mapping: AttributeMap key is the Keycloak source field name
	if target, ok := attributeMap["id"]; ok {
		result[target] = item.ID
	}
	if target, ok := attributeMap["username"]; ok {
		result[target] = item.Username
	}
	if target, ok := attributeMap["displayName"]; ok {
		result[target] = displayName
	}
	if target, ok := attributeMap["firstName"]; ok {
		result[target] = item.FirstName
	}
	if target, ok := attributeMap["lastName"]; ok {
		result[target] = item.LastName
	}
	if target, ok := attributeMap["email"]; ok {
		result[target] = item.Email
	}
	if target, ok := attributeMap["enabled"]; ok {
		result[target] = item.Enabled
	}
	if target, ok := attributeMap["emailVerified"]; ok {
		result[target] = item.EmailVerified
	}

	return result
}

// mapGroup maps a Keycloak group to a field map using the configured attribute mapping.
// Default mapping (when AttributeMap is empty):
//   - name -> name, path -> path
func (s *KeycloakSyncer) mapGroup(item keycloakGroupResponse, attributeMap map[string]string) map[string]interface{} {
	result := make(map[string]interface{})

	if len(attributeMap) == 0 {
		result["id"] = item.ID
		result["name"] = item.Name
		result["path"] = item.Path
		return result
	}

	if target, ok := attributeMap["id"]; ok {
		result[target] = item.ID
	}
	if target, ok := attributeMap["name"]; ok {
		result[target] = item.Name
	}
	if target, ok := attributeMap["path"]; ok {
		result[target] = item.Path
	}

	return result
}

// Ensure KeycloakSyncer implements Engine.
var _ Engine = (*KeycloakSyncer)(nil)
