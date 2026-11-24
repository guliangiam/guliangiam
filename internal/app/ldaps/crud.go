package ldaps

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/ldaps"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s Service) ListLdaps(ctx context.Context, q ListLdapsQuery) ([]LdapListItem, int64, error) {
	items, total, err := s.UOW.Ldaps().List(ctx, domain.ListQuery(q))
	if err != nil {
		return nil, 0, err
	}
	r := make([]LdapListItem, 0, len(items))
	for _, i := range items {
		r = append(r, toListItem(i))
	}
	return r, total, nil
}

func (s Service) GetLdap(ctx context.Context, id, organizationID string) (*LdapDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	l, err := s.UOW.Ldaps().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	d := toDetail(l)
	return &d, nil
}

func (s Service) CreateLdap(ctx context.Context, c CreateLdapCommand) error {
	if strings.TrimSpace(c.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(c.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	if err := validateLdapHost(c.Host); err != nil {
		return err
	}
	if err := validateLdapPort(c.Port); err != nil {
		return err
	}
	id, err := s.UUID.NewString()
	if err != nil {
		return err
	}
	now := s.Clock.Now()
	isEnabled := true
	if c.IsEnabled != nil {
		isEnabled = *c.IsEnabled
	}
	customAttrs := c.CustomAttributes
	if customAttrs == nil {
		customAttrs = map[string]string{}
	}
	// filter_fields / default_groups 在建表时是 NOT NULL；nil slice 会被 pgx 写成
	// NULL，导致建 LDAP 配置 500，因此与 custom_attributes 一样归一到空切片。
	filterFields := c.FilterFields
	if filterFields == nil {
		filterFields = []string{}
	}
	defaultGroups := c.DefaultGroups
	if defaultGroups == nil {
		defaultGroups = []string{}
	}
	l := &domain.Ldap{
		ID: id, OrganizationID: c.OrganizationID, Name: c.Name, DisplayName: c.DisplayName,
		ServerName: c.ServerName, Host: c.Host, Port: c.Port, Admin: c.Admin,
		BindDN: c.BindDN, BindPassword: c.BindPassword, BaseDN: c.BaseDN,
		Type: c.Type, AttributeName: c.AttributeName, PasswordType: c.PasswordType,
		Filter: c.Filter, FilterFields: filterFields,
		EnableSsl: c.EnableSsl, AllowSelfSignedCert: c.AllowSelfSignedCert,
		EnableGroups: c.EnableGroups, DefaultGroups: defaultGroups,
		CustomAttributes: customAttrs,
		IsEnabled:        isEnabled, SyncInterval: c.SyncInterval,
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Ldaps().Create(ctx, l); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			l.OrganizationID,
			"ldaps.create",
			"ldaps",
			l.ID,
			"ldaps",
			"ldaps.create",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

func (s Service) PatchLdap(ctx context.Context, organizationID string, c PatchLdapCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	if c.Host != nil {
		if err := validateLdapHost(*c.Host); err != nil {
			return err
		}
	}
	if c.Port != nil {
		if err := validateLdapPort(*c.Port); err != nil {
			return err
		}
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Ownership is decided before any mutation, so a cross-tenant request
		// leaves no write and no audit record behind.
		l, err := tx.Ldaps().GetByIDForOrganization(ctx, c.ID, organizationID)
		if err != nil {
			return err
		}
		if c.Revision > 0 && l.Revision != c.Revision {
			return apperrors.NewConflict("ldap revision mismatch")
		}
		if c.DisplayName != nil {
			l.DisplayName = *c.DisplayName
		}
		if c.ServerName != nil {
			l.ServerName = *c.ServerName
		}
		if c.Host != nil {
			l.Host = *c.Host
		}
		if c.Port != nil {
			l.Port = *c.Port
		}
		if c.Admin != nil {
			l.Admin = *c.Admin
		}
		if c.BindDN != nil {
			l.BindDN = *c.BindDN
		}
		if c.BindPassword != nil {
			l.BindPassword = *c.BindPassword
		}
		if c.BaseDN != nil {
			l.BaseDN = *c.BaseDN
		}
		if c.Filter != nil {
			l.Filter = *c.Filter
		}
		if c.Type != nil {
			l.Type = *c.Type
		}
		if c.AttributeName != nil {
			l.AttributeName = *c.AttributeName
		}
		if c.PasswordType != nil {
			l.PasswordType = *c.PasswordType
		}
		if c.FilterFields != nil {
			l.FilterFields = c.FilterFields
		}
		if c.EnableSsl != nil {
			l.EnableSsl = *c.EnableSsl
		}
		if c.AllowSelfSignedCert != nil {
			l.AllowSelfSignedCert = *c.AllowSelfSignedCert
		}
		if c.EnableGroups != nil {
			l.EnableGroups = *c.EnableGroups
		}
		if c.DefaultGroups != nil {
			l.DefaultGroups = c.DefaultGroups
		}
		if c.CustomAttributes != nil {
			l.CustomAttributes = c.CustomAttributes
		}
		if c.IsEnabled != nil {
			l.IsEnabled = *c.IsEnabled
		}
		if c.SyncInterval != nil {
			l.SyncInterval = *c.SyncInterval
		}
		l.UpdatedAt = s.Clock.Now()
		l.Revision++
		if err := tx.Ldaps().Update(ctx, l); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			l.OrganizationID,
			"ldaps.update",
			"ldaps",
			l.ID,
			"ldaps",
			"ldaps.update",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

func (s Service) DeleteLdap(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		l, err := tx.Ldaps().GetByIDForOrganization(ctx, id, organizationID)
		if err != nil {
			return err
		}

		if err := tx.Ldaps().Delete(ctx, id, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			l.OrganizationID,
			"ldaps.delete",
			"ldaps",
			id,
			"ldaps",
			"ldaps.delete",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
