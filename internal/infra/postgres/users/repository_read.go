package users

import (
	"context"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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
	if err := r.db.QueryRow(ctx, queryLockUserByID, id).Scan(&lockedID); err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewNotFound("user not found")
		}
		return err
	}
	return nil
}

func (r Repository) GetByUsername(ctx context.Context, organizationID, username string) (*domain.User, error) {
	row, err := r.getBaseBy(ctx, queryGetUserByUsername, organizationID, username)
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

func (r Repository) GetPasswordHistory(ctx context.Context, userID string, limit int) ([]domain.PasswordHistoryEntry, error) {
	rows, err := r.db.Query(ctx, queryGetPasswordHistory, userID, limit)
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

func (r Repository) getBaseBy(ctx context.Context, sql string, arguments ...any) (userRow, error) {
	var row userRow
	err := scanUserRow(r.db.QueryRow(ctx, sql, arguments...), &row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return userRow{}, apperrors.NewNotFound("user not found")
		}
		return userRow{}, err
	}

	return row, nil
}

// scanUserRow maps one base user row. Both single-row and candidate-row lookups
// share the column list of the aggregate query, so the field order must stay in
// sync with queryGetUserByID/queryGetUserByUsername/queryGetUserByLoginIdentifier.
func scanUserRow(scanner interface{ Scan(...any) error }, row *userRow) error {
	return scanner.Scan(&row.ID, &row.OrganizationID, &row.Username, &row.ExternalID, &row.UserType, &row.Status, &row.IsAdmin, &row.IsForbidden, &row.IsVerified, &row.IsOnline, &row.CreatedIp, &row.SignupApplicationID, &row.RegisterType, &row.RegisterSource, &row.Tag, &row.Score, &row.SigninWrongTimes, &row.LastSigninWrongTime, &row.PreferredMfaType, &row.RecoveryCodeHashes, &row.TotpSecret, &row.MfaRadiusUsername, &row.MfaRadiusProvider, &row.MfaPushReceiver, &row.MfaPushProvider, &row.MfaPushEnabled, &row.MfaRememberDeadline, &row.LastSigninAt, &row.LastSigninIP, &row.Balance, &row.BalanceCredit, &row.BalanceCurrency, &row.IsDeleted, &row.IpWhitelist, &row.Addresses, &row.ManagedAccounts, &row.FaceIds, &row.Cart, &row.MfaItems, &row.MfaAccounts, &row.ApplicationScopes, &row.Custom1, &row.Custom2, &row.Custom3, &row.Custom4, &row.Custom5, &row.Custom6, &row.Custom7, &row.Custom8, &row.Custom9, &row.Custom10, &row.CreatedAt, &row.UpdatedAt, &row.DeletedAt, &row.Revision)
}

// GetByLoginIdentifier implements domain.Repository. See the interface comment
// for the precedence and the fail-closed rule on ambiguous matches.
func (r Repository) GetByLoginIdentifier(ctx context.Context, organizationID, identifier string) (*domain.User, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, apperrors.NewNotFound("user not found")
	}

	rows, err := r.db.Query(ctx, queryGetUserByLoginIdentifier, organizationID, identifier)
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
		// A phone has no unique constraint, so the same number can belong to
		// several users. Only an exact username match is allowed to break the
		// tie; anything else fails closed as "not found" so no account is
		// selected by accident.
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

	// Use pgx Batch to pipeline all 4 sub-queries in a single round-trip
	// when the underlying connection is a pgxpool.Pool. This reduces
	// latency from 4 sequential round-trips to 1 pipelined round-trip.
	if pool, ok := r.db.(*pgxpool.Pool); ok {
		return r.loadAggregateBatched(ctx, pool, user)
	}

	// Fallback: sequential loading for non-pool connections (e.g. test mocks)
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
	if err := r.loadRoleAndGroupBindings(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// loadRoleAndGroupBindings fills RoleBindings/GroupMemberships for a single
// user. The permissions aggregate (app/permissions) reads them from the user
// aggregate; without this the effective-permissions response was always empty.
func (r Repository) loadRoleAndGroupBindings(ctx context.Context, user *domain.User) error {
	bindings, err := r.ListRoleBindingsByUserIDs(ctx, []string{user.ID})
	if err != nil {
		return err
	}
	user.RoleBindings = bindings[user.ID]

	memberships, err := r.ListGroupMembershipsByUserIDs(ctx, []string{user.ID})
	if err != nil {
		return err
	}
	user.GroupMemberships = memberships[user.ID]

	return nil
}

// loadAggregateBatched uses pgx Batch to pipeline all 6 sub-queries
// (profile, credentials, identities, mfaMethods, role bindings, group
// memberships) in a single round-trip.
func (r Repository) loadAggregateBatched(ctx context.Context, pool *pgxpool.Pool, user *domain.User) (*domain.User, error) {
	batch := &pgx.Batch{}

	batch.Queue(queryGetUserProfileByUserID, user.ID)
	batch.Queue(queryGetUserCredentialsByUserID, user.ID)
	batch.Queue(queryListUserIdentitiesByUserID, user.ID)
	batch.Queue(queryListUserMfaMethodsByUserID, user.ID)
	batch.Queue(queryListRoleBindingsByUserIDs, []string{user.ID})
	batch.Queue(queryListGroupMembershipsByUserIDs, []string{user.ID})

	br := pool.SendBatch(ctx, batch)
	defer br.Close()

	// Read profile result
	var profileRow userProfileRow
	err := br.QueryRow().Scan(&profileRow.UserID, &profileRow.DisplayName, &profileRow.FirstName, &profileRow.LastName, &profileRow.AvatarURL, &profileRow.Email, &profileRow.EmailVerified, &profileRow.Phone, &profileRow.PhoneVerified, &profileRow.CountryCode, &profileRow.Region, &profileRow.Location, &profileRow.Birthday, &profileRow.Gender, &profileRow.Bio, &profileRow.RealName, &profileRow.IDCardType, &profileRow.IDCard, &profileRow.PassportNo, &profileRow.DriverLicenseNo, &profileRow.IDCardSealed, &profileRow.PassportNoSealed, &profileRow.DriverLicenseNoSealed, &profileRow.IDCardExpiry, &profileRow.IssueCountry, &profileRow.IssuingAuthority, &profileRow.PassportPhotoURL, &profileRow.DriverLicenseFrontURL, &profileRow.DriverLicenseBackURL, &profileRow.Properties)
	if err != nil && err != pgx.ErrNoRows {
		return nil, apperrors.NewInternal("load user profile")
	}
	if err == nil {
		if err := applyProfile(user, profileRow); err != nil {
			return nil, err
		}
	}

	// Read credentials result
	var credRow userCredentialsRow
	err = br.QueryRow().Scan(&credRow.UserID, &credRow.PasswordHash, &credRow.PasswordAlgo, &credRow.PasswordUpdatedAt, &credRow.AccessKey, &credRow.AccessSecretHash, &credRow.AccessSecretRotatedAt, &credRow.NeedUpdatePassword)
	if err != nil && err != pgx.ErrNoRows {
		return nil, apperrors.NewInternal("load user credentials")
	}
	if err == nil {
		applyCredentials(user, credRow)
	}

	// Read identities result
	identities, err := readIdentityRows(br.Query())
	if err != nil {
		return nil, err
	}
	user.Identities = identities

	// Read mfa methods result
	methods, err := readMfaMethodRows(br.Query())
	if err != nil {
		return nil, err
	}
	user.MfaMethods = methods

	// Read role bindings result
	bindings, err := readUserRoleBindingRows(br.Query())
	if err != nil {
		return nil, err
	}
	user.RoleBindings = bindings

	// Read group memberships result
	memberships, err := readUserGroupMembershipRows(br.Query())
	if err != nil {
		return nil, err
	}
	user.GroupMemberships = memberships

	return user, nil
}

// readUserRoleBindingRows scans (user_id, role_id) rows into domain bindings.
func readUserRoleBindingRows(rows pgx.Rows, err error) ([]domain.UserRoleBinding, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bindings := make([]domain.UserRoleBinding, 0)
	for rows.Next() {
		var userID, roleID string
		if err := rows.Scan(&userID, &roleID); err != nil {
			return nil, err
		}
		bindings = append(bindings, domain.UserRoleBinding{UserID: userID, RoleID: roleID})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return bindings, nil
}

// readUserGroupMembershipRows scans (user_id, group_id) rows into domain memberships.
func readUserGroupMembershipRows(rows pgx.Rows, err error) ([]domain.UserGroupMembership, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	memberships := make([]domain.UserGroupMembership, 0)
	for rows.Next() {
		var userID, groupID string
		if err := rows.Scan(&userID, &groupID); err != nil {
			return nil, err
		}
		memberships = append(memberships, domain.UserGroupMembership{UserID: userID, GroupID: groupID})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return memberships, nil
}

// readIdentityRows scans all rows from a pgx.Rows result into domain identities.
func readIdentityRows(rows pgx.Rows, err error) ([]domain.UserIdentity, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	identities := make([]domain.UserIdentity, 0)
	for rows.Next() {
		var row userIdentityRow
		if err := rows.Scan(&row.ID, &row.UserID, &row.ProviderID, &row.IdentityType, &row.Subject, &row.ProviderAccount, &row.ProfileSnapshot, &row.LinkedAt); err != nil {
			return nil, err
		}
		identity, err := toIdentity(row)
		if err != nil {
			return nil, err
		}
		identities = append(identities, identity)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}

	return identities, nil
}

// readMfaMethodRows scans all rows from a pgx.Rows result into domain MFA methods.
func readMfaMethodRows(rows pgx.Rows, err error) ([]domain.UserMfaMethod, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	methods := make([]domain.UserMfaMethod, 0)
	for rows.Next() {
		var row userMfaMethodRow
		if err := rows.Scan(&row.ID, &row.UserID, &row.MethodType, &row.Enabled, &row.Verified, &row.DisplayName, &row.LastUsedAt); err != nil {
			return nil, err
		}
		methods = append(methods, toMfaMethod(row))
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}

	return methods, nil
}

func (r Repository) loadProfile(ctx context.Context, user *domain.User) error {
	var row userProfileRow
	err := r.db.QueryRow(ctx, queryGetUserProfileByUserID, user.ID).Scan(&row.UserID, &row.DisplayName, &row.FirstName, &row.LastName, &row.AvatarURL, &row.Email, &row.EmailVerified, &row.Phone, &row.PhoneVerified, &row.CountryCode, &row.Region, &row.Location, &row.Birthday, &row.Gender, &row.Bio, &row.RealName, &row.IDCardType, &row.IDCard, &row.PassportNo, &row.DriverLicenseNo, &row.IDCardSealed, &row.PassportNoSealed, &row.DriverLicenseNoSealed, &row.IDCardExpiry, &row.IssueCountry, &row.IssuingAuthority, &row.PassportPhotoURL, &row.DriverLicenseFrontURL, &row.DriverLicenseBackURL, &row.Properties)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		return apperrors.NewInternal("load user profile")
	}

	return applyProfile(user, row)
}

func (r Repository) loadCredentials(ctx context.Context, user *domain.User) error {
	var row userCredentialsRow
	err := r.db.QueryRow(ctx, queryGetUserCredentialsByUserID, user.ID).Scan(&row.UserID, &row.PasswordHash, &row.PasswordAlgo, &row.PasswordUpdatedAt, &row.AccessKey, &row.AccessSecretHash, &row.AccessSecretRotatedAt, &row.NeedUpdatePassword, &row.AccessSecretHashPrev, &row.AccessSecretPrevExpiresAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		return apperrors.NewInternal("load user credentials")
	}

	applyCredentials(user, row)
	return nil
}

func (r Repository) loadIdentities(ctx context.Context, user *domain.User) error {
	rows, err := r.db.Query(ctx, queryListUserIdentitiesByUserID, user.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	identities := make([]domain.UserIdentity, 0)
	for rows.Next() {
		var row userIdentityRow
		if err := rows.Scan(&row.ID, &row.UserID, &row.ProviderID, &row.IdentityType, &row.Subject, &row.ProviderAccount, &row.ProfileSnapshot, &row.LinkedAt); err != nil {
			return err
		}
		identity, err := toIdentity(row)
		if err != nil {
			return err
		}
		identities = append(identities, identity)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return rowsErr
	}

	user.Identities = identities
	return nil
}

func (r Repository) loadMfaMethods(ctx context.Context, user *domain.User) error {
	rows, err := r.db.Query(ctx, queryListUserMfaMethodsByUserID, user.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	methods := make([]domain.UserMfaMethod, 0)
	for rows.Next() {
		var row userMfaMethodRow
		if err := rows.Scan(&row.ID, &row.UserID, &row.MethodType, &row.Enabled, &row.Verified, &row.DisplayName, &row.LastUsedAt); err != nil {
			return err
		}
		methods = append(methods, toMfaMethod(row))
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return rowsErr
	}

	user.MfaMethods = methods
	return nil
}

// userSortSpec is the whitelist for GET /api/v1/users (手册 4.8.1).
//
// The keys are the **API** names a client sends and the values are the physical
// columns; the previous map was keyed by column names with each value equal to
// its key, so the documented `sortField=createdAt` fell outside the whitelist and
// silently produced the default order.
func (r Repository) Count(ctx context.Context, organizationID string) (int64, error) {
	var count int64
	if organizationID != "" {
		err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM guliangiam.users WHERE organization_id = $1 AND deleted_at IS NULL`, organizationID).Scan(&count)
		return count, err
	}
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM guliangiam.users WHERE deleted_at IS NULL`).Scan(&count)
	return count, err
}

// HasDeletedWithUsername checks whether a soft-deleted user with the given
// username exists in the specified organization.
func (r Repository) HasDeletedWithUsername(ctx context.Context, organizationID, username string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM guliangiam.users WHERE organization_id = $1::uuid AND username = $2 AND deleted_at IS NOT NULL)`,
		organizationID, username,
	).Scan(&exists)
	return exists, err
}
