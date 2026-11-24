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

// DingTalkSyncer syncs users and groups from DingTalk.
type DingTalkSyncer struct {
	// httpClient overrides the SSRF-safe client for contract tests
	// (mirrors the idp HTTPClient injection precedent).
	httpClient *http.Client
}

func (s *DingTalkSyncer) client() *http.Client {
	if s.httpClient != nil {
		return s.httpClient
	}
	return ssrf.SafeHTTPClient(10 * time.Second)
}

// Token endpoints follow the current official server API surface
// (open.dingtalk.com): the new-version POST api.dingtalk.com/v1.0/oauth2/accessToken
// is preferred; the legacy GET oapi.dingtalk.com/gettoken remains officially
// maintained and is kept as a compatibility fallback (G7-d3, OPEN-API-75 P10
// correction). Package-level vars so contract tests can retarget them at
// httptest servers.
var (
	dingTalkOAuthTokenURL  = "https://api.dingtalk.com/v1.0/oauth2/accessToken" //nolint:gosec // G101: endpoint URL, not a credential
	dingTalkLegacyTokenURL = "https://oapi.dingtalk.com/gettoken"               //nolint:gosec // G101: endpoint URL, not a credential
)

// dingTalkOAuthTokenResponse covers the new-version token endpoint:
// success {accessToken, expireIn} (expireIn in seconds, advisory — the
// syncer fetches a fresh token per operation and never hardcodes a
// lifetime) and failure {code, message, requestid} with non-200 HTTP.
type dingTalkOAuthTokenResponse struct {
	AccessToken string `json:"accessToken"`
	ExpireIn    int64  `json:"expireIn"`
	Code        string `json:"code"`
	Message     string `json:"message"`
	RequestID   string `json:"requestid"`
}

// dingTalkTokenResponse represents the response from DingTalk gettoken API.
type dingTalkTokenResponse struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// dingTalkDeptListResponse represents the response from DingTalk department list API.
type dingTalkDeptListResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	Result  struct {
		HasMore bool `json:"has_more"`
		List    []struct {
			DeptID   int    `json:"dept_id"`
			Name     string `json:"name"`
			ParentID int    `json:"parent_id"`
		} `json:"list"`
	} `json:"result"`
}

// dingTalkUserListResponse represents the response from DingTalk user list API.
type dingTalkUserListResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	Result  struct {
		HasMore bool `json:"has_more"`
		List    []struct {
			UserID     string `json:"userid"`
			Name       string `json:"name"`
			Mobile     string `json:"mobile"`
			Email      string `json:"email"`
			DeptIDList []int  `json:"dept_id_list"`
			ModifyTime int64  `json:"modify_time"` // Unix timestamp in milliseconds
		} `json:"list"`
	} `json:"result"`
}

// dingTalkDeptListRequest is the request body for listing sub-departments.
type dingTalkDeptListRequest struct {
	DeptID int `json:"dept_id"`
	Cursor int `json:"cursor"`
	Size   int `json:"size"`
}

// dingTalkUserListRequest is the request body for listing department users.
type dingTalkUserListRequest struct {
	DeptID     int    `json:"dept_id"`
	Cursor     int    `json:"cursor"`
	Size       int    `json:"size"`
	OrderField string `json:"order_field"`
}

// getDingTalkAccessToken obtains an access token. Default flow: new-version
// OAuth2 endpoint first, with the officially maintained legacy gettoken as
// a compatibility fallback when the new endpoint is unreachable
// (transport/parse failure or non-business HTTP status). Business errors
// (HTTP 400 with an error code) are surfaced without fallback. An explicit
// config.APIURL keeps the legacy query-parameter contract for custom
// deployments (it also rebases the directory pull).
func (s *DingTalkSyncer) getDingTalkAccessToken(ctx context.Context, config *SyncerConfig) (string, error) {
	if config.APIURL != "" {
		return s.getDingTalkLegacyToken(ctx, config, config.APIURL+"/gettoken")
	}

	token, fallback, err := s.getDingTalkOAuthToken(ctx, config)
	if err == nil {
		return token, nil
	}
	if !fallback {
		return "", err
	}
	return s.getDingTalkLegacyToken(ctx, config, dingTalkLegacyTokenURL)
}

// getDingTalkOAuthToken requests a token via the new-version endpoint
// (POST JSON appKey/appSecret). The second return value reports whether a
// legacy-endpoint fallback is warranted.
func (s *DingTalkSyncer) getDingTalkOAuthToken(ctx context.Context, config *SyncerConfig) (token string, fallback bool, err error) {
	reqBody, err := json.Marshal(map[string]string{
		"appKey":    config.ClientID,
		"appSecret": config.ClientSecret,
	})
	if err != nil {
		return "", false, apperrors.WrapInternalf(err, "dingtalk syncer marshal oauth token request")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dingTalkOAuthTokenURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", false, apperrors.WrapInternalf(err, "dingtalk syncer create oauth token request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client().Do(req)
	if err != nil {
		return "", true, apperrors.WrapInternalf(err, "dingtalk syncer get oauth token")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", true, apperrors.WrapInternalf(err, "dingtalk syncer read oauth token response")
	}

	var tokenResp dingTalkOAuthTokenResponse
	if resp.StatusCode != http.StatusOK {
		// HTTP 400 with a parseable error code is a business rejection
		// (bad credentials/permissions); both endpoints share the same
		// credential store, so fallback would only mask the real cause.
		if resp.StatusCode == http.StatusBadRequest && json.Unmarshal(body, &tokenResp) == nil && tokenResp.Code != "" {
			return "", false, apperrors.NewInternalf("dingtalk syncer oauth token error: code=%s, message=%s", tokenResp.Code, tokenResp.Message)
		}
		return "", true, apperrors.NewInternalf("dingtalk syncer oauth token endpoint status %d", resp.StatusCode)
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", true, apperrors.WrapInternalf(err, "dingtalk syncer parse oauth token response")
	}
	if tokenResp.AccessToken == "" {
		return "", false, apperrors.NewInternalf("dingtalk syncer oauth token response missing accessToken")
	}
	return tokenResp.AccessToken, false, nil
}

// getDingTalkLegacyToken requests a token via the legacy gettoken endpoint
// (GET with appkey/appsecret query parameters), officially maintained for
// compatibility.
func (s *DingTalkSyncer) getDingTalkLegacyToken(ctx context.Context, config *SyncerConfig, baseURL string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "dingtalk syncer parse token url")
	}
	q := u.Query()
	q.Set("appkey", config.ClientID)
	q.Set("appsecret", config.ClientSecret)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "dingtalk syncer create token request")
	}

	resp, err := s.client().Do(req)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "dingtalk syncer get token")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "dingtalk syncer read token response")
	}

	var tokenResp dingTalkTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", apperrors.WrapInternalf(err, "dingtalk syncer parse token response")
	}

	if tokenResp.ErrCode != 0 {
		return "", apperrors.NewInternalf("dingtalk syncer token error: code=%d, msg=%s", tokenResp.ErrCode, tokenResp.ErrMsg)
	}

	return tokenResp.AccessToken, nil
}

// SyncUsers synchronizes users from DingTalk.
func (s *DingTalkSyncer) SyncUsers(ctx context.Context, config *SyncerConfig) (*SyncResult, error) {
	token, err := s.getDingTalkAccessToken(ctx, config)
	if err != nil {
		return nil, err
	}

	result := &SyncResult{}

	// First, get department list to collect all department IDs
	deptIDs := s.listAllDepartmentIDs(ctx, token, config)

	// Collect users from all departments, deduplicating by UserID
	seen := make(map[string]bool)
	var allUsers []dingTalkUserItem

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
			modifyTime := time.UnixMilli(u.ModifyTime)
			if !modifyTime.After(*config.LastSyncAt) {
				continue
			}
		}

		result.TotalUsers++

		// Apply field mapping
		mapped := s.mapUser(u, config.AttributeMap)

		if config.OnUserSynced != nil {
			created, err := config.OnUserSynced(ctx, "dingtalk", u.UserID, mapped)
			if err != nil {
				result.FailedUsers++
				result.Errors = append(result.Errors, fmt.Sprintf("dingtalk user write-back failed: userid=%s, err=%v", u.UserID, err))
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

// SyncGroups synchronizes groups (departments) from DingTalk.
func (s *DingTalkSyncer) SyncGroups(ctx context.Context, config *SyncerConfig) (*SyncResult, error) {
	token, err := s.getDingTalkAccessToken(ctx, config)
	if err != nil {
		return nil, err
	}

	result := &SyncResult{}

	depts, errs := s.listDepartments(ctx, token, config, 1) // root dept ID is 1
	result.Errors = append(result.Errors, errs...)

	for _, dept := range depts {
		// Apply field mapping
		mapped := s.mapGroup(dept, config.AttributeMap)

		if config.OnGroupSynced != nil {
			created, err := config.OnGroupSynced(ctx, config.OrganizationID, fmt.Sprintf("%d", dept.DeptID), mapped)
			if err != nil {
				result.FailedGroups++
				result.Errors = append(result.Errors, fmt.Sprintf("dingtalk group write-back failed: dept_id=%d, err=%v", dept.DeptID, err))
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

// TestConnection tests the connection to DingTalk by obtaining an access token.
func (s *DingTalkSyncer) TestConnection(ctx context.Context, config *SyncerConfig) error {
	_, err := s.getDingTalkAccessToken(ctx, config)
	if err != nil {
		return apperrors.WrapInternalf(err, "dingtalk test connection failed")
	}
	return nil
}

// mapUser applies the AttributeMap to transform source fields to target fields for a DingTalk user.
// Default mapping when AttributeMap is empty:
//
//	userid -> username, name -> displayName, mobile -> phone, email -> email, dept_id_list -> groups
func (s *DingTalkSyncer) mapUser(u dingTalkUserItem, attributeMap map[string]string) map[string]interface{} {
	result := make(map[string]interface{})

	if len(attributeMap) == 0 {
		// Default DingTalk -> GuliangIAM field mapping
		result["username"] = u.UserID
		result["displayName"] = u.Name
		result["phone"] = u.Mobile
		result["email"] = u.Email
		result["groups"] = u.DeptIDList
		return result
	}

	// Apply custom mapping
	// Source fields: userid, name, mobile, email, dept_id_list
	if target, ok := attributeMap["userid"]; ok {
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
	if target, ok := attributeMap["dept_id_list"]; ok {
		result[target] = u.DeptIDList
	}

	return result
}

// mapGroup applies the AttributeMap to transform source fields to target fields for a DingTalk department.
// Default mapping when AttributeMap is empty:
//
//	dept_id -> groupId, name -> displayName, parent_id -> parentId
func (s *DingTalkSyncer) mapGroup(g dingTalkDeptItem, attributeMap map[string]string) map[string]interface{} {
	result := make(map[string]interface{})

	if len(attributeMap) == 0 {
		// Default DingTalk -> GuliangIAM field mapping
		result["groupId"] = g.DeptID
		result["displayName"] = g.Name
		result["parentId"] = g.ParentID
		return result
	}

	// Apply custom mapping
	// Source fields: dept_id, name, parent_id
	if target, ok := attributeMap["dept_id"]; ok {
		result[target] = g.DeptID
	}
	if target, ok := attributeMap["name"]; ok {
		result[target] = g.Name
	}
	if target, ok := attributeMap["parent_id"]; ok {
		result[target] = g.ParentID
	}

	return result
}

// listDepartments lists sub-departments under the given parent department ID.
func (s *DingTalkSyncer) listDepartments(ctx context.Context, token string, config *SyncerConfig, parentDeptID int) (departments []dingTalkDeptItem, errs []string) {
	baseURL := "https://oapi.dingtalk.com/topapi/v2/department/listsub"
	if config.APIURL != "" {
		baseURL = config.APIURL + "/topapi/v2/department/listsub"
	}

	var allDepts []dingTalkDeptItem
	var errors []string
	cursor := 0

	for {
		reqBody := dingTalkDeptListRequest{
			DeptID: parentDeptID,
			Cursor: cursor,
			Size:   100,
		}
		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			errors = append(errors, fmt.Sprintf("dingtalk marshal dept list request: %v", err))
			break
		}

		u, _ := url.Parse(baseURL)
		q := u.Query()
		q.Set("access_token", token)
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(bodyBytes))
		if err != nil {
			errors = append(errors, fmt.Sprintf("dingtalk create dept list request: %v", err))
			break
		}
		req.Header.Set("Content-Type", "application/json")

		client := s.client()
		resp, err := client.Do(req)
		if err != nil {
			errors = append(errors, fmt.Sprintf("dingtalk dept list request: %v", err))
			break
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			errors = append(errors, fmt.Sprintf("dingtalk read dept list response: %v", err))
			break
		}

		var deptResp dingTalkDeptListResponse
		if err := json.Unmarshal(body, &deptResp); err != nil {
			errors = append(errors, fmt.Sprintf("dingtalk parse dept list response: %v", err))
			break
		}

		if deptResp.ErrCode != 0 {
			errors = append(errors, fmt.Sprintf("dingtalk dept list error: code=%d, msg=%s", deptResp.ErrCode, deptResp.ErrMsg))
			break
		}

		allDepts = append(allDepts, deptResp.Result.List...)

		// Recursively fetch sub-departments
		for _, dept := range deptResp.Result.List {
			subDepts, subErrs := s.listDepartments(ctx, token, config, dept.DeptID)
			allDepts = append(allDepts, subDepts...)
			errors = append(errors, subErrs...)
		}

		if !deptResp.Result.HasMore {
			break
		}
		cursor++
	}

	return allDepts, errors
}

type dingTalkDeptItem = struct {
	DeptID   int    `json:"dept_id"`
	Name     string `json:"name"`
	ParentID int    `json:"parent_id"`
}

// listAllDepartmentIDs collects all department IDs by traversing the department tree.
func (s *DingTalkSyncer) listAllDepartmentIDs(ctx context.Context, token string, config *SyncerConfig) []int {
	depts, _ := s.listDepartments(ctx, token, config, 1)
	ids := make([]int, 0, len(depts)+1)
	ids = append(ids, 1) // root dept
	for _, d := range depts {
		ids = append(ids, d.DeptID)
	}
	return ids
}

type dingTalkUserItem = struct {
	UserID     string `json:"userid"`
	Name       string `json:"name"`
	Mobile     string `json:"mobile"`
	Email      string `json:"email"`
	DeptIDList []int  `json:"dept_id_list"`
	ModifyTime int64  `json:"modify_time"` // Unix timestamp in milliseconds
}

// listDepartmentUsers lists users in a specific department.
func (s *DingTalkSyncer) listDepartmentUsers(ctx context.Context, token string, config *SyncerConfig, deptID int) (users []dingTalkUserItem, errs []string) {
	baseURL := "https://oapi.dingtalk.com/topapi/v2/user/list"
	if config.APIURL != "" {
		baseURL = config.APIURL + "/topapi/v2/user/list"
	}

	var allUsers []dingTalkUserItem
	var errors []string
	cursor := 0

	for {
		reqBody := dingTalkUserListRequest{
			DeptID:     deptID,
			Cursor:     cursor,
			Size:       100,
			OrderField: "modify_desc",
		}
		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			errors = append(errors, fmt.Sprintf("dingtalk marshal user list request: %v", err))
			break
		}

		u, _ := url.Parse(baseURL)
		q := u.Query()
		q.Set("access_token", token)
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(bodyBytes))
		if err != nil {
			errors = append(errors, fmt.Sprintf("dingtalk create user list request: %v", err))
			break
		}
		req.Header.Set("Content-Type", "application/json")

		client := s.client()
		resp, err := client.Do(req)
		if err != nil {
			errors = append(errors, fmt.Sprintf("dingtalk user list request: %v", err))
			break
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			errors = append(errors, fmt.Sprintf("dingtalk read user list response: %v", err))
			break
		}

		var userResp dingTalkUserListResponse
		if err := json.Unmarshal(body, &userResp); err != nil {
			errors = append(errors, fmt.Sprintf("dingtalk parse user list response: %v", err))
			break
		}

		if userResp.ErrCode != 0 {
			errors = append(errors, fmt.Sprintf("dingtalk user list error: code=%d, msg=%s", userResp.ErrCode, userResp.ErrMsg))
			break
		}

		allUsers = append(allUsers, userResp.Result.List...)

		if !userResp.Result.HasMore {
			break
		}
		cursor++
	}

	return allUsers, errors
}

// Ensure DingTalkSyncer implements Engine.
var _ Engine = (*DingTalkSyncer)(nil)
