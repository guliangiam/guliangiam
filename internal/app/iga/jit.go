package iga

import (
	"context"
	"time"

	"github.com/google/uuid"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/iga"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s Service) now() time.Time {
	if s.Clock != nil {
		return s.Clock.Now().UTC()
	}
	return time.Now().UTC()
}

func newIGAIdentifier() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func createJITGrant(ctx context.Context, tx appuow.UnitOfWork, clk clock.Clock, lifecycle *domain.AccessLifecycle, subject, organizationSlug string) error {
	if lifecycle == nil || subject == "" || organizationSlug == "" {
		return apperrors.NewInvalidArgument("lifecycle, subject and organization slug are required")
	}
	if !lifecycle.ExpiresAt.After(lifecycle.StartsAt) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "expiresAt must be after startsAt", map[string]any{"field": "expiresAt", "value": lifecycle.ExpiresAt.UTC().Format(time.RFC3339), "expected": "after startsAt"})
	}
	if err := (SODChecker{UOW: tx, Clock: clk}).ValidateConflict(ctx, lifecycle.OrganizationID, lifecycle.RoleID, lifecycle.UserID); err != nil {
		return apperrors.NewForbidden(err.Error())
	}
	policyID, err := newIGAIdentifier()
	if err != nil {
		return err
	}
	lifecycle.PolicyID = policyID
	if err := tx.IGA().CreateLifecycle(ctx, lifecycle); err != nil {
		return err
	}
	if err := tx.IGA().CreateJITBinding(ctx, &domain.JITBinding{
		UserID: lifecycle.UserID, RoleID: lifecycle.RoleID, LifecycleID: lifecycle.ID,
		StartsAt: lifecycle.StartsAt, ExpiresAt: lifecycle.ExpiresAt,
	}); err != nil {
		return err
	}
	return tx.IGA().CreateJITPolicy(ctx, &domain.JITPolicy{
		ID: policyID, EnforcerID: lifecycle.EnforcerID, LifecycleID: lifecycle.ID,
		UserID: lifecycle.UserID, Subject: subject, OrganizationSlug: organizationSlug,
		Resource: lifecycle.Resource, Action: lifecycle.Action,
		StartsAt: lifecycle.StartsAt, ExpiresAt: lifecycle.ExpiresAt,
	})
}

func (s Service) grantWindow(ctx context.Context, orgID, userID, roleID string, startsAt, expiresAt time.Time, resource, action string) (*AccessLifecycle, error) {
	var lifecycle *AccessLifecycle
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		enforcerID, err := tx.IGA().ResolveDefaultEnforcer(ctx, orgID)
		if err != nil {
			return err
		}
		validity, err := tx.IGA().ValidateAccessTargets(ctx, orgID, userID, roleID, "", enforcerID)
		if err != nil {
			return err
		}
		if !validity.Valid() || validity.Subject == "" {
			return apperrors.NewForbidden("invalid JIT grant target")
		}
		if resource == "" {
			resource = "*"
		}
		if action == "" {
			action = "*"
		}
		id, err := newIGAIdentifier()
		if err != nil {
			return err
		}
		now := s.now()
		lifecycle = &AccessLifecycle{
			ID: id, OrganizationID: orgID, UserID: userID, RoleID: roleID,
			EnforcerID: enforcerID, Resource: resource, Action: action,
			GrantedAt: now, StartsAt: startsAt, ExpiresAt: expiresAt,
			Status: domain.LifecycleActive, CreatedAt: now, Revision: 1,
		}
		if err := createJITGrant(ctx, tx, s.Clock, lifecycle, validity.Subject, validity.OrganizationSlug); err != nil {
			return err
		}
		return s.embedAuditEvent(
			tx,
			ctx,
			orgID,
			userID,
			"iga.jit.granted",
			"lifecycle",
			lifecycle.ID,
			"lifecycle",
			"iga.jit.granted",
			map[string]any{keyLifecycleID: lifecycle.ID, keyPolicyID: lifecycle.PolicyID, keyStartsAt: startsAt, keyExpiresAt: expiresAt},
		)
	})
	return lifecycle, err
}

func (s Service) reclaimLifecycleByID(ctx context.Context, orgID, id string, status domain.LifecycleStatus, actorID string) (bool, error) {
	completed := false
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		var err error
		completed, err = s.reclaimLifecycleInTx(ctx, tx, orgID, id, status, actorID)
		return err
	})
	return completed, err
}

// reclaimLifecycleInTx reclaims one grant inside an existing transaction so
// batch callers (ExpireAccesses) can amortize N grants into a single tx
// (BE-D-17). Locks the row FOR UPDATE and embeds the audit event in-tx.
func (s Service) reclaimLifecycleInTx(ctx context.Context, tx appuow.UnitOfWork, orgID, id string, status domain.LifecycleStatus, actorID string) (bool, error) {
	lifecycle, err := tx.IGA().GetLifecycleForUpdate(ctx, orgID, id)
	if err != nil {
		return false, err
	}
	completed, err := reclaimJITGrant(ctx, tx, lifecycle, status, s.now())
	if err != nil || !completed {
		return completed, err
	}
	eventType := "iga.jit.revoked"
	if status == domain.LifecycleExpired {
		eventType = "iga.jit.expired"
	}
	return true, s.embedAuditEvent(
		tx,
		ctx,
		orgID,
		actorID,
		eventType,
		"lifecycle",
		id,
		"lifecycle",
		eventType,
		map[string]any{keyLifecycleID: id, keyPolicyID: lifecycle.PolicyID, "status": status},
	)
}

func reclaimJITGrant(ctx context.Context, tx appuow.UnitOfWork, lifecycle *domain.AccessLifecycle, status domain.LifecycleStatus, at time.Time) (bool, error) {
	if lifecycle == nil || lifecycle.Status != domain.LifecycleActive {
		return false, nil
	}
	if status != domain.LifecycleExpired && status != domain.LifecycleRevoked {
		return false, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "terminal lifecycle status must be expired or revoked", map[string]any{"field": "status", "value": string(status), "allowed": append([]string{}, terminalLifecycleStatuses...)})
	}
	if _, err := tx.IGA().RevokePrivilegedSessions(ctx, lifecycle.ID, at); err != nil {
		return false, err
	}
	if err := tx.IGA().RevokeJITArtifacts(ctx, lifecycle.ID, lifecycle.PolicyID); err != nil {
		return false, err
	}
	return tx.IGA().CompleteLifecycle(ctx, lifecycle.OrganizationID, lifecycle.ID, status, at)
}
