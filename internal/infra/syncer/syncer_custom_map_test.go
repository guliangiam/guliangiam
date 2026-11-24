package syncer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- AzureAD Custom Mapping ---

func TestAzureADSyncer_MapUser_CustomMapping(t *testing.T) {
	s := &AzureADSyncer{}
	item := azureADUserItem{
		ID:                "user-1",
		DisplayName:       "John Doe",
		UserPrincipalName: "john@contoso.com",
		Mail:              "john@contoso.com",
		GivenName:         "John",
		Surname:           "Doe",
		AccountEnabled:    true,
	}
	attributeMap := map[string]string{
		"id":                "custom_id",
		"userPrincipalName": "login",
		"displayName":       "full_name",
		"mail":              "email_addr",
		"givenName":         "first",
		"surname":           "last",
		"accountEnabled":    "active",
	}
	result := s.mapUser(item, attributeMap)
	assert.Equal(t, "user-1", result["custom_id"])
	assert.Equal(t, "john@contoso.com", result["login"])
	assert.Equal(t, "John Doe", result["full_name"])
	assert.Equal(t, "john@contoso.com", result["email_addr"])
	assert.Equal(t, "John", result["first"])
	assert.Equal(t, "Doe", result["last"])
	assert.Equal(t, true, result["active"])
}

func TestAzureADSyncer_MapGroup_CustomMapping(t *testing.T) {
	s := &AzureADSyncer{}
	item := azureADGroupItem{
		ID:              "group-1",
		DisplayName:     "Admins",
		Description:     "Admin group",
		Mail:            "admins@contoso.com",
		MailEnabled:     true,
		SecurityEnabled: true,
	}
	attributeMap := map[string]string{
		"id":              "custom_id",
		"displayName":     "group_name",
		"description":     "desc",
		"mail":            "email_addr",
		"mailEnabled":     "is_mail",
		"securityEnabled": "is_security",
	}
	result := s.mapGroup(item, attributeMap)
	assert.Equal(t, "group-1", result["custom_id"])
	assert.Equal(t, "Admins", result["group_name"])
	assert.Equal(t, "Admin group", result["desc"])
	assert.Equal(t, "admins@contoso.com", result["email_addr"])
	assert.Equal(t, true, result["is_mail"])
	assert.Equal(t, true, result["is_security"])
}

// --- Google Custom Mapping ---

func TestGoogleSyncer_MapUser_CustomMapping(t *testing.T) {
	s := &GoogleSyncer{}
	item := googleUserItem{
		ID:           "user-1",
		PrimaryEmail: "john@google.com",
		Suspended:    false,
		OrgUnitPath:  "/Engineering",
	}
	item.Name.GivenName = "John"
	item.Name.FamilyName = "Doe"
	item.Name.FullName = "John Doe"
	attributeMap := map[string]string{
		"id":              "custom_id",
		"primaryEmail":    "login",
		"name.fullName":   "full_name",
		"name.givenName":  "first",
		"name.familyName": "last",
		"suspended":       "disabled",
		"orgUnitPath":     "org_path",
	}
	result := s.mapUser(item, attributeMap)
	assert.Equal(t, "user-1", result["custom_id"])
	assert.Equal(t, "john@google.com", result["login"])
	assert.Equal(t, "John Doe", result["full_name"])
	assert.Equal(t, "John", result["first"])
	assert.Equal(t, "Doe", result["last"])
	assert.Equal(t, false, result["disabled"])
	assert.Equal(t, "/Engineering", result["org_path"])
}

func TestGoogleSyncer_MapGroup_CustomMapping(t *testing.T) {
	s := &GoogleSyncer{}
	item := googleGroupItem{
		ID:          "group-1",
		Email:       "admins@google.com",
		Name:        "Admins",
		Description: "Admin group",
	}
	attributeMap := map[string]string{
		"id":          "custom_id",
		"email":       "group_email",
		"name":        "group_name",
		"description": "desc",
	}
	result := s.mapGroup(item, attributeMap)
	assert.Equal(t, "group-1", result["custom_id"])
	assert.Equal(t, "admins@google.com", result["group_email"])
	assert.Equal(t, "Admins", result["group_name"])
	assert.Equal(t, "Admin group", result["desc"])
}

// --- DingTalk Custom Mapping ---

func TestDingTalkSyncer_MapUser_CustomMapping(t *testing.T) {
	s := &DingTalkSyncer{}
	item := dingTalkUserItem{
		UserID:     "user-1",
		Name:       "John Doe",
		Mobile:     "13800138000",
		Email:      "john@dingtalk.com",
		DeptIDList: []int{1, 2},
	}
	attributeMap := map[string]string{
		"userid":       "login",
		"name":         "full_name",
		"mobile":       "phone_number",
		"email":        "email_addr",
		"dept_id_list": "departments",
	}
	result := s.mapUser(item, attributeMap)
	assert.Equal(t, "user-1", result["login"])
	assert.Equal(t, "John Doe", result["full_name"])
	assert.Equal(t, "13800138000", result["phone_number"])
	assert.Equal(t, "john@dingtalk.com", result["email_addr"])
	assert.Equal(t, []int{1, 2}, result["departments"])
}

func TestDingTalkSyncer_MapGroup_CustomMapping(t *testing.T) {
	s := &DingTalkSyncer{}
	item := dingTalkDeptItem{
		DeptID:   1,
		Name:     "Engineering",
		ParentID: 0,
	}
	attributeMap := map[string]string{
		"dept_id":   "custom_id",
		"name":      "custom_name",
		"parent_id": "custom_parent",
	}
	result := s.mapGroup(item, attributeMap)
	assert.Equal(t, 1, result["custom_id"])
	assert.Equal(t, "Engineering", result["custom_name"])
	assert.Equal(t, 0, result["custom_parent"])
}

// --- Lark Custom Mapping ---

func TestLarkSyncer_MapUser_CustomMapping(t *testing.T) {
	s := &LarkSyncer{}
	item := larkUserItem{
		UserID:        "user-1",
		Name:          "John Doe",
		Mobile:        "13800138000",
		Email:         "john@lark.com",
		DepartmentIDs: []string{"dept-1", "dept-2"},
	}
	attributeMap := map[string]string{
		"user_id":        "login",
		"name":           "full_name",
		"mobile":         "phone_number",
		"email":          "email_addr",
		"department_ids": "departments",
	}
	result := s.mapUser(item, attributeMap)
	assert.Equal(t, "user-1", result["login"])
	assert.Equal(t, "John Doe", result["full_name"])
	assert.Equal(t, "13800138000", result["phone_number"])
	assert.Equal(t, "john@lark.com", result["email_addr"])
	assert.Equal(t, []string{"dept-1", "dept-2"}, result["departments"])
}

func TestLarkSyncer_MapGroup_CustomMapping(t *testing.T) {
	s := &LarkSyncer{}
	item := larkDeptItem{
		DepartmentID: "dept-1",
		Name:         "Engineering",
		ParentID:     "0",
	}
	attributeMap := map[string]string{
		"department_id":        "custom_id",
		"name":                 "custom_name",
		"parent_department_id": "custom_parent",
	}
	result := s.mapGroup(item, attributeMap)
	assert.Equal(t, "dept-1", result["custom_id"])
	assert.Equal(t, "Engineering", result["custom_name"])
	assert.Equal(t, "0", result["custom_parent"])
}

// --- WeCom Custom Mapping ---

func TestWeComSyncer_MapUser_CustomMapping(t *testing.T) {
	s := &WeComSyncer{}
	item := weComUserItem{
		UserID:     "user-1",
		Name:       "John Doe",
		Mobile:     "13800138000",
		Email:      "john@wecom.com",
		Department: []int{1, 2},
	}
	attributeMap := map[string]string{
		"userid":     "login",
		"name":       "full_name",
		"mobile":     "phone_number",
		"email":      "email_addr",
		"department": "departments",
	}
	result := s.mapUser(item, attributeMap)
	assert.Equal(t, "user-1", result["login"])
	assert.Equal(t, "John Doe", result["full_name"])
	assert.Equal(t, "13800138000", result["phone_number"])
	assert.Equal(t, "john@wecom.com", result["email_addr"])
	assert.Equal(t, []int{1, 2}, result["departments"])
}

func TestWeComSyncer_MapGroup_CustomMapping(t *testing.T) {
	s := &WeComSyncer{}
	item := weComDeptItem{
		ID:       1,
		Name:     "Engineering",
		ParentID: 0,
	}
	attributeMap := map[string]string{
		"id":       "custom_id",
		"name":     "custom_name",
		"parentid": "custom_parent",
	}
	result := s.mapGroup(item, attributeMap)
	assert.Equal(t, 1, result["custom_id"])
	assert.Equal(t, "Engineering", result["custom_name"])
	assert.Equal(t, 0, result["custom_parent"])
}

// --- Keycloak Custom Mapping ---

func TestKeycloakSyncer_MapUser_CustomMapping(t *testing.T) {
	s := &KeycloakSyncer{}
	item := keycloakUserResponse{
		ID:            "user-1",
		Username:      "jdoe",
		Enabled:       true,
		Email:         "jdoe@keycloak.org",
		FirstName:     "John",
		LastName:      "Doe",
		EmailVerified: true,
	}
	attributeMap := map[string]string{
		"id":            "custom_id",
		"username":      "login",
		"displayName":   "full_name",
		"firstName":     "first",
		"lastName":      "last",
		"email":         "email_addr",
		"enabled":       "active",
		"emailVerified": "verified",
	}
	result := s.mapUser(item, attributeMap)
	assert.Equal(t, "user-1", result["custom_id"])
	assert.Equal(t, "jdoe", result["login"])
	assert.Equal(t, "John Doe", result["full_name"])
	assert.Equal(t, "John", result["first"])
	assert.Equal(t, "Doe", result["last"])
	assert.Equal(t, "jdoe@keycloak.org", result["email_addr"])
	assert.Equal(t, true, result["active"])
	assert.Equal(t, true, result["verified"])
}

func TestKeycloakSyncer_MapUser_DisplayName_FirstNameOnly(t *testing.T) {
	s := &KeycloakSyncer{}
	item := keycloakUserResponse{
		FirstName: "John",
		LastName:  "",
	}
	result := s.mapUser(item, nil)
	assert.Equal(t, "John", result["displayName"])
}

func TestKeycloakSyncer_MapUser_DisplayName_LastNameOnly(t *testing.T) {
	s := &KeycloakSyncer{}
	item := keycloakUserResponse{
		FirstName: "",
		LastName:  "Doe",
	}
	result := s.mapUser(item, nil)
	assert.Equal(t, "Doe", result["displayName"])
}

func TestKeycloakSyncer_MapUser_DisplayName_BothNames(t *testing.T) {
	s := &KeycloakSyncer{}
	item := keycloakUserResponse{
		FirstName: "John",
		LastName:  "Doe",
	}
	result := s.mapUser(item, nil)
	assert.Equal(t, "John Doe", result["displayName"])
}

func TestKeycloakSyncer_MapGroup_CustomMapping(t *testing.T) {
	s := &KeycloakSyncer{}
	item := keycloakGroupResponse{
		ID:   "group-1",
		Name: "Admins",
		Path: "/admins",
	}
	attributeMap := map[string]string{
		"id":   "custom_id",
		"name": "group_name",
		"path": "group_path",
	}
	result := s.mapGroup(item, attributeMap)
	assert.Equal(t, "group-1", result["custom_id"])
	assert.Equal(t, "Admins", result["group_name"])
	assert.Equal(t, "/admins", result["group_path"])
}
