package users

import (
	"context"
	"fmt"

	domain "github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformsorting "github.com/guliangiam/guliangiam/internal/platform/sorting"
	"github.com/guliangiam/guliangiam/internal/platform/sqlguard"
	"github.com/jackc/pgx/v5"
)

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.User, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	if query.Offset > 0 {
		offset = query.Offset
	}
	orderBy, orderErr := buildOrderBy(query.SortField, query.SortOrder)
	if orderErr != nil {
		return nil, 0, orderErr
	}

	// Build column-specific search clause if searchField/searchValue are provided
	columnSearchClause, columnSearchValue := buildColumnSearch(query.SearchField, query.SearchValue)

	listSQL := fmt.Sprintf("%s %s %s LIMIT $6 OFFSET $7", queryListUsers, columnSearchClause, orderBy)

	var rows pgx.Rows
	var err error
	if columnSearchValue != nil {
		rows, err = r.db.Query(ctx, listSQL, query.OrganizationID, query.Keyword, query.Status, query.UserType, isAdminArg(query.IsAdmin), shared.ClampPageSize(query.PageSize), offset, columnSearchValue)
	} else {
		rows, err = r.db.Query(ctx, listSQL, query.OrganizationID, query.Keyword, query.Status, query.UserType, isAdminArg(query.IsAdmin), shared.ClampPageSize(query.PageSize), offset)
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.User, 0)
	for rows.Next() {
		var row userRow
		if err := rows.Scan(&row.ID, &row.OrganizationID, &row.Username, &row.ExternalID, &row.UserType, &row.Status, &row.IsAdmin, &row.IsForbidden, &row.IsVerified, &row.IsOnline, &row.CreatedIp, &row.SignupApplicationID, &row.RegisterType, &row.RegisterSource, &row.Tag, &row.Score, &row.SigninWrongTimes, &row.LastSigninWrongTime, &row.PreferredMfaType, &row.RecoveryCodeHashes, &row.TotpSecret, &row.MfaRadiusUsername, &row.MfaRadiusProvider, &row.MfaPushReceiver, &row.MfaPushProvider, &row.MfaPushEnabled, &row.MfaRememberDeadline, &row.LastSigninAt, &row.LastSigninIP, &row.Balance, &row.BalanceCredit, &row.BalanceCurrency, &row.IsDeleted, &row.IpWhitelist, &row.Addresses, &row.ManagedAccounts, &row.FaceIds, &row.Cart, &row.MfaItems, &row.MfaAccounts, &row.ApplicationScopes, &row.Custom1, &row.Custom2, &row.Custom3, &row.Custom4, &row.Custom5, &row.Custom6, &row.Custom7, &row.Custom8, &row.Custom9, &row.Custom10, &row.CreatedAt, &row.UpdatedAt, &row.DeletedAt, &row.Revision); err != nil {
			return nil, 0, err
		}
		user, err := toUser(row)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, user)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	// Batch-load profiles for all list items
	if err := r.batchLoadProfiles(ctx, items); err != nil {
		return nil, 0, err
	}

	// BE-A-10: lean 列表投影只保留主行 + profile；凭据/第三方身份/MFA
	// 聚合仅在完整投影（详情语义的调用方）时加载，避免列表页 N 倍无谓查询。
	if !query.Lean {
		// Batch-load credentials for all list items
		if err := r.batchLoadCredentials(ctx, items); err != nil {
			return nil, 0, err
		}

		// Batch-load identities for all list items
		if err := r.batchLoadIdentities(ctx, items); err != nil {
			return nil, 0, err
		}

		// Batch-load MFA methods for all list items
		if err := r.batchLoadMfaMethods(ctx, items); err != nil {
			return nil, 0, err
		}
	}

	// Count query with same column search filter
	var total int64
	if columnSearchValue != nil {
		countSQL := fmt.Sprintf("%s %s", queryCountUsers, columnSearchClause)
		if err := r.db.QueryRow(ctx, countSQL, query.OrganizationID, query.Keyword, query.Status, query.UserType, isAdminArg(query.IsAdmin), columnSearchValue).Scan(&total); err != nil {
			return nil, 0, err
		}
	} else {
		if err := r.db.QueryRow(ctx, queryCountUsers, query.OrganizationID, query.Keyword, query.Status, query.UserType, isAdminArg(query.IsAdmin)).Scan(&total); err != nil {
			return nil, 0, err
		}
	}

	return items, total, nil
}

// isAdminArg converts the optional admin filter into the nil-or-bool value the
// SQL predicate expects. A nil *bool must reach the driver as SQL NULL so the
// `IS NULL OR ...` guard disables the filter instead of matching is_admin=false.
func isAdminArg(isAdmin *bool) any {
	if isAdmin == nil {
		return nil
	}
	return *isAdmin
}

func (r Repository) ListRoleNames(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.Query(ctx, queryListRoleNamesForUser, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return names, nil
}

// columnUsername 是用户名列（也用于排序映射的 JSON 键）。
const columnUsername = "username"

var userSortSpec = platformsorting.NewSpec(map[string]string{
	"id":           "id",
	"createdAt":    "created_at",
	"updatedAt":    "updated_at",
	columnUsername: columnUsername,
	"userType":     "user_type",
	"status":       "status",
	"isAdmin":      "is_admin",
	"isForbidden":  "is_forbidden",
	"score":        "score",
	"lastSigninAt": "last_signin_at",
})

// allowedSearchFields maps API field names to database column names for column-specific search.
// Only whitelisted fields are allowed to prevent SQL injection.
var allowedSearchFields = map[string]string{
	"username": "u.username",
}

// buildColumnSearch constructs an AND clause for column-specific ILIKE search.
// Returns ("", nil) if no column search is requested.
func buildColumnSearch(searchField, searchValue string) (clause string, value any) {
	if searchField == "" || searchValue == "" {
		return "", nil
	}

	column, ok := allowedSearchFields[searchField]
	if !ok {
		return "", nil
	}

	if err := sqlguard.ValidColumnIdentifier(column); err != nil {
		return "", nil // 降级为空，行为同 whitelist miss
	}
	return fmt.Sprintf("AND %s ILIKE '%%' || $8 || '%%'", column), searchValue
}

// buildOrderBy constructs the ORDER BY clause from the client's sort parameters.
//
// It returns an error instead of falling back to a default order: a caller that
// asked for an unlisted field (or misspelled one) must not be handed a different
// order without being told. The clause always ends with the `id` tiebreaker, so
// rows sharing a timestamp have a defined order and consecutive pages cannot
// repeat or skip them.
func buildOrderBy(sortField, sortOrder string) (string, error) {
	return userSortSpec.Resolve(sortField, sortOrder, "created_at")
}

func (r Repository) batchLoadProfiles(ctx context.Context, users []*domain.User) error {
	if len(users) == 0 {
		return nil
	}

	ids := make([]string, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}

	rows, err := r.db.Query(ctx, queryBatchLoadProfiles, ids)
	if err != nil {
		return apperrors.NewInternal("batch load user profiles")
	}
	defer rows.Close()

	profileMap := make(map[string]userProfileRow, len(users))
	for rows.Next() {
		var row userProfileRow
		if err := rows.Scan(&row.UserID, &row.DisplayName, &row.FirstName, &row.LastName, &row.AvatarURL, &row.Email, &row.EmailVerified, &row.Phone, &row.PhoneVerified, &row.CountryCode, &row.Region, &row.Location, &row.Birthday, &row.Gender, &row.Bio, &row.RealName, &row.IDCardType, &row.IDCard, &row.PassportNo, &row.DriverLicenseNo, &row.IDCardSealed, &row.PassportNoSealed, &row.DriverLicenseNoSealed, &row.IDCardExpiry, &row.IssueCountry, &row.IssuingAuthority, &row.PassportPhotoURL, &row.DriverLicenseFrontURL, &row.DriverLicenseBackURL, &row.Properties); err != nil {
			return err
		}
		profileMap[row.UserID] = row
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return rowsErr
	}

	for _, u := range users {
		if profile, ok := profileMap[u.ID]; ok {
			if err := applyProfile(u, profile); err != nil {
				return err
			}
		}
	}

	return nil
}

// batchLoadCredentials loads credentials for a batch of users in a single query.
func (r Repository) batchLoadCredentials(ctx context.Context, users []*domain.User) error {
	if len(users) == 0 {
		return nil
	}

	ids := make([]string, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}

	rows, err := r.db.Query(ctx, queryBatchLoadCredentials, ids)
	if err != nil {
		return apperrors.NewInternal("batch load user credentials")
	}
	defer rows.Close()

	credMap := make(map[string]userCredentialsRow, len(users))
	for rows.Next() {
		var row userCredentialsRow
		if err := rows.Scan(&row.UserID, &row.PasswordHash, &row.PasswordAlgo, &row.PasswordUpdatedAt, &row.AccessKey, &row.AccessSecretHash, &row.AccessSecretRotatedAt, &row.NeedUpdatePassword, &row.AccessSecretHashPrev, &row.AccessSecretPrevExpiresAt); err != nil {
			return err
		}
		credMap[row.UserID] = row
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return rowsErr
	}

	for _, u := range users {
		if cred, ok := credMap[u.ID]; ok {
			applyCredentials(u, cred)
		}
	}

	return nil
}

// batchLoadIdentities loads identities for a batch of users in a single query.
func (r Repository) batchLoadIdentities(ctx context.Context, users []*domain.User) error {
	if len(users) == 0 {
		return nil
	}

	ids := make([]string, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}

	rows, err := r.db.Query(ctx, queryBatchLoadIdentities, ids)
	if err != nil {
		return apperrors.NewInternal("batch load user identities")
	}
	defer rows.Close()

	identityMap := make(map[string][]domain.UserIdentity, len(users))
	for rows.Next() {
		var row userIdentityRow
		if err := rows.Scan(&row.ID, &row.UserID, &row.ProviderID, &row.IdentityType, &row.Subject, &row.ProviderAccount, &row.ProfileSnapshot, &row.LinkedAt); err != nil {
			return err
		}
		identity, err := toIdentity(row)
		if err != nil {
			return err
		}
		identityMap[row.UserID] = append(identityMap[row.UserID], identity)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return rowsErr
	}

	for _, u := range users {
		if identities, ok := identityMap[u.ID]; ok {
			u.Identities = identities
		} else {
			u.Identities = []domain.UserIdentity{}
		}
	}

	return nil
}

// batchLoadMfaMethods loads MFA methods for a batch of users in a single query.
func (r Repository) batchLoadMfaMethods(ctx context.Context, users []*domain.User) error {
	if len(users) == 0 {
		return nil
	}

	ids := make([]string, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}

	rows, err := r.db.Query(ctx, queryBatchLoadMfaMethods, ids)
	if err != nil {
		return apperrors.NewInternal("batch load user mfa methods")
	}
	defer rows.Close()

	methodMap := make(map[string][]domain.UserMfaMethod, len(users))
	for rows.Next() {
		var row userMfaMethodRow
		if err := rows.Scan(&row.ID, &row.UserID, &row.MethodType, &row.Enabled, &row.Verified, &row.DisplayName, &row.LastUsedAt); err != nil {
			return err
		}
		methodMap[row.UserID] = append(methodMap[row.UserID], toMfaMethod(row))
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return rowsErr
	}

	for _, u := range users {
		if methods, ok := methodMap[u.ID]; ok {
			u.MfaMethods = methods
		} else {
			u.MfaMethods = []domain.UserMfaMethod{}
		}
	}

	return nil
}

// Count returns the total number of users, optionally filtered by organization.
func (r Repository) ListRoleBindingsByUserIDs(ctx context.Context, userIDs []string) (map[string][]domain.UserRoleBinding, error) {
	if len(userIDs) == 0 {
		return map[string][]domain.UserRoleBinding{}, nil
	}

	rows, err := r.db.Query(ctx, queryListRoleBindingsByUserIDs, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]domain.UserRoleBinding, len(userIDs))
	for rows.Next() {
		var userID, roleID string
		if err := rows.Scan(&userID, &roleID); err != nil {
			return nil, err
		}
		result[userID] = append(result[userID], domain.UserRoleBinding{UserID: userID, RoleID: roleID})
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}

	return result, nil
}

func (r Repository) ListGroupMembershipsByUserIDs(ctx context.Context, userIDs []string) (map[string][]domain.UserGroupMembership, error) {
	if len(userIDs) == 0 {
		return map[string][]domain.UserGroupMembership{}, nil
	}

	rows, err := r.db.Query(ctx, queryListGroupMembershipsByUserIDs, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]domain.UserGroupMembership, len(userIDs))
	for rows.Next() {
		var userID, groupID string
		if err := rows.Scan(&userID, &groupID); err != nil {
			return nil, err
		}
		result[userID] = append(result[userID], domain.UserGroupMembership{UserID: userID, GroupID: groupID})
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}

	return result, nil
}
