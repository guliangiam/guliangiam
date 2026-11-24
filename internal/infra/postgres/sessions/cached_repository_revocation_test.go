package sessions

import (
	"context"
	"testing"
	"time"

	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	sessionmock "github.com/guliangiam/guliangiam/internal/domain/sessions/mock"
	cacheinfra "github.com/guliangiam/guliangiam/internal/infra/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// Two CachedRepository instances sharing one SessionCache model two app
// instances behind a shared Redis. Revoking through one instance must remove
// the shared cache entry so the other instance cannot keep serving a revoked
// session from cache (ADR-013 cross-instance revocation).
func TestCachedRepository_RevokeInvalidatesSharedCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := sessionmock.NewMockRepository(ctrl)
	sharedCache := cacheinfra.NewMemorySessionCache(time.Minute)
	instanceA := NewCachedRepository(repo, sharedCache)
	instanceB := NewCachedRepository(repo, sharedCache)

	now := time.Now().UTC()
	active := &sessiondomain.Session{
		ID:           "sess-1",
		UserID:       "user-1",
		SessionState: sessiondomain.StateActive,
		ExpiresAt:    now.Add(time.Hour),
		LastSeenAt:   now,
		CreatedAt:    now,
	}

	repo.EXPECT().GetByID(gomock.Any(), "sess-1").Return(active, nil).Times(1)
	got, err := instanceA.GetByID(context.Background(), "sess-1")
	require.NoError(t, err)
	require.NotNil(t, got)

	// Instance B revokes: DB write + shared-cache delete.
	repo.EXPECT().Revoke(gomock.Any(), "sess-1", gomock.Any()).Return(nil).Times(1)
	require.NoError(t, instanceB.Revoke(context.Background(), "sess-1", now))

	// Instance A must miss the cache and read the revoked row from DB.
	revoked := *active
	revoked.RevokedAt = &now
	repo.EXPECT().GetByID(gomock.Any(), "sess-1").Return(&revoked, nil).Times(1)
	got, err = instanceA.GetByID(context.Background(), "sess-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotNil(t, got.RevokedAt, "revoked session must not be served from a stale cache")
}
