package syncer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// LarkSyncer syncs users and groups from Lark (Feishu).
type LarkSyncer struct {
	// httpClient overrides the SSRF-safe client for contract tests
	// (mirrors the idp HTTPClient injection precedent).
	httpClient *http.Client
}

func (s *LarkSyncer) client() *http.Client {
	if s.httpClient != nil {
		return s.httpClient
	}
	return ssrf.SafeHTTPClient(10 * time.Second)
}

// Token endpoints follow the current official OpenAPI surface (open.feishu.cn):
// the OAuth2 client_credentials endpoint accounts.feishu.cn/oauth/v3/token is
// preferred; the legacy auth/v3/tenant_access_token/internal is officially
// marked historical and kept only as a compatibility fallback (G7-d2,
// OPEN-API-75 P10 correction). Package-level vars so contract tests can
// retarget them at httptest servers.
var (
	larkOAuthTokenURL  = "https://accounts.feishu.cn/oauth/v3/token"                             //nolint:gosec // G101: endpoint URL, not a credential
	larkLegacyTokenURL = "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal" //nolint:gosec // G101: endpoint URL, not a credential
)

// larkOAuthTokenResponse covers both response shapes of the OAuth token
// endpoint: success {code:0, access_token, expires_in, token_type} and
// failure {code!=0, error, error_description}. expires_in is advisory
// (max 7200s per official docs); the syncer fetches a fresh token per
// operation and never hardcodes a lifetime.
type larkOAuthTokenResponse struct {
	Code             int    `json:"code"`
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	TokenType        string `json:"token_type"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
	Msg              string `json:"msg"`
}

// larkTokenResponse represents the response from the legacy Lark tenant access token API.
type larkTokenResponse struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
	Expire            int    `json:"expire"`
}

// larkTokenRequest is the request body for the legacy tenant access token API.
type larkTokenRequest struct {
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
}

// larkDepartmentResponse represents the response from Lark department list API.
type larkDepartmentResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		HasMore   bool   `json:"has_more"`
		PageToken string `json:"page_token"`
		Items     []struct {
			DepartmentID string `json:"department_id"`
			Name         string `json:"name"`
			ParentID     string `json:"parent_department_id"`
		} `json:"items"`
	} `json:"data"`
}

// larkUserResponse represents the response from Lark user list API.
type larkUserResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		HasMore   bool   `json:"has_more"`
		PageToken string `json:"page_token"`
		Items     []struct {
			UserID        string   `json:"user_id"`
			Name          string   `json:"name"`
			Mobile        string   `json:"mobile"`
			Email         string   `json:"email"`
			DepartmentIDs []string `json:"department_ids"`
			ModifyTime    int64    `json:"modify_time"` // Unix timestamp in seconds
		} `json:"items"`
	} `json:"data"`
}

// getLarkAccessToken obtains a tenant access token. Default flow: OAuth2
// client_credentials against accounts.feishu.cn/oauth/v3/token first, with
// the historical internal endpoint as a compatibility fallback when the new
// endpoint is unreachable (transport/HTTP/parse failure). Business errors
// (code!=0, e.g. bad secret) are surfaced without fallback — both endpoints
// share the same credential store. An explicit config.TokenURL keeps the
// legacy JSON contract for custom/private deployments.
func (s *LarkSyncer) getLarkAccessToken(ctx context.Context, config *SyncerConfig) (string, error) {
	if config.TokenURL != "" {
		return s.getLarkLegacyToken(ctx, config, config.TokenURL)
	}

	token, fallback, err := s.getLarkOAuthToken(ctx, config)
	if err == nil {
		return token, nil
	}
	if !fallback {
		return "", err
	}
	return s.getLarkLegacyToken(ctx, config, larkLegacyTokenURL)
}

// getLarkOAuthToken requests a token via the current OAuth2 endpoint.
// client_secret_post only (form body); adding HTTP Basic alongside would
// trigger official error 20070. The second return value reports whether a
// legacy-endpoint fallback is warranted.
func (s *LarkSyncer) getLarkOAuthToken(ctx context.Context, config *SyncerConfig) (token string, fallback bool, err error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", config.ClientID)
	form.Set("client_secret", config.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, larkOAuthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", false, apperrors.WrapInternalf(err, "lark syncer create oauth token request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client().Do(req)
	if err != nil {
		return "", true, apperrors.WrapInternalf(err, "lark syncer get oauth token")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", true, apperrors.WrapInternalf(err, "lark syncer read oauth token response")
	}
	if resp.StatusCode != http.StatusOK {
		return "", true, apperrors.NewInternalf("lark syncer oauth token endpoint status %d", resp.StatusCode)
	}

	var tokenResp larkOAuthTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", true, apperrors.WrapInternalf(err, "lark syncer parse oauth token response")
	}

	if tokenResp.Code != 0 {
		detail := tokenResp.ErrorDescription
		if detail == "" {
			detail = tokenResp.Msg
		}
		return "", false, apperrors.NewInternalf("lark syncer oauth token error: code=%d, error=%s, detail=%s", tokenResp.Code, tokenResp.Error, detail)
	}
	// Official docs: read access_token only; the legacy tenant_access_token
	// field must not be consumed from this endpoint.
	if tokenResp.AccessToken == "" {
		return "", false, apperrors.NewInternalf("lark syncer oauth token response missing access_token")
	}
	return tokenResp.AccessToken, false, nil
}

// getLarkLegacyToken requests a token via the historical internal endpoint
// (JSON app_id/app_secret body), officially marked historical but retained
// for multi-version compatibility.
func (s *LarkSyncer) getLarkLegacyToken(ctx context.Context, config *SyncerConfig, tokenURL string) (string, error) {
	reqBody := larkTokenRequest{
		AppID:     config.ClientID,
		AppSecret: config.ClientSecret,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "lark syncer marshal token request")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", apperrors.WrapInternalf(err, "lark syncer create token request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client().Do(req)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "lark syncer get token")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "lark syncer read token response")
	}

	var tokenResp larkTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", apperrors.WrapInternalf(err, "lark syncer parse token response")
	}

	if tokenResp.Code != 0 {
		return "", apperrors.NewInternalf("lark syncer token error: code=%d, msg=%s", tokenResp.Code, tokenResp.Msg)
	}

	return tokenResp.TenantAccessToken, nil
}

// SyncUsers synchronizes users from Lark.
func (s *LarkSyncer) SyncUsers(ctx context.Context, config *SyncerConfig) (*SyncResult, error) {
	token, err := s.getLarkAccessToken(ctx, config)
	if err != nil {
		return nil, err
	}

	result := &SyncResult{}

	// List all department IDs first
	deptIDs := s.listAllLarkDepartmentIDs(ctx, token, config)

	// Collect users from all departments, deduplicating by UserID
	seen := make(map[string]bool)
	var allUsers []larkUserItem

	for _, deptID := range deptIDs {
		users, errs := s.listDepartmentUsers(ctx, token, config, deptID)
		result.Errors = append(result.Errors, errs...)

		for _, u := range users {
			if seen[u.UserID] {
				continue
			}
			seen[u.UserID] = true
			allUsers = append(allUsers, u)
		}
	}

	// Apply incremental sync filter and field mapping
	for _, u := range allUsers {
		// Incremental sync: skip users not modified since LastSyncAt
		if config.LastSyncAt != nil && u.ModifyTime > 0 {
			modifyTime := time.Unix(u.ModifyTime, 0)
			if !modifyTime.After(*config.LastSyncAt) {
				continue
			}
		}

		result.TotalUsers++

		// Apply field mapping
		mapped := s.mapUser(u, config.AttributeMap)

		if config.OnUserSynced != nil {
			created, err := config.OnUserSynced(ctx, "lark", u.UserID, mapped)
			if err != nil {
				result.FailedUsers++
				result.Errors = append(result.Errors, fmt.Sprintf("lark user write-back failed: user_id=%s, err=%v", u.UserID, err))
				continue
			}
			if created {
				result.AddedUsers++
			} else {
				result.UpdatedUsers++
			}
		} else {
			// Read-only mode: count without persisting
			if config.LastSyncAt != nil {
				result.UpdatedUsers++
			} else {
				result.AddedUsers++
			}
		}
	}

	return result, nil
}

// SyncGroups synchronizes groups (departments) from Lark.
func (s *LarkSyncer) SyncGroups(ctx context.Context, config *SyncerConfig) (*SyncResult, error) {
	token, err := s.getLarkAccessToken(ctx, config)
	if err != nil {
		return nil, err
	}

	result := &SyncResult{}

	depts, errs := s.listDepartments(ctx, token, config, "") // root department
	result.Errors = append(result.Errors, errs...)

	for _, dept := range depts {
		result.TotalGroups++

		// Apply field mapping
		mapped := s.mapGroup(dept, config.AttributeMap)

		if config.OnGroupSynced != nil {
			created, err := config.OnGroupSynced(ctx, config.OrganizationID, dept.DepartmentID, mapped)
			if err != nil {
				result.FailedGroups++
				result.Errors = append(result.Errors, fmt.Sprintf("lark group write-back failed: dept_id=%s, err=%v", dept.DepartmentID, err))
				continue
			}
			if created {
				result.AddedGroups++
			} else {
				result.UpdatedGroups++
			}
		} else {
			// Read-only mode: count without persisting
			if config.LastSyncAt != nil {
				result.UpdatedGroups++
			} else {
				result.AddedGroups++
			}
		}
	}

	return result, nil
}

// TestConnection tests the connection to Lark by obtaining an access token.
func (s *LarkSyncer) TestConnection(ctx context.Context, config *SyncerConfig) error {
	_, err := s.getLarkAccessToken(ctx, config)
	if err != nil {
		return apperrors.WrapInternalf(err, "lark test connection failed")
	}
	return nil
}

// mapUser applies the AttributeMap to transform source fields to target fields for a Lark user.
// Default mapping when AttributeMap is empty:
//
//	user_id -> username, name -> displayName, mobile -> phone, email -> email, department_ids -> groups
func (s *LarkSyncer) mapUser(u larkUserItem, attributeMap map[string]string) map[string]interface{} {
	result := make(map[string]interface{})

	if len(attributeMap) == 0 {
		// Default Lark -> GuliangIAM field mapping
		result["username"] = u.UserID
		result["displayName"] = u.Name
		result["phone"] = u.Mobile
		result["email"] = u.Email
		result["groups"] = u.DepartmentIDs
		return result
	}

	// Apply custom mapping
	// Source fields: user_id, name, mobile, email, department_ids
	if target, ok := attributeMap["user_id"]; ok {
		result[target] = u.UserID
	}
	if target, ok := attributeMap["name"]; ok {
		result[target] = u.Name
	}
	if target, ok := attributeMap["mobile"]; ok {
		result[target] = u.Mobile
	}
	if target, ok := attributeMap["email"]; ok {
		result[target] = u.Email
	}
	if target, ok := attributeMap["department_ids"]; ok {
		result[target] = u.DepartmentIDs
	}

	return result
}

// mapGroup applies the AttributeMap to transform source fields to target fields for a Lark department.
// Default mapping when AttributeMap is empty:
//
//	department_id -> groupId, name -> displayName, parent_department_id -> parentId
func (s *LarkSyncer) mapGroup(g larkDeptItem, attributeMap map[string]string) map[string]interface{} {
	result := make(map[string]interface{})

	if len(attributeMap) == 0 {
		// Default Lark -> GuliangIAM field mapping
		result["groupId"] = g.DepartmentID
		result["displayName"] = g.Name
		result["parentId"] = g.ParentID
		return result
	}

	// Apply custom mapping
	// Source fields: department_id, name, parent_department_id
	if target, ok := attributeMap["department_id"]; ok {
		result[target] = g.DepartmentID
	}
	if target, ok := attributeMap["name"]; ok {
		result[target] = g.Name
	}
	if target, ok := attributeMap["parent_department_id"]; ok {
		result[target] = g.ParentID
	}

	return result
}

// listDepartments lists sub-departments under the given parent department ID.
// An empty parentID means the root department.
func (s *LarkSyncer) listDepartments(ctx context.Context, token string, config *SyncerConfig, parentID string) (departments []larkDeptItem, errs []string) {
	baseURL := "https://open.feishu.cn/open-apis/contact/v3/departments"
	if config.APIURL != "" {
		baseURL = config.APIURL + "/contact/v3/departments"
	}

	var allDepts []larkDeptItem
	var errors []string
	pageToken := ""

	for {
		u, _ := url.Parse(baseURL + "/" + parentID + "/sub_departments")
		q := u.Query()
		q.Set("department_id_type", "department_id")
		q.Set("page_size", "50")
		if pageToken != "" {
			q.Set("page_token", pageToken)
		}
		if config.Filter != "" {
			q.Set("fetch_child", config.Filter)
		}
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
		if err != nil {
			errors = append(errors, fmt.Sprintf("lark create dept list request: %v", err))
			break
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		client := s.client()
		resp, err := client.Do(req)
		if err != nil {
			errors = append(errors, fmt.Sprintf("lark dept list request: %v", err))
			break
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			errors = append(errors, fmt.Sprintf("lark read dept list response: %v", err))
			break
		}

		var deptResp larkDepartmentResponse
		if err := json.Unmarshal(body, &deptResp); err != nil {
			errors = append(errors, fmt.Sprintf("lark parse dept list response: %v", err))
			break
		}

		if deptResp.Code != 0 {
			errors = append(errors, fmt.Sprintf("lark dept list error: code=%d, msg=%s", deptResp.Code, deptResp.Msg))
			break
		}

		allDepts = append(allDepts, deptResp.Data.Items...)

		// Recursively fetch sub-departments
		for _, dept := range deptResp.Data.Items {
			subDepts, subErrs := s.listDepartments(ctx, token, config, dept.DepartmentID)
			allDepts = append(allDepts, subDepts...)
			errors = append(errors, subErrs...)
		}

		if !deptResp.Data.HasMore {
			break
		}
		pageToken = deptResp.Data.PageToken
	}

	return allDepts, errors
}

type larkDeptItem = struct {
	DepartmentID string `json:"department_id"`
	Name         string `json:"name"`
	ParentID     string `json:"parent_department_id"`
}

// listAllLarkDepartmentIDs collects all department IDs by traversing the department tree.
func (s *LarkSyncer) listAllLarkDepartmentIDs(ctx context.Context, token string, config *SyncerConfig) []string {
	depts, _ := s.listDepartments(ctx, token, config, "")
	ids := make([]string, 0, len(depts)+1)
	ids = append(ids, "0") // root department ID in Lark
	for _, d := range depts {
		ids = append(ids, d.DepartmentID)
	}
	return ids
}

type larkUserItem = struct {
	UserID        string   `json:"user_id"`
	Name          string   `json:"name"`
	Mobile        string   `json:"mobile"`
	Email         string   `json:"email"`
	DepartmentIDs []string `json:"department_ids"`
	ModifyTime    int64    `json:"modify_time"` // Unix timestamp in seconds
}

// listDepartmentUsers lists users in a specific department.
func (s *LarkSyncer) listDepartmentUsers(ctx context.Context, token string, config *SyncerConfig, deptID string) (users []larkUserItem, errs []string) {
	baseURL := "https://open.feishu.cn/open-apis/contact/v3/users"
	if config.APIURL != "" {
		baseURL = config.APIURL + "/contact/v3/users"
	}

	var allUsers []larkUserItem
	var errors []string
	pageToken := ""

	for {
		u, _ := url.Parse(baseURL)
		q := u.Query()
		q.Set("department_id", deptID)
		q.Set("department_id_type", "department_id")
		q.Set("page_size", "50")
		if pageToken != "" {
			q.Set("page_token", pageToken)
		}
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
		if err != nil {
			errors = append(errors, fmt.Sprintf("lark create user list request: %v", err))
			break
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		client := s.client()
		resp, err := client.Do(req)
		if err != nil {
			errors = append(errors, fmt.Sprintf("lark user list request: %v", err))
			break
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			errors = append(errors, fmt.Sprintf("lark read user list response: %v", err))
			break
		}

		var userResp larkUserResponse
		if err := json.Unmarshal(body, &userResp); err != nil {
			errors = append(errors, fmt.Sprintf("lark parse user list response: %v", err))
			break
		}

		if userResp.Code != 0 {
			errors = append(errors, fmt.Sprintf("lark user list error: code=%d, msg=%s", userResp.Code, userResp.Msg))
			break
		}

		allUsers = append(allUsers, userResp.Data.Items...)

		if !userResp.Data.HasMore {
			break
		}
		pageToken = userResp.Data.PageToken
	}

	return allUsers, errors
}

// Ensure LarkSyncer implements Engine.
var _ Engine = (*LarkSyncer)(nil)
