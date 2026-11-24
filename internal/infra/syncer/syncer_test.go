package syncer

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFactory_GetEngine_SupportedProviders(t *testing.T) {
	factory := NewFactory()

	providers := []string{
		"database", "ldap", "scim", "dingtalk", "lark",
		"wecom", "google", "okta", "azuread", "keycloak",
		"activedirectory", "ad", "awsiam",
	}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			engine, err := factory.GetEngine(provider)
			assert.NoError(t, err)
			assert.NotNil(t, engine)
		})
	}
}

func TestFactory_GetEngine_Unsupported(t *testing.T) {
	factory := NewFactory()
	_, err := factory.GetEngine("unsupported_provider")
	assert.Error(t, err)
}

func TestFactory_GetEngine_CaseInsensitive(t *testing.T) {
	factory := NewFactory()

	_, err := factory.GetEngine("DATABASE")
	assert.NoError(t, err)

	_, err = factory.GetEngine("LDAP")
	assert.NoError(t, err)

	_, err = factory.GetEngine("DingTalk")
	assert.NoError(t, err)
}

func TestFactory_GetEngine_Whitespace(t *testing.T) {
	factory := NewFactory()

	_, err := factory.GetEngine("  database  ")
	assert.NoError(t, err)

	_, err = factory.GetEngine(" ldap ")
	assert.NoError(t, err)
}

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	assert.NotNil(t, factory)
}

func TestNewScheduler(t *testing.T) {
	factory := NewFactory()
	scheduler := NewScheduler(factory)
	assert.NotNil(t, scheduler)
	assert.NotNil(t, scheduler.jobs)
}

func TestScheduler_AddAndRemoveJob(t *testing.T) {
	factory := NewFactory()
	scheduler := NewScheduler(factory)

	var called atomic.Int32
	err := scheduler.AddJob("sync-1", &SyncerConfig{SyncInterval: 3600}, func(ctx context.Context, config *SyncerConfig) error {
		called.Add(1)
		return nil
	})
	require.NoError(t, err)
	assert.Contains(t, scheduler.jobs, "sync-1")

	scheduler.RemoveJob("sync-1")
	assert.NotContains(t, scheduler.jobs, "sync-1")
}

func TestScheduler_AddJob_DefaultInterval(t *testing.T) {
	factory := NewFactory()
	scheduler := NewScheduler(factory)

	err := scheduler.AddJob("sync-default", &SyncerConfig{SyncInterval: 0}, func(ctx context.Context, config *SyncerConfig) error {
		return nil
	})
	require.NoError(t, err)
	assert.Contains(t, scheduler.jobs, "sync-default")

	scheduler.RemoveJob("sync-default")
}

func TestScheduler_AddJob_NegativeInterval(t *testing.T) {
	factory := NewFactory()
	scheduler := NewScheduler(factory)

	err := scheduler.AddJob("sync-neg", &SyncerConfig{SyncInterval: -100}, func(ctx context.Context, config *SyncerConfig) error {
		return nil
	})
	require.NoError(t, err)
	assert.Contains(t, scheduler.jobs, "sync-neg")

	scheduler.RemoveJob("sync-neg")
}

func TestScheduler_AddJob_ReplaceExisting(t *testing.T) {
	factory := NewFactory()
	scheduler := NewScheduler(factory)

	err := scheduler.AddJob("sync-replace", &SyncerConfig{SyncInterval: 3600}, func(ctx context.Context, config *SyncerConfig) error {
		return nil
	})
	require.NoError(t, err)

	err = scheduler.AddJob("sync-replace", &SyncerConfig{SyncInterval: 7200}, func(ctx context.Context, config *SyncerConfig) error {
		return nil
	})
	require.NoError(t, err)
	assert.Contains(t, scheduler.jobs, "sync-replace")
}

func TestScheduler_RemoveJob_NonExistent(t *testing.T) {
	factory := NewFactory()
	scheduler := NewScheduler(factory)

	scheduler.RemoveJob("nonexistent")
	assert.NotContains(t, scheduler.jobs, "nonexistent")
}

func TestScheduler_StartStop(t *testing.T) {
	factory := NewFactory()
	scheduler := NewScheduler(factory)

	scheduler.Start()
	time.Sleep(50 * time.Millisecond)
	scheduler.Stop()
}

func TestSyncerConfig_Fields(t *testing.T) {
	config := &SyncerConfig{
		ProviderType:   "ldap",
		Host:           "ldap.example.com",
		Port:           389,
		BaseDN:         "dc=example,dc=com",
		Username:       "cn=admin",
		Password:       "secret",
		Filter:         "(objectClass=person)",
		OrganizationID: "org-1",
		SyncInterval:   300,
		EnableSync:     true,
		IsReadOnly:     true,
	}

	assert.Equal(t, "ldap", config.ProviderType)
	assert.Equal(t, "ldap.example.com", config.Host)
	assert.Equal(t, 389, config.Port)
	assert.Equal(t, "dc=example,dc=com", config.BaseDN)
	assert.Equal(t, 300, config.SyncInterval)
	assert.True(t, config.EnableSync)
	assert.True(t, config.IsReadOnly)
}

func TestSyncResult_Fields(t *testing.T) {
	result := &SyncResult{
		TotalUsers:    100,
		AddedUsers:    10,
		UpdatedUsers:  20,
		FailedUsers:   2,
		TotalGroups:   50,
		AddedGroups:   5,
		UpdatedGroups: 3,
		FailedGroups:  1,
		Errors:        []string{"error1", "error2"},
	}

	assert.Equal(t, 100, result.TotalUsers)
	assert.Equal(t, 10, result.AddedUsers)
	assert.Equal(t, 2, result.FailedUsers)
	assert.Len(t, result.Errors, 2)
}

func TestNewFactoryAdapter(t *testing.T) {
	adapter := NewFactoryAdapter()
	assert.NotNil(t, adapter)
	assert.NotNil(t, adapter.factory)
}

func TestFactoryAdapter_GetEngine_Supported(t *testing.T) {
	adapter := NewFactoryAdapter()
	engine, err := adapter.GetEngine("database")
	assert.NoError(t, err)
	assert.NotNil(t, engine)
}

func TestFactoryAdapter_GetEngine_Unsupported(t *testing.T) {
	adapter := NewFactoryAdapter()
	_, err := adapter.GetEngine("unsupported")
	assert.Error(t, err)
}
