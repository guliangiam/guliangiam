package providers

import (
	"context"
	"errors"
	"strconv"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	_ domain.MessagingProviderLookup    = Repository{}
	_ domain.MessagingApplicationLookup = Repository{}
)

// FindActiveMessagingProviderByChannel resolves the organization's provider
// row for a delivery channel. RLS is enforced in the database; the explicit
// organization_id predicate is the application-side guard for non-RLS dialects.
//
// Disabled rows are filtered out by the query, so "configured but disabled"
// reads as "not configured" (ErrMessagingProviderUnconfigured) and stays
// eligible for the deployment fallback.
func (r Repository) FindActiveMessagingProviderByChannel(ctx context.Context, organizationID string, channel domain.MessagingChannel) (*domain.MessagingProviderConfig, error) {
	kinds, err := messagingKindsForChannel(channel)
	if err != nil {
		return nil, err
	}
	return r.scanMessagingProvider(channel, r.db.QueryRow(ctx, queryFindMessagingProviderByChannel, organizationID, kinds))
}

// FindActiveMessagingProviderByApplication resolves the provider bound to an
// application. No binding (or a binding to a disabled/deleted provider) reads
// as ErrMessagingProviderUnconfigured so the caller falls back to the
// organization default. A binding whose provider belongs to another tenant is
// excluded by the organization predicate.
func (r Repository) FindActiveMessagingProviderByApplication(ctx context.Context, organizationID, applicationID string, channel domain.MessagingChannel) (*domain.MessagingProviderConfig, error) {
	// The identifier is caller-supplied (login/signup/forgot links carry it in
	// the URL), so a slug or display name must not reach the uuid cast in the
	// query: that would surface as a 503 and break verification-code delivery
	// for every page. Treat a non-UUID as "no application binding" and let the
	// organization default apply.
	if applicationID == "" || !platformuuid.IsValid(applicationID) {
		return nil, domain.ErrMessagingProviderUnconfigured
	}
	kinds, err := messagingKindsForChannel(channel)
	if err != nil {
		return nil, err
	}
	return r.scanMessagingProvider(channel, r.db.QueryRow(ctx, queryFindMessagingProviderByApplication, applicationID, organizationID, kinds))
}

func messagingKindsForChannel(channel domain.MessagingChannel) ([]string, error) {
	kindValues := domain.MessagingKindsForChannel(channel)
	if len(kindValues) == 0 {
		return nil, apperrors.NewInvalidArgumentf("unknown messaging channel: %s", channel)
	}
	kinds := make([]string, len(kindValues))
	for i, kind := range kindValues {
		kinds[i] = string(kind)
	}
	return kinds, nil
}

func (r Repository) scanMessagingProvider(channel domain.MessagingChannel, row pgx.Row) (*domain.MessagingProviderConfig, error) {
	var (
		id, orgID, providerKind  string
		enabled                  bool
		host, admin              string
		port                     int
		password                 string
		appID, appSecret, region string
		template, customAPI      string
		customHeader, customBody string
		contentType, sender      string
	)

	if err := row.Scan(
		&id, &orgID, &providerKind, &enabled,
		&host, &port, &admin, &password,
		&appID, &appSecret, &region,
		&template, &customAPI, &customHeader,
		&customBody, &contentType, &sender,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrMessagingProviderUnconfigured
		}
		return nil, apperrors.NewInternalf("find messaging provider for channel %s: %v", channel, err)
	}
	// Defensive guard only: the query already excludes disabled rows, so a
	// disabled scan result indicates the predicate was bypassed.
	if !enabled {
		return nil, apperrors.NewUnavailablef("messaging provider for channel %s is disabled", channel)
	}

	password, err := openProviderSecret(id, providerColumnPassword, password)
	if err != nil {
		return nil, err
	}
	appSecret, err = openProviderSecret(id, providerColumnAppSecret, appSecret)
	if err != nil {
		return nil, err
	}

	return &domain.MessagingProviderConfig{
		ProviderID:     id,
		OrganizationID: orgID,
		Kind:           domain.Kind(providerKind),
		Channel:        channel,
		Values: map[string]string{
			"host":         host,
			"port":         strconv.Itoa(port),
			"admin":        admin,
			"password":     password,
			"appId":        appID,
			"appSecret":    appSecret,
			"region":       region,
			"template":     template,
			"customApi":    customAPI,
			"customHeader": customHeader,
			"customBody":   customBody,
			"contentType":  contentType,
			"sender":       sender,
		},
	}, nil
}
