package scim

import (
	"context"
	"strings"

	"github.com/guliangiam/guliangiam/internal/domain/groups"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s Service) ListGroups(ctx context.Context, orgID string, query ScimListQuery) (*ScimListResponse[ScimGroup], error) {
	startIndex := query.StartIndex
	if startIndex < 1 {
		startIndex = 1
	}
	count, err := normalizeScimCount(query.Count)
	if err != nil {
		return nil, err
	}
	sortField, sortOrder, err := resolveScimSort(query.SortBy, query.SortOrder, scimGroupSortPaths)
	if err != nil {
		return nil, err
	}

	groupQuery := groups.ListQuery{
		OrganizationID: orgID,
		Page:           1,
		PageSize:       count,
		// SCIM startIndex is a 1-based row offset (BE-E-07).
		Offset:    startIndex - 1,
		SortField: sortField,
		SortOrder: sortOrder,
	}

	domainGroups, total, err := s.UOW.Groups().List(ctx, groupQuery)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "list groups")
	}

	resources := make([]ScimGroup, 0, len(domainGroups))
	for _, g := range domainGroups {
		resources = append(resources, toScimGroup(g))
	}

	return &ScimListResponse[ScimGroup]{
		TotalResults: int(total),
		ItemsPerPage: count,
		StartIndex:   startIndex,
		Schemas:      []string{SchemaListResponse},
		Resources:    resources,
	}, nil
}

// GetGroup returns a single SCIM group by ID.
func (s Service) GetGroup(ctx context.Context, orgID, id string) (*ScimGroup, error) {
	group, err := s.loadGroupForOrganization(ctx, orgID, id)
	if err != nil {
		return nil, err
	}

	result := toScimGroup(group)
	return &result, nil
}

// loadGroupForOrganization resolves the SCIM by-ID group with the repository
// tenant predicate. The entity comparison after the read is a second assertion:
// a repository implementation that lost its WHERE clause still cannot serve a
// foreign group, and both causes stay a SCIM 404.
func (s Service) loadGroupForOrganization(ctx context.Context, orgID, id string) (*groups.Group, error) {
	group, err := s.UOW.Groups().GetByIDForOrganization(ctx, id, orgID)
	if err != nil {
		return nil, err
	}
	if group.OrganizationID != orgID {
		return nil, apperrors.NewNotFound("group not found")
	}
	return group, nil
}

// CreateGroup creates a group from a SCIM request.
func (s Service) CreateGroup(ctx context.Context, orgID string, scimGroup ScimGroup) (*ScimGroup, error) {
	if scimGroup.DisplayName == "" {
		return nil, apperrors.NewInvalidArgument("displayName is required")
	}

	group := &groups.Group{
		OrganizationID: orgID,
		Name:           scimGroup.DisplayName,
		DisplayName:    scimGroup.DisplayName,
		IsEnabled:      true,
	}

	// Map members
	if len(scimGroup.Members) > 0 {
		members := make([]string, 0, len(scimGroup.Members))
		for _, m := range scimGroup.Members {
			members = append(members, m.Value)
		}
		group.Users = members
	}

	if err := s.UOW.Groups().Create(ctx, group); err != nil {
		return nil, apperrors.WrapInternal(err, "create group")
	}

	result := toScimGroup(group)
	return &result, nil
}

// DeleteGroup deletes a group.
func (s Service) DeleteGroup(ctx context.Context, orgID, id string) error {
	group, err := s.loadGroupForOrganization(ctx, orgID, id)
	if err != nil {
		return err
	}

	return s.UOW.Groups().Delete(ctx, group.ID, orgID)
}

// ReplaceGroup replaces a group entirely (PUT).
func (s Service) ReplaceGroup(ctx context.Context, orgID, id string, scimGroup ScimGroup) (*ScimGroup, error) {
	group, err := s.loadGroupForOrganization(ctx, orgID, id)
	if err != nil {
		return nil, err
	}

	// Update fields from SCIM
	group.DisplayName = scimGroup.DisplayName
	if scimGroup.DisplayName != "" {
		group.Name = scimGroup.DisplayName
	}

	// Map members
	if len(scimGroup.Members) > 0 {
		members := make([]string, 0, len(scimGroup.Members))
		for _, m := range scimGroup.Members {
			members = append(members, m.Value)
		}
		group.Users = members
	} else {
		group.Users = nil
	}

	if err := s.UOW.Groups().Update(ctx, group); err != nil {
		return nil, apperrors.WrapInternal(err, "replace group")
	}

	result := toScimGroup(group)
	return &result, nil
}

// PatchGroup applies a partial update to a group (PATCH).
func (s Service) PatchGroup(ctx context.Context, orgID, id string, patchOp ScimPatchOp) (*ScimGroup, error) {
	group, err := s.loadGroupForOrganization(ctx, orgID, id)
	if err != nil {
		return nil, err
	}

	for _, op := range patchOp.Operations {
		switch strings.ToLower(op.Op) {
		case "replace":
			applyGroupPatchReplace(group, op)
		case "add":
			applyGroupPatchAdd(group, op)
		case "remove":
			applyGroupPatchRemove(group, op)
		default:
			return nil, apperrors.NewInvalidArgumentf("unsupported PATCH op: %s", op.Op)
		}
	}

	if err := s.UOW.Groups().Update(ctx, group); err != nil {
		return nil, apperrors.WrapInternal(err, "patch group")
	}

	result := toScimGroup(group)
	return &result, nil
}

// ─────────────────────────────────────────────
// Mappers
// ─────────────────────────────────────────────
