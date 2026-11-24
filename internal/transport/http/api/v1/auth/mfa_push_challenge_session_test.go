package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	mfachallengedomain "github.com/guliangiam/guliangiam/internal/domain/mfachallenges"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domainproviders "github.com/guliangiam/guliangiam/internal/domain/providers"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	domainusers "github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type pushMfaChallengeProviderRepo struct {
	domainproviders.Repository
	provider *domainproviders.Provider
}

func (r *pushMfaChallengeProviderRepo) GetByID(_ context.Context, id string) (*domainproviders.Provider, error) {
	if r.provider == nil || r.provider.ID != id {
		return nil, apperrors.NewNotFound("provider not found")
	}
	return r.provider, nil
}

func (r *pushMfaChallengeProviderRepo) GetConfig(_ context.Context, providerID string) (*domainproviders.ProviderConfig, error) {
	return &domainproviders.ProviderConfig{ProviderID: providerID, Config: map[string]any{}}, nil
}

type pushMfaChallengeRepo struct {
	items    map[string]*mfachallengedomain.Challenge
	created  *mfachallengedomain.Challenge
	deleted  []string
	getCalls int
}

func (r *pushMfaChallengeRepo) GetByID(_ context.Context, id string) (*mfachallengedomain.Challenge, error) {
	r.getCalls++
	challenge, ok := r.items[id]
	if !ok {
		return nil, apperrors.NewNotFound("login challenge not found")
	}
	return challenge, nil
}

func (r *pushMfaChallengeRepo) Create(_ context.Context, challenge *mfachallengedomain.Challenge) error {
	r.items[challenge.ID] = challenge
	r.created = challenge
	return nil
}

func (r *pushMfaChallengeRepo) Update(_ context.Context, challenge *mfachallengedomain.Challenge) error {
	r.items[challenge.ID] = challenge
	return nil
}

func (r *pushMfaChallengeRepo) Delete(_ context.Context, id string) error {
	if _, ok := r.items[id]; !ok {
		return apperrors.NewNotFound("login challenge not found")
	}
	delete(r.items, id)
	r.deleted = append(r.deleted, id)
	return nil
}

type pushMfaChallengeUOW struct {
	appuow.UnitOfWork
	users      *pushMfaUserRepo
	providers  *pushMfaChallengeProviderRepo
	challenges *pushMfaChallengeRepo
}

func (u *pushMfaChallengeUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func (u *pushMfaChallengeUOW) Users() domainusers.Repository                { return u.users }
func (u *pushMfaChallengeUOW) Providers() domainproviders.Repository        { return u.providers }
func (u *pushMfaChallengeUOW) MfaChallenges() mfachallengedomain.Repository { return u.challenges }
func (u *pushMfaChallengeUOW) Records() domainrecords.Repository            { return &pushMfaRecordRepo{} }
func (u *pushMfaChallengeUOW) Outbox() domainoutbox.Repository              { return &pushMfaOutboxRepo{} }

func newPushMfaChallengeTestHandler(t *testing.T) (Handler, *pushMfaUserRepo, *pushMfaChallengeRepo) {
	users := &pushMfaUserRepo{user: &domainusers.User{
		ID:              pushMfaSessionUser,
		OrganizationID:  pushMfaOrgID,
		Username:        "alice",
		MfaPushReceiver: "receiver-a",
		MfaPushProvider: pushMfaProviderID,
	}}
	challenges := &pushMfaChallengeRepo{items: make(map[string]*mfachallengedomain.Challenge)}
	uow := &pushMfaChallengeUOW{
		users: users,
		providers: &pushMfaChallengeProviderRepo{provider: &domainproviders.Provider{
			ID:           pushMfaProviderID,
			Category:     domainproviders.ProviderCategory("Notification"),
			ProviderType: domainproviders.ProviderType("pushbullet"),
		}},
		challenges: challenges,
	}
	service := &appauth.Service{Clock: clock.SystemClock{}, UUIDGenerator: pushMfaUUID{}, UOW: uow}
	return Handler{Service: service}, users, challenges
}

func stubPushMfaSend(t *testing.T) *[]string {
	sends := &[]string{}
	original := appauth.SendPushAuthenticationForTest()
	t.Cleanup(func() { appauth.SetSendPushAuthenticationForTest(original) })
	appauth.SetSendPushAuthenticationForTest(func(_ context.Context, _ appauth.PushProviderConfigForTest, receiver, _, _ string) error {
		*sends = append(*sends, receiver)
		return nil
	})
	return sends
}

func approvedPushMfaChallenge(id, userID string) *mfachallengedomain.Challenge {
	return &mfachallengedomain.Challenge{
		ID:             id,
		OrganizationID: pushMfaOrgID,
		UserID:         userID,
		State:          mfachallengedomain.StateApproved,
		ExpiresAt:      time.Now().Add(time.Hour),
	}
}

func TestPushMfaInitiateBindsSubjectToSession(t *testing.T) {
	cases := []struct {
		name          string
		bodyUserID    string
		sessionUserID string
		wantStatus    int
		wantCode      string
		wantRequested []string
		wantSends     []string
	}{
		{"owner succeeds", "", pushMfaSessionUser, http.StatusOK, "", []string{pushMfaSessionUser}, []string{"receiver-a"}},
		{"explicit foreign userId rejected", pushMfaForeignUser, pushMfaSessionUser, http.StatusForbidden, "forbidden", nil, []string{}},
		{"missing session rejected", "", "", http.StatusUnauthorized, "unauthorized", nil, []string{}},
		{"missing local user rejected", "", "ghost-user", http.StatusUnauthorized, "unauthorized", []string{"ghost-user"}, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, users, challenges := newPushMfaChallengeTestHandler(t)
			sends := stubPushMfaSend(t)

			body := `{"organizationId":"` + pushMfaOrgID + `","applicationId":"app-a"}`
			if tc.bodyUserID != "" {
				body = `{"userId":"` + tc.bodyUserID + `","organizationId":"` + pushMfaOrgID + `","applicationId":"app-a"}`
			}
			req := pushMfaSessionRequest("/mfa/push/initiate", body, tc.sessionUserID)
			recorder := httptest.NewRecorder()
			pushMfaRouter(handler).ServeHTTP(recorder, req)

			require.Equal(t, tc.wantStatus, recorder.Code)
			assert.Equal(t, tc.wantRequested, users.requested)
			assert.Equal(t, tc.wantSends, *sends)
			if tc.wantCode != "" {
				assert.Contains(t, recorder.Body.String(), `"code":"`+tc.wantCode+`"`)
				assert.Nil(t, challenges.created, "a rejected initiate must not mint a challenge")
				return
			}
			require.NotNil(t, challenges.created)
			assert.Equal(t, pushMfaSessionUser, challenges.created.UserID,
				"the challenge owner must be the session user")
		})
	}
}

func TestPushMfaVerifyOwnerConsumesChallengeOnce(t *testing.T) {
	handler, _, challenges := newPushMfaChallengeTestHandler(t)
	challenges.items["challenge-own"] = approvedPushMfaChallenge("challenge-own", pushMfaSessionUser)

	first := pushMfaSessionRequest("/mfa/push/verify", `{"challengeId":"challenge-own"}`, pushMfaSessionUser)
	firstRecorder := httptest.NewRecorder()
	pushMfaRouter(handler).ServeHTTP(firstRecorder, first)

	require.Equal(t, http.StatusOK, firstRecorder.Code)
	var result appauth.VerifyPushMfaResult
	require.NoError(t, json.NewDecoder(firstRecorder.Body).Decode(&result))
	assert.True(t, result.Approved)
	assert.Equal(t, []string{"challenge-own"}, challenges.deleted)
	assert.NotContains(t, challenges.items, "challenge-own", "approval must consume the challenge")
	second := pushMfaSessionRequest("/mfa/push/verify", `{"challengeId":"challenge-own"}`, pushMfaSessionUser)
	secondRecorder := httptest.NewRecorder()
	pushMfaRouter(handler).ServeHTTP(secondRecorder, second)

	require.Equal(t, http.StatusUnauthorized, secondRecorder.Code)
	assert.Contains(t, secondRecorder.Body.String(), `"code":"unauthorized"`)
	assert.Equal(t, []string{"challenge-own"}, challenges.deleted, "consume must happen exactly once")
}

func TestPushMfaVerifyRejectsForeignChallenge(t *testing.T) {
	handler, _, challenges := newPushMfaChallengeTestHandler(t)
	challenges.items["challenge-victim"] = approvedPushMfaChallenge("challenge-victim", pushMfaForeignUser)

	req := pushMfaSessionRequest("/mfa/push/verify", `{"challengeId":"challenge-victim"}`, pushMfaSessionUser)
	recorder := httptest.NewRecorder()
	pushMfaRouter(handler).ServeHTTP(recorder, req)

	// Foreign and missing challenges share the module's invalid/expired shape
	// instead of a 403 that would confirm the challenge id exists.
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"unauthorized"`)
	assert.Contains(t, recorder.Body.String(), "mfa challenge is invalid or expired")
	assert.Contains(t, challenges.items, "challenge-victim",
		"a foreign session must not consume the victim's challenge")
}

func TestPushMfaVerifyRequiresSession(t *testing.T) {
	handler, _, challenges := newPushMfaChallengeTestHandler(t)

	req := pushMfaSessionRequest("/mfa/push/verify", `{"challengeId":"challenge-own"}`, "")
	recorder := httptest.NewRecorder()
	pushMfaRouter(handler).ServeHTTP(recorder, req)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"unauthorized"`)
	assert.Zero(t, challenges.getCalls, "no session must be rejected before any challenge lookup")
}
