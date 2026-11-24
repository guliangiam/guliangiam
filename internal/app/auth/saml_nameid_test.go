package auth

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
)

// pagedNameIDUserRepository serves one page at a time and reports the unpaged
// total, like the postgres users repository.
type pagedNameIDUserRepository struct {
	userdomain.Repository
	users []*userdomain.User
}

func (r pagedNameIDUserRepository) List(_ context.Context, query userdomain.ListQuery) ([]*userdomain.User, int64, error) {
	offset := platformpagination.Offset(query.Page, query.PageSize)
	if offset >= len(r.users) {
		return nil, int64(len(r.users)), nil
	}
	end := offset + query.PageSize
	if end > len(r.users) {
		end = len(r.users)
	}
	return r.users[offset:end], int64(len(r.users)), nil
}

type pagedNameIDUoW struct {
	appuow.AuthUoW
	repo userdomain.Repository
}

func (u pagedNameIDUoW) Users() userdomain.Repository { return u.repo }

// NameID lookup must scan the whole tenant: the previous PageSize:1000 call was
// clamped to 100, so a user outside the first page looked like a missing
// account and the SAML assertion was rejected.
func TestFindUserByNameID_ScansBeyondTheFirstPage(t *testing.T) {
	users := make([]*userdomain.User, 2*platformpagination.MaxPageSize+1)
	for i := range users {
		users[i] = &userdomain.User{
			ID:             fmt.Sprintf("user-%03d", i),
			OrganizationID: "org-1",
			Username:       fmt.Sprintf("user-%03d", i),
			Profile:        userdomain.UserProfile{Email: fmt.Sprintf("user-%03d@example.com", i)},
		}
	}
	users[len(users)-1].Profile.Email = "last@example.com"

	got, err := findUserByNameID(context.Background(),
		pagedNameIDUoW{repo: pagedNameIDUserRepository{users: users}}, "org-1", "last@example.com")
	require.NoError(t, err)
	require.Equal(t, "user-200", got.ID)
}
