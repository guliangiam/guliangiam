package scim

const (
	// ==================== ScimSchema Queries ====================

	QueryGetScimSchemaBySchemaID = `
		SELECT id, organization_id, schema_id, name, description, attributes, created_at, updated_at, revision
		FROM guliangiam.scim_schemas
		WHERE schema_id = $1
	`

	QueryCountScimSchemas = `
		SELECT COUNT(*)
		FROM guliangiam.scim_schemas
		WHERE organization_id = $1
	`

	QueryCreateScimSchema = `
		INSERT INTO guliangiam.scim_schemas (
			id, organization_id, schema_id, name, description, attributes,
			created_at, updated_at, revision
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (organization_id, schema_id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			attributes = EXCLUDED.attributes,
			updated_at = EXCLUDED.updated_at,
			revision = scim_schemas.revision + 1
	`

	QueryUpdateScimSchema = `
		UPDATE guliangiam.scim_schemas
		SET name = $2, description = $3, attributes = $4, updated_at = $5, revision = revision + 1
		WHERE schema_id = $1 AND organization_id = $6
		RETURNING revision
	`

	QueryDeleteScimSchema = `
		DELETE FROM guliangiam.scim_schemas WHERE schema_id = $1
	`

	// ==================== ScimUser Queries ====================

	// display_name / email / phone live in user_profiles, not in users (手册 3.7
	// 宽表拆分）。The join is LEFT so a user without a profile row still reads back.
	QueryGetScimUserByID = `
		SELECT u.id, u.organization_id, u.username,
		       COALESCE(p.display_name, ''), COALESCE(p.email, ''), COALESCE(p.phone, ''),
		       u.status, u.is_admin, u.is_forbidden, u.created_at, u.updated_at, u.revision
		FROM guliangiam.users u
		LEFT JOIN guliangiam.user_profiles p ON p.user_id = u.id
		WHERE u.id = $1 AND u.deleted_at IS NULL
	`

	QueryCountScimUsers = `
		SELECT COUNT(*)
		FROM guliangiam.users u
		WHERE u.organization_id = $1 AND u.deleted_at IS NULL
	`

	// The profile columns are written separately by QueryCreateUserProfile, so
	// this statement must not mention them — before DB-026 was fixed it named
	// users.display_name/email/phone, which do not exist and made every SCIM
	// provisioning write fail at parse time.
	QueryCreateScimUser = `
		INSERT INTO guliangiam.users (
			id, organization_id, username,
			status, is_admin, is_forbidden, created_at, updated_at,
			revision
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (organization_id, username) WHERE deleted_at IS NULL DO UPDATE SET
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at,
			revision = users.revision + 1
	`

	QueryUpdateScimUser = `
		UPDATE guliangiam.users
		SET status = $2, updated_at = $3, revision = revision + 1
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING revision
	`

	QueryDeleteScimUser = `
		UPDATE guliangiam.users SET deleted_at = NOW() WHERE id = $1
	`

	// User profile queries (for email/phone/name data)
	QueryGetUserEmail = `
		SELECT email FROM guliangiam.user_profiles
		WHERE user_id = $1 AND email IS NOT NULL AND email != ''
	`

	QueryGetUserPhone = `
		SELECT phone FROM guliangiam.user_profiles
		WHERE user_id = $1 AND phone IS NOT NULL AND phone != ''
	`

	QueryCreateUserProfile = `
		INSERT INTO guliangiam.user_profiles (
			user_id, display_name, first_name, last_name, email, phone, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			email = EXCLUDED.email,
			phone = EXCLUDED.phone,
			updated_at = EXCLUDED.updated_at
	`

	QueryUpdateUserProfile = `
		UPDATE guliangiam.user_profiles
		SET display_name = $2, email = $3, phone = $4,
		    first_name = $5, last_name = $6, updated_at = $7
		WHERE user_id = $1
	`

	// ==================== ScimGroup Queries ====================

	QueryGetScimGroupByID = `
		SELECT id, organization_id, name, display_name, created_at, updated_at, revision
		FROM guliangiam.groups
		WHERE id = $1
	`

	QueryCountScimGroups = `
		SELECT COUNT(*)
		FROM guliangiam.groups
		WHERE organization_id = $1
	`

	QueryCreateScimGroup = `
		INSERT INTO guliangiam.groups (
			id, organization_id, name, display_name, created_at, updated_at, revision
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (organization_id, name) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			updated_at = EXCLUDED.updated_at,
			revision = groups.revision + 1
	`

	QueryUpdateScimGroup = `
		UPDATE guliangiam.groups
		SET display_name = $2, name = $3, updated_at = $4, revision = revision + 1
		WHERE id = $1
		RETURNING revision
	`

	// groups has no deleted_at (it is not a soft-delete table), so deleting a SCIM
	// group is a hard DELETE — the same statement the groups module uses, which
	// lets the FK CASCADE clean up user_group_memberships.
	QueryDeleteScimGroup = `
		DELETE FROM guliangiam.groups WHERE id = $1
	`

	// ==================== Group Membership Queries ====================

	QueryGetUserGroups = `
		SELECT g.id, g.name, g.display_name
		FROM guliangiam.user_group_memberships ugm
		JOIN guliangiam.groups g ON ugm.group_id = g.id
		WHERE ugm.user_id = $1
		ORDER BY g.display_name
	`

	QueryGetGroupMembers = `
		SELECT u.id, u.username, COALESCE(p.display_name, '')
		FROM guliangiam.user_group_memberships ugm
		JOIN guliangiam.users u ON ugm.user_id = u.id
		LEFT JOIN guliangiam.user_profiles p ON p.user_id = u.id
		WHERE ugm.group_id = $1 AND u.deleted_at IS NULL
		ORDER BY p.display_name
	`

	QueryAddGroupMember = `
		INSERT INTO guliangiam.user_group_memberships (user_id, group_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, group_id) DO NOTHING
	`

	QueryRemoveGroupMember = `
		DELETE FROM guliangiam.user_group_memberships
		WHERE user_id = $1 AND group_id = $2
	`

	QueryRemoveGroupMembersByUser = `
		DELETE FROM guliangiam.user_group_memberships WHERE user_id = $1
	`

	QueryRemoveGroupMembersByGroup = `
		DELETE FROM guliangiam.user_group_memberships WHERE group_id = $1
	`
)
