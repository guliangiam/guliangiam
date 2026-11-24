package scim

import (
	"github.com/guliangiam/guliangiam/internal/domain/scim"
)

func (r *Repository) applyReplacePatch(user *scim.ScimUser, patch scim.PatchOperation) {
	switch patch.Path {
	case attrUserName:
		if s, ok := patch.Value.(string); ok {
			user.UserName = s
		}
	case attrDisplayName:
		if s, ok := patch.Value.(string); ok {
			user.DisplayName = s
		}
	case attrActive:
		if b, ok := patch.Value.(bool); ok {
			user.Active = &b
		}
	case attrEmails:
		user.Emails = patchToEmails(patch.Value)
	case attrPhoneNumbers:
		user.PhoneNumbers = patchToPhones(patch.Value)
	case attrName:
		if n, ok := patch.Value.(map[string]interface{}); ok {
			if user.Name == nil {
				user.Name = &scim.Name{}
			}
			if gn, ok := n["givenName"].(string); ok {
				user.Name.GivenName = gn
			}
			if fn, ok := n["familyName"].(string); ok {
				user.Name.FamilyName = fn
			}
		}
	}
}

func (r *Repository) applyAddPatch(user *scim.ScimUser, patch scim.PatchOperation) {
	switch patch.Path {
	case attrUserName:
		if s, ok := patch.Value.(string); ok {
			user.UserName = s
		}
	case attrDisplayName:
		if s, ok := patch.Value.(string); ok {
			user.DisplayName = s
		}
	case attrActive:
		if b, ok := patch.Value.(bool); ok {
			user.Active = &b
		}
	case attrEmails:
		newEmails := patchToEmails(patch.Value)
		user.Emails = append(user.Emails, newEmails...)
	case attrPhoneNumbers:
		newPhones := patchToPhones(patch.Value)
		user.PhoneNumbers = append(user.PhoneNumbers, newPhones...)
	case attrGroups:
		newGroups := patchToGroupRefs(patch.Value)
		user.Groups = append(user.Groups, newGroups...)
	}
}

func (r *Repository) applyRemovePatch(user *scim.ScimUser, patch scim.PatchOperation) {
	// Handle remove operation
	switch patch.Path {
	case attrUserName:
		user.UserName = ""
	case attrDisplayName:
		user.DisplayName = ""
	case attrActive:
		user.Active = nil
	case attrEmails:
		user.Emails = nil
	case attrPhoneNumbers:
		user.PhoneNumbers = nil
	case attrGroups:
		user.Groups = nil
	}
}

// applyGroupReplacePatch handles SCIM PATCH replace on a Group.
func (r *Repository) applyGroupReplacePatch(group *scim.ScimGroup, patch scim.PatchOperation) {
	switch patch.Path {
	case attrDisplayName:
		if s, ok := patch.Value.(string); ok {
			group.DisplayName = s
		}
	case attrMembers:
		group.Members = patchToMembers(patch.Value)
	}
}

// applyGroupAddPatch handles SCIM PATCH add on a Group.
func (r *Repository) applyGroupAddPatch(group *scim.ScimGroup, patch scim.PatchOperation) {
	switch patch.Path {
	case attrDisplayName:
		if s, ok := patch.Value.(string); ok {
			group.DisplayName = s
		}
	case attrMembers:
		newMembers := patchToMembers(patch.Value)
		// Append members, avoiding duplicates
		existing := make(map[string]bool)
		for _, m := range group.Members {
			existing[m.Value] = true
		}
		for _, m := range newMembers {
			if !existing[m.Value] {
				group.Members = append(group.Members, m)
			}
		}
	}
}

// applyGroupRemovePatch handles SCIM PATCH remove on a Group.
func (r *Repository) applyGroupRemovePatch(group *scim.ScimGroup, patch scim.PatchOperation) {
	switch patch.Path {
	case attrDisplayName:
		group.DisplayName = ""
	case attrMembers:
		if patch.Value != nil {
			// Remove specific members
			removeIDs := patchToMemberIDs(patch.Value)
			filtered := group.Members[:0]
			for _, m := range group.Members {
				if !removeIDs[m.Value] {
					filtered = append(filtered, m)
				}
			}
			group.Members = filtered
		} else {
			// Remove all members
			group.Members = nil
		}
	}
}

// patchToMembers converts a patch Value to a slice of Member.
func patchToMembers(value interface{}) []scim.Member {
	var members []scim.Member

	// Try as []interface{} first (common JSON decode result)
	switch v := value.(type) {
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				member := scim.Member{Type: resourceTypeUser}
				if val, ok := m["value"].(string); ok {
					member.Value = val
				}
				if disp, ok := m["display"].(string); ok {
					member.Display = disp
				}
				members = append(members, member)
			}
		}
	case []scim.Member:
		members = v
	}

	return members
}

// patchToMemberIDs extracts member IDs from a patch Value for removal.
func patchToMemberIDs(value interface{}) map[string]bool {
	ids := make(map[string]bool)

	switch v := value.(type) {
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				if val, ok := m["value"].(string); ok {
					ids[val] = true
				}
			}
			if s, ok := item.(string); ok {
				ids[s] = true
			}
		}
	case string:
		ids[v] = true
	}

	return ids
}

// patchToEmails converts a patch Value to a slice of Email.
func patchToEmails(value interface{}) []scim.Email {
	var emails []scim.Email

	switch v := value.(type) {
	case []interface{}:
		for _, item := range v {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			email := scim.Email{Type: attrTypeWork}
			if val, ok := m["value"].(string); ok {
				email.Value = val
			}
			if t, ok := m["type"].(string); ok {
				email.Type = t
			}
			if p, ok := m["primary"].(bool); ok {
				email.Primary = p
			}
			emails = append(emails, email)
		}
	case []scim.Email:
		emails = v
	}

	return emails
}

// patchToPhones converts a patch Value to a slice of PhoneNumber.
func patchToPhones(value interface{}) []scim.PhoneNumber {
	var phones []scim.PhoneNumber

	switch v := value.(type) {
	case []interface{}:
		for _, item := range v {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			phone := scim.PhoneNumber{Type: attrTypeWork}
			if val, ok := m["value"].(string); ok {
				phone.Value = val
			}
			if t, ok := m["type"].(string); ok {
				phone.Type = t
			}
			if p, ok := m["primary"].(bool); ok {
				phone.Primary = p
			}
			phones = append(phones, phone)
		}
	case []scim.PhoneNumber:
		phones = v
	}

	return phones
}

// patchToGroupRefs converts a patch Value to a slice of GroupRef.
func patchToGroupRefs(value interface{}) []scim.GroupRef {
	var groups []scim.GroupRef

	switch v := value.(type) {
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				group := scim.GroupRef{Type: resourceTypeGroup}
				if val, ok := m["value"].(string); ok {
					group.Value = val
				}
				if disp, ok := m["display"].(string); ok {
					group.Display = disp
				}
				groups = append(groups, group)
			}
		}
	case []scim.GroupRef:
		groups = v
	}

	return groups
}

func stringPtrToString(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}
