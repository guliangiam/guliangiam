package providers

import (
	"context"
	"errors"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindActiveMessagingProviderByChannel_MapsNativeFields(t *testing.T) {
	sealedPassword := encryptTestSecret(t, "token")

	db := &shared.MockDBTX{QueryRowFn: func(_ context.Context, query string, args ...any) pgx.Row {
		assert.Equal(t, queryFindMessagingProviderByChannel, query)
		assert.Equal(t, "org-1", args[0])
		assert.Equal(t, []string{"smtp", "sendgrid", "resend", "azure_acs", "custom_http", "submail_email", "mailtrap_email", "directmail", "tencent_ses", "sendcloud", "mailgun", "amazon_ses", "postmark", "mailersend", "brevo", "zoho_mail"}, args[1])
		return shared.NewMockRow([]any{
			"provider-1", "org-1", "smtp", true,
			"smtp.example.com", 587, "admin@example.com", sealedPassword,
			"", "", "",
			"", "", "",
			"", "", "from@example.com",
		}, nil)
	}}
	repo := NewRepository(db)

	config, err := repo.FindActiveMessagingProviderByChannel(context.Background(), "org-1", domain.MessagingChannelEmail)
	require.NoError(t, err)
	assert.Equal(t, "provider-1", config.ProviderID)
	assert.Equal(t, "org-1", config.OrganizationID)
	assert.Equal(t, domain.Kind("smtp"), config.Kind)
	assert.Equal(t, domain.MessagingChannelEmail, config.Channel)
	assert.Equal(t, "587", config.Values["port"])
	assert.Equal(t, "admin@example.com", config.Values["admin"])
	assert.Equal(t, "token", config.Values["password"])
	assert.Equal(t, "from@example.com", config.Values["sender"])
}

func TestFindActiveMessagingProviderByChannel_RejectsPlaintextSecrets(t *testing.T) {
	db := &shared.MockDBTX{QueryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return shared.NewMockRow([]any{
			"provider-1", "org-1", "smtp", true,
			"smtp.example.com", 587, "admin", "plaintext-token",
			"", "", "",
			"", "", "",
			"", "", "from@example.com",
		}, nil)
	}}
	repo := NewRepository(db)

	_, err := repo.FindActiveMessagingProviderByChannel(context.Background(), "org-1", domain.MessagingChannelEmail)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not envelope-encrypted")
}

func TestFindActiveMessagingProviderByChannel_NoRowsIsUnconfigured(t *testing.T) {
	db := &shared.MockDBTX{QueryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return shared.NewMockRow(nil, pgx.ErrNoRows)
	}}
	repo := NewRepository(db)

	config, err := repo.FindActiveMessagingProviderByChannel(context.Background(), "org-1", domain.MessagingChannelSMS)
	require.Error(t, err)
	assert.Nil(t, config)
	assert.True(t, errors.Is(err, domain.ErrMessagingProviderUnconfigured))
}

func TestFindActiveMessagingProviderByChannel_DisabledRowIsUnavailable(t *testing.T) {
	db := &shared.MockDBTX{QueryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return shared.NewMockRow([]any{
			"provider-1", "org-1", "smtp", false,
			"smtp.example.com", 587, "admin", "",
			"", "", "",
			"", "", "",
			"", "", "",
		}, nil)
	}}
	repo := NewRepository(db)

	_, err := repo.FindActiveMessagingProviderByChannel(context.Background(), "org-1", domain.MessagingChannelEmail)
	appErr, ok := err.(*apperrors.Error)
	require.True(t, ok)
	assert.Equal(t, 503, appErr.HTTPStatus())
}

func TestFindActiveMessagingProviderByChannel_RejectsUnknownChannel(t *testing.T) {
	repo := NewRepository(&shared.MockDBTX{})

	_, err := repo.FindActiveMessagingProviderByChannel(context.Background(), "org-1", domain.MessagingChannel("pigeon"))
	appErr, ok := err.(*apperrors.Error)
	require.True(t, ok)
	assert.Equal(t, 400, appErr.HTTPStatus())
}

func TestFindActiveMessagingProviderByApplication_NonUUIDFallsBackToOrganization(t *testing.T) {
	// Login/signup/forgot-password links historically carried the application
	// slug. Casting that to uuid in SQL returns an error and breaks every
	// verification-code send for the page, so it must be treated as "no
	// application binding" before the query runs.
	db := &shared.MockDBTX{QueryRowFn: func(context.Context, string, ...any) pgx.Row {
		t.Fatal("non-UUID application id must not reach the SQL query")
		return nil
	}}
	repo := NewRepository(db)

	_, err := repo.FindActiveMessagingProviderByApplication(context.Background(), "org-1", "app-built-in", domain.MessagingChannelSMS)
	require.ErrorIs(t, err, domain.ErrMessagingProviderUnconfigured)
}

func TestFindActiveMessagingProviderByApplication_UUIDReadsBinding(t *testing.T) {
	const applicationID = "11111111-1111-4111-8111-111111111111"
	db := &shared.MockDBTX{QueryRowFn: func(_ context.Context, query string, args ...any) pgx.Row {
		assert.Equal(t, queryFindMessagingProviderByApplication, query)
		assert.Equal(t, applicationID, args[0])
		assert.Equal(t, "org-1", args[1])
		return shared.NewMockRow(nil, pgx.ErrNoRows)
	}}
	repo := NewRepository(db)

	_, err := repo.FindActiveMessagingProviderByApplication(context.Background(), "org-1", applicationID, domain.MessagingChannelSMS)
	require.ErrorIs(t, err, domain.ErrMessagingProviderUnconfigured)
}
