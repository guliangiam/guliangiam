package adapters

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/adapters"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type ListAdaptersQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
	DatabaseType   string
}

type CreateAdapterCommand struct {
	OrganizationID string
	Name           string
	DisplayName    string
	Description    string
	Type           string
	Server         string
	Database       string
	TableName      string
	Username       string
	Password       string
	// Phase 2 fields
	Owner        string
	UseSameDb    bool
	DatabaseType string
	Host         string
	Port         *int
	DbUser       string
	DbPassword   string
	DatabaseName string
	Policy       string
	IsEnabled    bool
}

type PatchAdapterCommand struct {
	ID          string
	DisplayName *string
	Description *string
	Type        *string
	Server      *string
	Database    *string
	TableName   *string
	Username    *string
	Password    *string
	Revision    int64
	// Phase 2 fields
	Owner        *string
	UseSameDb    *bool
	DatabaseType *string
	Host         *string
	Port         *int
	DbUser       *string
	DbPassword   *string
	DatabaseName *string
	Policy       *string
	IsEnabled    *bool
}

func (s *Service) ListAdapters(ctx context.Context, query ListAdaptersQuery) ([]AdapterListItem, int64, error) {
	adapters, total, err := s.UOW.Adapters().List(ctx, domain.ListQuery(query))
	if err != nil {
		return nil, 0, err
	}
	items := make([]AdapterListItem, 0, len(adapters))
	for _, a := range adapters {
		items = append(items, toListItem(a))
	}
	return items, total, nil
}

func requireOrganizationID(organizationID string) error {
	if strings.TrimSpace(organizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	return nil
}

func (s *Service) GetAdapter(ctx context.Context, organizationID, id string) (*AdapterDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	// 租户谓词在仓储层显式生效；MySQL 无 RLS，不能依赖数据库兜底。
	a, err := s.UOW.Adapters().GetByID(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	detail := toDetail(a)
	return &detail, nil
}

func (s *Service) CreateAdapter(ctx context.Context, command CreateAdapterCommand) error {
	if strings.TrimSpace(command.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(command.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}

	id, err := s.UUID.NewString()
	if err != nil {
		return err
	}
	now := s.Clock.Now()

	adapter := &domain.Adapter{
		ID:             id,
		OrganizationID: command.OrganizationID,
		Name:           command.Name,
		DisplayName:    command.DisplayName,
		Description:    command.Description,
		Type:           command.Type,
		Server:         command.Server,
		Database:       command.Database,
		TableName:      command.TableName,
		Username:       command.Username,
		Password:       command.Password,
		Owner:          command.Owner,
		UseSameDb:      command.UseSameDb,
		DatabaseType:   command.DatabaseType,
		Host:           command.Host,
		Port:           command.Port,
		DbUser:         command.DbUser,
		DbPassword:     command.DbPassword,
		DatabaseName:   command.DatabaseName,
		Policy:         command.Policy,
		IsEnabled:      command.IsEnabled,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Adapters().Create(ctx, adapter); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			adapter.OrganizationID, "",
			"adapters.create", "adapters", adapter.ID,
			"adapters", "adapters.create",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

func (s *Service) PatchAdapter(ctx context.Context, organizationID string, command PatchAdapterCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		adapter, err := tx.Adapters().GetByID(ctx, command.ID, organizationID)
		if err != nil {
			return err
		}
		if command.Revision > 0 && adapter.Revision != command.Revision {
			return apperrors.NewConflict("adapter revision mismatch")
		}
		if command.DisplayName != nil {
			adapter.DisplayName = *command.DisplayName
		}
		if command.Description != nil {
			adapter.Description = *command.Description
		}
		if command.Type != nil {
			adapter.Type = *command.Type
		}
		if command.Server != nil {
			adapter.Server = *command.Server
		}
		if command.Database != nil {
			adapter.Database = *command.Database
		}
		if command.TableName != nil {
			adapter.TableName = *command.TableName
		}
		if command.Username != nil {
			adapter.Username = *command.Username
		}
		if command.Password != nil {
			adapter.Password = *command.Password
		}
		// Phase 2 field patching
		if command.Owner != nil {
			adapter.Owner = *command.Owner
		}
		if command.UseSameDb != nil {
			adapter.UseSameDb = *command.UseSameDb
		}
		if command.DatabaseType != nil {
			adapter.DatabaseType = *command.DatabaseType
		}
		if command.Host != nil {
			adapter.Host = *command.Host
		}
		if command.Port != nil {
			adapter.Port = command.Port
		}
		if command.DbUser != nil {
			adapter.DbUser = *command.DbUser
		}
		if command.DbPassword != nil {
			adapter.DbPassword = *command.DbPassword
		}
		if command.DatabaseName != nil {
			adapter.DatabaseName = *command.DatabaseName
		}
		if command.Policy != nil {
			adapter.Policy = *command.Policy
		}
		if command.IsEnabled != nil {
			adapter.IsEnabled = *command.IsEnabled
		}
		adapter.UpdatedAt = s.Clock.Now()
		adapter.Revision++
		if err := tx.Adapters().Update(ctx, adapter); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			adapter.OrganizationID, "",
			"adapters.update", "adapters", adapter.ID,
			"adapters", "adapters.update",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

func (s *Service) DeleteAdapter(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		adapter, err := tx.Adapters().GetByID(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if err := tx.Adapters().Delete(ctx, id, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			adapter.OrganizationID, "",
			"adapters.delete", "adapters", id,
			"adapters", "adapters.delete",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

// TestConnectionCommand specifies the database connection parameters to test.
type TestConnectionCommand struct {
	DatabaseType string
	Host         string
	Port         int
	Username     string
	Password     string
	DatabaseName string
}

// TestConnectionResult contains the result of a database connection test.
type TestConnectionResult struct {
	Connected bool
	Error     string
}

// TestConnection attempts to connect to the specified database using the provided parameters.
// If the request fields are partially empty, it falls back to the adapter's stored configuration.
// The adapter must belong to the trusted tenant: a foreign adapter is reported as not-found
// before its stored credentials are used.
func (s *Service) TestConnection(ctx context.Context, organizationID, adapterID string, command TestConnectionCommand) (*TestConnectionResult, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	adapter, err := s.UOW.Adapters().GetByID(ctx, adapterID, organizationID)
	if err != nil {
		return nil, err
	}

	// Use command fields if provided, otherwise fall back to adapter's stored config
	host := command.Host
	if host == "" {
		host = adapter.Host
		if host == "" {
			host = adapter.Server
		}
	}
	port := command.Port
	if port == 0 {
		if adapter.Port != nil {
			port = *adapter.Port
		}
	}
	username := command.Username
	if username == "" {
		username = adapter.DbUser
		if username == "" {
			username = adapter.Username
		}
	}
	password := command.Password
	if password == "" {
		password = adapter.DbPassword
		if password == "" {
			password = adapter.Password
		}
	}
	databaseName := command.DatabaseName
	if databaseName == "" {
		databaseName = adapter.DatabaseName
		if databaseName == "" {
			databaseName = adapter.Database
		}
	}
	databaseType := command.DatabaseType
	if databaseType == "" {
		databaseType = adapter.DatabaseType
		if databaseType == "" {
			databaseType = adapter.Type
		}
	}

	return s.testDatabaseConnection(ctx, databaseType, host, port, username, password, databaseName)
}

// ─── Policy operations (delegated to CasbinManager) ─────────────

func (s *Service) GetPolicies(ctx context.Context, organizationID, adapterID string) ([][]string, error) {
	adapter, err := s.getAdapter(ctx, organizationID, adapterID)
	if err != nil {
		return nil, err
	}
	modelText := adapter.Policy
	return s.CasbinManager.GetPolicies(adapterID, modelText)
}

func (s *Service) GetFilteredPolicies(ctx context.Context, organizationID, adapterID string, fieldIndex int, fieldValues ...string) ([][]string, error) {
	adapter, err := s.getAdapter(ctx, organizationID, adapterID)
	if err != nil {
		return nil, err
	}
	modelText := adapter.Policy
	return s.CasbinManager.GetFilteredPolicies(adapterID, modelText, fieldIndex, fieldValues...)
}

func (s *Service) AddPolicy(ctx context.Context, organizationID, adapterID string, params []string) error {
	adapter, err := s.getAdapter(ctx, organizationID, adapterID)
	if err != nil {
		return err
	}
	modelText := adapter.Policy
	return s.CasbinManager.AddPolicy(adapterID, modelText, params)
}

func (s *Service) RemovePolicy(ctx context.Context, organizationID, adapterID string, params []string) error {
	adapter, err := s.getAdapter(ctx, organizationID, adapterID)
	if err != nil {
		return err
	}
	modelText := adapter.Policy
	return s.CasbinManager.RemovePolicy(adapterID, modelText, params)
}

func (s *Service) UpdatePolicy(ctx context.Context, organizationID, adapterID string, oldPolicy, newPolicy []string) error {
	adapter, err := s.getAdapter(ctx, organizationID, adapterID)
	if err != nil {
		return err
	}
	modelText := adapter.Policy
	return s.CasbinManager.UpdatePolicy(adapterID, modelText, oldPolicy, newPolicy)
}

func (s *Service) getAdapter(ctx context.Context, organizationID, id string) (*AdapterDetail, error) {
	return s.GetAdapter(ctx, organizationID, id)
}

// 安全边界：PEP 中间件（APIAuthorizationMiddleware）已通过 Casbin 验证了权限。
// RLS 提升只在 service 内部生效（事务内 LOCAL set_config），事务结束自动清除。
