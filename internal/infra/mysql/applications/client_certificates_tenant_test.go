package applications

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
)

var clientCertificateRowColumns = []string{
	"id", "organization_id", "application_id", "auth_method", "certificate_pem",
	"fingerprint_sha256", "subject_dn", "issuer_dn", "not_before", "not_after",
	"status", "retired_at", "created_at", "updated_at", "revision",
}

// TestRepository_ListActiveClientCertificates_TenantPredicate pins the MySQL
// tenant predicate: this dialect has no RLS, so organization_id must be part of
// the statement or a cross-tenant read would leak rows.
func TestRepository_ListActiveClientCertificates_TenantPredicate(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("FROM `app_client_certificates`")).
		WithArgs("org-B", "app-1").
		WillReturnRows(sqlmock.NewRows(clientCertificateRowColumns))

	certificates, err := NewRepository(db).ListActiveClientCertificates(context.Background(), "org-B", "app-1")

	require.NoError(t, err)
	assert.Empty(t, certificates, "跨租户读取必须为空集")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestRepository_ReplaceClientCertificates_TenantPredicate pins both statements
// of the replace path and the tenant argument order.
func TestRepository_ReplaceClientCertificates_TenantPredicate(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	replacedAt := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `app_client_certificates`")).
		WithArgs(replacedAt, replacedAt, "org-A", "app-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `app_client_certificates`")).
		WithArgs("cert-1", "org-A", "app-1", "tls_client_auth", "-----BEGIN CERTIFICATE-----",
			"fp-1", "", "", nil, nil, replacedAt, replacedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	certificate := &domain.ClientCertificate{
		ID:                "cert-1",
		AuthMethod:        "tls_client_auth",
		CertificatePEM:    "-----BEGIN CERTIFICATE-----",
		FingerprintSHA256: "fp-1",
	}
	err := NewRepository(db).ReplaceClientCertificates(context.Background(), "org-A", "app-1",
		[]*domain.ClientCertificate{certificate}, replacedAt)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, "org-A", certificate.OrganizationID)
	assert.Equal(t, "app-1", certificate.ApplicationID)
	assert.Equal(t, domain.ClientCertificateStatusActive, certificate.Status)
}
