package risk

import (
	"context"
	"errors"
	"testing"

	notificationproviderdomain "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	notificationdomain "github.com/guliangiam/guliangiam/internal/domain/notifications"
	domain "github.com/guliangiam/guliangiam/internal/domain/risk"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── fakes for the notification side of the risk gate ────────────────────

type fakeUserRepository struct {
	userdomain.Repository
	users     map[string]*userdomain.User
	admins    []*userdomain.User
	listErr   error
	getErr    error
	lastQuery userdomain.ListQuery
}

func (r *fakeUserRepository) GetByID(_ context.Context, id string) (*userdomain.User, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	user, ok := r.users[id]
	if !ok {
		return nil, errors.New("user not found")
	}
	return user, nil
}

func (r *fakeUserRepository) List(_ context.Context, query userdomain.ListQuery) ([]*userdomain.User, int64, error) {
	r.lastQuery = query
	if r.listErr != nil {
		return nil, 0, r.listErr
	}
	return r.admins, int64(len(r.admins)), nil
}

type fakeNotificationRepository struct {
	notificationdomain.Repository
	created []*notificationdomain.Notification
	err     error
}

func (r *fakeNotificationRepository) Create(_ context.Context, notification *notificationdomain.Notification) error {
	if r.err != nil {
		return r.err
	}
	r.created = append(r.created, notification)
	return nil
}

type fakeNotificationProviderRepository struct {
	notificationproviderdomain.Repository
	providers []*notificationproviderdomain.NotificationProvider
	err       error
	lastQuery notificationproviderdomain.ListQuery
}

func (r *fakeNotificationProviderRepository) List(_ context.Context, query notificationproviderdomain.ListQuery) ([]*notificationproviderdomain.NotificationProvider, int64, error) {
	r.lastQuery = query
	if r.err != nil {
		return nil, 0, r.err
	}
	return r.providers, int64(len(r.providers)), nil
}

// ── fixture ────────────────────────────────────────────────────────────

type alertFixture struct {
	*serviceFixture
	users     *fakeUserRepository
	notifs    *fakeNotificationRepository
	providers *fakeNotificationProviderRepository
}

func newAlertFixture() *alertFixture {
	base := newServiceFixture()
	users := &fakeUserRepository{users: map[string]*userdomain.User{}}
	notifs := &fakeNotificationRepository{}
	providers := &fakeNotificationProviderRepository{}
	base.uow.users = users
	base.uow.notifications = notifs
	base.uow.notificationProviders = providers
	return &alertFixture{serviceFixture: base, users: users, notifs: notifs, providers: providers}
}

func (f *alertFixture) addUser(id, username, email string, isAdmin bool) *userdomain.User {
	user := &userdomain.User{
		ID:             id,
		OrganizationID: testOrgID,
		Username:       username,
		IsAdmin:        isAdmin,
		Profile:        userdomain.UserProfile{UserID: id, Email: email},
	}
	f.users.users[id] = user
	return user
}

func criticalSignals() map[string]any {
	return map[string]any{"is_tor": true, "is_vpn": true}
}

// ── tests ──────────────────────────────────────────────────────────────

func TestEvaluateAndRecordLoginRisk_DenyNotifiesUserAndAdmins(t *testing.T) {
	f := newAlertFixture()
	user := f.addUser(testActorID, "alice", "alice@example.com", false)
	f.users.admins = []*userdomain.User{
		f.addUser("77777777-7777-4777-8777-777777777777", "admin1", "admin1@example.com", true),
		// duplicate of the affected user's address must not create a second row
		f.addUser("88888888-8888-4888-8888-888888888888", "admin2", "ALICE@example.com", true),
		f.addUser("99999999-9999-4999-8999-999999999999", "admin3", "", true), // no inbox → skipped
	}
	f.providers.providers = []*notificationproviderdomain.NotificationProvider{
		{ID: "provider-1", OrganizationID: testOrgID, IsEnabled: true, ProviderType: "email"},
	}

	result, err := f.service.EvaluateAndRecordLoginRisk(context.Background(), f.uow, loginRiskCommand(user.ID))
	require.NoError(t, err)
	assert.Equal(t, "deny", result.Decision)

	// Two distinct recipients: alice@example.com (user, also an admin dup) and
	// admin1@example.com. admin3 has no email so it is dropped.
	require.Len(t, f.notifs.created, 2)
	seen := map[string]bool{}
	for _, n := range f.notifs.created {
		seen[n.Recipient] = true
		assert.Equal(t, "pending", n.Status)
		assert.Equal(t, "provider-1", n.ProviderID)
		assert.Equal(t, testOrgID, n.OrganizationID)
		assert.Contains(t, n.Subject, "Security Alert")
		assert.Contains(t, n.Content, result.DecisionID)
		assert.Equal(t, "risk.critical_login_denied", n.Variables["category"])
	}
	assert.True(t, seen["alice@example.com"])
	assert.True(t, seen["admin1@example.com"])

	// Administrator enumeration must use the is_admin filter, not a full scan.
	require.NotNil(t, f.users.lastQuery.IsAdmin)
	assert.True(t, *f.users.lastQuery.IsAdmin)

	// Every provider lookup must be scoped to the tenant and to enabled rows.
	assert.Equal(t, testOrgID, f.providers.lastQuery.OrganizationID)
	require.NotNil(t, f.providers.lastQuery.IsEnabled)
	assert.True(t, *f.providers.lastQuery.IsEnabled)
}

func TestEvaluateAndRecordLoginRisk_DenyWithoutProviderStillDenies(t *testing.T) {
	f := newAlertFixture()
	user := f.addUser(testActorID, "bob", "bob@example.com", true)
	f.providers.providers = nil

	result, err := f.service.EvaluateAndRecordLoginRisk(context.Background(), f.uow, loginRiskCommand(user.ID))
	require.NoError(t, err, "a missing provider must not turn a deny into an error")
	assert.Equal(t, "deny", result.Decision)
	assert.Empty(t, f.notifs.created)
}

func TestEvaluateAndRecordLoginRisk_ProviderLookupFailureIsSwallowed(t *testing.T) {
	f := newAlertFixture()
	user := f.addUser(testActorID, "carol", "carol@example.com", true)
	f.providers.err = errors.New("db unavailable")

	result, err := f.service.EvaluateAndRecordLoginRisk(context.Background(), f.uow, loginRiskCommand(user.ID))
	require.NoError(t, err)
	assert.Equal(t, "deny", result.Decision)
	assert.Empty(t, f.notifs.created)
}

func TestEvaluateAndRecordLoginRisk_AllowDoesNotNotify(t *testing.T) {
	f := newAlertFixture()
	user := f.addUser(testActorID, "dave", "dave@example.com", false)
	f.providers.providers = []*notificationproviderdomain.NotificationProvider{
		{ID: "provider-1", OrganizationID: testOrgID, IsEnabled: true, ProviderType: "email"},
	}

	cmd := loginRiskCommand(user.ID)
	cmd.Signals = map[string]any{} // no risk signals → score 0 → allow

	result, err := f.service.EvaluateAndRecordLoginRisk(context.Background(), f.uow, cmd)
	require.NoError(t, err)
	assert.Equal(t, "allow", result.Decision)
	assert.Empty(t, f.notifs.created, "only critical (deny) decisions page a human")
}

func TestEvaluateAndRecordLoginRisk_ChallengeDoesNotNotify(t *testing.T) {
	f := newAlertFixture()
	user := f.addUser(testActorID, "erin", "erin@example.com", false)
	f.providers.providers = []*notificationproviderdomain.NotificationProvider{
		{ID: "provider-1", OrganizationID: testOrgID, IsEnabled: true, ProviderType: "email"},
	}

	cmd := loginRiskCommand(user.ID)
	cmd.Signals = map[string]any{"is_tor": true} // score 40 → challenge

	result, err := f.service.EvaluateAndRecordLoginRisk(context.Background(), f.uow, cmd)
	require.NoError(t, err)
	assert.Equal(t, "challenge", result.Decision)
	assert.Empty(t, f.notifs.created)
}

func TestEvaluateAndRecordLoginRisk_RejectsInvalidOrganization(t *testing.T) {
	f := newAlertFixture()
	cmd := loginRiskCommand("")
	cmd.OrganizationID = "not-a-uuid"

	_, err := f.service.EvaluateAndRecordLoginRisk(context.Background(), f.uow, cmd)
	require.Error(t, err)
	assert.Empty(t, f.notifs.created)
}

// The schema declares CHECK (score BETWEEN 0 AND 100) on risk_decisions. Five
// independent signals add up to 130, so the engine must clamp before the value
// is persisted; an unclamped sum would fail the insert and break the login.
func TestBuiltinRuleEngine_ClampsScoreToSchemaRange(t *testing.T) {
	result := builtinRuleEngine{}.Evaluate(context.Background(), domain.EvaluateRiskCommand{
		OrganizationID: testOrgID,
		LoginChannel:   domain.ChannelPassword,
		Signals: map[string]any{
			"is_vpn":                 true, // +30
			"is_tor":                 true, // +40
			"recent_failures_1h":     5,    // +25
			"device_fingerprint_new": true, // +15
			"user_home_country":      "CN",
			"geo_country":            "RU", // +20 → 130
		},
	})
	assert.Equal(t, 100, result.Score)
	assert.Equal(t, domain.DecisionDeny, result.Decision)
}

// loginRiskCommand builds a password-channel command for the fixture.
func loginRiskCommand(userID string) domain.LoginRiskCommand {
	var uid *string
	if userID != "" {
		uid = &userID
	}
	ip := "203.0.113.9"
	return domain.LoginRiskCommand{
		OrganizationID: testOrgID,
		UserID:         uid,
		LoginChannel:   "password",
		IP:             &ip,
		UserAgent:      "risk-alert-test",
		Signals:        criticalSignals(),
	}
}
