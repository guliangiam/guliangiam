package ldaps

import (
	"context"
	"fmt"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/domain/groups"
	domain "github.com/guliangiam/guliangiam/internal/domain/ldaps"
	"github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

type LDAPPreviewUserResult struct {
	Users      []*domain.LDAPPreviewUser `json:"users"`
	ExistUUIDs []string                  `json:"existUuids"`
	TotalCount int                       `json:"totalCount"`
	ExistCount int                       `json:"existCount"`
}

type LDAPSyncResult struct {
	TotalUsers   int      `json:"totalUsers"`
	AddedUsers   int      `json:"addedUsers"`
	UpdatedUsers int      `json:"updatedUsers"`
	FailedUsers  int      `json:"failedUsers"`
	TotalGroups  int      `json:"totalGroups"`
	AddedGroups  int      `json:"addedGroups"`
	FailedGroups int      `json:"failedGroups"`
	Errors       []string `json:"errors,omitempty"`
}

func (s Service) TestConnection(ctx context.Context, id, organizationID string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	// Ownership is checked before the bind credential is read or an external
	// connection is opened.
	l, err := s.UOW.Ldaps().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return err
	}

	if s.LDAPClient == nil {
		return apperrors.NewUnimplemented("ldap client is not configured")
	}

	// EnableSsl is the operator's explicit choice and wins over port inference:
	// a directory on a non-standard port with SSL enabled must still be dialed
	// over TLS, otherwise the bind password travels in cleartext.
	useTLS := l.EnableSsl || l.Port == 636 || l.Port == 6360
	if err := s.LDAPClient.TestConnection(l.Host, l.Port, l.BindDN, l.BindPassword, useTLS); err != nil {
		return apperrors.NewUnavailablef("ldap connection test failed: %v", err)
	}

	return nil
}

func (s Service) PreviewUsers(ctx context.Context, id, organizationID string) (*LDAPPreviewUserResult, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	// Ownership is checked before the bind credential is read or the external
	// directory is queried.
	l, err := s.UOW.Ldaps().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}

	if s.LDAPSyncer == nil {
		return nil, apperrors.NewUnimplemented("ldap syncer is not configured")
	}

	syncerConfig := s.buildSyncerConfig(l)
	previewUsers, err := s.LDAPSyncer.PreviewUsersDomain(ctx, syncerConfig)
	if err != nil {
		return nil, apperrors.NewInternalf("ldap preview failed: %v", err)
	}

	existUUIDs := s.findExistingUUIDs(ctx, l.OrganizationID, previewUsers)
	existSet := make(map[string]bool, len(existUUIDs))
	for _, id := range existUUIDs {
		existSet[id] = true
	}

	existCount := 0
	for _, u := range previewUsers {
		if existSet[u.UUID] {
			u.ExistsLocal = true
			existCount++
		}
	}

	return &LDAPPreviewUserResult{
		Users:      previewUsers,
		ExistUUIDs: existUUIDs,
		TotalCount: len(previewUsers),
		ExistCount: existCount,
	}, nil
}

func (s Service) SyncLdap(ctx context.Context, id string, uuids []string, organizationID string) (*LDAPSyncResult, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	// Resolve ownership under the request scope before elevating to system RLS:
	// the elevation is needed for the user/group writes, not for loading the
	// configuration of another tenant.
	l, err := s.UOW.Ldaps().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	if !l.IsEnabled {
		return nil, apperrors.NewInvalidArgument("ldap is disabled")
	}

	ctx = tenantctx.System(ctx)

	if s.LDAPSyncer == nil {
		return nil, apperrors.NewUnimplemented("ldap syncer is not configured")
	}

	syncerConfig := s.buildSyncerConfig(l)
	result := &LDAPSyncResult{}

	mappedUsers, err := s.LDAPSyncer.GetMappedUsersDomain(ctx, syncerConfig, uuids)
	if err != nil {
		return nil, apperrors.NewInternalf("ldap sync failed: %v", err)
	}
	result.TotalUsers = len(mappedUsers)

	// Build the LDAP-identity index once per sync instead of scanning every
	// local user for each directory entry (O(N×M) → O(N+M)).
	identityIndex, err := s.ldapIdentityIndex(ctx, l.OrganizationID)
	if err != nil {
		return nil, apperrors.NewInternalf("ldap sync: index local users: %v", err)
	}

	for _, mu := range mappedUsers {
		if mu.Username == "" {
			result.FailedUsers++
			result.Errors = append(result.Errors, fmt.Sprintf("ldap user missing uid: dn=%s", mu.DN))
			continue
		}

		added, err := s.upsertLDAPUser(ctx, l.OrganizationID, l.ID, mu, identityIndex)
		if err != nil {
			result.FailedUsers++
			result.Errors = append(result.Errors, fmt.Sprintf("ldap user upsert failed uid=%s: %v", mu.Username, err))
			continue
		}
		if added {
			result.AddedUsers++
		} else {
			result.UpdatedUsers++
		}
	}

	mappedGroups, err := s.LDAPSyncer.GetMappedGroupsDomain(ctx, syncerConfig)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("ldap group sync failed: %v", err))
	} else {
		result.TotalGroups = len(mappedGroups)
		for _, mg := range mappedGroups {
			if err := s.upsertLDAPGroup(ctx, l.OrganizationID, mg); err != nil {
				result.FailedGroups++
				result.Errors = append(result.Errors, fmt.Sprintf("ldap group upsert failed cn=%s: %v", mg.Name, err))
				continue
			}
			result.AddedGroups++
		}
	}

	now := s.Clock.Now()
	l.LastSyncAt = &now
	if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Ldaps().Update(ctx, l); err != nil {
			return err
		}
		if err := s.embedAuditEvent(
			tx,
			ctx,
			l.OrganizationID,
			"ldaps.update",
			"ldaps",
			l.ID,
			"ldaps",
			"ldaps.update",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	}); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to update lastSyncAt: %v", err))
	}

	return result, nil
}

func (s Service) upsertLDAPUser(ctx context.Context, orgID, ldapProviderID string, mu *domain.LDAPMappedUser, identityIndex map[string]string) (bool, error) {
	var existingUser *users.User
	if userID, ok := identityIndex[mu.UUID]; ok {
		fullUser, err := s.UOW.Users().GetByID(ctx, userID)
		if err != nil {
			return false, apperrors.WrapInternal(err, "load existing ldap user")
		}
		existingUser = fullUser
	}

	if existingUser != nil {
		existingUser.Profile.DisplayName = mu.DisplayName
		existingUser.Profile.Email = mu.Email
		existingUser.Profile.Phone = mu.Phone
		existingUser.Profile.FirstName = mu.GivenName
		existingUser.Profile.LastName = mu.Surname

		if mu.Disabled {
			existingUser.Status = statusDisabled
		} else if existingUser.Status == statusDisabled {
			existingUser.Status = "active"
		}

		if existingUser.Profile.Properties == nil {
			existingUser.Profile.Properties = make(map[string]any)
		}
		existingUser.Profile.Properties["title"] = mu.Title
		existingUser.Profile.Properties["department"] = mu.Department
		existingUser.Profile.Properties["employeeNumber"] = mu.EmployeeNumber
		existingUser.Profile.Properties["ldapDN"] = mu.DN

		return false, s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
			if err := tx.Users().UpdateProfile(ctx, &existingUser.Profile); err != nil {
				return apperrors.WrapInternal(err, "update user profile")
			}
			if err := s.embedAuditEvent(
				tx,
				ctx,
				existingUser.OrganizationID,
				"users.updated",
				"users",
				existingUser.ID,
				"users",
				"users.updated",
				map[string]any{},
			); err != nil {
				return err
			}
			if err := tx.Users().Update(ctx, existingUser); err != nil {
				return apperrors.WrapInternal(err, "update user")
			}
			return nil
		})
	}

	id, err := s.UUID.NewString()
	if err != nil {
		return false, apperrors.WrapInternal(err, "generate user id")
	}
	identityID, err := s.UUID.NewString()
	if err != nil {
		return false, apperrors.WrapInternal(err, "generate identity id")
	}

	newUser := &users.User{
		ID:             id,
		OrganizationID: orgID,
		Username:       mu.Username,
		UserType:       "normal",
		Status:         "active",
		RegisterType:   identityTypeLDAP,
		RegisterSource: identityTypeLDAP,
		Profile: users.UserProfile{
			UserID:      "",
			DisplayName: mu.DisplayName,
			FirstName:   mu.GivenName,
			LastName:    mu.Surname,
			Email:       mu.Email,
			Phone:       mu.Phone,
			Properties: map[string]any{
				"title":          mu.Title,
				"department":     mu.Department,
				"employeeNumber": mu.EmployeeNumber,
				"ldapDN":         mu.DN,
			},
		},
		Identities: []users.UserIdentity{
			{
				ID:           identityID,
				IdentityType: identityTypeLDAP,
				ProviderID:   ldapProviderID,
				Subject:      mu.UUID,
				LinkedAt:     s.Clock.Now(),
			},
		},
	}
	newUser.Profile.UserID = newUser.ID

	if mu.Disabled {
		newUser.Status = statusDisabled
	}

	return true, s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		existingByUsername, err := tx.Users().GetByUsername(ctx, orgID, mu.Username)
		if err == nil && existingByUsername != nil {
			newUser.Username = fmt.Sprintf("%s-ldap-%s", mu.Username, ldapUsernameSuffix(mu.UUID))
		}

		if err := tx.Users().Create(ctx, newUser); err != nil {
			return apperrors.WrapInternal(err, "create ldap user")
		}
		if err := s.embedAuditEvent(
			tx,
			ctx,
			newUser.OrganizationID,
			"users.created",
			"users",
			newUser.ID,
			"users",
			"users.created",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

func (s Service) upsertLDAPGroup(ctx context.Context, orgID string, mg *domain.LDAPMappedGroup) error {
	existing, err := s.findGroupByName(ctx, orgID, mg.Name)
	if err != nil {
		return err
	}

	if existing != nil {
		existing.DisplayName = mg.DisplayName
		return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
			if err := tx.Groups().Update(ctx, existing); err != nil {
				return err
			}
			if err := s.embedAuditEvent(
				tx,
				ctx,
				existing.OrganizationID,
				"groups.updated",
				"groups",
				existing.ID,
				"groups",
				"groups.updated",
				map[string]any{},
			); err != nil {
				return err
			}
			return nil
		})
	}

	groupID, err := s.UUID.NewString()
	if err != nil {
		return apperrors.WrapInternal(err, "generate group id")
	}
	newGroup := &groups.Group{
		ID:             groupID,
		OrganizationID: orgID,
		Name:           mg.Name,
		DisplayName:    mg.DisplayName,
		Type:           identityTypeLDAP,
		IsEnabled:      true,
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Groups().Create(ctx, newGroup); err != nil {
			return err
		}
		if err := s.embedAuditEvent(
			tx,
			ctx,
			newGroup.OrganizationID,
			"groups.created",
			"groups",
			newGroup.ID,
			"groups",
			"groups.created",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

// groupByNameRepository is the optional exact-name lookup implemented by the
// postgres groups repository. It stays outside the domain Repository interface
// so mirrors (MySQL) and test doubles without the capability keep working via
// the keyword-scan fallback below.
type groupByNameRepository interface {
	GetByNameForOrganization(ctx context.Context, organizationID, name string) (*groups.Group, error)
}

// findGroupByName resolves an existing group by exact name. The equality
// predicate replaces the old first-page keyword scan, which missed exact
// matches past page one and created duplicate groups.
func (s Service) findGroupByName(ctx context.Context, orgID, name string) (*groups.Group, error) {
	if byName, ok := s.UOW.Groups().(groupByNameRepository); ok {
		g, err := byName.GetByNameForOrganization(ctx, orgID, name)
		if err != nil {
			if apperrors.FromError(err).Code == apperrors.CodeNotFound {
				return nil, nil
			}
			return nil, apperrors.WrapInternal(err, "lookup existing group by name")
		}
		return g, nil
	}

	// Fallback for repositories without the exact-name capability: keyword
	// search is a substring match, so walk every matching page instead of only
	// the first one, otherwise an exact-name group past page one is missed.
	existingGroups, err := platformpagination.CollectAll(func(page, pageSize int) ([]*groups.Group, int64, error) {
		return s.UOW.Groups().List(ctx, groups.ListQuery{
			OrganizationID: orgID,
			Keyword:        name,
			Page:           page,
			PageSize:       pageSize,
		})
	})
	if err != nil {
		return nil, apperrors.WrapInternal(err, "search existing group")
	}
	for _, g := range existingGroups {
		if g.Name == name {
			return g, nil
		}
	}
	return nil, nil
}

func (s Service) findUserByLDAPIdentity(ctx context.Context, orgID, ldapUUID string) *users.User {
	index, err := s.ldapIdentityIndex(ctx, orgID)
	if err != nil {
		return nil
	}
	userID, ok := index[ldapUUID]
	if !ok {
		return nil
	}
	fullUser, err := s.UOW.Users().GetByID(ctx, userID)
	if err != nil {
		return nil
	}
	return fullUser
}

// ldapIdentityIndex walks the organization's users once and maps every LDAP
// identity subject to its user ID. Sync resolves each directory entry against
// this map instead of re-scanning all users per entry.
func (s Service) ldapIdentityIndex(ctx context.Context, orgID string) (map[string]string, error) {
	userList, err := platformpagination.CollectAll(func(page, pageSize int) ([]*users.User, int64, error) {
		return s.UOW.Users().List(ctx, users.ListQuery{
			OrganizationID: orgID,
			Page:           page,
			PageSize:       pageSize,
		})
	})
	if err != nil {
		return nil, err
	}
	index := make(map[string]string)
	for _, u := range userList {
		for i := range u.Identities {
			identity := &u.Identities[i]
			if identity.IdentityType == identityTypeLDAP {
				index[identity.Subject] = u.ID
			}
		}
	}
	return index, nil
}

// ldapUsernameSuffix derives the conflict-rename suffix from the directory
// uid. LDAP uids can be shorter than 8 characters ("abc"), so the slice must
// be bounded by the actual length instead of panicking.
func ldapUsernameSuffix(uuid string) string {
	if len(uuid) > 8 {
		return uuid[:8]
	}
	return uuid
}

func (s Service) findExistingUUIDs(ctx context.Context, orgID string, previewUsers []*domain.LDAPPreviewUser) []string {
	existUUIDs := []string{}

	// Preview must compare against every local user, not just the first served
	// page: the previous PageSize:1000 read was clamped to MaxPageSize.
	userList, err := platformpagination.CollectAll(func(page, pageSize int) ([]*users.User, int64, error) {
		return s.UOW.Users().List(ctx, users.ListQuery{
			OrganizationID: orgID,
			Page:           page,
			PageSize:       pageSize,
		})
	})
	if err != nil {
		return existUUIDs
	}

	ldapSubjects := make(map[string]bool)
	for _, u := range userList {
		for i := range u.Identities {
			identity := &u.Identities[i]
			if identity.IdentityType == identityTypeLDAP {
				ldapSubjects[identity.Subject] = true
			}
		}
	}

	for _, pu := range previewUsers {
		if ldapSubjects[pu.UUID] {
			existUUIDs = append(existUUIDs, pu.UUID)
		}
	}

	return existUUIDs
}
