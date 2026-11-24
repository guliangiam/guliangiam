package syncers

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	domain "github.com/guliangiam/guliangiam/internal/domain/syncers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func testNow() time.Time {
	return time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
}

func errNew(msg string) error {
	return errors.New(msg)
}

func TestDomainToSyncerConfig_Basic(t *testing.T) {
	syncer := &domain.Syncer{
		Type:           "okta",
		Host:           "  example.okta.com  ",
		Port:           443,
		Database:       "  dc=example,dc=com  ",
		Username:       "  user  ",
		Password:       "  pass  ",
		TableName:      "  filter  ",
		OrganizationID: "  org1  ",
		IsEnabled:      true,
		SyncInterval:   300,
	}

	config := domainToSyncerConfig(syncer)

	assert.Equal(t, "okta", config.ProviderType)
	assert.Equal(t, "example.okta.com", config.Host)
	assert.Equal(t, 443, config.Port)
	assert.Equal(t, "dc=example,dc=com", config.BaseDN)
	assert.Equal(t, "user", config.Username)
	assert.Equal(t, "pass", config.Password)
	assert.Equal(t, "filter", config.Filter)
	assert.Equal(t, "org1", config.OrganizationID)
	assert.True(t, config.EnableSync)
	assert.Equal(t, 300, config.SyncInterval)
}

func TestDomainToSyncerConfig_FieldMappingArray(t *testing.T) {
	mappings := []domain.FieldMapping{
		{SourceField: "mail", TargetField: "email"},
		{SourceField: "cn", TargetField: "displayName"},
	}
	mappingJSON, _ := json.Marshal(mappings)

	syncer := &domain.Syncer{
		Type:         "ldap",
		Host:         "ldap.example.com",
		FieldMapping: string(mappingJSON),
	}

	config := domainToSyncerConfig(syncer)
	assert.Equal(t, "email", config.AttributeMap["mail"])
	assert.Equal(t, "displayName", config.AttributeMap["cn"])
}

func TestDomainToSyncerConfig_FieldMappingEmptySource(t *testing.T) {
	mappings := []domain.FieldMapping{
		{SourceField: "", TargetField: "email"},
		{SourceField: "cn", TargetField: ""},
	}
	mappingJSON, _ := json.Marshal(mappings)

	syncer := &domain.Syncer{
		Type:         "ldap",
		Host:         "ldap.example.com",
		FieldMapping: string(mappingJSON),
	}

	config := domainToSyncerConfig(syncer)
	// Empty source/target fields are skipped, so AttributeMap is empty map (not nil)
	assert.Empty(t, config.AttributeMap)
}

func TestDomainToSyncerConfig_FieldMappingMap(t *testing.T) {
	attrMap := map[string]string{
		"mail": "email",
		"cn":   "displayName",
	}
	mappingJSON, _ := json.Marshal(attrMap)

	// When FieldMapping is not an array, it should try map format
	syncer := &domain.Syncer{
		Type:         "ldap",
		Host:         "ldap.example.com",
		FieldMapping: string(mappingJSON),
	}

	config := domainToSyncerConfig(syncer)
	assert.Equal(t, "email", config.AttributeMap["mail"])
	assert.Equal(t, "displayName", config.AttributeMap["cn"])
}

func TestDomainToSyncerConfig_FieldMappingInvalid(t *testing.T) {
	syncer := &domain.Syncer{
		Type:         "ldap",
		Host:         "ldap.example.com",
		FieldMapping: "not valid json{{{",
	}

	config := domainToSyncerConfig(syncer)
	assert.Nil(t, config.AttributeMap)
}

func TestDomainToSyncerConfig_EmptyFieldMapping(t *testing.T) {
	syncer := &domain.Syncer{
		Type:         "ldap",
		Host:         "ldap.example.com",
		FieldMapping: "   ",
	}

	config := domainToSyncerConfig(syncer)
	assert.Nil(t, config.AttributeMap)
}

func TestDomainToSyncerConfig_LastSyncAt(t *testing.T) {
	now := testNow()
	syncer := &domain.Syncer{
		Type:       "ldap",
		Host:       "ldap.example.com",
		LastSyncAt: &now,
	}

	config := domainToSyncerConfig(syncer)
	assert.NotNil(t, config.LastSyncAt)
}

func TestIsSyncerRunning(t *testing.T) {
	svc := Service{mu: &sync.Mutex{}, running: map[string]struct{}{}}

	assert.False(t, svc.isSyncerRunning("id1"))

	svc.running["id1"] = struct{}{}
	assert.True(t, svc.isSyncerRunning("id1"))
	assert.False(t, svc.isSyncerRunning("id2"))
}

func TestRetryableSyncError_Nil(t *testing.T) {
	assert.False(t, retryableSyncError(nil))
}

func TestRetryableSyncError_Timeout(t *testing.T) {
	assert.True(t, retryableSyncError(errNew("connection timeout")))
}

func TestRetryableSyncError_ConnectionRefused(t *testing.T) {
	assert.True(t, retryableSyncError(errNew("connection refused")))
}

func TestRetryableSyncError_Temporary(t *testing.T) {
	assert.True(t, retryableSyncError(errNew("temporary failure")))
}

func TestRetryableSyncError_IOTimeout(t *testing.T) {
	assert.True(t, retryableSyncError(errNew("i/o timeout")))
}

func TestRetryableSyncError_Network(t *testing.T) {
	assert.True(t, retryableSyncError(errNew("network error")))
}

func TestRetryableSyncError_Other(t *testing.T) {
	assert.False(t, retryableSyncError(errNew("invalid argument")))
}

func TestExtractString_String(t *testing.T) {
	m := map[string]interface{}{"key": "value"}
	assert.Equal(t, "value", extractString(m, "key"))
}

func TestExtractString_StringPtr(t *testing.T) {
	s := "value"
	m := map[string]interface{}{"key": &s}
	assert.Equal(t, "value", extractString(m, "key"))
}

func TestExtractString_StringPtrNil(t *testing.T) {
	var s *string
	m := map[string]interface{}{"key": s}
	assert.Equal(t, "", extractString(m, "key"))
}

func TestExtractString_OtherType(t *testing.T) {
	m := map[string]interface{}{"key": 123}
	assert.Equal(t, "123", extractString(m, "key"))
}

func TestExtractString_Missing(t *testing.T) {
	m := map[string]interface{}{"other": "value"}
	assert.Equal(t, "", extractString(m, "key"))
}

func TestSyncerRunResult_Fields(t *testing.T) {
	r := SyncerRunResult{}
	assert.Nil(t, r.UserSync)
	assert.Nil(t, r.GroupSync)
}

func TestGroupWriteBack_EmptyNameAnswers422(t *testing.T) {
	svc := &Service{}
	fn := svc.createGroupWriteBack()
	_, err := fn(context.Background(), "org-1", "ext-1", map[string]interface{}{"name": ""})
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, "name", appErr.Meta["field"])
	assert.Equal(t, "ext-1", appErr.Meta["externalID"])
}
