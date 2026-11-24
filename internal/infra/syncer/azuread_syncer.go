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

// AzureADSyncer syncs users and groups from Azure Active Directory (Microsoft Entra ID).
type AzureADSyncer struct{}

// azureADTokenResponse represents the response from Microsoft OAuth2 token endpoint.
type azureADTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	ExpiresOn   string `json:"expires_on"`
	Scope       string `json:"scope"`
}

// azureADPage is the common Microsoft Graph list envelope shared by users/groups.
type azureADPage[T any] struct {
	NextLink string `json:"@odata.nextLink"`
	Value    []T    `json:"value"`
}

// getAzureADAccessToken obtains an access token using client credentials flow.
func (s *AzureADSyncer) getAzureADAccessToken(ctx context.Context, config *SyncerConfig) (string, error) {
	tenantID := config.TenantID
	if tenantID == "" {
		tenantID = "common"
	}

	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantID)
	if config.TokenURL != "" {
		tokenURL = config.TokenURL
	}

	formData := url.Values{}
	formData.Set("client_id", config.ClientID)
	formData.Set("client_secret", config.ClientSecret)
	formData.Set("scope", "https://graph.microsoft.com/.default")
	formData.Set("grant_type", "client_credentials")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewReader([]byte(formData.Encode())))
	if err != nil {
		return "", apperrors.WrapInternalf(err, "azuread syncer create token request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := ssrf.SafeHTTPClient(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "azuread syncer get token")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "azuread syncer read token response")
	}

	var tokenResp azureADTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", apperrors.WrapInternalf(err, "azuread syncer parse token response")
	}

	if tokenResp.AccessToken == "" {
		return "", apperrors.NewNotFound("azuread syncer token error: empty access token received")
	}

	return tokenResp.AccessToken, nil
}

// SyncUsers synchronizes users from Azure AD via Microsoft Graph API.
func (s *AzureADSyncer) SyncUsers(ctx context.Context, config *SyncerConfig) (*SyncResult, error) {
	token, err := s.getAzureADAccessToken(ctx, config)
	if err != nil {
		return nil, err
	}

	graphURL := "https://graph.microsoft.com/v1.0"
	if config.APIURL != "" {
		graphURL = config.APIURL
	}

	result := &SyncResult{}

	users, errs := s.listUsers(ctx, token, graphURL, config)
	result.TotalUsers = len(users)
	result.Errors = append(result.Errors, errs...)

	// Map users and persist via write-back callback
	isIncremental := config.LastSyncAt != nil
	for _, u := range users {
		if u.UserPrincipalName == "" {
			result.FailedUsers++
			result.Errors = append(result.Errors, fmt.Sprintf("azuread user missing userPrincipalName: id=%s", u.ID))
			continue
		}

		mapped := s.mapUser(u, config.AttributeMap)

		if config.OnUserSynced != nil {
			created, err := config.OnUserSynced(ctx, "azuread", u.ID, mapped)
			if err != nil {
				result.FailedUsers++
				result.Errors = append(result.Errors, fmt.Sprintf("azuread user write-back failed: id=%s, err=%v", u.ID, err))
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

// SyncGroups synchronizes groups from Azure AD via Microsoft Graph API.
func (s *AzureADSyncer) SyncGroups(ctx context.Context, config *SyncerConfig) (*SyncResult, error) {
	token, err := s.getAzureADAccessToken(ctx, config)
	if err != nil {
		return nil, err
	}

	graphURL := "https://graph.microsoft.com/v1.0"
	if config.APIURL != "" {
		graphURL = config.APIURL
	}

	result := &SyncResult{}

	groups, errs := s.listGroups(ctx, token, graphURL, config)
	result.TotalGroups = len(groups)
	result.Errors = append(result.Errors, errs...)

	// Map groups and persist via write-back callback
	isIncremental := config.LastSyncAt != nil
	for _, g := range groups {
		if g.DisplayName == "" {
			result.FailedGroups++
			result.Errors = append(result.Errors, fmt.Sprintf("azuread group missing displayName: id=%s", g.ID))
			continue
		}

		mapped := s.mapGroup(g, config.AttributeMap)

		if config.OnGroupSynced != nil {
			created, err := config.OnGroupSynced(ctx, config.OrganizationID, g.ID, mapped)
			if err != nil {
				result.FailedGroups++
				result.Errors = append(result.Errors, fmt.Sprintf("azuread group write-back failed: id=%s, err=%v", g.ID, err))
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

// TestConnection tests the connection to Azure AD by obtaining an access token.
func (s *AzureADSyncer) TestConnection(ctx context.Context, config *SyncerConfig) error {
	_, err := s.getAzureADAccessToken(ctx, config)
	if err != nil {
		return apperrors.WrapInternalf(err, "azuread test connection failed")
	}
	return nil
}

type azureADUserItem = struct {
	ID                string `json:"id"`
	DisplayName       string `json:"displayName"`
	UserPrincipalName string `json:"userPrincipalName"`
	Mail              string `json:"mail"`
	GivenName         string `json:"givenName"`
	Surname           string `json:"surname"`
	AccountEnabled    bool   `json:"accountEnabled"`
}

// listUsers lists all users from Microsoft Graph API.
func (s *AzureADSyncer) listUsers(ctx context.Context, token, graphURL string, config *SyncerConfig) (users []azureADUserItem, errors []string) {
	u, _ := url.Parse(graphURL + "/users")
	q := u.Query()
	q.Set("$top", "999")
	q.Set("$select", "id,displayName,userPrincipalName,mail,givenName,surname,accountEnabled")
	if config.Filter != "" {
		q.Set("$filter", config.Filter) // OData filter, e.g., "accountEnabled eq true"
	}
	// Note: Azure AD Graph API does not support a universal modified timestamp filter
	// for incremental sync. Full sync is performed regardless of LastSyncAt setting.
	u.RawQuery = q.Encode()
	return fetchAzureADPages[azureADUserItem](ctx, token, u.String(), "user")
}

type azureADGroupItem = struct {
	ID              string `json:"id"`
	DisplayName     string `json:"displayName"`
	Description     string `json:"description"`
	Mail            string `json:"mail"`
	MailEnabled     bool   `json:"mailEnabled"`
	SecurityEnabled bool   `json:"securityEnabled"`
}

// listGroups lists all groups from Microsoft Graph API.
func (s *AzureADSyncer) listGroups(ctx context.Context, token, graphURL string, config *SyncerConfig) (groups []azureADGroupItem, errors []string) {
	u, _ := url.Parse(graphURL + "/groups")
	q := u.Query()
	q.Set("$top", "999")
	q.Set("$select", "id,displayName,description,mail,mailEnabled,securityEnabled")
	if config.Filter != "" {
		q.Set("$filter", config.Filter) // OData filter, e.g., "securityEnabled eq true"
	}
	// Note: Azure AD Graph API does not support a universal modified timestamp filter
	// for incremental sync. Full sync is performed regardless of LastSyncAt setting.
	u.RawQuery = q.Encode()
	return fetchAzureADPages[azureADGroupItem](ctx, token, u.String(), "group")
}

// fetchAzureADPages follows Microsoft Graph's @odata.nextLink pagination and
// returns the accumulated values plus any per-page errors.
func fetchAzureADPages[T any](ctx context.Context, token, nextURL, kind string) (items []T, errors []string) {
	for nextURL != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, nextURL, http.NoBody)
		if err != nil {
			errors = append(errors, fmt.Sprintf("azuread create %s list request: %v", kind, err))
			break
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		client := ssrf.SafeHTTPClient(30 * time.Second)
		resp, err := client.Do(req)
		if err != nil {
			errors = append(errors, fmt.Sprintf("azuread %s list request: %v", kind, err))
			break
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			errors = append(errors, fmt.Sprintf("azuread read %s list response: %v", kind, err))
			break
		}

		if resp.StatusCode != http.StatusOK {
			errors = append(errors, fmt.Sprintf("azuread %s list HTTP error: status=%d, body=%s", kind, resp.StatusCode, string(body)))
			break
		}

		var page azureADPage[T]
		if err := json.Unmarshal(body, &page); err != nil {
			errors = append(errors, fmt.Sprintf("azuread parse %s list response: %v", kind, err))
			break
		}

		items = append(items, page.Value...)
		nextURL = page.NextLink
	}

	return items, errors
}

// mapUser maps an Azure AD user to a field map using the configured attribute mapping.
// Default mapping (when AttributeMap is empty):
//   - userPrincipalName -> username, displayName -> displayName, mail -> email, accountEnabled -> enabled
func (s *AzureADSyncer) mapUser(item azureADUserItem, attributeMap map[string]string) map[string]interface{} {
	result := make(map[string]interface{})

	if len(attributeMap) == 0 {
		// Default AzureAD -> GuliangIAM field mapping
		result["id"] = item.ID
		result["username"] = item.UserPrincipalName
		result["displayName"] = item.DisplayName
		result["email"] = item.Mail
		result[dirAttrGivenName] = item.GivenName
		result["surname"] = item.Surname
		result["enabled"] = item.AccountEnabled
		return result
	}

	// Apply custom mapping: AttributeMap key is the AzureAD source field name
	if target, ok := attributeMap["id"]; ok {
		result[target] = item.ID
	}
	if target, ok := attributeMap["userPrincipalName"]; ok {
		result[target] = item.UserPrincipalName
	}
	if target, ok := attributeMap["displayName"]; ok {
		result[target] = item.DisplayName
	}
	if target, ok := attributeMap["mail"]; ok {
		result[target] = item.Mail
	}
	if target, ok := attributeMap[dirAttrGivenName]; ok {
		result[target] = item.GivenName
	}
	if target, ok := attributeMap["surname"]; ok {
		result[target] = item.Surname
	}
	if target, ok := attributeMap["accountEnabled"]; ok {
		result[target] = item.AccountEnabled
	}

	return result
}

// mapGroup maps an Azure AD group to a field map using the configured attribute mapping.
// Default mapping (when AttributeMap is empty):
//   - displayName -> name, description -> description, mail -> email
func (s *AzureADSyncer) mapGroup(item azureADGroupItem, attributeMap map[string]string) map[string]interface{} {
	result := make(map[string]interface{})

	if len(attributeMap) == 0 {
		result["id"] = item.ID
		result["name"] = item.DisplayName
		result["description"] = item.Description
		result["email"] = item.Mail
		result["mailEnabled"] = item.MailEnabled
		result["securityEnabled"] = item.SecurityEnabled
		return result
	}

	if target, ok := attributeMap["id"]; ok {
		result[target] = item.ID
	}
	if target, ok := attributeMap["displayName"]; ok {
		result[target] = item.DisplayName
	}
	if target, ok := attributeMap["description"]; ok {
		result[target] = item.Description
	}
	if target, ok := attributeMap["mail"]; ok {
		result[target] = item.Mail
	}
	if target, ok := attributeMap["mailEnabled"]; ok {
		result[target] = item.MailEnabled
	}
	if target, ok := attributeMap["securityEnabled"]; ok {
		result[target] = item.SecurityEnabled
	}

	return result
}

// Ensure AzureADSyncer implements Engine.
var _ Engine = (*AzureADSyncer)(nil)
