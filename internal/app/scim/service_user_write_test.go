package scim

import (
	"context"
	"errors"
	"testing"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scimRecordingRepo is an in-memory users.Repository slice: it records the
// aggregate/profile handed to it and lets tests inject failures at each write.
// Unused interface methods are promoted from the embedded nil interface.
type scimRecordingRepo struct {
	users.Repository

	store        map[string]*users.User
	profiles     map[string]users.UserProfile
	createErr    error
	updateErr    error
	profileErr   error
	createCalls  int
	updateCalls  int
	profileCalls int
}

func newSCIMRecordingRepo() *scimRecordingRepo {
	return &scimRecordingRepo{
		store:    map[string]*users.User{},
		profiles: map[string]users.UserProfile{},
	}
}

func (r *scimRecordingRepo) GetByUsername(_ context.Context, organizationID, username string) (*users.User, error) {
	for _, user := range r.store {
		if user.OrganizationID == organizationID && user.Username == username {
			copied := *user
			return &copied, nil
		}
	}
	return nil, apperrors.NewNotFound("user not found")
}

func (r *scimRecordingRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*users.User, error) {
	user, ok := r.store[id]
	if !ok || user.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("user not found")
	}
	copied := *user
	return &copied, nil
}

func (r *scimRecordingRepo) Create(_ context.Context, user *users.User) error {
	r.createCalls++
	copied := *user
	r.store[user.ID] = &copied
	r.profiles[user.Profile.UserID] = user.Profile
	return r.createErr
}

func (r *scimRecordingRepo) UpdateForOrganization(_ context.Context, user *users.User, organizationID string) error {
	r.updateCalls++
	if r.updateErr != nil {
		return r.updateErr
	}
	current, ok := r.store[user.ID]
	if !ok || current.OrganizationID != organizationID {
		return apperrors.NewNotFound("user not found")
	}
	if current.Revision != user.Revision-1 {
		return apperrors.NewConflict("user revision conflict")
	}
	copied := *user
	r.store[user.ID] = &copied
	return nil
}

func (r *scimRecordingRepo) UpdateProfile(_ context.Context, profile *users.UserProfile) error {
	r.profileCalls++
	if r.profileErr != nil {
		return r.profileErr
	}
	r.profiles[profile.UserID] = *profile
	return nil
}

func (r *scimRecordingRepo) snapshot() (map[string]*users.User, map[string]users.UserProfile) {
	userCopy := make(map[string]*users.User, len(r.store))
	for id, user := range r.store {
		copied := *user
		userCopy[id] = &copied
	}
	profileCopy := make(map[string]users.UserProfile, len(r.profiles))
	for id, profile := range r.profiles {
		profileCopy[id] = profile
	}
	return userCopy, profileCopy
}

func (r *scimRecordingRepo) restore(userCopy map[string]*users.User, profileCopy map[string]users.UserProfile) {
	r.store = userCopy
	r.profiles = profileCopy
}

// scimRecordingUOW gives WithinTransaction rollback semantics so the tests can
// assert that a failing profile write leaves no aggregate change behind.
type scimRecordingUOW struct {
	appuow.UnitOfWork
	repo *scimRecordingRepo
}

func (u *scimRecordingUOW) Users() users.Repository { return u.repo }

func (u *scimRecordingUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	usersBefore, profilesBefore := u.repo.snapshot()
	if err := fn(ctx, u); err != nil {
		u.repo.restore(usersBefore, profilesBefore)
		return err
	}
	return nil
}

func newSCIMRecordingService(repo *scimRecordingRepo) Service {
	return NewService(
		scimFixedClock{now: scimTestNow},
		scimFixedUUID{id: scimTestUserID},
		&scimRecordingUOW{repo: repo},
	)
}

func seedSCIMRecordingUser(repo *scimRecordingRepo) *users.User {
	user := scimWriteFixture()
	user.Profile = users.UserProfile{
		UserID:      user.ID,
		DisplayName: "Seed Display",
		Email:       "seed@example.com",
		Phone:       "+10000000000",
	}
	copied := *user
	repo.store[user.ID] = &copied
	repo.profiles[user.ID] = user.Profile
	return user
}

// TestService_CreateUser_BuildsCompleteAggregate pins every field that used to
// be zero: generated id, clock timestamps, Revision 1, and a profile keyed by
// the same id.
func TestService_CreateUser_BuildsCompleteAggregate(t *testing.T) {
	repo := newSCIMRecordingRepo()
	svc := newSCIMRecordingService(repo)

	ret, err := svc.CreateUser(context.Background(), "org-1", ScimUser{
		UserName:     "alice",
		DisplayName:  "Alice",
		Active:       true,
		Name:         &ScimName{GivenName: "Alice", FamilyName: "Smith"},
		Emails:       []ScimEmail{{Value: "alice@example.com"}},
		PhoneNumbers: []ScimPhone{{Value: "+1234567890"}},
	})
	require.NoError(t, err)
	require.NotNil(t, ret)

	created := repo.store[scimTestUserID]
	require.NotNil(t, created)
	assert.Equal(t, scimTestUserID, created.ID)
	assert.Equal(t, scimTestUserID, created.Profile.UserID)
	assert.Equal(t, scimTestNow, created.CreatedAt)
	assert.Equal(t, scimTestNow, created.UpdatedAt)
	assert.Equal(t, int64(1), created.Revision)
	assert.Equal(t, "active", created.Status)
	assert.False(t, created.IsForbidden)
	assert.Equal(t, "alice@example.com", created.Profile.Email)
	assert.Equal(t, "+1234567890", created.Profile.Phone)
	assert.Equal(t, "Alice", created.Profile.FirstName)
	assert.Equal(t, "Smith", created.Profile.LastName)
	assert.NotNil(t, created.Addresses)
	assert.NotNil(t, created.ApplicationScopes)
	assert.Equal(t, scimTestUserID, ret.ID)
}

// TestService_CreateUser_RollsBackPartialWriteOnFailure simulates the repository
// writing rows and then failing at the profile insert; the transaction wrapper
// must discard the partial aggregate.
func TestService_CreateUser_RollsBackPartialWriteOnFailure(t *testing.T) {
	repo := newSCIMRecordingRepo()
	repo.createErr = errors.New("user_profiles insert failed")
	svc := newSCIMRecordingService(repo)

	ret, err := svc.CreateUser(context.Background(), "org-1", ScimUser{UserName: "alice", Active: true})
	require.Error(t, err)
	assert.Nil(t, ret)
	assert.Equal(t, 1, repo.createCalls)
	assert.Equal(t, 0, repo.profileCalls)
	assert.Empty(t, repo.store, "failed create must not leave a half-written user")
	assert.Empty(t, repo.profiles, "failed create must not leave a half-written profile")
}

// TestService_ReplaceUser_PersistsProfileInTransaction pins that PUT writes the
// user_profiles row, not only the users aggregate.
func TestService_ReplaceUser_PersistsProfileInTransaction(t *testing.T) {
	repo := newSCIMRecordingRepo()
	seedSCIMRecordingUser(repo)
	svc := newSCIMRecordingService(repo)

	ret, err := svc.ReplaceUser(context.Background(), "org-1", "user-1", ScimUser{
		UserName:     "alice-renamed",
		DisplayName:  "Alice Renamed",
		Active:       true,
		Name:         &ScimName{GivenName: "Alice", FamilyName: "Chen"},
		Emails:       []ScimEmail{{Value: "alice.renamed@example.com"}},
		PhoneNumbers: []ScimPhone{{Value: "+8613800000000"}},
	})
	require.NoError(t, err)
	require.NotNil(t, ret)

	stored := repo.store["user-1"]
	require.NotNil(t, stored)
	assert.Equal(t, int64(6), stored.Revision)
	assert.Equal(t, "alice-renamed", stored.Username)
	assert.Equal(t, 1, repo.profileCalls)

	profile := repo.profiles["user-1"]
	assert.Equal(t, "user-1", profile.UserID)
	assert.Equal(t, "Alice Renamed", profile.DisplayName)
	assert.Equal(t, "Alice", profile.FirstName)
	assert.Equal(t, "Chen", profile.LastName)
	assert.Equal(t, "alice.renamed@example.com", profile.Email)
	assert.Equal(t, "+8613800000000", profile.Phone)
}

// TestService_PatchUser_PersistsProfileInTransaction covers the PATCH/PUT
// shared profile write for the attributes PATCH actually mutates.
func TestService_PatchUser_PersistsProfileInTransaction(t *testing.T) {
	repo := newSCIMRecordingRepo()
	seeded := seedSCIMRecordingUser(repo)
	svc := newSCIMRecordingService(repo)

	_, err := svc.PatchUser(context.Background(), "org-1", seeded.ID, ScimPatchOp{
		Operations: []ScimOperation{
			{Op: "replace", Path: "displayName", Value: "Patched Name"},
			{Op: "replace", Path: "emails", Value: map[string]any{"value": "patched@example.com"}},
		},
	})
	require.NoError(t, err)

	profile := repo.profiles[seeded.ID]
	assert.Equal(t, "user-1", profile.UserID)
	assert.Equal(t, "Patched Name", profile.DisplayName)
	assert.Equal(t, "patched@example.com", profile.Email)
	assert.Equal(t, 1, repo.profileCalls)
}

// TestService_ReplaceUser_ProfileFailureRollsBackAggregate pins that a failing
// profile write cannot leave the users aggregate updated in its own transaction.
func TestService_ReplaceUser_ProfileFailureRollsBackAggregate(t *testing.T) {
	repo := newSCIMRecordingRepo()
	seeded := seedSCIMRecordingUser(repo)
	repo.profileErr = errors.New("user_profiles insert failed")
	svc := newSCIMRecordingService(repo)

	_, err := svc.ReplaceUser(context.Background(), "org-1", seeded.ID, ScimUser{
		UserName:    "alice-should-not-persist",
		DisplayName: "Should Not Persist",
		Active:      true,
	})
	require.Error(t, err)

	stored := repo.store[seeded.ID]
	require.NotNil(t, stored)
	assert.Equal(t, seeded.Username, stored.Username, "aggregate write must be rolled back")
	assert.Equal(t, seeded.Revision, stored.Revision)
	assert.Equal(t, seeded.Profile.Email, repo.profiles[seeded.ID].Email, "profile must stay unchanged")
}
