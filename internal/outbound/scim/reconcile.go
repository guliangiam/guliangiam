package scim

import (
	"sort"
	"strings"
)

// UserRecord is the subset of a SCIM User needed for reconciliation. Both the
// local expected state and the downstream listing are normalized into this
// shape so the diff engine stays independent of transport and storage.
type UserRecord struct {
	ID         string            `json:"id"`
	UserName   string            `json:"userName"`
	ExternalID string            `json:"externalId"`
	Active     bool              `json:"active"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// AttributeDrift describes one attribute whose remote value differs from the
// expected value.
type AttributeDrift struct {
	UserName string
	Field    string
	Expected string
	Remote   string
}

// StateDrift describes a user whose active flag differs.
type StateDrift struct {
	UserName string
	Expected bool
	Remote   bool
}

// ReconciliationReport is the PROV-011 drift report: users missing downstream,
// extra downstream users, attribute mismatches and activation-state mismatches.
type ReconciliationReport struct {
	Missing           []string         `json:"missing"`
	Extra             []string         `json:"extra"`
	AttributeMismatch []AttributeDrift `json:"attributeMismatch"`
	StateMismatch     []StateDrift     `json:"stateMismatch"`
}

// HasDrift reports whether any difference was found.
func (r ReconciliationReport) HasDrift() bool {
	return len(r.Missing) > 0 || len(r.Extra) > 0 || len(r.AttributeMismatch) > 0 || len(r.StateMismatch) > 0
}

// ReconcileUsers diffs expected (local truth) against remote (downstream
// listing). Users are keyed by externalId when present, otherwise by
// case-insensitive userName (SCIM userName comparison is case-insensitive).
// Attribute comparison only checks expected keys: downstream may carry extra
// read-only attributes without that being drift.
func ReconcileUsers(expected, remote []UserRecord) ReconciliationReport {
	expectedByKey := indexUsers(expected)
	remoteByKey := indexUsers(remote)

	var report ReconciliationReport
	for key, want := range expectedByKey {
		got, ok := remoteByKey[key]
		if !ok {
			report.Missing = append(report.Missing, key)
			continue
		}
		if want.Active != got.Active {
			report.StateMismatch = append(report.StateMismatch, StateDrift{
				UserName: key, Expected: want.Active, Remote: got.Active,
			})
		}
		for field, expectedValue := range want.Attributes {
			remoteValue, ok := got.Attributes[field]
			if !ok || remoteValue != expectedValue {
				report.AttributeMismatch = append(report.AttributeMismatch, AttributeDrift{
					UserName: key, Field: field, Expected: expectedValue, Remote: remoteValue,
				})
			}
		}
	}
	for key := range remoteByKey {
		if _, ok := expectedByKey[key]; !ok {
			report.Extra = append(report.Extra, key)
		}
	}

	sort.Strings(report.Missing)
	sort.Strings(report.Extra)
	sort.Slice(report.AttributeMismatch, func(i, j int) bool {
		if report.AttributeMismatch[i].UserName != report.AttributeMismatch[j].UserName {
			return report.AttributeMismatch[i].UserName < report.AttributeMismatch[j].UserName
		}
		return report.AttributeMismatch[i].Field < report.AttributeMismatch[j].Field
	})
	sort.Slice(report.StateMismatch, func(i, j int) bool {
		return report.StateMismatch[i].UserName < report.StateMismatch[j].UserName
	})
	return report
}

func indexUsers(users []UserRecord) map[string]UserRecord {
	indexed := make(map[string]UserRecord, len(users))
	for _, user := range users {
		key := strings.TrimSpace(user.ExternalID)
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(user.UserName))
		}
		indexed[key] = user
	}
	return indexed
}

// GroupRecord is the subset of a SCIM Group needed for reconciliation.
type GroupRecord struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"displayName"`
	ExternalID  string   `json:"externalId"`
	Members     []string `json:"members,omitempty"`
}

// GroupMemberDrift describes member-set differences for one group.
type GroupMemberDrift struct {
	Group          string   `json:"group"`
	MissingMembers []string `json:"missingMembers"`
	ExtraMembers   []string `json:"extraMembers"`
}

// GroupReconciliationReport is the group half of the PROV-011 drift report.
type GroupReconciliationReport struct {
	Missing        []string           `json:"missing"`
	Extra          []string           `json:"extra"`
	MemberMismatch []GroupMemberDrift `json:"memberMismatch"`
}

// HasDrift reports whether any group difference was found.
func (r GroupReconciliationReport) HasDrift() bool {
	return len(r.Missing) > 0 || len(r.Extra) > 0 || len(r.MemberMismatch) > 0
}

// ReconcileGroups diffs expected groups against the downstream listing. Groups
// are keyed by externalId when present, otherwise by case-insensitive
// displayName. Member ids are compared as sets and sorted for stable reports.
func ReconcileGroups(expected, remote []GroupRecord) GroupReconciliationReport {
	expectedByKey := indexGroups(expected)
	remoteByKey := indexGroups(remote)

	var report GroupReconciliationReport
	for key, want := range expectedByKey {
		got, ok := remoteByKey[key]
		if !ok {
			report.Missing = append(report.Missing, key)
			continue
		}
		missing, extra := diffStringSets(want.Members, got.Members)
		if len(missing) > 0 || len(extra) > 0 {
			report.MemberMismatch = append(report.MemberMismatch, GroupMemberDrift{
				Group: key, MissingMembers: missing, ExtraMembers: extra,
			})
		}
	}
	for key := range remoteByKey {
		if _, ok := expectedByKey[key]; !ok {
			report.Extra = append(report.Extra, key)
		}
	}

	sort.Strings(report.Missing)
	sort.Strings(report.Extra)
	sort.Slice(report.MemberMismatch, func(i, j int) bool {
		return report.MemberMismatch[i].Group < report.MemberMismatch[j].Group
	})
	return report
}

func indexGroups(groups []GroupRecord) map[string]GroupRecord {
	indexed := make(map[string]GroupRecord, len(groups))
	for _, group := range groups {
		key := strings.TrimSpace(group.ExternalID)
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(group.DisplayName))
		}
		indexed[key] = group
	}
	return indexed
}

func diffStringSets(expected, remote []string) (missing, extra []string) {
	want := make(map[string]struct{}, len(expected))
	for _, id := range expected {
		want[id] = struct{}{}
	}
	got := make(map[string]struct{}, len(remote))
	for _, id := range remote {
		got[id] = struct{}{}
		if _, ok := want[id]; !ok {
			extra = append(extra, id)
		}
	}
	for id := range want {
		if _, ok := got[id]; !ok {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	return missing, extra
}
