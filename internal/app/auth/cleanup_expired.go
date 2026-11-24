package auth

import (
	"context"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	rememberme "github.com/guliangiam/guliangiam/internal/domain/rememberme"
	trusteddevices "github.com/guliangiam/guliangiam/internal/domain/trusteddevices"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// CleanupExpiredRememberMe physically deletes one bounded batch of expired
// remember-me rows (tokens first, then families whose tokens cascade).
func (s *Service) CleanupExpiredRememberMe(ctx context.Context, now time.Time, limit int) (int64, error) {
	var deleted int64
	err := s.UOW.WithinTransaction(tenantctx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
		repo, err := rememberMeRepo(tx)
		if err != nil {
			return err
		}
		cleaner, ok := repo.(rememberme.ExpiredCleaner)
		if !ok {
			return nil
		}
		deleted, err = cleaner.DeleteExpired(ctx, now, limit)
		return err
	})
	return deleted, err
}

// CleanupExpiredTrustedDevices physically deletes one bounded batch of expired
// trusted-device rows.
func (s *Service) CleanupExpiredTrustedDevices(ctx context.Context, now time.Time, limit int) (int64, error) {
	var deleted int64
	err := s.UOW.WithinTransaction(tenantctx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
		repo, err := trustedDeviceRepo(tx)
		if err != nil {
			return err
		}
		cleaner, ok := repo.(trusteddevices.ExpiredCleaner)
		if !ok {
			return nil
		}
		deleted, err = cleaner.DeleteExpired(ctx, now, limit)
		return err
	})
	return deleted, err
}

// RevokeLegacyPlaintextTrustedDevices explicitly revokes rows whose credential
// predates the SHA-256 digest switch. They fail closed in FindActiveByCredential
// anyway, but revoking them makes the forced re-registration observable.
func (s *Service) RevokeLegacyPlaintextTrustedDevices(ctx context.Context, now time.Time) (int64, error) {
	var revoked int64
	err := s.UOW.WithinTransaction(tenantctx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
		repo, err := trustedDeviceRepo(tx)
		if err != nil {
			return err
		}
		revoker, ok := repo.(trusteddevices.LegacyPlaintextRevoker)
		if !ok {
			return nil
		}
		revoked, err = revoker.RevokeLegacyPlaintext(ctx, now)
		return err
	})
	return revoked, err
}
