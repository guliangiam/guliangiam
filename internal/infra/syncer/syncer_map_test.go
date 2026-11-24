package syncer

import (
	"testing"

	domainsyncers "github.com/guliangiam/guliangiam/internal/domain/syncers"
	"github.com/stretchr/testify/assert"
)

// --- AzureAD Syncer ---

func TestAzureADSyncer_MapUser_Default(t *testing.T) {
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
	result := s.mapUser(item, nil)
	assert.Equal(t, "user-1", result["id"])
	assert.Equal(t, "john@contoso.com", result["username"])
	assert.Equal(t, "John Doe", result["displayName"])
	assert.Equal(t, "john@contoso.com", result["email"])
	assert.Equal(t, "John", result["givenName"])
	assert.Equal(t, "Doe", result["surname"])
	assert.Equal(t, true, result["enabled"])
}

func TestAzureADSyncer_MapGroup_Default(t *testing.T) {
	s := &AzureADSyncer{}
	item := azureADGroupItem{
		ID:              "group-1",
		DisplayName:     "Admins",
		Description:     "Admin group",
		Mail:            "admins@contoso.com",
		MailEnabled:     true,
		SecurityEnabled: true,
	}
	result := s.mapGroup(item, nil)
	assert.Equal(t, "group-1", result["id"])
	assert.Equal(t, "Admins", result["name"])
	assert.Equal(t, "Admin group", result["description"])
	assert.Equal(t, "admins@contoso.com", result["email"])
	assert.Equal(t, true, result["mailEnabled"])
	assert.Equal(t, true, result["securityEnabled"])
}

// --- Google Syncer ---

func TestGoogleSyncer_MapUser_Default(t *testing.T) {
	s := &GoogleSyncer{}
	item := googleUserItem{
		ID:           "user-1",
		PrimaryEmail: "john@google.com",
	}
	item.Name.GivenName = "John"
	item.Name.FamilyName = "Doe"
	item.Name.FullName = "John Doe"
	result := s.mapUser(item, nil)
	assert.Equal(t, "user-1", result["id"])
	assert.Equal(t, "john@google.com", result["username"])
	assert.Equal(t, "John Doe", result["displayName"])
	assert.Equal(t, "John", result["givenName"])
	assert.Equal(t, "Doe", result["familyName"])
}

func TestGoogleSyncer_MapGroup_Default(t *testing.T) {
	s := &GoogleSyncer{}
	item := googleGroupItem{
		ID:          "group-1",
		Email:       "admins@google.com",
		Name:        "Admins",
		Description: "Admin group",
	}
	result := s.mapGroup(item, nil)
	assert.Equal(t, "group-1", result["id"])
	assert.Equal(t, "admins@google.com", result["name"])
	assert.Equal(t, "Admins", result["displayName"])
	assert.Equal(t, "Admin group", result["description"])
}

// --- DingTalk Syncer ---

func TestDingTalkSyncer_MapUser_Default(t *testing.T) {
	s := &DingTalkSyncer{}
	item := dingTalkUserItem{
		UserID:     "user-1",
		Name:       "John Doe",
		Mobile:     "13800138000",
		Email:      "john@dingtalk.com",
		DeptIDList: []int{1, 2},
	}
	result := s.mapUser(item, nil)
	assert.Equal(t, "user-1", result["username"])
	assert.Equal(t, "John Doe", result["displayName"])
	assert.Equal(t, "13800138000", result["phone"])
	assert.Equal(t, "john@dingtalk.com", result["email"])
	assert.Equal(t, []int{1, 2}, result["groups"])
}

func TestDingTalkSyncer_MapGroup_Default(t *testing.T) {
	s := &DingTalkSyncer{}
	item := dingTalkDeptItem{
		DeptID:   1,
		Name:     "Engineering",
		ParentID: 0,
	}
	result := s.mapGroup(item, nil)
	assert.Equal(t, 1, result["groupId"])
	assert.Equal(t, "Engineering", result["displayName"])
	assert.Equal(t, 0, result["parentId"])
}

// --- Lark Syncer ---

func TestLarkSyncer_MapUser_Default(t *testing.T) {
	s := &LarkSyncer{}
	item := larkUserItem{
		UserID:        "user-1",
		Name:          "John Doe",
		Mobile:        "13800138000",
		Email:         "john@lark.com",
		DepartmentIDs: []string{"dept-1", "dept-2"},
	}
	result := s.mapUser(item, nil)
	assert.Equal(t, "user-1", result["username"])
	assert.Equal(t, "John Doe", result["displayName"])
	assert.Equal(t, "13800138000", result["phone"])
	assert.Equal(t, "john@lark.com", result["email"])
	assert.Equal(t, []string{"dept-1", "dept-2"}, result["groups"])
}

func TestLarkSyncer_MapGroup_Default(t *testing.T) {
	s := &LarkSyncer{}
	item := larkDeptItem{
		DepartmentID: "dept-1",
		Name:         "Engineering",
		ParentID:     "0",
	}
	result := s.mapGroup(item, nil)
	assert.Equal(t, "dept-1", result["groupId"])
	assert.Equal(t, "Engineering", result["displayName"])
	assert.Equal(t, "0", result["parentId"])
}

// --- WeCom Syncer ---

func TestWeComSyncer_MapUser_Default(t *testing.T) {
	s := &WeComSyncer{}
	item := weComUserItem{
		UserID:     "user-1",
		Name:       "John Doe",
		Mobile:     "13800138000",
		Email:      "john@wecom.com",
		Department: []int{1, 2},
	}
	result := s.mapUser(item, nil)
	assert.Equal(t, "user-1", result["username"])
	assert.Equal(t, "John Doe", result["displayName"])
	assert.Equal(t, "13800138000", result["phone"])
	assert.Equal(t, "john@wecom.com", result["email"])
	assert.Equal(t, []int{1, 2}, result["groups"])
}

func TestWeComSyncer_MapGroup_Default(t *testing.T) {
	s := &WeComSyncer{}
	item := weComDeptItem{
		ID:       1,
		Name:     "Engineering",
		ParentID: 0,
		Order:    1,
	}
	result := s.mapGroup(item, nil)
	assert.Equal(t, 1, result["groupId"])
	assert.Equal(t, "Engineering", result["displayName"])
	assert.Equal(t, 0, result["parentId"])
}

// --- Keycloak Syncer ---

func TestKeycloakSyncer_MapUser_Default(t *testing.T) {
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
	result := s.mapUser(item, nil)
	assert.Equal(t, "user-1", result["id"])
	assert.Equal(t, "jdoe", result["username"])
	assert.Equal(t, "John Doe", result["displayName"])
	assert.Equal(t, "jdoe@keycloak.org", result["email"])
	assert.Equal(t, true, result["enabled"])
	assert.Equal(t, true, result["emailVerified"])
}

func TestKeycloakSyncer_MapGroup_Default(t *testing.T) {
	s := &KeycloakSyncer{}
	item := keycloakGroupResponse{
		ID:   "group-1",
		Name: "Admins",
		Path: "/admins",
	}
	result := s.mapGroup(item, nil)
	assert.Equal(t, "group-1", result["id"])
	assert.Equal(t, "Admins", result["name"])
	assert.Equal(t, "/admins", result["path"])
}

func TestKeycloakSyncer_KeycloakAdminBaseURL(t *testing.T) {
	s := &KeycloakSyncer{}

	tests := []struct {
		name     string
		config   *SyncerConfig
		expected string
	}{
		{"api_url", &SyncerConfig{APIURL: "https://keycloak.example.com/admin/realms/master"}, "https://keycloak.example.com/admin/realms/master"},
		{"host_with_realm", &SyncerConfig{Host: "keycloak.example.com", BaseDN: "myrealm"}, "keycloak.example.com/admin/realms/myrealm"},
		{"host_no_realm", &SyncerConfig{Host: "keycloak.example.com"}, "keycloak.example.com/admin/realms/master"},
		{"empty", &SyncerConfig{}, "/admin/realms/master"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := s.keycloakAdminBaseURL(tc.config)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// --- AWS IAM Syncer ---

func TestAWSIAMSyncer_MapUser_Default(t *testing.T) {
	s := &AWSIAMSyncer{}
	item := awsIAMUserItem{
		UserName:   "jdoe",
		UserID:     "AIDA123456789",
		ARN:        "arn:aws:iam::123456789:user/jdoe",
		CreateDate: "2024-01-01T00:00:00Z",
		Path:       "/",
	}
	result := s.mapUser(item, nil)
	assert.Equal(t, "AIDA123456789", result["id"])
	assert.Equal(t, "jdoe", result["username"])
	assert.Equal(t, "arn:aws:iam::123456789:user/jdoe", result["arn"])
	assert.Equal(t, "/", result["path"])
	assert.Equal(t, "2024-01-01T00:00:00Z", result["createDate"])
}

func TestAWSIAMSyncer_MapGroup_Default(t *testing.T) {
	s := &AWSIAMSyncer{}
	item := awsIAMGroupItem{
		GroupName:  "Admins",
		GroupID:    "AGPA123456789",
		ARN:        "arn:aws:iam::123456789:group/Admins",
		CreateDate: "2024-01-01T00:00:00Z",
		Path:       "/",
	}
	result := s.mapGroup(item, nil)
	assert.Equal(t, "AGPA123456789", result["id"])
	assert.Equal(t, "Admins", result["name"])
	assert.Equal(t, "arn:aws:iam::123456789:group/Admins", result["arn"])
}

func TestAWSIAMSyncer_GetIAMEndpoint(t *testing.T) {
	s := &AWSIAMSyncer{}

	t.Run("api_url", func(t *testing.T) {
		result := s.getIAMEndpoint(&SyncerConfig{APIURL: "https://custom-iam.example.com"})
		assert.Equal(t, "https://custom-iam.example.com", result)
	})

	t.Run("custom_host", func(t *testing.T) {
		result := s.getIAMEndpoint(&SyncerConfig{Host: "iam.amazonaws.com"})
		assert.Equal(t, "iam.amazonaws.com", result)
	})

	t.Run("empty_host", func(t *testing.T) {
		result := s.getIAMEndpoint(&SyncerConfig{})
		assert.Equal(t, "https://iam.amazonaws.com/", result)
	})
}

// --- Adapter ---

func TestDomainConfigToInfra(t *testing.T) {
	domainConfig := &domainsyncers.SyncEngineConfig{
		ProviderType:   "ldap",
		Host:           "ldap.example.com",
		Port:           389,
		BaseDN:         "dc=example,dc=com",
		Username:       "cn=admin",
		Password:       "secret",
		Filter:         "(objectClass=person)",
		OrganizationID: "org-1",
		EnableSync:     true,
		SyncInterval:   300,
		IsReadOnly:     true,
		ClientID:       "client-1",
		ClientSecret:   "secret-1",
		TenantID:       "tenant-1",
		TokenURL:       "https://token.example.com",
		APIURL:         "https://api.example.com",
	}

	infraConfig := domainConfigToInfra(domainConfig)
	assert.Equal(t, "ldap", infraConfig.ProviderType)
	assert.Equal(t, "ldap.example.com", infraConfig.Host)
	assert.Equal(t, 389, infraConfig.Port)
	assert.Equal(t, "dc=example,dc=com", infraConfig.BaseDN)
	assert.Equal(t, "cn=admin", infraConfig.Username)
	assert.Equal(t, "secret", infraConfig.Password)
	assert.Equal(t, "(objectClass=person)", infraConfig.Filter)
	assert.Equal(t, "org-1", infraConfig.OrganizationID)
	assert.Equal(t, true, infraConfig.EnableSync)
	assert.Equal(t, 300, infraConfig.SyncInterval)
	assert.Equal(t, true, infraConfig.IsReadOnly)
	assert.Equal(t, "client-1", infraConfig.ClientID)
	assert.Equal(t, "secret-1", infraConfig.ClientSecret)
	assert.Equal(t, "tenant-1", infraConfig.TenantID)
	assert.Equal(t, "https://token.example.com", infraConfig.TokenURL)
	assert.Equal(t, "https://api.example.com", infraConfig.APIURL)
}

func TestInfraResultToDomain(t *testing.T) {
	t.Run("nil_result", func(t *testing.T) {
		result := infraResultToDomain(nil)
		assert.Nil(t, result)
	})

	t.Run("valid_result", func(t *testing.T) {
		infraResult := &SyncResult{
			TotalUsers:    100,
			AddedUsers:    10,
			UpdatedUsers:  20,
			FailedUsers:   2,
			TotalGroups:   50,
			AddedGroups:   5,
			UpdatedGroups: 3,
			FailedGroups:  1,
			Errors:        []string{"error1"},
		}
		domainResult := infraResultToDomain(infraResult)
		assert.Equal(t, 100, domainResult.TotalUsers)
		assert.Equal(t, 10, domainResult.AddedUsers)
		assert.Equal(t, 20, domainResult.UpdatedUsers)
		assert.Equal(t, 2, domainResult.FailedUsers)
		assert.Equal(t, 50, domainResult.TotalGroups)
		assert.Equal(t, 5, domainResult.AddedGroups)
		assert.Equal(t, 3, domainResult.UpdatedGroups)
		assert.Equal(t, 1, domainResult.FailedGroups)
		assert.Equal(t, []string{"error1"}, domainResult.Errors)
	})
}
