package scim

import (
	"strconv"
	"strings"

	"github.com/guliangiam/guliangiam/internal/domain/groups"
	"github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func toScimUser(u *users.User) ScimUser {
	displayName := u.Profile.DisplayName
	if displayName == "" {
		displayName = u.Username
	}

	su := ScimUser{
		Schemas:     []string{SchemaUser},
		ID:          u.ID,
		UserName:    u.Username,
		DisplayName: displayName,
		Active:      !u.IsForbidden,
		Name: &ScimName{
			GivenName:  u.Profile.FirstName,
			FamilyName: u.Profile.LastName,
			Formatted:  displayName,
		},
		Meta: &ScimMeta{
			ResourceType: resourceTypeUser,
			Created:      u.CreatedAt,
			LastModified: u.UpdatedAt,
		},
	}

	if u.Profile.Email != "" {
		su.Emails = []ScimEmail{{Value: u.Profile.Email, Type: "work", Primary: true}}
	}
	if u.Profile.Phone != "" {
		su.PhoneNumbers = []ScimPhone{{Value: u.Profile.Phone, Type: "work", Primary: true}}
	}
	if u.ExternalID != "" {
		su.ExternalID = u.ExternalID
	}

	return su
}

func toScimGroup(g *groups.Group) ScimGroup {
	sg := ScimGroup{
		Schemas:     []string{SchemaGroup},
		ID:          g.ID,
		DisplayName: g.DisplayName,
		Meta: &ScimMeta{
			ResourceType: resourceTypeGroup,
			Created:      g.CreatedAt,
			LastModified: g.UpdatedAt,
		},
	}

	if len(g.Users) > 0 {
		sg.Members = make([]ScimGroupMember, 0, len(g.Users))
		for _, userID := range g.Users {
			sg.Members = append(sg.Members, ScimGroupMember{Value: userID})
		}
	}

	return sg
}

// ─────────────────────────────────────────────
// Filter application (AST-based)
// ─────────────────────────────────────────────

// applyUserFilterAST walks the AST and applies the filter to the user query.
// Since the domain ListQuery has limited filter fields, we handle the most
// common SCIM attributes and fall back to keyword search for others.
func applyUserFilterAST(node *FilterNode, query *users.ListQuery) {
	if node == nil {
		return
	}

	switch node.Op {
	case FilterAnd:
		applyUserFilterAST(node.Left, query)
		applyUserFilterAST(node.Right, query)
	case FilterOr:
		// For OR, we use keyword search as fallback since the query doesn't support OR natively
		applyUserFilterAST(node.Left, query)
	case FilterNot:
		// NOT is not directly supported by the query, skip for now
	case FilterPr:
		// "present" - no direct query mapping
	case FilterEq, FilterNe, FilterSw, FilterEw, FilterCo, FilterGt, FilterLt, FilterGe, FilterLe:
		applyUserComparisonFilter(node, query)
	default:
		applyUserComparisonFilter(node, query)
	}
}

func applyUserComparisonFilter(node *FilterNode, query *users.ListQuery) {
	attr := strings.ToLower(node.AttrPath)

	switch attr {
	case pathUsername:
		if node.Op == FilterEq {
			query.SearchField = pathUsername
			query.SearchValue = node.Value
		}
		if node.Op == FilterSw || node.Op == FilterCo {
			query.Keyword = node.Value
		}
	case pathEmailsValue, scimAttrEmails:
		eqEmail := node.Op == FilterEq
		keywordEmail := node.Op == FilterSw || node.Op == FilterCo
		if eqEmail {
			query.SearchField = "email"
			query.SearchValue = node.Value
		} else if keywordEmail {
			query.Keyword = node.Value
		}
	case scimAttrActive:
		if node.Op == FilterEq || node.Op == FilterNe {
			wantActive := strings.EqualFold(node.Value, "true")
			if node.Op == FilterNe {
				wantActive = !wantActive
			}
			if wantActive {
				query.Status = userStatusActive
			} else {
				query.Status = userStatusDisabled
			}
		}
	case pathDisplayName, pathNameFamilyName, pathNameGivenName:
		if node.Op == FilterEq || node.Op == FilterSw || node.Op == FilterCo {
			query.Keyword = node.Value
		}
	case "externalid":
		if node.Op == FilterEq {
			query.SearchField = "external_id"
			query.SearchValue = node.Value
		}
	default:
		// Fallback: use value as keyword search
		if node.Op == FilterEq || node.Op == FilterSw || node.Op == FilterCo {
			query.Keyword = node.Value
		}
	}
}

// ─────────────────────────────────────────────
// PATCH operations
// ─────────────────────────────────────────────

func applyPatchReplace(user *users.User, op ScimOperation) error {
	switch strings.ToLower(op.Path) {
	case scimAttrActive:
		val, ok := op.Value.(bool)
		if !ok {
			s, ok := op.Value.(string)
			if !ok {
				return apperrors.NewInvalidArgumentf("invalid value for active: %v", op.Value)
			}
			var err error
			val, err = strconv.ParseBool(s)
			if err != nil {
				return apperrors.NewInvalidArgumentf("invalid boolean value: %s", s)
			}
		}
		user.IsForbidden = !val
	case pathUsername:
		if s, ok := op.Value.(string); ok {
			user.Username = s
		}
	case pathDisplayName:
		if s, ok := op.Value.(string); ok {
			user.Profile.DisplayName = s
		}
	case pathNameGivenName:
		if s, ok := op.Value.(string); ok {
			user.Profile.FirstName = s
		}
	case pathNameFamilyName:
		if s, ok := op.Value.(string); ok {
			user.Profile.LastName = s
		}
	case scimAttrEmails:
		if m, ok := op.Value.(map[string]any); ok {
			if v, ok := m["value"].(string); ok {
				user.Profile.Email = v
			}
		}
	case pathPhoneNumbers:
		if m, ok := op.Value.(map[string]any); ok {
			if v, ok := m["value"].(string); ok {
				user.Profile.Phone = v
			}
		}
	default:
		// No path - value is a map of attributes
		if op.Path == "" {
			if m, ok := op.Value.(map[string]any); ok {
				if v, ok := m[scimAttrActive].(bool); ok {
					user.IsForbidden = !v
				}
				if v, ok := m[scimAttrUserName].(string); ok {
					user.Username = v
				}
				if v, ok := m["displayName"].(string); ok {
					user.Profile.DisplayName = v
				}
			}
		}
	}
	return nil
}

func applyPatchAdd(user *users.User, op ScimOperation) error {
	// "add" is similar to "replace" for simple attributes
	return applyPatchReplace(user, op)
}

// applyPatchRemove handles SCIM PATCH "remove" operation.
// Per RFC 7644, removing a non-existent attribute is a no-op (not an error).
func applyPatchRemove(user *users.User, op ScimOperation) {
	switch strings.ToLower(op.Path) {
	case pathUsername:
		// Cannot remove username, no-op
	case pathDisplayName:
		user.Profile.DisplayName = ""
	case pathNameGivenName:
		user.Profile.FirstName = ""
	case pathNameFamilyName:
		user.Profile.LastName = ""
	case scimAttrEmails, pathEmailsValue:
		user.Profile.Email = ""
	case pathPhoneNumbers, pathPhoneNumbersValue:
		user.Profile.Phone = ""
	case scimAttrActive:
		// Cannot remove active status, no-op
	default:
		// No path - value contains attributes to remove
		if op.Path == "" {
			if m, ok := op.Value.(map[string]any); ok {
				for k := range m {
					switch strings.ToLower(k) {
					case pathDisplayName:
						user.Profile.DisplayName = ""
					case pathNameGivenName:
						user.Profile.FirstName = ""
					case pathNameFamilyName:
						user.Profile.LastName = ""
					case scimAttrEmails, pathEmailsValue:
						user.Profile.Email = ""
					case pathPhoneNumbers, pathPhoneNumbersValue:
						user.Profile.Phone = ""
					}
				}
			}
		}
	}
}

// ─────────────────────────────────────────────
// Group PATCH operations
// ─────────────────────────────────────────────

func applyGroupPatchReplace(group *groups.Group, op ScimOperation) {
	switch strings.ToLower(op.Path) {
	case pathDisplayName:
		if s, ok := op.Value.(string); ok {
			group.DisplayName = s
			group.Name = s
		}
	case scimAttrMembers:
		// Value can be a single member object or an array of members
		switch v := op.Value.(type) {
		case map[string]any:
			if id, ok := v["value"].(string); ok {
				group.Users = []string{id}
			}
		case []interface{}:
			members := make([]string, 0, len(v))
			for _, item := range v {
				if m, ok := item.(map[string]any); ok {
					if id, ok := m["value"].(string); ok {
						members = append(members, id)
					}
				}
			}
			group.Users = members
		}
	default:
		// No path - value is a map of attributes
		if op.Path == "" {
			if m, ok := op.Value.(map[string]any); ok {
				if v, ok := m["displayName"].(string); ok {
					group.DisplayName = v
					group.Name = v
				}
			}
		}
	}
}

func applyGroupPatchAdd(group *groups.Group, op ScimOperation) {
	switch strings.ToLower(op.Path) {
	case scimAttrMembers:
		// Add members to existing list
		switch v := op.Value.(type) {
		case map[string]any:
			if id, ok := v["value"].(string); ok {
				group.Users = append(group.Users, id)
			}
		case []interface{}:
			for _, item := range v {
				if m, ok := item.(map[string]any); ok {
					if id, ok := m["value"].(string); ok {
						group.Users = append(group.Users, id)
					}
				}
			}
		}
	default:
		applyGroupPatchReplace(group, op)
	}
}

// applyGroupPatchRemove handles SCIM PATCH "remove" operation for groups.
func applyGroupPatchRemove(group *groups.Group, op ScimOperation) {
	switch strings.ToLower(op.Path) {
	case scimAttrMembers:
		// Remove specified members
		switch v := op.Value.(type) {
		case map[string]any:
			if id, ok := v["value"].(string); ok {
				group.Users = removeStringFromSlice(group.Users, id)
			}
		case []interface{}:
			for _, item := range v {
				if m, ok := item.(map[string]any); ok {
					if id, ok := m["value"].(string); ok {
						group.Users = removeStringFromSlice(group.Users, id)
					}
				}
			}
		default:
			// Remove all members if no value specified
			group.Users = nil
		}
	case pathDisplayName:
		group.DisplayName = ""
		group.Name = ""
	default:
		// No path - no-op for unknown attributes
	}
}

func removeStringFromSlice(slice []string, target string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != target {
			result = append(result, s)
		}
	}
	return result
}
