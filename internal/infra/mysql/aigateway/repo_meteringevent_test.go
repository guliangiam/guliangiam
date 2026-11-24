package aigateway

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
)

func TestMeteringEventRepo_Save_UsesIdempotentInsert(t *testing.T) {
	require.Contains(t, strings.ToUpper(queryMeteringEventSave), "INSERT IGNORE",
		"audit sink must tolerate outbox retries without duplicate-key failure")

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryMeteringEventSave)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	repo := NewMeteringEventRepo(db)
	require.NoError(t, repo.Save(context.Background(), &domain.MeteringEvent{ID: "evt-1", OutboxID: "evt-1"}))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMeteringEventRepo_Save_ErrorPropagates(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	wantErr := errors.New("db error")
	mock.ExpectExec(regexp.QuoteMeta(queryMeteringEventSave)).WillReturnError(wantErr)

	repo := NewMeteringEventRepo(db)
	require.ErrorIs(t, repo.Save(context.Background(), &domain.MeteringEvent{ID: "evt-1", OutboxID: "evt-1"}), wantErr)
}

func TestProvidersRepository_CreateAnchorsProviderID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryProviderInsert)).
		WithArgs(
			"provider-1", "org-1", "provider-1", "", "", "", "[]", "", "", "", "", "",
			0, 0, 0, 0, nil, nil, time.Time{}, time.Time{}, nil, int64(0),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	repo := NewProvidersRepository(db)
	provider := &domain.LlmProvider{ID: "provider-1", OrganizationID: "org-1", SupportedProtocols: []string{}}
	require.NoError(t, repo.Create(context.Background(), provider))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMeteringEventRepo_Save_EmptyIDRejected(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewMeteringEventRepo(db)
	require.Error(t, repo.Save(context.Background(), &domain.MeteringEvent{}))
	require.NoError(t, mock.ExpectationsWereMet())
}
