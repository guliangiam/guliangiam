package groups

import (
	"context"
	"errors"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

const maxGroupTreeDepth = 64

// validateGroupParent enforces the group tree invariants that the console
// filters client-side but every API caller must also satisfy:
//   - parent exists in the same organization;
//   - no self-reference;
//   - no cycle through descendants;
//   - bounded depth.
func validateGroupParent(ctx context.Context, tx appuow.UnitOfWork, organizationID, groupID, parentID string) error {
	if parentID == "" {
		return nil
	}
	if parentID == groupID {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
			"group cannot be its own parent",
			map[string]any{"field": "parentId", "value": parentID})
	}
	seen := map[string]bool{groupID: true}
	current := parentID
	for depth := 0; depth < maxGroupTreeDepth; depth++ {
		if seen[current] {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
				"group parent would create a cycle",
				map[string]any{"field": "parentId", "value": current})
		}
		seen[current] = true
		parent, err := tx.Groups().GetByIDForOrganization(ctx, current, organizationID)
		if err != nil {
			var appErr *apperrors.Error
			if errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound {
				return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
					"parent group not found in this organization",
					map[string]any{"field": "parentId", "value": current})
			}
			return err
		}
		if parent.ParentID == "" {
			return nil
		}
		current = parent.ParentID
	}
	return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
		"group tree depth exceeds the supported maximum",
		map[string]any{"field": "parentId", "maxDepth": maxGroupTreeDepth})
}

// normalizeGroupUsers accepts both member forms — user id and username (the
// seeds and the console pickers speak usernames, the Casbin sub convention) —
// and rejects references that belong to another organization or do not exist.
// Members are persisted as given; the only server-side rewrite is deduplication
// by resolved user id, so the same user cannot enter twice under two forms.
func normalizeGroupUsers(ctx context.Context, tx appuow.UnitOfWork, organizationID string, members []string) ([]string, error) {
	if len(members) == 0 {
		return nil, nil
	}
	users := tx.Users()
	if users == nil {
		// Unit tests that do not exercise membership keep a nil user repo.
		return append([]string(nil), members...), nil
	}
	seen := make(map[string]bool, len(members))
	resolved := make([]string, 0, len(members))
	for _, member := range members {
		if member == "" {
			return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
				"group member user id or username is required", map[string]any{"field": "users"})
		}
		id := member
		if platformuuid.IsValid(member) {
			if _, err := users.GetByIDForOrganization(ctx, member, organizationID); err != nil {
				var appErr *apperrors.Error
				if errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound {
					return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
						"group member not found in this organization",
						map[string]any{"field": "users", "value": member})
				}
				return nil, err
			}
		} else {
			// Not a user id: resolve as username (the console picker contract).
			user, err := users.GetByUsername(ctx, organizationID, member)
			if err != nil {
				var appErr *apperrors.Error
				if errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound {
					return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
						"group member not found in this organization",
						map[string]any{"field": "users", "value": member})
				}
				return nil, err
			}
			id = user.ID
		}
		if seen[id] {
			return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
				"duplicate group member", map[string]any{"field": "users", "value": member})
		}
		seen[id] = true
		resolved = append(resolved, member)
	}
	return resolved, nil
}
