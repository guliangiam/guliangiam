package users

import (
	"context"
	"database/sql"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (r Repository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	row, err := r.getBaseBy(ctx, queryGetUserByID, id)
	if err != nil {
		return nil, err
	}
	return r.loadAggregate(ctx, row)
}

// GetByIDForOrganization scopes the by-ID read to the trusted tenant. A row
// owned by another organization is reported exactly like a missing one.
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.User, error) {
	row, err := r.getBaseBy(ctx, queryGetUserByIDForOrganization, id, organizationID)
	if err != nil {
		return nil, err
	}
	return r.loadAggregate(ctx, row)
}

// LockByID takes a row-level lock on the user row for the remainder of the
// current transaction. See domain.Repository.LockByID for why the anchor is the
// user row rather than the session rows.
func (r Repository) LockByID(ctx context.Context, id string) error {
	var lockedID string
	if err := r.db.QueryRowContext(ctx, queryLockUserByID, id).Scan(&lockedID); err != nil {
		if err == sql.ErrNoRows {
			return apperrors.NewNotFound("user not found")
		}
		return err
	}
	return nil
}

func (r Repository) GetByUsername(ctx context.Context, organizationID, username string) (*domain.User, error) {
	row, err := r.getBaseBy(ctx, queryGetUserByUsername, organizationID, organizationID, username)
	if err != nil {
		return nil, err
	}
	return r.loadAggregate(ctx, row)
}

func (r Repository) GetByProviderIdentity(ctx context.Context, providerID, subject string) (*domain.User, error) {
	row, err := r.getBaseBy(ctx, queryGetUserByProviderIdentity, providerID, subject)
	if err != nil {
		return nil, err
	}
	return r.loadAggregate(ctx, row)
}

func (r Repository) Count(ctx context.Context, organizationID string) (int64, error) {
	var count int64
	if organizationID != "" {
		err := r.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM `users` WHERE organization_id = ? AND deleted_at IS NULL",
			organizationID,
		).Scan(&count)
		return count, err
	}
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM `users` WHERE deleted_at IS NULL",
	).Scan(&count)
	return count, err
}

func (r Repository) HasDeletedWithUsername(ctx context.Context, organizationID, username string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM `users` WHERE organization_id = ? AND username = ? AND deleted_at IS NOT NULL)",
		organizationID, username,
	).Scan(&exists)
	return exists, err
}

// ── Write ───────────────────────────────────────────────────────────────

func (r Repository) GetPasswordHistory(ctx context.Context, userID string, limit int) ([]domain.PasswordHistoryEntry, error) {
	rows, err := r.db.QueryContext(ctx, queryGetPasswordHistory, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]domain.PasswordHistoryEntry, 0)
	for rows.Next() {
		var entry domain.PasswordHistoryEntry
		if err := rows.Scan(&entry.ID, &entry.UserID, &entry.PasswordHash, &entry.PasswordAlgo, &entry.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return entries, nil
}

func scanUserRow(scanner interface{ Scan(...any) error }, row *userRow) error {
	return scanner.Scan(
		&row.ID, &row.OrganizationID, &row.Username, &row.ExternalID,
		&row.UserType, &row.Status, &row.IsAdmin, &row.IsForbidden, &row.IsVerified, &row.IsOnline,
		&row.CreatedIp, &row.SignupApplicationID, &row.RegisterType, &row.RegisterSource,
		&row.Tag, &row.Score,
		&row.SigninWrongTimes, &row.LastSigninWrongTime,
		&row.PreferredMfaType, &row.RecoveryCodeHashes, &row.TotpSecret,
		&row.MfaRadiusUsername, &row.MfaRadiusProvider,
		&row.MfaPushReceiver, &row.MfaPushProvider, &row.MfaPushEnabled,
		&row.MfaRememberDeadline, &row.LastSigninAt, &row.LastSigninIP,
		&row.Balance, &row.BalanceCredit, &row.BalanceCurrency,
		&row.IsDeleted, &row.IpWhitelist,
		&row.Addresses, &row.ManagedAccounts, &row.FaceIds, &row.Cart,
		&row.MfaItems, &row.MfaAccounts, &row.ApplicationScopes,
		&row.Custom1, &row.Custom2, &row.Custom3, &row.Custom4, &row.Custom5,
		&row.Custom6, &row.Custom7, &row.Custom8, &row.Custom9, &row.Custom10,
		&row.CreatedAt, &row.UpdatedAt, &row.DeletedAt, &row.Revision,
	)
}

func (r Repository) getBaseBy(ctx context.Context, sqlStr string, args ...any) (userRow, error) {
	var row userRow
	err := scanUserRow(r.db.QueryRowContext(ctx, sqlStr, args...), &row)
	if err != nil {
		if err == sql.ErrNoRows {
			return userRow{}, apperrors.NewNotFound("user not found")
		}
		return userRow{}, err
	}
	return row, nil
}

// GetByLoginIdentifier implements domain.Repository with the same precedence
// and fail-closed ambiguity rule as the PostgreSQL repository.
func (r Repository) GetByLoginIdentifier(ctx context.Context, organizationID, identifier string) (*domain.User, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, apperrors.NewNotFound("user not found")
	}

	rows, err := r.db.QueryContext(ctx, queryGetUserByLoginIdentifier, organizationID, organizationID, identifier, identifier, identifier)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	matches := make([]userRow, 0, 1)
	seen := make(map[string]struct{}, 1)
	for rows.Next() {
		var row userRow
		if scanErr := scanUserRow(rows, &row); scanErr != nil {
			return nil, scanErr
		}
		if _, ok := seen[row.ID]; ok {
			continue
		}
		seen[row.ID] = struct{}{}
		matches = append(matches, row)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}

	switch len(matches) {
	case 0:
		return nil, apperrors.NewNotFound("user not found")
	case 1:
		return r.loadAggregate(ctx, matches[0])
	default:
		for _, row := range matches {
			if row.Username == identifier {
				return r.loadAggregate(ctx, row)
			}
		}
		return nil, apperrors.NewNotFound("user not found")
	}
}

func (r Repository) loadAggregate(ctx context.Context, base userRow) (*domain.User, error) {
	user, err := toUser(base)
	if err != nil {
		return nil, err
	}
	if err := r.loadProfile(ctx, user); err != nil {
		return nil, err
	}
	if err := r.loadCredentials(ctx, user); err != nil {
		return nil, err
	}
	if err := r.loadIdentities(ctx, user); err != nil {
		return nil, err
	}
	if err := r.loadMfaMethods(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (r Repository) loadProfile(ctx context.Context, user *domain.User) error {
	var row userProfileRow
	err := r.db.QueryRowContext(ctx, queryGetUserProfileByUserID, user.ID).Scan(
		&row.UserID, &row.DisplayName, &row.FirstName, &row.LastName,
		&row.AvatarURL, &row.Email, &row.EmailVerified,
		&row.Phone, &row.PhoneVerified,
		&row.CountryCode, &row.Region, &row.Location,
		&row.Birthday, &row.Gender, &row.Bio,
		&row.RealName, &row.IDCardType, &row.IDCard,
		&row.PassportNo, &row.DriverLicenseNo, &row.IDCardExpiry,
		&row.IssueCountry, &row.IssuingAuthority, &row.PassportPhotoURL,
		&row.DriverLicenseFrontURL, &row.DriverLicenseBackURL,
		&row.Properties,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return apperrors.NewInternal("load user profile")
	}
	return applyProfile(user, row)
}

func (r Repository) loadCredentials(ctx context.Context, user *domain.User) error {
	var row userCredentialsRow
	err := r.db.QueryRowContext(ctx, queryGetUserCredentialsByUserID, user.ID).Scan(
		&row.UserID, &row.PasswordHash, &row.PasswordAlgo, &row.PasswordUpdatedAt,
		&row.AccessKey, &row.AccessSecretHash, &row.AccessSecretRotatedAt, &row.NeedUpdatePassword,
		&row.AccessSecretHashPrev, &row.AccessSecretPrevExpiresAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return apperrors.NewInternal("load user credentials")
	}
	applyCredentials(user, row)
	return nil
}

func (r Repository) loadIdentities(ctx context.Context, user *domain.User) error {
	rows, err := r.db.QueryContext(ctx, queryListUserIdentitiesByUserID, user.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	identities := make([]domain.UserIdentity, 0)
	for rows.Next() {
		var row userIdentityRow
		if err := rows.Scan(
			&row.ID, &row.UserID, &row.ProviderID, &row.IdentityType,
			&row.Subject, &row.ProviderAccount, &row.ProfileSnapshot, &row.LinkedAt,
		); err != nil {
			return err
		}
		identity, err := toIdentity(row)
		if err != nil {
			return err
		}
		identities = append(identities, identity)
	}
	if rows.Err() != nil {
		return rows.Err()
	}
	user.Identities = identities
	return nil
}

func (r Repository) loadMfaMethods(ctx context.Context, user *domain.User) error {
	rows, err := r.db.QueryContext(ctx, queryListUserMfaMethodsByUserID, user.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	methods := make([]domain.UserMfaMethod, 0)
	for rows.Next() {
		var row userMfaMethodRow
		if err := rows.Scan(
			&row.ID, &row.UserID, &row.MethodType, &row.Enabled, &row.Verified,
			&row.DisplayName, &row.LastUsedAt,
		); err != nil {
			return err
		}
		methods = append(methods, toMfaMethod(row))
	}
	if rows.Err() != nil {
		return rows.Err()
	}
	user.MfaMethods = methods
	return nil
}
