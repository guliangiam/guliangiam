package users

import (
	"context"
	"fmt"

	domain "github.com/guliangiam/guliangiam/internal/domain/users"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformsorting "github.com/guliangiam/guliangiam/internal/platform/sorting"
	"github.com/guliangiam/guliangiam/internal/platform/sqlguard"
)

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.User, int64, error) {
	offset := mysqlshared.PageOffset(query.Page, query.PageSize)
	// SCIM startIndex 等调用方以显式 Offset 覆盖页推导（domain ListQuery 契约，
	// 与 postgres 镜像同构）。
	if query.Offset > 0 {
		offset = query.Offset
	}
	orderBy, orderErr := buildOrderBy(query.SortField, query.SortOrder)
	if orderErr != nil {
		return nil, 0, orderErr
	}

	// Build column-specific search clause if searchField/searchValue are provided
	columnSearchClause, columnSearchValue := buildColumnSearch(query.SearchField, query.SearchValue)

	listSQL := fmt.Sprintf("%s %s %s LIMIT ? OFFSET ?", queryListUsers, columnSearchClause, orderBy)

	// queryListUsers 参数按 SQL 顺序：orgID×2、keyword×3（前缀守卫 + username/external_id 两次 LIKE）、
	// status×2、userType×2、isAdmin×2，然后 LIMIT/OFFSET；columnSearchValue 追加在最后。
	isAdminValue := isAdminArg(query.IsAdmin)
	args := []any{
		query.OrganizationID, query.OrganizationID,
		query.Keyword, query.Keyword, query.Keyword,
		query.Status, query.Status,
		query.UserType, query.UserType,
		isAdminValue, isAdminValue,
		mysqlshared.ClampPageSize(query.PageSize), offset,
	}
	if columnSearchValue != nil {
		args = append(args, columnSearchValue)
	}

	rows, err := r.db.QueryContext(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.User, 0)
	for rows.Next() {
		var row userRow
		if err := scanUserRow(rows, &row); err != nil {
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

	if err := r.batchLoadProfiles(ctx, items); err != nil {
		return nil, 0, err
	}
	// BE-A-10 镜像：lean 列表投影只保留主行 + profile；凭据/身份/MFA 聚合
	// 仅完整投影时加载（与 postgres repository_list.go 同构）。
	if !query.Lean {
		if err := r.batchLoadCredentials(ctx, items); err != nil {
			return nil, 0, err
		}
		if err := r.batchLoadIdentities(ctx, items); err != nil {
			return nil, 0, err
		}
		if err := r.batchLoadMfaMethods(ctx, items); err != nil {
			return nil, 0, err
		}
	}

	var total int64
	countArgs := []any{
		query.OrganizationID, query.OrganizationID,
		query.Keyword, query.Keyword, query.Keyword,
		query.Status, query.Status,
		query.UserType, query.UserType,
		isAdminValue, isAdminValue,
	}
	if columnSearchValue != nil {
		countArgs = append(countArgs, columnSearchValue)
	}
	countSQL := queryCountUsers
	if columnSearchClause != "" {
		countSQL = queryCountUsers + " " + columnSearchClause
	}
	if err := r.db.QueryRowContext(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// isAdminArg converts the optional admin filter into the nil-or-bool value the
// SQL predicate expects. A nil *bool must reach the driver as SQL NULL so the
// `? IS NULL OR ...` guard disables the filter instead of matching is_admin=0.
func isAdminArg(isAdmin *bool) any {
	if isAdmin == nil {
		return nil
	}
	return *isAdmin
}

func (r Repository) ListRoleNames(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, queryListRoleNamesForUser, userID)
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

func (r Repository) ListRoleBindingsByUserIDs(ctx context.Context, userIDs []string) (map[string][]domain.UserRoleBinding, error) {
	if len(userIDs) == 0 {
		return map[string][]domain.UserRoleBinding{}, nil
	}
	sqlStr := replaceInPlaceholders(queryListRoleBindingsByUserIDs, len(userIDs))
	args := make([]any, len(userIDs))
	for i, id := range userIDs {
		args[i] = id
	}
	rows, err := r.db.QueryContext(ctx, sqlStr, args...)
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
	sqlStr := replaceInPlaceholders(queryListGroupMembershipsByUserIDs, len(userIDs))
	args := make([]any, len(userIDs))
	for i, id := range userIDs {
		args[i] = id
	}
	rows, err := r.db.QueryContext(ctx, sqlStr, args...)
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

// ── Internal helpers ────────────────────────────────────────────────────

// scanUserRow 从 *sql.Rows 扫描完整 userRow（56 列：users 主表 + 5 张 user_* 子表聚合）。
func (r Repository) batchLoadProfiles(ctx context.Context, users []*domain.User) error {
	if len(users) == 0 {
		return nil
	}
	ids := make([]string, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	sqlStr := replaceInPlaceholders(queryBatchLoadProfiles, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := r.db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return apperrors.NewInternal("batch load user profiles")
	}
	defer rows.Close()

	profileMap := make(map[string]userProfileRow, len(users))
	for rows.Next() {
		var row userProfileRow
		if err := rows.Scan(
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
		); err != nil {
			return err
		}
		profileMap[row.UserID] = row
	}
	if rows.Err() != nil {
		return rows.Err()
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

func (r Repository) batchLoadCredentials(ctx context.Context, users []*domain.User) error {
	if len(users) == 0 {
		return nil
	}
	ids := make([]string, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	sqlStr := replaceInPlaceholders(queryBatchLoadCredentials, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := r.db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return apperrors.NewInternal("batch load user credentials")
	}
	defer rows.Close()

	credMap := make(map[string]userCredentialsRow, len(users))
	for rows.Next() {
		var row userCredentialsRow
		if err := rows.Scan(
			&row.UserID, &row.PasswordHash, &row.PasswordAlgo, &row.PasswordUpdatedAt,
			&row.AccessKey, &row.AccessSecretHash, &row.AccessSecretRotatedAt, &row.NeedUpdatePassword,
			&row.AccessSecretHashPrev, &row.AccessSecretPrevExpiresAt,
		); err != nil {
			return err
		}
		credMap[row.UserID] = row
	}
	if rows.Err() != nil {
		return rows.Err()
	}
	for _, u := range users {
		if cred, ok := credMap[u.ID]; ok {
			applyCredentials(u, cred)
		}
	}
	return nil
}

func (r Repository) batchLoadIdentities(ctx context.Context, users []*domain.User) error {
	if len(users) == 0 {
		return nil
	}
	ids := make([]string, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	sqlStr := replaceInPlaceholders(queryBatchLoadIdentities, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := r.db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return apperrors.NewInternal("batch load user identities")
	}
	defer rows.Close()

	identityMap := make(map[string][]domain.UserIdentity, len(users))
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
		identityMap[row.UserID] = append(identityMap[row.UserID], identity)
	}
	if rows.Err() != nil {
		return rows.Err()
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

func (r Repository) batchLoadMfaMethods(ctx context.Context, users []*domain.User) error {
	if len(users) == 0 {
		return nil
	}
	ids := make([]string, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	sqlStr := replaceInPlaceholders(queryBatchLoadMfaMethods, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := r.db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return apperrors.NewInternal("batch load user mfa methods")
	}
	defer rows.Close()

	methodMap := make(map[string][]domain.UserMfaMethod, len(users))
	for rows.Next() {
		var row userMfaMethodRow
		if err := rows.Scan(
			&row.ID, &row.UserID, &row.MethodType, &row.Enabled, &row.Verified,
			&row.DisplayName, &row.LastUsedAt,
		); err != nil {
			return err
		}
		methodMap[row.UserID] = append(methodMap[row.UserID], toMfaMethod(row))
	}
	if rows.Err() != nil {
		return rows.Err()
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

// ── Sort / Search helpers ───────────────────────────────────────────────

var userSortSpec = platformsorting.NewSpec(map[string]string{
	"id":           "id",
	"createdAt":    "created_at",
	"updatedAt":    columnUpdatedAt,
	columnUsername: columnUsername,
	"userType":     "user_type",
	"status":       "status",
	"isAdmin":      "is_admin",
	"isForbidden":  "is_forbidden",
	"score":        "score",
	"lastSigninAt": "last_signin_at",
})

var allowedSearchFields = map[string]string{
	columnUsername: "u.username",
}

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
	return fmt.Sprintf("AND %s LIKE CONCAT('%%', ?, '%%')", column), searchValue
}

func buildOrderBy(sortField, sortOrder string) (string, error) {
	return userSortSpec.Resolve(sortField, sortOrder, "created_at")
}
