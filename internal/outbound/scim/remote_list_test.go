package scim

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPagedSCIMServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start, _ := strconv.Atoi(r.URL.Query().Get("startIndex"))
		if start < 1 {
			start = 1
		}
		w.Header().Set("Content-Type", "application/scim+json")
		switch r.URL.Path {
		case "/Users":
			users := []string{
				`{"id":"u-1","userName":"alice","externalId":"e-1","active":true,"title":"Engineer","emails":[{"value":"alice@example.com","primary":true}]}`,
				`{"id":"u-2","userName":"bob","externalId":"e-2","active":false,"emails":[{"value":"bob@example.com"}]}`,
			}
			page := users[start-1:]
			if len(page) > 1 {
				page = page[:1]
			}
			fmt.Fprintf(w, `{"totalResults":%d,"startIndex":%d,"itemsPerPage":1,"Resources":[%s]}`, len(users), start, joinStrings(page))
		case "/Groups":
			fmt.Fprint(w, `{"totalResults":1,"startIndex":1,"itemsPerPage":1,"Resources":[{"id":"g-1","displayName":"engineering","externalId":"g-ext-1","members":[{"value":"u-1"}]}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
}

func joinStrings(values []string) string {
	out := ""
	for i, value := range values {
		if i > 0 {
			out += ","
		}
		out += value
	}
	return out
}

func TestListRemoteUsers_Paginates(t *testing.T) {
	server := newPagedSCIMServer(t)
	defer server.Close()
	client := NewClient(server.URL, "token", 5000, 0)

	users, err := ListRemoteUsers(context.Background(), client, 1)
	require.NoError(t, err)
	require.Len(t, users, 2)
	assert.Equal(t, "e-1", users[0].ExternalID)
	assert.Equal(t, "alice@example.com", users[0].Attributes["email"])
	assert.Equal(t, "Engineer", users[0].Attributes["title"])
	assert.False(t, users[1].Active)
}

func TestListRemoteGroups(t *testing.T) {
	server := newPagedSCIMServer(t)
	defer server.Close()
	client := NewClient(server.URL, "token", 5000, 0)

	groups, err := ListRemoteGroups(context.Background(), client, 100)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Equal(t, []string{"u-1"}, groups[0].Members)
}

func TestFetchAndReconcile(t *testing.T) {
	server := newPagedSCIMServer(t)
	defer server.Close()
	client := NewClient(server.URL, "token", 5000, 0)

	userReport, groupReport, err := FetchAndReconcile(context.Background(), client,
		[]UserRecord{{ExternalID: "e-1", UserName: "alice", Active: true, Attributes: map[string]string{"email": "alice@example.com", "title": "Engineer"}}},
		[]GroupRecord{{ExternalID: "g-ext-1", DisplayName: "engineering", Members: []string{"u-1"}}},
		100,
	)
	require.NoError(t, err)
	assert.True(t, userReport.HasDrift(), "e-2 exists remotely but not locally")
	assert.Equal(t, []string{"e-2"}, userReport.Extra)
	assert.False(t, groupReport.HasDrift())
}
