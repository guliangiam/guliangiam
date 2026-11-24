package users

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	customattributes "github.com/guliangiam/guliangiam/internal/domain/customattributes"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// CreateCustomAttributeCommand declares a tenant-scoped custom attribute.
type CreateCustomAttributeCommand struct {
	OrganizationID string
	// ActorID is the operator user id (empty for system actions).
	ActorID     string
	Key         string
	DataType    string
	Required    bool
	Options     []string
	Writable    *bool
	Visibility  string
	Description string
	Enabled     *bool
}

// UpdateCustomAttributeCommand updates a definition. Key is the lookup key and
// is immutable; all other fields are replaced.
type UpdateCustomAttributeCommand struct {
	OrganizationID string
	// ActorID is the operator user id (empty for system actions).
	ActorID     string
	Key         string
	DataType    string
	Required    bool
	Options     []string
	Writable    *bool
	Visibility  string
	Description string
	Enabled     *bool
}

func (s *Service) ListCustomAttributes(ctx context.Context, organizationID string, includeDisabled bool) ([]*customattributes.Definition, error) {
	var definitions []*customattributes.Definition
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		repo, err := customAttributeRepo(tx)
		if err != nil {
			return err
		}
		definitions, err = repo.ListByOrganization(ctx, organizationID, includeDisabled)
		return err
	})
	return definitions, err
}

func (s *Service) CreateCustomAttribute(ctx context.Context, command CreateCustomAttributeCommand) (*customattributes.Definition, error) {
	definition := &customattributes.Definition{
		OrganizationID: command.OrganizationID,
		Key:            command.Key,
		DataType:       customattributes.DataType(command.DataType),
		Required:       command.Required,
		Options:        command.Options,
		Writable:       boolOr(command.Writable, true),
		Visibility:     customattributes.Visibility(command.Visibility),
		Description:    command.Description,
		Enabled:        boolOr(command.Enabled, true),
		Revision:       1,
	}
	if definition.Visibility == "" {
		definition.Visibility = customattributes.VisibilityPrivate
	}
	if err := definition.Validate(); err != nil {
		return nil, err
	}
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, err
	}
	definition.ID = id
	now := s.Clock.Now()
	definition.CreatedAt = now
	definition.UpdatedAt = now
	if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		repo, err := customAttributeRepo(tx)
		if err != nil {
			return err
		}
		if err := repo.Create(ctx, definition); err != nil {
			return err
		}
		// Audit the definition only — never any user-supplied property value.
		return s.embedAuditEvent(
			tx,
			ctx,
			definition.OrganizationID,
			command.ActorID,
			"user.custom_attribute.created",
			"CustomAttribute",
			definition.ID,
			"user",
			"user.custom_attribute.created",
			map[string]any{
				"key":        definition.Key,
				"dataType":   string(definition.DataType),
				"visibility": string(definition.Visibility),
				"required":   definition.Required,
				"enabled":    definition.Enabled,
			},
		)
	}); err != nil {
		return nil, err
	}
	return definition, nil
}

func (s *Service) UpdateCustomAttribute(ctx context.Context, command UpdateCustomAttributeCommand) (*customattributes.Definition, error) {
	var updated *customattributes.Definition
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		repo, err := customAttributeRepo(tx)
		if err != nil {
			return err
		}
		definition, err := repo.GetByOrganizationAndKey(ctx, command.OrganizationID, command.Key)
		if err != nil {
			return err
		}
		definition.DataType = customattributes.DataType(command.DataType)
		definition.Required = command.Required
		definition.Options = command.Options
		if command.Writable != nil {
			definition.Writable = *command.Writable
		}
		definition.Visibility = customattributes.Visibility(command.Visibility)
		if definition.Visibility == "" {
			definition.Visibility = customattributes.VisibilityPrivate
		}
		definition.Description = command.Description
		if command.Enabled != nil {
			definition.Enabled = *command.Enabled
		}
		if err := definition.Validate(); err != nil {
			return err
		}
		definition.UpdatedAt = s.Clock.Now()
		definition.Revision++
		if err := repo.Update(ctx, definition); err != nil {
			return err
		}
		// Audit the definition only — never any user-supplied property value.
		if err := s.embedAuditEvent(
			tx,
			ctx,
			definition.OrganizationID,
			command.ActorID,
			"user.custom_attribute.updated",
			"CustomAttribute",
			definition.ID,
			"user",
			"user.custom_attribute.updated",
			map[string]any{
				"key":        definition.Key,
				"dataType":   string(definition.DataType),
				"visibility": string(definition.Visibility),
				"required":   definition.Required,
				"enabled":    definition.Enabled,
				"revision":   definition.Revision,
			},
		); err != nil {
			return err
		}
		updated = definition
		return nil
	})
	return updated, err
}

func (s *Service) DeleteCustomAttribute(ctx context.Context, organizationID, id, actorID string) error {
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		repo, err := customAttributeRepo(tx)
		if err != nil {
			return err
		}
		if err := repo.Delete(ctx, organizationID, id); err != nil {
			return err
		}
		return s.embedAuditEvent(
			tx,
			ctx,
			organizationID,
			actorID,
			"user.custom_attribute.deleted",
			"CustomAttribute",
			id,
			"user",
			"user.custom_attribute.deleted",
			map[string]any{},
		)
	})
}

// systemProfilePropertyKeys 是后端折叠进 profile.properties 的系统字段，
// 与前端 SYSTEM_PROFILE_PROPERTY_KEYS（web/src/pages/users/user-detail-payload.ts）
// 同一口径。它们不是自定义属性：合法性由 user 域与 profile DTO 校验控制，
// 不得按租户 definitions 拒绝——新建租户没有任何定义，否则保存任何 profile
// 都会因 "property \"karma\" is not declared" 返回 422。
const (
	profileKeyAffiliation = "affiliation"
	profileKeyHomepage    = "homepage"
	profileKeyEducation   = "education"
	profileKeyTitle       = "title"
	profileKeyLanguage    = "language"
)

var systemProfilePropertyKeys = map[string]struct{}{
	profileKeyAffiliation:   {},
	profileKeyHomepage:      {},
	profileKeyEducation:     {},
	profileKeyTitle:         {},
	profileKeyLanguage:      {},
	"idCardFront":           {},
	"idCardBack":            {},
	"idCardWithPerson":      {},
	"passportNo":            {},
	"driverLicenseNo":       {},
	"idCardExpiry":          {},
	"issueCountry":          {},
	"issuingAuthority":      {},
	"passportPhotoUrl":      {},
	"driverLicenseFrontUrl": {},
	"driverLicenseBackUrl":  {},
	"karma":                 {},
	"ranking":               {},
	"realName":              {},
	"idCardType":            {},
	"idCard":                {},
	"bio":                   {},
	"gender":                {},
	"birthday":              {},
	"region":                {},
	"location":              {},
}

// isSystemProfilePropertyKey reports whether key is a system-managed profile
// property that is exempt from tenant custom-attribute definition checks.
func isSystemProfilePropertyKey(key string) bool {
	_, ok := systemProfilePropertyKeys[key]
	return ok
}

// validateCustomAttributeProperties enforces the tenant's definitions on a
// profile update. Repositories that do not expose definitions (legacy fakes,
// non-PostgreSQL deployments) keep the pre-existing structural validation.
func (s *Service) validateCustomAttributeProperties(ctx context.Context, tx appuow.UnitOfWork, organizationID string, properties map[string]any) error {
	repo, ok := tx.(interface {
		CustomAttributeDefinitions() customattributes.Repository
	})
	if !ok || repo.CustomAttributeDefinitions() == nil {
		return nil
	}
	definitions, err := repo.CustomAttributeDefinitions().ListByOrganization(ctx, organizationID, false)
	if err != nil {
		return err
	}
	values := make([]customattributes.Definition, 0, len(definitions))
	for _, definition := range definitions {
		values = append(values, *definition)
	}
	// 系统保留键不交给 ValidateProperties：租户 definitions 不是它们的守门人。
	custom := make(map[string]any, len(properties))
	for key, value := range properties {
		if isSystemProfilePropertyKey(key) {
			continue
		}
		custom[key] = value
	}
	return customattributes.ValidateProperties(custom, values)
}

func customAttributeRepo(tx appuow.UnitOfWork) (customattributes.Repository, error) {
	provider, ok := tx.(interface {
		CustomAttributeDefinitions() customattributes.Repository
	})
	if !ok || provider.CustomAttributeDefinitions() == nil {
		return nil, apperrors.NewUnavailable("custom attribute definitions are not configured")
	}
	return provider.CustomAttributeDefinitions(), nil
}

func boolOr(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}
