package syncers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/domain/groups"
	domain "github.com/guliangiam/guliangiam/internal/domain/syncers"
	"github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
)

// isNotFound reports whether err carries the platform NotFound code.
func isNotFound(err error) bool {
	var appErr *apperrors.Error
	return errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound
}

type SyncerRunResult struct {
	UserSync  *domain.SyncEngineResult `json:"userSync,omitempty"`
	GroupSync *domain.SyncEngineResult `json:"groupSync,omitempty"`
}

// TestConnectionForOrganization opens a connection from the stored syncer
// config only after the row is resolved inside the trusted tenant. A foreign
// or missing id is NotFound, so the caller cannot probe another tenant's
// credentials.
func (s *Service) TestConnectionForOrganization(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	syncer, err := s.UOW.Syncers().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return err
	}
	if err := ensureSyncerTenantScope(syncer, organizationID); err != nil {
		return err
	}

	engine, err := s.Factory.GetEngine(string(syncer.Type))
	if err != nil {
		return apperrors.NewInvalidArgument(err.Error())
	}

	config := domainToSyncerConfig(syncer)
	return engine.TestConnection(ctx, config)
}

// RunSyncerForOrganization runs a sync inside the trusted tenant. Ownership is
// resolved before the running-guard and before any engine work, so a foreign
// id neither learns whether the syncer is busy nor reaches the write-back.
func (s *Service) RunSyncerForOrganization(ctx context.Context, organizationID, id string) (*SyncerRunResult, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	syncer, err := s.UOW.Syncers().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	if err := ensureSyncerTenantScope(syncer, organizationID); err != nil {
		return nil, err
	}

	s.mu.Lock()
	if _, ok := s.running[id]; ok {
		s.mu.Unlock()
		return nil, apperrors.NewConflict("syncer is already running")
	}
	s.running[id] = struct{}{}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.running, id)
		s.mu.Unlock()
	}()

	// BE-B-21: the in-process guard above only serializes this replica. With
	// multiple replicas two instances could sync the same directory (and write
	// back the same users/groups) concurrently. Reuse the DB-061 advisory-lock
	// pattern: a per-syncer named lock held for the whole run.
	if s.locks != nil {
		release, acquired, lockErr := s.locks.TryLock(ctx, "syncers:syncer:"+id)
		if lockErr != nil {
			return nil, apperrors.WrapInternal(lockErr, "acquire syncer lock")
		}
		if !acquired {
			return nil, apperrors.NewConflict("syncer is already running")
		}
		defer func() {
			if release == nil {
				return
			}
			// Fresh context: the request ctx may already be cancelled when the
			// run ends, and the lock must still be released.
			releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := release(releaseCtx); err != nil {
				slog.WarnContext(releaseCtx, "syncers: cannot release sync lock", "syncer_id", id, "error", err)
			}
		}()
	}

	if !syncer.IsEnabled {
		return nil, apperrors.NewInvalidArgument("syncer is disabled")
	}

	engine, err := s.Factory.GetEngine(string(syncer.Type))
	if err != nil {
		return nil, apperrors.NewInvalidArgument(err.Error())
	}

	config := domainToSyncerConfig(syncer)
	orgID := config.OrganizationID

	config.OnUserSynced = s.createUserWriteBack(orgID)
	config.OnGroupSynced = s.createGroupWriteBack()

	result := &SyncerRunResult{}

	userResult, err := engine.SyncUsers(ctx, config)
	if err != nil {
		return nil, apperrors.NewInternalf("sync users failed: %s", err.Error())
	}
	result.UserSync = userResult

	groupResult, err := engine.SyncGroups(ctx, config)
	if err != nil {
		result.GroupSync = &domain.SyncEngineResult{Errors: []string{fmt.Sprintf("sync groups failed: %s", err.Error())}}
	} else {
		result.GroupSync = groupResult
	}

	now := s.Clock.Now()
	syncer.LastSyncAt = &now
	if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {

		if err := tx.Syncers().Update(ctx, syncer); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			syncer.OrganizationID,
			"",
			"syncer.sync_completed",
			"syncer",
			syncer.ID,
			"syncer",
			"syncer.sync_completed",
			map[string]any{

				keySyncerID: syncer.ID,

				keyOrgID: syncer.OrganizationID,
			},
		); err != nil {

			return err

		}
		return nil
	}); err != nil {
		return result, err
	}

	return result, nil
}

func (s *Service) createUserWriteBack(orgID string) domain.UserWriteBackFunc {
	return func(ctx context.Context, providerID string, externalID string, mapped map[string]interface{}) (bool, error) {
		username := extractString(mapped, "username")
		displayName := extractString(mapped, "displayName")
		email := extractString(mapped, "email")
		phone := extractString(mapped, "phone")
		firstName := extractString(mapped, "givenName")
		lastName := extractString(mapped, "familyName")

		existing, err := s.UOW.Users().GetByProviderIdentity(ctx, providerID, externalID)
		if err != nil && !isNotFound(err) {
			// A lookup failure (e.g. database outage) must not fall through to
			// the create branch: surface it so the engine counts a failure.
			return false, apperrors.WrapInternalf(err, "lookup user by provider identity %s", externalID)
		}
		if err == nil && existing != nil {
			existing.Profile.DisplayName = displayName
			existing.Profile.Email = email
			existing.Profile.Phone = phone
			existing.Profile.FirstName = firstName
			existing.Profile.LastName = lastName
			if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
				if err := tx.Users().Update(ctx, existing); err != nil {
					return err
				}

				if err := tx.Users().UpdateProfile(ctx, &existing.Profile); err != nil {
					return err
				}

				if err := s.embedAuditEvent(
					tx,
					ctx,
					existing.OrganizationID,
					existing.ID,
					"syncer.user.updated",
					"user",
					existing.ID,
					"user",
					"syncer.user.updated",
					map[string]any{

						keyUserID: existing.ID,

						keyOrgID: existing.OrganizationID,
					},
				); err != nil {

					return err

				}
				return nil
			}); err != nil {
				return false, apperrors.WrapInternalf(err, "update user %s", existing.ID)
			}
			return false, nil
		}

		if username != "" {
			existing, err = s.UOW.Users().GetByUsername(ctx, orgID, username)
			if err != nil && !isNotFound(err) {
				return false, apperrors.WrapInternalf(err, "lookup user by username %s", username)
			}
			if err == nil && existing != nil {
				existing.Profile.DisplayName = displayName
				existing.Profile.Email = email
				existing.Profile.Phone = phone
				existing.Profile.FirstName = firstName
				existing.Profile.LastName = lastName
				if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
					if err := tx.Users().Update(ctx, existing); err != nil {
						return err
					}
					if err := tx.Users().UpdateProfile(ctx, &existing.Profile); err != nil {
						return err
					}
					identityID, idErr := s.UUID.NewString()
					if idErr != nil {
						return idErr
					}
					identity := &users.UserIdentity{
						ID:           identityID,
						UserID:       existing.ID,
						ProviderID:   providerID,
						IdentityType: "external",
						Subject:      externalID,
						LinkedAt:     s.Clock.Now(),
					}

					if err := tx.Users().CreateIdentity(ctx, identity); err != nil {
						return err
					}

					if err := s.embedAuditEvent(
						tx,
						ctx,
						existing.OrganizationID,
						existing.ID,
						"syncer.user.updated",
						"user",
						existing.ID,
						"user",
						"syncer.user.updated",
						map[string]any{

							keyUserID: existing.ID,

							keyOrgID: existing.OrganizationID,
						},
					); err != nil {

						return err

					}
					return nil
				}); err != nil {
					return false, apperrors.WrapInternalf(err, "update user %s by username", existing.ID)
				}
				return false, nil
			}
		}

		if username == "" {
			return false, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, fmt.Sprintf("cannot create user: username is empty, externalID=%s", externalID), map[string]any{"field": "username", "externalID": externalID})
		}

		id, err := s.UUID.NewString()
		if err != nil {
			return false, apperrors.WrapInternal(err, "generate user id")
		}
		identityID, err := s.UUID.NewString()
		if err != nil {
			return false, apperrors.WrapInternal(err, "generate user identity id")
		}

		now := s.Clock.Now()
		newUser := &users.User{
			ID:             id,
			OrganizationID: orgID,
			Username:       username,
			ExternalID:     externalID,
			UserType:       "normal",
			Status:         "active",
			RegisterSource: providerID,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
			Profile: users.UserProfile{
				UserID:      id,
				DisplayName: displayName,
				Email:       email,
				Phone:       phone,
				FirstName:   firstName,
				LastName:    lastName,
			},
			Identities: []users.UserIdentity{
				{
					ID:           identityID,
					UserID:       id,
					ProviderID:   providerID,
					IdentityType: "external",
					Subject:      externalID,
					LinkedAt:     now,
				},
			},
		}

		if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
			if err := tx.Users().Create(ctx, newUser); err != nil {
				return err
			}
			if err := s.embedAuditEvent(
				tx,
				ctx,
				newUser.OrganizationID,
				newUser.ID,
				"syncer.user.created",
				"user",
				newUser.ID,
				"user",
				"syncer.user.created",
				map[string]any{
					keyUserID: newUser.ID,
					keyOrgID:  newUser.OrganizationID,
				},
			); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return false, apperrors.WrapInternalf(err, "create user %s", username)
		}

		return true, nil
	}
}

func (s *Service) createGroupWriteBack() domain.GroupWriteBackFunc {
	return func(ctx context.Context, organizationID string, externalID string, mapped map[string]interface{}) (bool, error) {
		name := extractString(mapped, "name")
		displayName := extractString(mapped, "displayName")

		if name == "" {
			return false, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, fmt.Sprintf("cannot sync group: name is empty, externalID=%s", externalID), map[string]any{"field": "name", "externalID": externalID})
		}

		if displayName == "" {
			displayName = name
		}

		// Keyword search is fuzzy and paginated: scan all pages so an exact
		// name match beyond the first page is not mistaken for a new group.
		existingGroups, err := platformpagination.CollectAll(func(page, pageSize int) ([]*groups.Group, int64, error) {
			return s.UOW.Groups().List(ctx, groups.ListQuery{
				OrganizationID: organizationID,
				Keyword:        name,
				Page:           page,
				PageSize:       pageSize,
			})
		})
		if err != nil {
			return false, apperrors.WrapInternal(err, "lookup group by name")
		}

		for _, g := range existingGroups {
			if g.Name == name {
				g.DisplayName = displayName
				g.Type = "virtual"
				if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {

					if err := tx.Groups().Update(ctx, g); err != nil {
						return err
					}

					if err := s.embedAuditEvent(
						tx,
						ctx,
						g.OrganizationID,
						"",
						"syncer.group.updated",
						"group",
						g.ID,
						"group",
						"syncer.group.updated",
						map[string]any{

							"groupId": g.ID,

							keyOrgID: g.OrganizationID,
						},
					); err != nil {

						return err

					}
					return nil
				}); err != nil {
					return false, apperrors.WrapInternalf(err, "update group %s", g.ID)
				}
				return false, nil
			}
		}

		id, err := s.UUID.NewString()
		if err != nil {
			return false, apperrors.WrapInternal(err, "generate group id")
		}

		now := s.Clock.Now()
		newGroup := &groups.Group{
			ID:             id,
			OrganizationID: organizationID,
			Name:           name,
			DisplayName:    displayName,
			Type:           "virtual",
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
			if err := tx.Groups().Create(ctx, newGroup); err != nil {
				return err
			}
			if err := s.embedAuditEvent(
				tx,
				ctx,
				newGroup.OrganizationID,
				"",
				"syncer.group.created",
				"group",
				newGroup.ID,
				"group",
				"syncer.group.created",
				map[string]any{
					"groupId": newGroup.ID,
					keyOrgID:  newGroup.OrganizationID,
				},
			); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return false, apperrors.WrapInternalf(err, "create group %s", name)
		}

		return true, nil
	}
}

func extractString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case string:
			return val
		case *string:
			if val != nil {
				return *val
			}
		default:
			return fmt.Sprintf("%v", val)
		}
	}
	return ""
}

func domainToSyncerConfig(s *domain.Syncer) *domain.SyncEngineConfig {
	config := &domain.SyncEngineConfig{
		ProviderType:   strings.TrimSpace(string(s.Type)),
		Host:           strings.TrimSpace(s.Host),
		Port:           s.Port,
		BaseDN:         strings.TrimSpace(s.Database),
		Username:       strings.TrimSpace(s.Username),
		Password:       strings.TrimSpace(s.Password),
		Filter:         strings.TrimSpace(s.TableName),
		OrganizationID: strings.TrimSpace(s.OrganizationID),
		EnableSync:     s.IsEnabled,
		SyncInterval:   s.SyncInterval,
		LastSyncAt:     s.LastSyncAt,
	}

	if strings.TrimSpace(s.FieldMapping) != "" {
		var mappings []domain.FieldMapping
		if err := json.Unmarshal([]byte(s.FieldMapping), &mappings); err == nil {
			config.AttributeMap = make(map[string]string, len(mappings))
			for _, m := range mappings {
				if strings.TrimSpace(m.SourceField) != "" && strings.TrimSpace(m.TargetField) != "" {
					config.AttributeMap[m.SourceField] = m.TargetField
				}
			}
		}
		if config.AttributeMap == nil {
			var attrMap map[string]string
			if err := json.Unmarshal([]byte(s.FieldMapping), &attrMap); err == nil {
				config.AttributeMap = attrMap
			} else {
				slog.Warn("failed to parse syncer field mapping", "syncer_id", s.ID, "error", err)
			}
		}
	}

	return config
}

func (s *Service) isSyncerRunning(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.running[id]
	return ok
}

func retryableSyncError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "temporary") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "network")
}
