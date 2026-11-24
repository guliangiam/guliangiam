package adapters

import (
	"context"
	"testing"
	"time"

	uowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/adapters"
	adaptersmock "github.com/guliangiam/guliangiam/internal/domain/adapters/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// recordingTester captures the merged probe parameters and returns a canned
// outcome, standing in for the infra/adaptertest implementation.
type recordingTester struct {
	connected bool
	failure   string
	calls     int
	gotType   string
	gotHost   string
	gotPort   int
	gotUser   string
	gotPass   string
	gotDB     string
}

func (r *recordingTester) TestDatabaseConnection(_ context.Context, databaseType, host string, port int, username, password, databaseName string) (bool, string) {
	r.calls++
	r.gotType, r.gotHost, r.gotPort = databaseType, host, port
	r.gotUser, r.gotPass, r.gotDB = username, password, databaseName
	return r.connected, r.failure
}

func testConnectionService(t *testing.T, tester ConnectionTester, stored *domain.Adapter) Service {
	t.Helper()
	ctrl := gomock.NewController(t)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := adaptersmock.NewMockRepository(ctrl)
	uow.EXPECT().Adapters().Return(repo).AnyTimes()
	repo.EXPECT().GetByID(gomock.Any(), "a1", "org-1").Return(stored, nil)
	return Service{
		Clock:            fakeClock{fixedTime: time.Now()},
		UUID:             fakeUUIDGenerator{fixedID: "test"},
		UOW:              uow,
		ConnectionTester: tester,
	}
}

// The probe must run through the injected port with the command fields winning
// over the stored adapter configuration.
func TestTestConnection_DelegatesToPortWithCommandOverrides(t *testing.T) {
	tester := &recordingTester{connected: true}
	svc := testConnectionService(t, tester, &domain.Adapter{
		ID: "a1", OrganizationID: "org-1",
		Host: "stored-host", DatabaseType: "mysql", DbUser: "stored-user", DbPassword: "stored-pass", DatabaseName: "stored-db",
	})

	result, err := svc.TestConnection(context.Background(), "org-1", "a1", TestConnectionCommand{
		DatabaseType: "postgresql", Host: "cmd-host", Port: 5433, Username: "cmd-user", Password: "cmd-pass", DatabaseName: "cmd-db",
	})
	require.NoError(t, err)
	assert.True(t, result.Connected)
	assert.Empty(t, result.Error)
	assert.Equal(t, 1, tester.calls)
	assert.Equal(t, "postgresql", tester.gotType)
	assert.Equal(t, "cmd-host", tester.gotHost)
	assert.Equal(t, 5433, tester.gotPort)
	assert.Equal(t, "cmd-user", tester.gotUser)
	assert.Equal(t, "cmd-pass", tester.gotPass)
	assert.Equal(t, "cmd-db", tester.gotDB)
}

// Empty command fields fall back to the stored adapter configuration.
func TestTestConnection_FallsBackToStoredConfig(t *testing.T) {
	tester := &recordingTester{connected: true}
	port := 3307
	svc := testConnectionService(t, tester, &domain.Adapter{
		ID: "a1", OrganizationID: "org-1",
		Host: "stored-host", Port: &port, DatabaseType: "mysql", DbUser: "stored-user", DbPassword: "stored-pass", DatabaseName: "stored-db",
	})

	result, err := svc.TestConnection(context.Background(), "org-1", "a1", TestConnectionCommand{})
	require.NoError(t, err)
	assert.True(t, result.Connected)
	assert.Equal(t, "stored-host", tester.gotHost)
	assert.Equal(t, 3307, tester.gotPort)
	assert.Equal(t, "mysql", tester.gotType)
	assert.Equal(t, "stored-user", tester.gotUser)
	assert.Equal(t, "stored-pass", tester.gotPass)
	assert.Equal(t, "stored-db", tester.gotDB)
}

// A failed probe is a result, not a transport error: the endpoint answers 200
// with connected=false and the failure reason.
func TestTestConnection_FailureIsAResult(t *testing.T) {
	tester := &recordingTester{connected: false, failure: "connection failed: refused"}
	svc := testConnectionService(t, tester, &domain.Adapter{ID: "a1", OrganizationID: "org-1", Host: "h"})

	result, err := svc.TestConnection(context.Background(), "org-1", "a1", TestConnectionCommand{})
	require.NoError(t, err)
	assert.False(t, result.Connected)
	assert.Equal(t, "connection failed: refused", result.Error)
}

// Without the infra port wired the endpoint reports an explicit configuration
// error instead of dialing from the app layer.
func TestTestConnection_NoTesterIsUnimplemented(t *testing.T) {
	svc := testConnectionService(t, nil, &domain.Adapter{ID: "a1", OrganizationID: "org-1", Host: "h"})

	_, err := svc.TestConnection(context.Background(), "org-1", "a1", TestConnectionCommand{})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnimplemented, apperrors.FromError(err).Code)
}
