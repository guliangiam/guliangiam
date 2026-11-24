package workload

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"time"

	"github.com/google/uuid"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	domain "github.com/guliangiam/guliangiam/internal/domain/workload"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

const (
	serviceAccountIDPrefix       = "gl_cid_"
	serviceAccountSecretPrefix   = "gl_cs_"
	serviceAccountIDEntropy      = 24
	serviceAccountSecretEntropy  = 32
	defaultAccessTokenTTLSeconds = 900
	minimumAccessTokenTTLSeconds = 60
	maximumAccessTokenTTLSeconds = 3600
)

type UnitOfWork = appuow.WorkloadUoW
type (
	Pool                       = domain.Pool
	FederationTrust            = domain.FederationTrust
	FederationTrustStatus      = domain.FederationTrustStatus
	ServiceAccount             = domain.ServiceAccount
	ServiceAccountStatus       = domain.ServiceAccountStatus
	ListPoolsQuery             = domain.ListPoolsQuery
	ListTrustsQuery            = domain.ListTrustsQuery
	ListAccountsQuery          = domain.ListAccountsQuery
	CreatePoolCommand          = domain.CreatePoolCommand
	CreateTrustCommand         = domain.CreateTrustCommand
	CreateAccountCommand       = domain.CreateAccountCommand
	SetTrustStatusCommand      = domain.SetTrustStatusCommand
	DeleteTrustCommand         = domain.DeleteTrustCommand
	SetAccountStatusCommand    = domain.SetAccountStatusCommand
	RotateAccountSecretCommand = domain.RotateAccountSecretCommand
	DeleteAccountCommand       = domain.DeleteAccountCommand
	Repository                 = domain.Repository
)

const (
	TrustStatusActive   = domain.TrustStatusActive
	TrustStatusDisabled = domain.TrustStatusDisabled
	TrustStatusDeleted  = domain.TrustStatusDeleted
	SAStatusActive      = domain.SAStatusActive
	SAStatusDisabled    = domain.SAStatusDisabled
	SAStatusDeleted     = domain.SAStatusDeleted
)

// status 闭集的单一源：校验分支与 422 的 `allowed` 元数据共用。
var trustStatuses = []string{string(TrustStatusActive), string(TrustStatusDisabled)}

var serviceAccountStatuses = []string{string(SAStatusActive), string(SAStatusDisabled)}

// 请求字段校验器由 domain/workload 重导出。transport 层只允许依赖 app 层
// （scripts/check-layers.sh Rule 2），直接 import domain 会形成越层依赖。
var (
	ValidateHTTPSURI        = domain.ValidateHTTPSURI
	ValidateSubjectPatterns = domain.ValidateSubjectPatterns
)

type Service struct {
	UOW   UnitOfWork
	Clock clock.Clock
	UUID  platformuuid.Generator
}

func NewService(uow UnitOfWork) Service {
	return Service{UOW: uow, Clock: clock.SystemClock{}, UUID: platformuuid.V7Generator{}}
}

type PoolDTO struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organizationId"`
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	Description    string `json:"description"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
	Revision       int64  `json:"revision"`
}

type TrustDTO struct {
	ID             string   `json:"id"`
	OrganizationID string   `json:"organizationId"`
	PoolID         string   `json:"poolId"`
	Name           string   `json:"name"`
	Issuer         string   `json:"issuer"`
	JWKSURI        string   `json:"jwksUri"`
	Audience       []string `json:"audience"`
	Subjects       []string `json:"subjects"`
	Status         string   `json:"status"`
	CreatedAt      string   `json:"createdAt"`
	UpdatedAt      string   `json:"updatedAt"`
	Revision       int64    `json:"revision"`
}

type AccountDTO struct {
	ID                    string   `json:"id"`
	OrganizationID        string   `json:"organizationId"`
	PoolID                string   `json:"poolId"`
	Name                  string   `json:"name"`
	Description           string   `json:"description"`
	ClientID              string   `json:"clientId"`
	ClientSecretPlain     string   `json:"clientSecretPlain,omitempty"`
	Scopes                []string `json:"scopes"`
	AccessTokenTTLSeconds int      `json:"accessTokenTtlSeconds"`
	LastRotatedAt         *string  `json:"lastRotatedAt"`
	Status                string   `json:"status"`
	SecretSet             bool     `json:"secretSet"`
	CreatedAt             string   `json:"createdAt"`
	UpdatedAt             string   `json:"updatedAt"`
	Revision              int64    `json:"revision"`
}

type SubjectMatchDTO struct {
	Subject string `json:"subject"`
	Allowed bool   `json:"allowed"`
}

type AccountDeletePrecheckDTO struct {
	ID               string   `json:"id"`
	OrganizationID   string   `json:"organizationId"`
	PoolID           string   `json:"poolId"`
	ActiveTokenCount int64    `json:"activeTokenCount"`
	Scopes           []string `json:"scopes"`
	Status           string   `json:"status"`
	CanDelete        bool     `json:"canDelete"`
	Revision         int64    `json:"revision"`
}

func fmtTime(t time.Time) string { return t.Format("2006-01-02T15:04:05.000Z07:00") }

func fmtOptionalTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	formatted := fmtTime(*t)
	return &formatted
}

func toPool(p *Pool) PoolDTO {
	return PoolDTO{ID: p.ID, OrganizationID: p.OrganizationID, Name: p.Name, DisplayName: p.DisplayName, Description: p.Description, CreatedAt: fmtTime(p.CreatedAt), UpdatedAt: fmtTime(p.UpdatedAt), Revision: p.Revision}
}

func toTrust(t *FederationTrust) TrustDTO {
	return TrustDTO{ID: t.ID, OrganizationID: t.OrganizationID, PoolID: t.PoolID, Name: t.Name, Issuer: t.Issuer, JWKSURI: t.JWKSURI, Audience: nonNilStrings(t.Audience), Subjects: nonNilStrings(t.Subjects), Status: string(t.Status), CreatedAt: fmtTime(t.CreatedAt), UpdatedAt: fmtTime(t.UpdatedAt), Revision: t.Revision}
}

func toAccount(a *ServiceAccount) AccountDTO {
	return AccountDTO{ID: a.ID, OrganizationID: a.OrganizationID, PoolID: a.PoolID, Name: a.Name, Description: a.Description, ClientID: a.ClientID, Scopes: nonNilStrings(a.Scopes), AccessTokenTTLSeconds: a.AccessTokenTTLSeconds, LastRotatedAt: fmtOptionalTime(a.LastRotatedAt), Status: string(a.Status), SecretSet: a.ClientSecretHash != "", CreatedAt: fmtTime(a.CreatedAt), UpdatedAt: fmtTime(a.UpdatedAt), Revision: a.Revision}
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func (s *Service) now() time.Time {
	if s.Clock == nil {
		return time.Now().UTC()
	}
	return s.Clock.Now().UTC()
}

func (s *Service) newID() (string, error) {
	if s.UUID != nil {
		return s.UUID.NewString()
	}
	id, err := uuid.NewV7()
	return id.String(), err
}

func (s *Service) PreviewSubjectMatches(patterns, samples []string) ([]SubjectMatchDTO, error) {
	if err := domain.ValidateSubjectPatterns(patterns); err != nil {
		return nil, apperrors.NewInvalidArgument(err.Error())
	}
	matches := make([]SubjectMatchDTO, len(samples))
	for i, sample := range samples {
		matches[i] = SubjectMatchDTO{Subject: sample, Allowed: domain.MatchSubject(patterns, sample)}
	}
	return matches, nil
}

func (s *Service) ListPools(ctx context.Context, q ListPoolsQuery) ([]PoolDTO, int64, error) {
	if strings.TrimSpace(q.OrganizationID) == "" {
		return nil, 0, apperrors.NewInvalidArgument("organization_id is required")
	}
	var items []*Pool
	var total int64
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		var err error
		items, total, err = tx.Workload().ListPools(ctx, q)
		return err
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]PoolDTO, len(items))
	for i, p := range items {
		out[i] = toPool(p)
	}
	return out, total, nil
}

func (s *Service) CreatePool(ctx context.Context, cmd CreatePoolCommand) (*PoolDTO, error) {
	if strings.TrimSpace(cmd.OrganizationID) == "" {
		return nil, apperrors.NewInvalidArgument("organization_id is required")
	}
	if strings.TrimSpace(cmd.Name) == "" {
		return nil, apperrors.NewInvalidArgument("name is required")
	}
	id, err := s.newID()
	if err != nil {
		return nil, apperrors.WrapInternal(err, "generate identity pool id")
	}
	now := s.now()
	p := &Pool{ID: id, OrganizationID: cmd.OrganizationID, Name: cmd.Name, DisplayName: cmd.DisplayName, Description: cmd.Description, CreatedAt: now, UpdatedAt: now, Revision: 1}
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Workload().CreatePool(ctx, p); err != nil {
			return apperrors.WrapInternalf(err, "create pool %s", cmd.Name)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	d := toPool(p)
	return &d, nil
}

func (s *Service) GetPool(ctx context.Context, orgID, id string) (*PoolDTO, error) {
	if strings.TrimSpace(orgID) == "" || strings.TrimSpace(id) == "" {
		return nil, apperrors.NewInvalidArgument("organization_id and pool id are required")
	}
	var p *Pool
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		var err error
		p, err = tx.Workload().GetPool(ctx, id, orgID)
		return err
	})
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, apperrors.NewNotFound("identity pool not found")
	}
	d := toPool(p)
	return &d, nil
}

func (s *Service) ListTrusts(ctx context.Context, q ListTrustsQuery) ([]TrustDTO, int64, error) {
	if strings.TrimSpace(q.OrganizationID) == "" {
		return nil, 0, apperrors.NewInvalidArgument("organization_id is required")
	}
	var items []*FederationTrust
	var total int64
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		var err error
		items, total, err = tx.Workload().ListTrusts(ctx, q)
		return err
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]TrustDTO, len(items))
	for i, t := range items {
		out[i] = toTrust(t)
	}
	return out, total, nil
}

func (s *Service) CreateTrust(ctx context.Context, cmd CreateTrustCommand) (*TrustDTO, error) {
	if err := validateOrgPoolResource(cmd.OrganizationID, cmd.PoolID, cmd.Name); err != nil {
		return nil, err
	}
	if err := domain.ValidateHTTPSURI(cmd.Issuer); err != nil {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "issuer must be a valid HTTPS URI", map[string]any{"field": "issuer", "value": cmd.Issuer, "format": "https_uri"})
	}
	if err := domain.ValidateHTTPSURI(cmd.JWKSURI); err != nil {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "jwks_uri must be a valid HTTPS URI", map[string]any{"field": "jwks_uri", "value": cmd.JWKSURI, "format": "https_uri"})
	}
	if err := validateNonEmptyValues("audience", cmd.Audience); err != nil {
		return nil, err
	}
	if err := domain.ValidateSubjectPatterns(cmd.Subjects); err != nil {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, err.Error(), map[string]any{"field": "subjects"})
	}
	id, err := s.newID()
	if err != nil {
		return nil, apperrors.WrapInternal(err, "generate federation trust id")
	}
	now := s.now()
	t := &FederationTrust{ID: id, OrganizationID: cmd.OrganizationID, PoolID: cmd.PoolID, Name: cmd.Name, Issuer: cmd.Issuer, JWKSURI: cmd.JWKSURI, Audience: append([]string{}, cmd.Audience...), Subjects: append([]string{}, cmd.Subjects...), Status: domain.TrustStatusActive, CreatedAt: now, UpdatedAt: now, Revision: 1}
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := requirePool(ctx, tx, cmd.OrganizationID, cmd.PoolID); err != nil {
			return err
		}
		if err := tx.Workload().CreateTrust(ctx, t); err != nil {
			return apperrors.WrapInternalf(err, "create federation trust %s", cmd.Name)
		}
		return s.embedAuditEvent(tx, ctx, t.OrganizationID, "federation_trust.created", "federation_trust", t.ID, map[string]any{
			fieldPoolID: t.PoolID, "issuer": t.Issuer, "audience": t.Audience, "subjects": t.Subjects,
		})
	})
	if err != nil {
		return nil, err
	}
	d := toTrust(t)
	return &d, nil
}

func (s *Service) GetTrust(ctx context.Context, orgID, poolID, id string) (*TrustDTO, error) {
	if err := validateOrgPoolResource(orgID, poolID, id); err != nil {
		return nil, err
	}
	var t *FederationTrust
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		var err error
		t, err = getScopedTrust(ctx, tx, orgID, poolID, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	d := toTrust(t)
	return &d, nil
}

func (s *Service) SetTrustStatus(ctx context.Context, cmd SetTrustStatusCommand) (*TrustDTO, error) {
	if err := validateOrgPoolResource(cmd.OrganizationID, cmd.PoolID, cmd.TrustID); err != nil {
		return nil, err
	}
	if cmd.Status != domain.TrustStatusActive && cmd.Status != domain.TrustStatusDisabled {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "status must be active or disabled", map[string]any{"field": "status", "value": string(cmd.Status), "allowed": append([]string{}, trustStatuses...)})
	}
	var updated *TrustDTO
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		t, err := getScopedTrust(ctx, tx, cmd.OrganizationID, cmd.PoolID, cmd.TrustID)
		if err != nil {
			return err
		}
		if t.Status == domain.TrustStatusDeleted {
			return apperrors.NewConflict("deleted federation trust cannot change status")
		}
		if err := tx.Workload().SetTrustStatus(ctx, t.ID, t.OrganizationID, cmd.Status); err != nil {
			return err
		}
		t.Status = cmd.Status
		t.UpdatedAt = s.now()
		d := toTrust(t)
		updated = &d
		return s.embedAuditEvent(tx, ctx, t.OrganizationID, "federation_trust.status_changed", "federation_trust", t.ID, map[string]any{fieldPoolID: t.PoolID, "status": cmd.Status})
	})
	return updated, err
}

func (s *Service) DeleteTrust(ctx context.Context, cmd DeleteTrustCommand) error {
	if err := validateOrgPoolResource(cmd.OrganizationID, cmd.PoolID, cmd.TrustID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		t, err := getScopedTrust(ctx, tx, cmd.OrganizationID, cmd.PoolID, cmd.TrustID)
		if err != nil {
			return err
		}
		if t.Status == domain.TrustStatusDeleted {
			return apperrors.NewConflict("federation trust is already deleted")
		}
		if err := tx.Workload().DeleteTrust(ctx, t.ID, t.OrganizationID); err != nil {
			return err
		}
		return s.embedAuditEvent(tx, ctx, t.OrganizationID, "federation_trust.deleted", "federation_trust", t.ID, map[string]any{fieldPoolID: t.PoolID})
	})
}

func (s *Service) ListAccounts(ctx context.Context, q ListAccountsQuery) ([]AccountDTO, int64, error) {
	if strings.TrimSpace(q.OrganizationID) == "" || strings.TrimSpace(q.PoolID) == "" {
		return nil, 0, apperrors.NewInvalidArgument("organization_id and pool_id are required")
	}
	var items []*ServiceAccount
	var total int64
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		var err error
		items, total, err = tx.Workload().ListAccounts(ctx, q)
		return err
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]AccountDTO, len(items))
	for i, a := range items {
		out[i] = toAccount(a)
	}
	return out, total, nil
}

func (s *Service) CreateAccount(ctx context.Context, cmd CreateAccountCommand) (*AccountDTO, error) {
	if err := validateOrgPoolResource(cmd.OrganizationID, cmd.PoolID, cmd.Name); err != nil {
		return nil, err
	}
	ttl, err := validateAccessTokenTTL(cmd.AccessTokenTTLSeconds)
	if err != nil {
		return nil, err
	}
	if err := validateNonEmptyValuesAllowEmpty("scope", cmd.Scopes); err != nil {
		return nil, err
	}
	id, err := s.newID()
	if err != nil {
		return nil, apperrors.WrapInternal(err, "generate service account id")
	}
	clientID, err := generateServiceAccountClientID()
	if err != nil {
		return nil, err
	}
	clientSecret, err := serviceAccountSecret(cmd.ClientSecretPlain)
	if err != nil {
		return nil, err
	}
	secretHash, err := platformcrypto.HashClientSecret(clientSecret)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "hash service account client_secret")
	}
	now := s.now()
	a := &ServiceAccount{ID: id, OrganizationID: cmd.OrganizationID, PoolID: cmd.PoolID, Name: cmd.Name, Description: cmd.Description, ClientID: clientID, ClientSecretHash: secretHash, Scopes: append([]string{}, cmd.Scopes...), AccessTokenTTLSeconds: ttl, Status: domain.SAStatusActive, CreatedAt: now, UpdatedAt: now, Revision: 1}
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := requirePool(ctx, tx, cmd.OrganizationID, cmd.PoolID); err != nil {
			return err
		}
		if err := tx.Workload().CreateAccount(ctx, a); err != nil {
			return apperrors.WrapInternalf(err, "create service account %s", cmd.Name)
		}
		return s.embedAuditEvent(tx, ctx, a.OrganizationID, "service_account.created", "service_account", a.ID, map[string]any{
			fieldPoolID: a.PoolID, "clientId": a.ClientID, "scopes": a.Scopes, "accessTokenTtlSeconds": a.AccessTokenTTLSeconds,
		})
	})
	if err != nil {
		return nil, err
	}
	d := toAccount(a)
	d.ClientSecretPlain = clientSecret
	return &d, nil
}

func (s *Service) GetAccount(ctx context.Context, orgID, poolID, id string) (*AccountDTO, error) {
	if err := validateOrgPoolResource(orgID, poolID, id); err != nil {
		return nil, err
	}
	var a *ServiceAccount
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		var err error
		a, err = getScopedAccount(ctx, tx, orgID, poolID, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	d := toAccount(a)
	return &d, nil
}

func (s *Service) SetAccountStatus(ctx context.Context, cmd SetAccountStatusCommand) (*AccountDTO, error) {
	if err := validateAccountMutation(cmd.OrganizationID, cmd.PoolID, cmd.AccountID, cmd.Revision); err != nil {
		return nil, err
	}
	if cmd.Status != domain.SAStatusActive && cmd.Status != domain.SAStatusDisabled {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "status must be active or disabled", map[string]any{"field": "status", "value": string(cmd.Status), "allowed": append([]string{}, serviceAccountStatuses...)})
	}
	var updated *AccountDTO
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		a, err := getScopedAccount(ctx, tx, cmd.OrganizationID, cmd.PoolID, cmd.AccountID)
		if err != nil {
			return err
		}
		if err := checkRevision(a.Revision, cmd.Revision); err != nil {
			return err
		}
		if a.Status == domain.SAStatusDeleted {
			return apperrors.NewConflict("deleted service account cannot change status")
		}
		if err := tx.Workload().SetAccountStatus(ctx, a.ID, a.OrganizationID, cmd.Status, cmd.Revision); err != nil {
			return err
		}
		if cmd.Status == domain.SAStatusDisabled {
			if err := revokeAccountTokens(ctx, tx, a.ID, s.now()); err != nil {
				return err
			}
		}
		a.Status = cmd.Status
		a.Revision++
		a.UpdatedAt = s.now()
		d := toAccount(a)
		updated = &d
		return s.embedAuditEvent(tx, ctx, a.OrganizationID, "service_account.status_changed", "service_account", a.ID, map[string]any{fieldPoolID: a.PoolID, "status": cmd.Status, fieldRevision: a.Revision})
	})
	return updated, err
}

func (s *Service) RotateAccountSecret(ctx context.Context, cmd RotateAccountSecretCommand) (*AccountDTO, error) {
	if err := validateAccountMutation(cmd.OrganizationID, cmd.PoolID, cmd.AccountID, cmd.Revision); err != nil {
		return nil, err
	}
	clientSecret, err := serviceAccountSecret(cmd.ClientSecretPlain)
	if err != nil {
		return nil, err
	}
	secretHash, err := platformcrypto.HashClientSecret(clientSecret)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "hash service account client_secret")
	}
	var rotated *AccountDTO
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		a, err := getScopedAccount(ctx, tx, cmd.OrganizationID, cmd.PoolID, cmd.AccountID)
		if err != nil {
			return err
		}
		if err := checkRevision(a.Revision, cmd.Revision); err != nil {
			return err
		}
		if a.Status == domain.SAStatusDeleted {
			return apperrors.NewConflict("deleted service account cannot rotate secret")
		}
		now := s.now()
		if err := tx.Workload().RotateAccountSecret(ctx, a.ID, a.OrganizationID, secretHash, now, cmd.Revision); err != nil {
			return err
		}
		if err := revokeAccountTokens(ctx, tx, a.ID, now); err != nil {
			return err
		}
		a.ClientSecretHash = secretHash
		a.LastRotatedAt = &now
		a.Revision++
		a.UpdatedAt = now
		d := toAccount(a)
		d.ClientSecretPlain = clientSecret
		rotated = &d
		return s.embedAuditEvent(tx, ctx, a.OrganizationID, "service_account.secret_rotated", "service_account", a.ID, map[string]any{fieldPoolID: a.PoolID, "clientId": a.ClientID, "lastRotatedAt": fmtTime(now), fieldRevision: a.Revision})
	})
	return rotated, err
}

func (s *Service) GetAccountDeletePrecheck(ctx context.Context, orgID, poolID, id string) (*AccountDeletePrecheckDTO, error) {
	if err := validateOrgPoolResource(orgID, poolID, id); err != nil {
		return nil, err
	}
	var precheck *AccountDeletePrecheckDTO
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		a, err := getScopedAccount(ctx, tx, orgID, poolID, id)
		if err != nil {
			return err
		}
		precheck, err = buildDeletePrecheck(ctx, tx, a)
		return err
	})
	return precheck, err
}

func (s *Service) DeleteAccount(ctx context.Context, cmd DeleteAccountCommand) error {
	if err := validateAccountMutation(cmd.OrganizationID, cmd.PoolID, cmd.AccountID, cmd.Revision); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		a, err := getScopedAccount(ctx, tx, cmd.OrganizationID, cmd.PoolID, cmd.AccountID)
		if err != nil {
			return err
		}
		if err := checkRevision(a.Revision, cmd.Revision); err != nil {
			return err
		}
		if a.Status == domain.SAStatusDeleted {
			return apperrors.NewConflict("service account is already deleted")
		}
		precheck, err := buildDeletePrecheck(ctx, tx, a)
		if err != nil {
			return err
		}
		if !precheck.CanDelete {
			return apperrors.NewConflict("service account has active tokens; disable it before deletion")
		}
		if err := tx.Workload().SetAccountStatus(ctx, a.ID, a.OrganizationID, domain.SAStatusDisabled, cmd.Revision); err != nil {
			return err
		}
		now := s.now()
		if err := revokeAccountTokens(ctx, tx, a.ID, now); err != nil {
			return err
		}
		if err := tx.Workload().DeleteAccount(ctx, a.ID, a.OrganizationID, cmd.Revision+1); err != nil {
			return err
		}
		if err := revokeAccountTokens(ctx, tx, a.ID, now); err != nil {
			return err
		}
		return s.embedAuditEvent(tx, ctx, a.OrganizationID, "service_account.deleted", "service_account", a.ID, map[string]any{fieldPoolID: a.PoolID, "scopes": a.Scopes, fieldRevision: cmd.Revision + 2})
	})
}

func validateOrgPoolResource(orgID, poolID, resource string) error {
	if strings.TrimSpace(orgID) == "" {
		return apperrors.NewInvalidArgument("organization_id is required")
	}
	if strings.TrimSpace(poolID) == "" {
		return apperrors.NewInvalidArgument("pool_id is required")
	}
	if strings.TrimSpace(resource) == "" {
		return apperrors.NewInvalidArgument("resource id or name is required")
	}
	return nil
}

func validateAccountMutation(orgID, poolID, id string, revision int64) error {
	if err := validateOrgPoolResource(orgID, poolID, id); err != nil {
		return err
	}
	if revision <= 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "revision must be greater than zero", map[string]any{"field": "revision", "value": revision, "min": 1})
	}
	return nil
}

func validateAccessTokenTTL(ttl int) (int, error) {
	if ttl == 0 {
		return defaultAccessTokenTTLSeconds, nil
	}
	if ttl < minimumAccessTokenTTLSeconds || ttl > maximumAccessTokenTTLSeconds {
		return 0, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "access_token_ttl_seconds must be between 60 and 3600", map[string]any{"field": "access_token_ttl_seconds", "value": ttl, "min": minimumAccessTokenTTLSeconds, "max": maximumAccessTokenTTLSeconds})
	}
	return ttl, nil
}

func validateNonEmptyValues(field string, values []string) error {
	if len(values) == 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, field+" must not be empty", map[string]any{"field": field, "min": 1})
	}
	return validateNonEmptyValuesAllowEmpty(field, values)
}

func validateNonEmptyValuesAllowEmpty(field string, values []string) error {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, field+" values must not be empty", map[string]any{"field": field})
		}
	}
	return nil
}

func checkRevision(current, expected int64) error {
	if current != expected {
		return apperrors.NewConflict("service account revision conflict")
	}
	return nil
}

func requirePool(ctx context.Context, tx appuow.UnitOfWork, orgID, poolID string) error {
	pool, err := tx.Workload().GetPool(ctx, poolID, orgID)
	if err != nil {
		return err
	}
	if pool == nil {
		return apperrors.NewNotFound("identity pool not found in organization")
	}
	return nil
}

func getScopedTrust(ctx context.Context, tx appuow.UnitOfWork, orgID, poolID, id string) (*FederationTrust, error) {
	t, err := tx.Workload().GetTrust(ctx, id, orgID)
	if err != nil {
		return nil, err
	}
	if t == nil || t.PoolID != poolID {
		return nil, apperrors.NewNotFound("federation trust not found in pool")
	}
	return t, nil
}

func getScopedAccount(ctx context.Context, tx appuow.UnitOfWork, orgID, poolID, id string) (*ServiceAccount, error) {
	a, err := tx.Workload().GetAccount(ctx, id, orgID)
	if err != nil {
		return nil, err
	}
	if a == nil || a.PoolID != poolID {
		return nil, apperrors.NewNotFound("service account not found in pool")
	}
	return a, nil
}

func buildDeletePrecheck(ctx context.Context, tx appuow.UnitOfWork, a *ServiceAccount) (*AccountDeletePrecheckDTO, error) {
	activeOnly := true
	_, activeTokens, err := tx.Tokens().List(ctx, tokendomain.ListQuery{Page: 1, PageSize: 1, UserID: a.ID, ActiveOnly: &activeOnly})
	if err != nil {
		return nil, err
	}
	return &AccountDeletePrecheckDTO{ID: a.ID, OrganizationID: a.OrganizationID, PoolID: a.PoolID, ActiveTokenCount: activeTokens, Scopes: nonNilStrings(a.Scopes), Status: string(a.Status), CanDelete: activeTokens == 0 && a.Status != domain.SAStatusDeleted, Revision: a.Revision}, nil
}

func revokeAccountTokens(ctx context.Context, tx appuow.UnitOfWork, accountID string, revokedAt time.Time) error {
	_, err := tx.Tokens().RevokeByQuery(ctx, tokendomain.RevokeQuery{UserID: accountID}, revokedAt)
	return err
}

func generateServiceAccountClientID() (string, error) {
	idBytes := make([]byte, serviceAccountIDEntropy)
	if _, err := rand.Read(idBytes); err != nil {
		return "", apperrors.WrapInternal(err, "generate service account client_id")
	}
	return serviceAccountIDPrefix + base64.RawURLEncoding.EncodeToString(idBytes), nil
}

func serviceAccountSecret(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	secretBytes := make([]byte, serviceAccountSecretEntropy)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", apperrors.WrapInternal(err, "generate service account client_secret")
	}
	return serviceAccountSecretPrefix + base64.RawURLEncoding.EncodeToString(secretBytes), nil
}
