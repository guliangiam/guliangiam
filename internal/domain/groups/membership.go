package groups

// GroupMembership joins a user to a group (guliangiam.user_group_memberships).
// This is a pure join table with no additional columns — the full row identity
// is the (user_id, group_id) composite key.
type GroupMembership struct {
	UserID  string
	GroupID string
}
