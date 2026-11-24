package scim

import (
	"context"
	"errors"
	"strings"

	"github.com/guliangiam/guliangiam/internal/app/jml"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	"github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s Service) ListUsers(ctx context.Context, orgID string, query ScimListQuery) (*ScimListResponse[ScimUser], error) {
	startIndex := query.StartIndex
	if startIndex < 1 {
		startIndex = 1
	}
	count, err := normalizeScimCount(query.Count)
	if err != nil {
		return nil, err
	}
	sortField, sortOrder, err := resolveScimSort(query.SortBy, query.SortOrder, scimUserSortPaths)
	if err != nil {
		return nil, err
	}

	userQuery := users.ListQuery{
		OrganizationID: orgID,
		Page:           1,
		PageSize:       count,
		// SCIM startIndex is a 1-based row offset, not a page number
		// (BE-E-07): startIndex=2&count=100 must skip exactly one row.
		Offset:    startIndex - 1,
		SortField: sortField,
		SortOrder: sortOrder,
	}

	// Apply filter
	if query.Filter != "" {
		filterNode, err := ParseFilter(query.Filter)
		if err != nil {
			return nil, apperrors.NewInvalidArgumentf("invalid filter: %s", err)
		}
		applyUserFilterAST(filterNode, &userQuery)
	}

	domainUsers, total, err := s.UOW.Users().List(ctx, userQuery)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "list users")
	}

	resources := make([]ScimUser, 0, len(domainUsers))
	for _, u := range domainUsers {
		resources = append(resources, toScimUser(u))
	}

	return &ScimListResponse[ScimUser]{
		TotalResults: int(total),
		ItemsPerPage: count,
		StartIndex:   startIndex,
		Schemas:      []string{SchemaListResponse},
		Resources:    resources,
	}, nil
}

// GetUser returns a single SCIM user by ID.
func (s Service) GetUser(ctx context.Context, orgID, id string) (*ScimUser, error) {
	user, err := s.loadUserForOrganization(ctx, orgID, id)
	if err != nil {
		return nil, err
	}

	result := toScimUser(user)
	return &result, nil
}

// loadUserForOrganization resolves the SCIM by-ID user with the repository
// tenant predicate. The entity comparison after the read is a second assertion:
// a repository implementation that lost its WHERE clause still cannot serve a
// foreign user, and both causes stay a SCIM 404.
func (s Service) loadUserForOrganization(ctx context.Context, orgID, id string) (*users.User, error) {
	user, err := s.UOW.Users().GetByIDForOrganization(ctx, id, orgID)
	if err != nil {
		return nil, err
	}
	if user.OrganizationID != orgID {
		return nil, apperrors.NewNotFound("user not found")
	}
	return user, nil
}

// CreateUser creates a user from a SCIM request.
//
// The SCIM payload carries no primary key or timestamps. Building the aggregate
// without them made every POST fail at the database boundary: PostgreSQL
// rejected the empty users.id uuid cast (and then the user_profiles FK), while
// MySQL rolled the profile insert back. The aggregate now owns id, CreatedAt/
// UpdatedAt, Profile.UserID and Revision, and the users/profile/child writes
// share one transaction.
func (s Service) CreateUser(ctx context.Context, orgID string, scimUser ScimUser) (*ScimUser, error) {
	if err := requireOrganizationID(orgID); err != nil {
		return nil, err
	}
	if scimUser.UserName == "" {
		return nil, apperrors.NewInvalidArgument("userName is required")
	}

	user, err := s.newScimUser(orgID, scimUser)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "generate user id")
	}

	if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		repo := tx.Users()

		existing, err := repo.GetByUsername(ctx, orgID, scimUser.UserName)
		if err == nil && existing != nil {
			return apperrors.NewConflictf("user with userName %q already exists", scimUser.UserName)
		}
		if err != nil && !isNotFoundError(err) {
			return err
		}

		if err := repo.Create(ctx, user); err != nil {
			return err
		}
		if s.JML != nil {
			// Joiner（ADR-030）：SCIM 建号同事务触发 JML（SCIM 载荷不带角色，
			// initialRoles 为空；出站 user.created 仍会入 outbox）。
			return s.JML.OnUserCreated(ctx, tx, user.ID, orgID, jml.SafeUserPayload(user), nil)
		}
		return nil
	}); err != nil {
		return nil, scimWriteError(err, "create user")
	}

	result := toScimUser(user)
	return &result, nil
}

// newScimUser builds the complete user aggregate the repositories expect:
// generated UUID, injected clock timestamps, Revision 1, a profile keyed by the
// same id, and non-nil empty child collections so the five user_* upserts store
// empty JSON arrays rather than JSON null.
func (s Service) newScimUser(orgID string, scimUser ScimUser) (*users.User, error) {
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, err
	}

	now := s.Clock.Now()
	profile := users.UserProfile{
		UserID:      id,
		DisplayName: scimUser.DisplayName,
		Properties:  map[string]any{},
	}
	if scimUser.Name != nil {
		profile.FirstName = scimUser.Name.GivenName
		profile.LastName = scimUser.Name.FamilyName
	}
	if len(scimUser.Emails) > 0 {
		profile.Email = scimUser.Emails[0].Value
	}
	if len(scimUser.PhoneNumbers) > 0 {
		profile.Phone = scimUser.PhoneNumbers[0].Value
	}

	return &users.User{
		ID:                 id,
		OrganizationID:     orgID,
		Username:           scimUser.UserName,
		ExternalID:         scimUser.ExternalID,
		Status:             userStatusActive,
		IsForbidden:        !scimUser.Active,
		CreatedAt:          now,
		UpdatedAt:          now,
		Revision:           1,
		RecoveryCodeHashes: []string{},
		Addresses:          []users.Address{},
		ManagedAccounts:    []users.ManagedAccount{},
		FaceIds:            []users.FaceId{},
		Cart:               []users.ProductInfo{},
		MfaItems:           []users.MfaItem{},
		MfaAccounts:        []users.MfaAccount{},
		ApplicationScopes:  []users.ConsentRecord{},
		Profile:            profile,
	}, nil
}

func isNotFoundError(err error) bool {
	var appErr *apperrors.Error
	return errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound
}

// ReplaceUser replaces a user entirely (PUT).
func (s Service) ReplaceUser(ctx context.Context, orgID, id string, scimUser ScimUser) (*ScimUser, error) {
	if err := requireWriteScope(orgID, id); err != nil {
		return nil, err
	}

	user, err := s.loadUserForOrganization(ctx, orgID, id)
	if err != nil {
		return nil, err
	}

	// Update fields from SCIM
	wasForbidden := user.IsForbidden
	user.Username = scimUser.UserName
	user.ExternalID = scimUser.ExternalID
	user.IsForbidden = !scimUser.Active
	user.Profile.DisplayName = scimUser.DisplayName

	if scimUser.Name != nil {
		user.Profile.FirstName = scimUser.Name.GivenName
		user.Profile.LastName = scimUser.Name.FamilyName
	}
	if len(scimUser.Emails) > 0 {
		user.Profile.Email = scimUser.Emails[0].Value
	}
	if len(scimUser.PhoneNumbers) > 0 {
		user.Profile.Phone = scimUser.PhoneNumbers[0].Value
	}

	user.UpdatedAt = s.Clock.Now()
	user.Revision++
	if err := s.persistUserWithProfile(ctx, user, orgID, s.leaverHook(user, orgID, wasForbidden)); err != nil {
		return nil, err
	}

	result := toScimUser(user)
	return &result, nil
}

// PatchUser applies a partial update to a user (PATCH).
func (s Service) PatchUser(ctx context.Context, orgID, id string, patchOp ScimPatchOp) (*ScimUser, error) {
	if err := requireWriteScope(orgID, id); err != nil {
		return nil, err
	}

	user, err := s.loadUserForOrganization(ctx, orgID, id)
	if err != nil {
		return nil, err
	}

	wasForbidden := user.IsForbidden
	for _, op := range patchOp.Operations {
		switch strings.ToLower(op.Op) {
		case "replace":
			if err := applyPatchReplace(user, op); err != nil {
				return nil, err
			}
		case "add":
			if err := applyPatchAdd(user, op); err != nil {
				return nil, err
			}
		case "remove":
			applyPatchRemove(user, op)
		default:
			return nil, apperrors.NewInvalidArgumentf("unsupported PATCH op: %s", op.Op)
		}
	}

	user.UpdatedAt = s.Clock.Now()
	user.Revision++
	if err := s.persistUserWithProfile(ctx, user, orgID, s.leaverHook(user, orgID, wasForbidden)); err != nil {
		return nil, err
	}

	result := toScimUser(user)
	return &result, nil
}

// DeleteUser disables a user. SCIM DELETE on this surface is "disable", not a
// physical delete (docs/design/protocol/scim.md), so the write only flips
// is_forbidden on the aggregate that was loaded for this tenant. The previous
// implementation passed a partial User{ID, IsForbidden, Revision}; once the
// revision guard matched, upsertUserSubtables rewrote balance/MFA/collections/
// custom attributes from zero values. Loading the full aggregate keeps those
// child rows intact while revision++ still participates in the optimistic lock.
// DeleteUser implements the SCIM "DELETE = deactivate" contract: the user row
// is not physically deleted. Deactivation must land with the same side effects
// as the identity-management retirement flow — audit + outbox, session and token
// revocation — all in one transaction, otherwise the disabled account keeps
// usable credentials.
func (s Service) DeleteUser(ctx context.Context, orgID, id string) error {
	if err := requireWriteScope(orgID, id); err != nil {
		return err
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		user, err := tx.Users().GetByIDForOrganization(ctx, id, orgID)
		if err != nil {
			return err
		}
		if user.OrganizationID != orgID {
			return apperrors.NewNotFound("user not found")
		}

		now := s.Clock.Now()
		user.IsForbidden = true
		user.UpdatedAt = now
		user.Revision++
		if err := tx.Users().UpdateForOrganization(ctx, user, orgID); err != nil {
			return scimWriteError(err, "deactivate user")
		}
		if err := s.embedAuditEvent(
			tx, ctx,
			user.OrganizationID, "", "user.updated", "User", user.ID, "user", "user.updated",
			map[string]any{"deactivated": true, "source": "scim"},
		); err != nil {
			return err
		}
		if _, err := tx.Sessions().RevokeByUserExcept(ctx, user.ID, "", now); err != nil {
			return err
		}
		if _, err := tx.Tokens().RevokeByQuery(ctx, tokendomain.RevokeQuery{UserID: user.ID}, now); err != nil {
			return err
		}
		if s.JML != nil {
			// Leaver（ADR-030）：停用同事务清理角色绑定并写出站事件。
			return s.JML.OnUserDeactivated(ctx, tx, user.ID, orgID, jml.SafeUserPayload(user))
		}
		return nil
	})
}

// persistUserAggregate writes the complete aggregate with the repository's
// tenant predicate. Callers must increment Revision first: the repository
// compares against Revision-1 (the value the row held when it was loaded), so
// an untouched revision makes every write miss and turn into NotFound.
func (s Service) persistUserAggregate(ctx context.Context, user *users.User, orgID string) error {
	if err := s.UOW.Users().UpdateForOrganization(ctx, user, orgID); err != nil {
		return scimWriteError(err, "update user")
	}
	return nil
}

// persistUserWithProfile writes the aggregate and its user_profiles row in the
// same transaction. PUT/PATCH expose displayName/name/emails/phoneNumbers, so a
// successful save that skipped UpdateProfile would only echo the request back
// while leaving the previous profile in place. Profile user_id is forced to the
// aggregate id so the upsert cannot target a foreign or empty row.
func (s Service) persistUserWithProfile(ctx context.Context, user *users.User, orgID string, afterWrite ...func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	user.Profile.UserID = user.ID

	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		repo := tx.Users()
		if err := repo.UpdateForOrganization(ctx, user, orgID); err != nil {
			return err
		}
		if err := repo.UpdateProfile(ctx, &user.Profile); err != nil {
			return apperrors.WrapInternal(err, "update user profile")
		}
		for _, hook := range afterWrite {
			if hook == nil {
				continue
			}
			if err := hook(ctx, tx); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return scimWriteError(err, "update user")
	}
	return nil
}

// leaverHook 在 active → forbidden 翻转时返回同事务 Leaver 钩子；
// 其余情况（JML 未注入、本就停用、未停用）返回 nil。
func (s Service) leaverHook(user *users.User, orgID string, wasForbidden bool) func(ctx context.Context, tx appuow.UnitOfWork) error {
	if s.JML == nil || wasForbidden || !user.IsForbidden {
		return nil
	}
	return func(ctx context.Context, tx appuow.UnitOfWork) error {
		return s.JML.OnUserDeactivated(ctx, tx, user.ID, orgID, jml.SafeUserPayload(user))
	}
}

// scimWriteError keeps typed application errors (not_found / conflict / ...)
// intact; wrapping them as internal would turn a tenant-scoped 404 or a stale
// revision 409 into a 500.
func scimWriteError(err error, message string) error {
	var appErr *apperrors.Error
	if errors.As(err, &appErr) {
		return err
	}
	return apperrors.WrapInternal(err, message)
}

func requireOrganizationID(organizationID string) error {
	if strings.TrimSpace(organizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	return nil
}

func requireWriteScope(organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	return nil
}

// ─────────────────────────────────────────────
// Group CRUD
// ─────────────────────────────────────────────

// ListGroups returns a SCIM list of groups for the given organization.
