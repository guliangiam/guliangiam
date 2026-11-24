package syncer

import (
	"context"

	domainsyncers "github.com/guliangiam/guliangiam/internal/domain/syncers"
)

var _ domainsyncers.SyncEngineFactory = (*FactoryAdapter)(nil)

type FactoryAdapter struct {
	factory *Factory
}

func NewFactoryAdapter() *FactoryAdapter {
	return &FactoryAdapter{factory: NewFactory()}
}

func (a *FactoryAdapter) GetEngine(providerType string) (domainsyncers.SyncEngine, error) {
	engine, err := a.factory.GetEngine(providerType)
	if err != nil {
		return nil, err
	}
	return &engineAdapter{engine: engine}, nil
}

type engineAdapter struct {
	engine Engine
}

var _ domainsyncers.SyncEngine = (*engineAdapter)(nil)

func (a *engineAdapter) SyncUsers(ctx context.Context, config *domainsyncers.SyncEngineConfig) (*domainsyncers.SyncEngineResult, error) {
	infraConfig := domainConfigToInfra(config)
	result, err := a.engine.SyncUsers(ctx, infraConfig)
	if err != nil {
		return nil, err
	}
	return infraResultToDomain(result), nil
}

func (a *engineAdapter) SyncGroups(ctx context.Context, config *domainsyncers.SyncEngineConfig) (*domainsyncers.SyncEngineResult, error) {
	infraConfig := domainConfigToInfra(config)
	result, err := a.engine.SyncGroups(ctx, infraConfig)
	if err != nil {
		return nil, err
	}
	return infraResultToDomain(result), nil
}

func (a *engineAdapter) TestConnection(ctx context.Context, config *domainsyncers.SyncEngineConfig) error {
	infraConfig := domainConfigToInfra(config)
	return a.engine.TestConnection(ctx, infraConfig)
}

func domainConfigToInfra(c *domainsyncers.SyncEngineConfig) *SyncerConfig {
	return &SyncerConfig{
		ProviderType:   c.ProviderType,
		Host:           c.Host,
		Port:           c.Port,
		BaseDN:         c.BaseDN,
		Username:       c.Username,
		Password:       c.Password,
		Filter:         c.Filter,
		AttributeMap:   c.AttributeMap,
		OrganizationID: c.OrganizationID,
		EnableSync:     c.EnableSync,
		SyncInterval:   c.SyncInterval,
		IsReadOnly:     c.IsReadOnly,
		LastSyncAt:     c.LastSyncAt,
		ClientID:       c.ClientID,
		ClientSecret:   c.ClientSecret,
		TenantID:       c.TenantID,
		TokenURL:       c.TokenURL,
		APIURL:         c.APIURL,
		OnUserSynced:   c.OnUserSynced,
		OnGroupSynced:  c.OnGroupSynced,
	}
}

func infraResultToDomain(r *SyncResult) *domainsyncers.SyncEngineResult {
	if r == nil {
		return nil
	}
	return &domainsyncers.SyncEngineResult{
		TotalUsers:    r.TotalUsers,
		AddedUsers:    r.AddedUsers,
		UpdatedUsers:  r.UpdatedUsers,
		FailedUsers:   r.FailedUsers,
		TotalGroups:   r.TotalGroups,
		AddedGroups:   r.AddedGroups,
		UpdatedGroups: r.UpdatedGroups,
		FailedGroups:  r.FailedGroups,
		Errors:        r.Errors,
	}
}
