package scim

import (
	"context"
	"errors" // stdlib errors retained for errors.Is/errors.As only; apperrors used for error creation
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/guliangiam/guliangiam/internal/domain/scim"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) GetUserByID(ctx context.Context, userID string) (*scim.ScimUser, error) {
	var model ScimUserModel
	var status string
	err := r.pool.QueryRow(ctx, QueryGetScimUserByID, userID).Scan(
		&model.ID,
		&model.OrganizationID,
		&model.Username,
		&model.DisplayName,
		&model.Email,
		&model.Phone,
		&status,
		&model.IsAdmin,
		&model.IsForbidden,
		&model.CreatedAt,
		&model.UpdatedAt,
		&model.Revision,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, apperrors.WrapInternalf(err, "failed to get scim user")
	}

	model.IsActive = (status == userStatusActive)

	// Get user emails from user_profiles
	emails, err := r.getUserEmails(ctx, userID)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "failed to get user emails")
	}

	// Get user phones
	phones, err := r.getUserPhones(ctx, userID)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "failed to get user phones")
	}

	// Get user groups
	groups, err := r.getUserGroups(ctx, userID)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "failed to get user groups")
	}

	return r.modelToUser(&model, emails, phones, groups)
}

func (r *Repository) ListUsers(ctx context.Context, organizationID string, query *scim.ListQuery) ([]*scim.ScimUser, int64, error) {
	// Implementation similar to ListSchemas
	// This is a simplified version - full implementation would include proper filter parsing
	// display_name / email / phone are on user_profiles, not on users (手册 3.7
	// 宽表拆分); the LEFT JOIN keeps users without a profile row visible, and the
	// ORDER BY uses the same expression so LIMIT/OFFSET paging stays deterministic.
	baseQuery := `FROM guliangiam.users u
		LEFT JOIN guliangiam.user_profiles p ON p.user_id = u.id
		WHERE u.organization_id = $1 AND u.deleted_at IS NULL`
	countQuery := `SELECT COUNT(*) ` + baseQuery
	selectQuery := `SELECT u.id, u.organization_id, u.username,
	                      COALESCE(p.display_name, ''), COALESCE(p.email, ''), COALESCE(p.phone, ''),
	                      u.status, u.is_admin, u.is_forbidden, u.created_at, u.updated_at, u.revision ` + baseQuery

	args := []interface{}{organizationID}

	// Get total count
	var total int64
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, apperrors.WrapInternalf(err, "failed to count scim users")
	}

	// Add ordering and pagination
	selectQuery += " ORDER BY u.username"

	if query != nil && query.Count > 0 {
		selectQuery += " LIMIT $2"
		args = append(args, query.Count)
	}

	if query != nil && query.StartIndex > 0 {
		offset := query.StartIndex - 1
		if len(args) == 1 {
			selectQuery += " OFFSET $2"
		} else {
			selectQuery += " OFFSET $3"
		}
		args = append(args, offset)
	}

	// Execute query
	rows, err := r.pool.Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, apperrors.WrapInternalf(err, "failed to list scim users")
	}
	defer rows.Close()

	var users []*scim.ScimUser
	for rows.Next() {
		var model ScimUserModel
		var status string
		err := rows.Scan(
			&model.ID,
			&model.OrganizationID,
			&model.Username,
			&model.DisplayName,
			&model.Email,
			&model.Phone,
			&status,
			&model.IsAdmin,
			&model.IsForbidden,
			&model.CreatedAt,
			&model.UpdatedAt,
			&model.Revision,
		)
		if err != nil {
			return nil, 0, apperrors.WrapInternalf(err, "failed to scan scim user")
		}

		model.IsActive = (status == userStatusActive)

		// Get related data
		emails, _ := r.getUserEmails(ctx, model.ID.String())
		phones, _ := r.getUserPhones(ctx, model.ID.String())
		groups, _ := r.getUserGroups(ctx, model.ID.String())

		user, err := r.modelToUser(&model, emails, phones, groups)
		if err != nil {
			return nil, 0, apperrors.WrapInternalf(err, "failed to convert scim user")
		}
		users = append(users, user)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	return users, total, nil
}

func (r *Repository) CreateUser(ctx context.Context, user *scim.ScimUser) error {
	active := user.Active != nil && *user.Active
	status := userStatusActive
	if !active {
		status = userStatusDisabled
	}

	// Safely extract primary email and phone
	var primaryEmail, primaryPhone *string
	if len(user.Emails) > 0 {
		for _, e := range user.Emails {
			if e.Primary {
				primaryEmail = &e.Value
				break
			}
		}
		if primaryEmail == nil {
			primaryEmail = &user.Emails[0].Value
		}
	}
	if len(user.PhoneNumbers) > 0 {
		for _, p := range user.PhoneNumbers {
			if p.Primary {
				primaryPhone = &p.Value
				break
			}
		}
		if primaryPhone == nil {
			primaryPhone = &user.PhoneNumbers[0].Value
		}
	}

	now := time.Now()
	// display_name / email / phone are not columns of users; they are written to
	// user_profiles by the statement below (DB-026).
	_, err := r.pool.Exec(ctx, QueryCreateScimUser,
		user.ID,
		user.OrganizationID,
		user.UserName,
		status,
		false, // is_admin
		false, // is_forbidden
		now,
		now,
		user.Revision,
	)

	if err != nil {
		return apperrors.WrapInternalf(err, "failed to create scim user")
	}

	// Create user_profile with email/phone/name data
	firstName, lastName := "", ""
	if user.Name != nil {
		firstName = user.Name.GivenName
		lastName = user.Name.FamilyName
	}
	var profileEmail, profilePhone string
	if primaryEmail != nil {
		profileEmail = *primaryEmail
	}
	if primaryPhone != nil {
		profilePhone = *primaryPhone
	}
	_, _ = r.pool.Exec(ctx, QueryCreateUserProfile,
		user.ID,
		user.DisplayName,
		firstName,
		lastName,
		profileEmail,
		profilePhone,
		now,
		now,
	)

	// Add user to specified groups
	for _, group := range user.Groups {
		_ = r.addGroupMember(ctx, group.Value, user.ID)
	}

	return nil
}

func (r *Repository) UpdateUser(ctx context.Context, user *scim.ScimUser) error {
	// SCIM PUT: full replacement of user attributes
	active := user.Active != nil && *user.Active
	status := userStatusActive
	if !active {
		status = userStatusDisabled
	}

	// Extract primary email and phone for users table
	var primaryEmail, primaryPhone *string
	if len(user.Emails) > 0 {
		for _, e := range user.Emails {
			if e.Primary {
				primaryEmail = &e.Value
				break
			}
		}
		if primaryEmail == nil {
			primaryEmail = &user.Emails[0].Value
		}
	}
	if len(user.PhoneNumbers) > 0 {
		for _, p := range user.PhoneNumbers {
			if p.Primary {
				primaryPhone = &p.Value
				break
			}
		}
		if primaryPhone == nil {
			primaryPhone = &user.PhoneNumbers[0].Value
		}
	}

	// Update users table (status only; profile columns are updated separately)
	var newRevision int64
	err := r.pool.QueryRow(ctx, QueryUpdateScimUser,
		user.ID,
		status,
		time.Now(),
	).Scan(&newRevision)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return apperrors.WrapInternalf(err, "failed to update scim user")
	}

	user.Revision = newRevision
	user.UpdatedAt = time.Now()

	// Update user_profiles (email, phone, display_name, name components)
	firstName, lastName := "", ""
	if user.Name != nil {
		firstName = user.Name.GivenName
		lastName = user.Name.FamilyName
	}
	_, _ = r.pool.Exec(ctx, QueryUpdateUserProfile,
		user.ID,
		user.DisplayName,
		primaryEmail,
		primaryPhone,
		firstName,
		lastName,
		time.Now(),
	)

	// Sync group memberships: delete all then re-insert
	_, _ = r.pool.Exec(ctx, QueryRemoveGroupMembersByUser, user.ID)
	for _, group := range user.Groups {
		_, _ = r.pool.Exec(ctx, QueryAddGroupMember,
			user.ID, group.Value,
		)
	}

	return nil
}

func (r *Repository) DeleteUser(ctx context.Context, userID string) error {
	result, err := r.pool.Exec(ctx, QueryDeleteScimUser, userID)
	if err != nil {
		return apperrors.WrapInternalf(err, "failed to delete scim user")
	}

	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (r *Repository) PatchUser(ctx context.Context, userID string, patches []scim.PatchOperation) (*scim.ScimUser, error) {
	// Get current user
	user, err := r.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Apply patches
	for _, patch := range patches {
		switch patch.Op {
		case "replace":
			r.applyReplacePatch(user, patch)
		case "add":
			r.applyAddPatch(user, patch)
		case "remove":
			r.applyRemovePatch(user, patch)
		}
	}

	// Update in database
	err = r.UpdateUser(ctx, user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// ==================== ScimGroup Operations ====================
