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

// WeComSyncer syncs users and groups from WeCom (WeChat Work).
type WeComSyncer struct{}

// weComTokenResponse represents the response from WeCom gettoken API.
type weComTokenResponse struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// weComDepartmentResponse represents the response from WeCom department list API.
type weComDepartmentResponse struct {
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
	Department []struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		ParentID int    `json:"parentid"`
		Order    int    `json:"order"`
	} `json:"department"`
}

// weComUserListResponse represents the response from WeCom user list API.
type weComUserListResponse struct {
	ErrCode  int    `json:"errcode"`
	ErrMsg   string `json:"errmsg"`
	UserList []struct {
		UserID     string `json:"userid"`
		Name       string `json:"name"`
		Mobile     string `json:"mobile"`
		Email      string `json:"email"`
		Department []int  `json:"department"`
		Status     int    `json:"status"`
		ModifyTime int64  `json:"modify_time"` // Unix timestamp in seconds, if available
	} `json:"userlist"`
}

// getWeComAccessToken obtains an access token using corpid and corpsecret.
// Endpoints follow the current official server API surface
// (developer.work.weixin.qq.com): GET /cgi-bin/gettoken for the token and
// GET /cgi-bin/department/list|/user/list for the directory pull
// (OPEN-API-75 P10 review, no deviation).
func (s *WeComSyncer) getWeComAccessToken(ctx context.Context, config *SyncerConfig) (string, error) {
	baseURL := "https://qyapi.weixin.qq.com/cgi-bin/gettoken"
	if config.APIURL != "" {
		baseURL = config.APIURL + "/gettoken"
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "wecom syncer parse token url")
	}
	q := u.Query()
	q.Set("corpid", config.ClientID)
	q.Set("corpsecret", config.ClientSecret)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "wecom syncer create token request")
	}

	client := ssrf.SafeHTTPClient(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "wecom syncer get token")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "wecom syncer read token response")
	}

	var tokenResp weComTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", apperrors.WrapInternalf(err, "wecom syncer parse token response")
	}

	if tokenResp.ErrCode != 0 {
		return "", apperrors.NewInternalf("wecom syncer token error: code=%d, msg=%s", tokenResp.ErrCode, tokenResp.ErrMsg)
	}

	return tokenResp.AccessToken, nil
}

// SyncUsers synchronizes users from WeCom.
func (s *WeComSyncer) SyncUsers(ctx context.Context, config *SyncerConfig) (*SyncResult, error) {
	token, err := s.getWeComAccessToken(ctx, config)
	if err != nil {
		return nil, err
	}

	result := &SyncResult{}

	// List all departments first
	deptIDs := s.listAllWeComDepartmentIDs(ctx, token, config)

	// Collect users from all departments, deduplicating by UserID
	seen := make(map[string]bool)
	var allUsers []weComUserItem

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
		// Incremental sync: skip users not modified since LastSyncAt.
		// Note: WeCom's user list API does not reliably return modify_time for all users.
		// When modify_time is 0, we include the user to avoid missing updates.
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
			created, err := config.OnUserSynced(ctx, "wecom", u.UserID, mapped)
			if err != nil {
				result.FailedUsers++
				result.Errors = append(result.Errors, fmt.Sprintf("wecom user write-back failed: userid=%s, err=%v", u.UserID, err))
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

// SyncGroups synchronizes groups (departments) from WeCom.
func (s *WeComSyncer) SyncGroups(ctx context.Context, config *SyncerConfig) (*SyncResult, error) {
	token, err := s.getWeComAccessToken(ctx, config)
	if err != nil {
		return nil, err
	}

	result := &SyncResult{}

	depts, errs := s.listDepartments(ctx, token, config)
	result.Errors = append(result.Errors, errs...)

	for _, dept := range depts {
		result.TotalGroups++

		// Apply field mapping
		mapped := s.mapGroup(dept, config.AttributeMap)

		if config.OnGroupSynced != nil {
			created, err := config.OnGroupSynced(ctx, config.OrganizationID, fmt.Sprintf("%d", dept.ID), mapped)
			if err != nil {
				result.FailedGroups++
				result.Errors = append(result.Errors, fmt.Sprintf("wecom group write-back failed: dept_id=%d, err=%v", dept.ID, err))
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

// TestConnection tests the connection to WeCom by obtaining an access token.
func (s *WeComSyncer) TestConnection(ctx context.Context, config *SyncerConfig) error {
	_, err := s.getWeComAccessToken(ctx, config)
	if err != nil {
		return apperrors.WrapInternalf(err, "wecom test connection failed")
	}
	return nil
}

// mapUser applies the AttributeMap to transform source fields to target fields for a WeCom user.
// Default mapping when AttributeMap is empty:
//
//	userid -> username, name -> displayName, mobile -> phone, email -> email, department -> groups
func (s *WeComSyncer) mapUser(u weComUserItem, attributeMap map[string]string) map[string]interface{} {
	result := make(map[string]interface{})

	if len(attributeMap) == 0 {
		// Default WeCom -> GuliangIAM field mapping
		result["username"] = u.UserID
		result["displayName"] = u.Name
		result["phone"] = u.Mobile
		result["email"] = u.Email
		result["groups"] = u.Department
		return result
	}

	// Apply custom mapping
	// Source fields: userid, name, mobile, email, department
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
	if target, ok := attributeMap["department"]; ok {
		result[target] = u.Department
	}

	return result
}

// mapGroup applies the AttributeMap to transform source fields to target fields for a WeCom department.
// Default mapping when AttributeMap is empty:
//
//	id -> groupId, name -> displayName, parentid -> parentId
func (s *WeComSyncer) mapGroup(g weComDeptItem, attributeMap map[string]string) map[string]interface{} {
	result := make(map[string]interface{})

	if len(attributeMap) == 0 {
		// Default WeCom -> GuliangIAM field mapping
		result["groupId"] = g.ID
		result["displayName"] = g.Name
		result["parentId"] = g.ParentID
		return result
	}

	// Apply custom mapping
	// Source fields: id, name, parentid
	if target, ok := attributeMap["id"]; ok {
		result[target] = g.ID
	}
	if target, ok := attributeMap["name"]; ok {
		result[target] = g.Name
	}
	if target, ok := attributeMap["parentid"]; ok {
		result[target] = g.ParentID
	}

	return result
}

// listDepartments lists all departments from WeCom.
func (s *WeComSyncer) listDepartments(ctx context.Context, token string, config *SyncerConfig) (departments []weComDeptItem, errs []string) {
	baseURL := "https://qyapi.weixin.qq.com/cgi-bin/department/list"
	if config.APIURL != "" {
		baseURL = config.APIURL + "/department/list"
	}

	allDepts := make([]weComDeptItem, 0, 16)
	var errors []string

	u, _ := url.Parse(baseURL)
	q := u.Query()
	q.Set("access_token", token)
	if config.Filter != "" {
		q.Set("id", config.Filter) // parent department ID filter
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		errors = append(errors, fmt.Sprintf("wecom create dept list request: %v", err))
		return allDepts, errors
	}

	client := ssrf.SafeHTTPClient(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		errors = append(errors, fmt.Sprintf("wecom dept list request: %v", err))
		return allDepts, errors
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		errors = append(errors, fmt.Sprintf("wecom read dept list response: %v", err))
		return allDepts, errors
	}

	var deptResp weComDepartmentResponse
	if err := json.Unmarshal(body, &deptResp); err != nil {
		errors = append(errors, fmt.Sprintf("wecom parse dept list response: %v", err))
		return allDepts, errors
	}

	if deptResp.ErrCode != 0 {
		errors = append(errors, fmt.Sprintf("wecom dept list error: code=%d, msg=%s", deptResp.ErrCode, deptResp.ErrMsg))
		return allDepts, errors
	}

	allDepts = append(allDepts, deptResp.Department...)

	return allDepts, errors
}

type weComDeptItem = struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	ParentID int    `json:"parentid"`
	Order    int    `json:"order"`
}

// listAllWeComDepartmentIDs collects all department IDs.
func (s *WeComSyncer) listAllWeComDepartmentIDs(ctx context.Context, token string, config *SyncerConfig) []int {
	depts, _ := s.listDepartments(ctx, token, config)
	ids := make([]int, 0, len(depts)+1)
	ids = append(ids, 1) // root department
	for _, d := range depts {
		ids = append(ids, d.ID)
	}
	return ids
}

type weComUserItem = struct {
	UserID     string `json:"userid"`
	Name       string `json:"name"`
	Mobile     string `json:"mobile"`
	Email      string `json:"email"`
	Department []int  `json:"department"`
	Status     int    `json:"status"`
	ModifyTime int64  `json:"modify_time"` // Unix timestamp in seconds, if available
}

// listDepartmentUsers lists users in a specific department.
func (s *WeComSyncer) listDepartmentUsers(ctx context.Context, token string, config *SyncerConfig, deptID int) (users []weComUserItem, errs []string) {
	baseURL := "https://qyapi.weixin.qq.com/cgi-bin/user/list"
	if config.APIURL != "" {
		baseURL = config.APIURL + "/user/list"
	}

	allUsers := make([]weComUserItem, 0, 32)
	var errors []string

	u, _ := url.Parse(baseURL)
	q := u.Query()
	q.Set("access_token", token)
	q.Set("department_id", fmt.Sprintf("%d", deptID))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		errors = append(errors, fmt.Sprintf("wecom create user list request: %v", err))
		return allUsers, errors
	}

	client := ssrf.SafeHTTPClient(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		errors = append(errors, fmt.Sprintf("wecom user list request: %v", err))
		return allUsers, errors
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		errors = append(errors, fmt.Sprintf("wecom read user list response: %v", err))
		return allUsers, errors
	}

	var userResp weComUserListResponse
	if err := json.Unmarshal(body, &userResp); err != nil {
		errors = append(errors, fmt.Sprintf("wecom parse user list response: %v", err))
		return allUsers, errors
	}

	if userResp.ErrCode != 0 {
		errors = append(errors, fmt.Sprintf("wecom user list error: code=%d, msg=%s", userResp.ErrCode, userResp.ErrMsg))
		return allUsers, errors
	}

	allUsers = append(allUsers, userResp.UserList...)

	return allUsers, errors
}

// Ensure WeComSyncer implements Engine.
var _ Engine = (*WeComSyncer)(nil)
