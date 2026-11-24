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

// GoogleSyncer syncs users and groups from Google Workspace.
type GoogleSyncer struct{}

// googleTokenResponse represents the response from Google OAuth2 token endpoint.
type googleTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

// googleTokenRequest is the request body for obtaining an access token.
type googleTokenRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// googleUsersResponse represents the response from Google Directory API users list.
type googleUsersResponse struct {
	Kind          string `json:"kind"`
	NextPageToken string `json:"nextPageToken"`
	Users         []struct {
		ID           string `json:"id"`
		PrimaryEmail string `json:"primaryEmail"`
		Name         struct {
			GivenName  string `json:"givenName"`
			FamilyName string `json:"familyName"`
			FullName   string `json:"fullName"`
		} `json:"name"`
		Suspended   bool   `json:"suspended"`
		OrgUnitPath string `json:"orgUnitPath"`
	} `json:"users"`
}

// googleGroupsResponse represents the response from Google Directory API groups list.
type googleGroupsResponse struct {
	Kind          string `json:"kind"`
	NextPageToken string `json:"nextPageToken"`
	Groups        []struct {
		ID          string `json:"id"`
		Email       string `json:"email"`
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"groups"`
}

// getGoogleAccessToken obtains an access token using OAuth2 client credentials.
// For Google Workspace, a service account or refresh token is typically required.
// This implementation supports the refresh_token grant type.
func (s *GoogleSyncer) getGoogleAccessToken(ctx context.Context, config *SyncerConfig) (string, error) {
	tokenURL := "https://oauth2.googleapis.com/token" //nolint:gosec // G101: endpoint URL, not a credential
	if config.TokenURL != "" {
		tokenURL = config.TokenURL
	}

	reqBody := googleTokenRequest{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		GrantType:    "refresh_token",
		RefreshToken: config.Password, // Use Password field to store refresh token
		Scope:        "https://www.googleapis.com/auth/admin.directory.user https://www.googleapis.com/auth/admin.directory.group",
	}
	bodyBytes, err := json.Marshal(reqBody) //nolint:gosec // G117: refresh_token grant body; client_secret is required and sent over TLS only
	if err != nil {
		return "", apperrors.WrapInternalf(err, "google syncer marshal token request")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", apperrors.WrapInternalf(err, "google syncer create token request")
	}
	req.Header.Set("Content-Type", "application/json")

	client := ssrf.SafeHTTPClient(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "google syncer get token")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "google syncer read token response")
	}

	var tokenResp googleTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", apperrors.WrapInternalf(err, "google syncer parse token response")
	}

	if tokenResp.AccessToken == "" {
		return "", apperrors.NewNotFound("google syncer token error: empty access token received")
	}

	return tokenResp.AccessToken, nil
}

// SyncUsers synchronizes users from Google Workspace.
func (s *GoogleSyncer) SyncUsers(ctx context.Context, config *SyncerConfig) (*SyncResult, error) {
	token, err := s.getGoogleAccessToken(ctx, config)
	if err != nil {
		return nil, err
	}

	result := &SyncResult{}

	users, errs := s.listUsers(ctx, token, config)
	result.TotalUsers = len(users)
	result.Errors = append(result.Errors, errs...)

	// Map users and persist via write-back callback
	isIncremental := config.LastSyncAt != nil
	for _, u := range users {
		if u.PrimaryEmail == "" {
			result.FailedUsers++
			result.Errors = append(result.Errors, fmt.Sprintf("google user missing primaryEmail: id=%s", u.ID))
			continue
		}

		mapped := s.mapUser(u, config.AttributeMap)

		if config.OnUserSynced != nil {
			created, err := config.OnUserSynced(ctx, "google", u.ID, mapped)
			if err != nil {
				result.FailedUsers++
				result.Errors = append(result.Errors, fmt.Sprintf("google user write-back failed: id=%s, err=%v", u.ID, err))
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

// SyncGroups synchronizes groups from Google Workspace.
func (s *GoogleSyncer) SyncGroups(ctx context.Context, config *SyncerConfig) (*SyncResult, error) {
	token, err := s.getGoogleAccessToken(ctx, config)
	if err != nil {
		return nil, err
	}

	result := &SyncResult{}

	groups, errs := s.listGroups(ctx, token, config)
	result.TotalGroups = len(groups)
	result.Errors = append(result.Errors, errs...)

	// Map groups and persist via write-back callback
	isIncremental := config.LastSyncAt != nil
	for _, g := range groups {
		if g.Email == "" {
			result.FailedGroups++
			result.Errors = append(result.Errors, fmt.Sprintf("google group missing email: id=%s", g.ID))
			continue
		}

		mapped := s.mapGroup(g, config.AttributeMap)

		if config.OnGroupSynced != nil {
			created, err := config.OnGroupSynced(ctx, config.OrganizationID, g.ID, mapped)
			if err != nil {
				result.FailedGroups++
				result.Errors = append(result.Errors, fmt.Sprintf("google group write-back failed: id=%s, err=%v", g.ID, err))
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

// TestConnection tests the connection to Google Workspace by obtaining an access token.
func (s *GoogleSyncer) TestConnection(ctx context.Context, config *SyncerConfig) error {
	_, err := s.getGoogleAccessToken(ctx, config)
	if err != nil {
		return apperrors.WrapInternalf(err, "google test connection failed")
	}
	return nil
}

type googleUserItem = struct {
	ID           string `json:"id"`
	PrimaryEmail string `json:"primaryEmail"`
	Name         struct {
		GivenName  string `json:"givenName"`
		FamilyName string `json:"familyName"`
		FullName   string `json:"fullName"`
	} `json:"name"`
	Suspended   bool   `json:"suspended"`
	OrgUnitPath string `json:"orgUnitPath"`
}

// listUsers lists all users from Google Workspace Directory API.
func (s *GoogleSyncer) listUsers(ctx context.Context, token string, config *SyncerConfig) (users []googleUserItem, errs []string) {
	baseURL := "https://admin.googleapis.com/admin/directory/v1/users"
	if config.APIURL != "" {
		baseURL = config.APIURL + "/admin/directory/v1/users"
	}
	return fetchGooglePages[googleUserItem](ctx, token, baseURL, "user", config,
		func(body []byte) ([]googleUserItem, string, error) {
			var userResp googleUsersResponse
			if err := json.Unmarshal(body, &userResp); err != nil {
				return nil, "", err
			}
			return userResp.Users, userResp.NextPageToken, nil
		})
}

type googleGroupItem = struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// listGroups lists all groups from Google Workspace Directory API.
func (s *GoogleSyncer) listGroups(ctx context.Context, token string, config *SyncerConfig) (groups []googleGroupItem, errs []string) {
	baseURL := "https://admin.googleapis.com/admin/directory/v1/groups"
	if config.APIURL != "" {
		baseURL = config.APIURL + "/admin/directory/v1/groups"
	}
	return fetchGooglePages[googleGroupItem](ctx, token, baseURL, "group", config,
		func(body []byte) ([]googleGroupItem, string, error) {
			var groupResp googleGroupsResponse
			if err := json.Unmarshal(body, &groupResp); err != nil {
				return nil, "", err
			}
			return groupResp.Groups, groupResp.NextPageToken, nil
		})
}

// fetchGooglePages follows Google Directory pageToken pagination and returns
// the accumulated items plus per-page errors.
func fetchGooglePages[T any](
	ctx context.Context,
	token, baseURL, kind string,
	config *SyncerConfig,
	decode func([]byte) ([]T, string, error),
) (items []T, errs []string) {
	pageToken := ""

	for {
		u, _ := url.Parse(baseURL)
		q := u.Query()
		q.Set("domain", config.Host) // Google Workspace domain
		q.Set("maxResults", "200")
		if config.Filter != "" {
			q.Set("query", config.Filter) // Google query string filter
		}
		// Add incremental sync query: filter by modifiedTime when LastSyncAt is set
		if config.LastSyncAt != nil {
			incrementalQuery := fmt.Sprintf("modifiedTime > '%s'", config.LastSyncAt.Format(time.RFC3339))
			existingQuery := q.Get("query")
			if existingQuery != "" {
				q.Set("query", existingQuery+" "+incrementalQuery)
			} else {
				q.Set("query", incrementalQuery)
			}
		}
		if pageToken != "" {
			q.Set("pageToken", pageToken)
		}
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
		if err != nil {
			errs = append(errs, fmt.Sprintf("google create %s list request: %v", kind, err))
			break
		}
		req.Header.Set("Authorization", "Bearer "+token)

		client := ssrf.SafeHTTPClient(30 * time.Second)
		resp, err := client.Do(req)
		if err != nil {
			errs = append(errs, fmt.Sprintf("google %s list request: %v", kind, err))
			break
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			errs = append(errs, fmt.Sprintf("google read %s list response: %v", kind, err))
			break
		}

		if resp.StatusCode != http.StatusOK {
			errs = append(errs, fmt.Sprintf("google %s list HTTP error: status=%d, body=%s", kind, resp.StatusCode, string(body)))
			break
		}

		pageItems, next, err := decode(body)
		if err != nil {
			errs = append(errs, fmt.Sprintf("google parse %s list response: %v", kind, err))
			break
		}
		items = append(items, pageItems...)
		if next == "" {
			break
		}
		pageToken = next
	}

	return items, errs
}

func (s *GoogleSyncer) mapUser(item googleUserItem, attributeMap map[string]string) map[string]interface{} {
	result := make(map[string]interface{})

	if len(attributeMap) == 0 {
		// Default Google -> GuliangIAM field mapping
		result["id"] = item.ID
		result["username"] = item.PrimaryEmail
		result["displayName"] = item.Name.FullName
		result[dirAttrGivenName] = item.Name.GivenName
		result["familyName"] = item.Name.FamilyName
		result["disabled"] = item.Suspended
		result["orgUnitPath"] = item.OrgUnitPath
		return result
	}

	// Apply custom mapping: AttributeMap key is the Google source field name
	if target, ok := attributeMap["id"]; ok {
		result[target] = item.ID
	}
	if target, ok := attributeMap["primaryEmail"]; ok {
		result[target] = item.PrimaryEmail
	}
	if target, ok := attributeMap["name.fullName"]; ok {
		result[target] = item.Name.FullName
	}
	if target, ok := attributeMap["name.givenName"]; ok {
		result[target] = item.Name.GivenName
	}
	if target, ok := attributeMap["name.familyName"]; ok {
		result[target] = item.Name.FamilyName
	}
	if target, ok := attributeMap["suspended"]; ok {
		result[target] = item.Suspended
	}
	if target, ok := attributeMap["orgUnitPath"]; ok {
		result[target] = item.OrgUnitPath
	}

	return result
}

// mapGroup maps a Google group to a field map using the configured attribute mapping.
// Default mapping (when AttributeMap is empty):
//   - email -> name, name -> displayName, description -> description
func (s *GoogleSyncer) mapGroup(item googleGroupItem, attributeMap map[string]string) map[string]interface{} {
	result := make(map[string]interface{})

	if len(attributeMap) == 0 {
		result["id"] = item.ID
		result["name"] = item.Email
		result["displayName"] = item.Name
		result["description"] = item.Description
		return result
	}

	if target, ok := attributeMap["id"]; ok {
		result[target] = item.ID
	}
	if target, ok := attributeMap["email"]; ok {
		result[target] = item.Email
	}
	if target, ok := attributeMap["name"]; ok {
		result[target] = item.Name
	}
	if target, ok := attributeMap["description"]; ok {
		result[target] = item.Description
	}

	return result
}

// Ensure GoogleSyncer implements Engine.
var _ Engine = (*GoogleSyncer)(nil)
