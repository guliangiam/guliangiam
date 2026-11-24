package ldaps

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/ldaps"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock      clock.Clock
	UUID       platformuuid.Generator
	UOW        appuow.UnitOfWork
	Scheduler  domain.SyncScheduler
	LDAPClient domain.LDAPTestClient
	LDAPSyncer domain.LDAPSyncer
}

func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork appuow.UnitOfWork) Service {
	return Service{Clock: clk, UUID: uuidGenerator, UOW: unitOfWork}
}

func NewServiceWithScheduler(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork appuow.UnitOfWork, scheduler domain.SyncScheduler) Service {
	return Service{Clock: clk, UUID: uuidGenerator, UOW: unitOfWork, Scheduler: scheduler}
}

func (s Service) StartAutoSync(ctx context.Context, id string) error {
	ctx = tenantctx.System(ctx)

	if s.Scheduler == nil {
		return nil
	}

	l, err := s.UOW.Ldaps().GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !l.IsEnabled || l.SyncInterval <= 0 {
		return nil
	}

	syncerConfig := s.buildSyncerConfig(l)
	return s.Scheduler.AddDomainJob(l.ID, syncerConfig, func(ctx context.Context, cfg *domain.SyncerConfig) error {
		_, err := s.SyncLdap(ctx, id, nil, l.OrganizationID)
		return err
	})
}

func (s Service) StopAutoSync(id string) {
	if s.Scheduler != nil {
		s.Scheduler.RemoveJob(id)
	}
}

func (s Service) ScheduleAllEnabledAutoSyncs(ctx context.Context) error {
	if s.Scheduler == nil {
		return nil
	}

	// Scheduling every enabled config is an internal "whole set" read: walk the
	// pages instead of asking for PageSize 1000, which the repository silently
	// clamped to 100 and left the remaining configs unscheduled.
	ldapConfigs, err := platformpagination.CollectAll(func(page, pageSize int) ([]*domain.Ldap, int64, error) {
		return s.UOW.Ldaps().List(ctx, domain.ListQuery{Page: page, PageSize: pageSize})
	})
	if err != nil {
		return err
	}

	for _, l := range ldapConfigs {
		if l.IsEnabled && l.SyncInterval > 0 {
			syncerConfig := s.buildSyncerConfig(l)
			_ = s.Scheduler.AddDomainJob(l.ID, syncerConfig, func(ctx context.Context, cfg *domain.SyncerConfig) error {
				_, err := s.SyncLdap(ctx, l.ID, nil, l.OrganizationID)
				return err
			})
		}
	}

	return nil
}

func (s Service) buildSyncerConfig(l *domain.Ldap) *domain.SyncerConfig {
	return &domain.SyncerConfig{
		ProviderType:   identityTypeLDAP,
		Host:           l.Host,
		Port:           l.Port,
		Username:       l.BindDN,
		Password:       l.BindPassword,
		BaseDN:         l.BaseDN,
		Filter:         l.Filter,
		OrganizationID: l.OrganizationID,
		UseTLS:         l.EnableSsl,
	}
}

// 安全边界：PEP 中间件（APIAuthorizationMiddleware）已通过 Casbin 验证了权限。
// RLS 提升只在 service 内部生效（事务内 LOCAL set_config），事务结束自动清除。
