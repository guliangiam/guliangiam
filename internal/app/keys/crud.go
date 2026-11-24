package keys

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/keys"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s Service) ListKeys(ctx context.Context, q ListKeysQuery) ([]KeyListItem, int64, error) {
	items, total, err := s.UOW.Keys().List(ctx, domain.ListQuery{
		Page: q.Page, PageSize: q.PageSize, OrganizationID: q.OrganizationID,
		Keyword: q.Keyword, KeyType: q.KeyType, IsEnabled: q.IsEnabled,
	})
	if err != nil {
		return nil, 0, err
	}
	r := make([]KeyListItem, 0, len(items))
	for _, i := range items {
		r = append(r, toListItem(i))
	}
	return r, total, nil
}

func requireOrganizationID(organizationID string) error {
	if strings.TrimSpace(organizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	return nil
}

func (s Service) GetKey(ctx context.Context, organizationID, id string) (*KeyDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	k, err := s.UOW.Keys().GetByID(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	d := toDetail(k)
	return &d, nil
}

func (s Service) CreateKey(ctx context.Context, c CreateKeyCommand) error {
	if strings.TrimSpace(c.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(c.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	if strings.TrimSpace(c.KeyType) == "" {
		return apperrors.NewInvalidArgument("keyType is required")
	}
	id, err := s.UUID.NewString()
	if err != nil {
		return err
	}
	now := s.Clock.Now()
	algorithm := "RS256"
	if c.Algorithm != "" {
		algorithm = c.Algorithm
	}
	k := &domain.Key{
		ID: id, OrganizationID: c.OrganizationID, Name: c.Name,
		DisplayName: c.DisplayName, KeyType: c.KeyType,
		PublicKey: c.PublicKey, PrivateKey: c.PrivateKey,
		Algorithm: algorithm, ExpiresAt: c.ExpiresAt,
		IsEnabled: c.IsEnabled, CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {

		if err := tx.Keys().Create(ctx, k); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			k.OrganizationID,
			"key.created",
			k.ID,
			"key.created",
			map[string]any{

				keyKeyID: k.ID,

				keyName: k.Name,

				keyOrgID: k.OrganizationID,
			},
		); err != nil {

			return err

		}
		return nil
	})
}

func (s Service) PatchKey(ctx context.Context, organizationID string, c PatchKeyCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	c.OrganizationID = organizationID
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		k, err := tx.Keys().GetByID(ctx, c.ID, organizationID)
		if err != nil {
			return err
		}
		if c.Revision > 0 && k.Revision != c.Revision {
			return apperrors.NewConflict("key revision mismatch")
		}
		if c.DisplayName != nil {
			k.DisplayName = *c.DisplayName
		}
		if c.PublicKey != nil {
			k.PublicKey = *c.PublicKey
		}
		if c.PrivateKey != nil {
			k.PrivateKey = *c.PrivateKey
		}
		if c.Algorithm != nil {
			k.Algorithm = *c.Algorithm
		}
		if c.ExpiresAt != nil {
			k.ExpiresAt = c.ExpiresAt
		}
		if c.IsEnabled != nil {
			k.IsEnabled = *c.IsEnabled
		}
		k.UpdatedAt = s.Clock.Now()
		k.Revision++

		if err := tx.Keys().Update(ctx, k); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			k.OrganizationID,
			"key.updated",
			k.ID,
			"key.updated",
			map[string]any{

				keyKeyID: k.ID,

				keyName: k.Name,

				keyOrgID: k.OrganizationID,
			},
		); err != nil {

			return err

		}
		return nil
	})
}

func (s Service) DeleteKey(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		k, err := tx.Keys().GetByID(ctx, id, organizationID)
		if err != nil {
			return err
		}

		if err := tx.Keys().Delete(ctx, id, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			k.OrganizationID,
			"key.deleted",
			k.ID,
			"key.deleted",
			map[string]any{

				keyKeyID: k.ID,

				keyName: k.Name,

				keyOrgID: k.OrganizationID,
			},
		); err != nil {

			return err

		}
		return nil
	})
}
