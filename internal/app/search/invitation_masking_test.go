package search

import (
	"context"
	"strings"
	"testing"

	domaininvitations "github.com/guliangiam/guliangiam/internal/domain/invitations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSearch_InvitationCodeNeverLeaks pins BE-C-20: global search is reachable
// by any searchable member, so the invitation credential (Code) must never
// appear in the response — only the lifecycle status belongs in Subtitle.
func TestSearch_InvitationCodeNeverLeaks(t *testing.T) {
	service, f := newFixture(t)
	const secretCode = "inv-secret-code-123"
	f.invitationsList = func(query domaininvitations.ListQuery) ([]*domaininvitations.Invitation, int64, error) {
		return []*domaininvitations.Invitation{{
			ID: "inv-1", OrganizationID: "org-1", Email: "bob@acme.example",
			Code: secretCode, Status: "pending",
		}}, 1, nil
	}

	result, err := service.Search(context.Background(), Query{Keyword: "bob", OrganizationID: "org-1"})
	require.NoError(t, err)

	var found bool
	for _, group := range result.Groups {
		if group.Entity != "invitations" {
			continue
		}
		for _, item := range group.Items {
			found = true
			assert.Equal(t, "pending", item.Subtitle, "Subtitle must carry the status, not the credential")
			assert.False(t, strings.Contains(item.Subtitle, secretCode), "invitation code leaked in Subtitle")
			assert.False(t, strings.Contains(item.Title, secretCode), "invitation code leaked in Title")
			assert.False(t, strings.Contains(item.Name, secretCode), "invitation code leaked in Name")
		}
	}
	require.True(t, found, "expected an invitations group item")
}
