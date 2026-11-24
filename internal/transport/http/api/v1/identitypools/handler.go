package identitypools

import (
	"context"

	appw "github.com/guliangiam/guliangiam/internal/app/workload"
)

type Handler struct{ Service appw.Service }

type UpdateTrustStatusRequest struct {
	Status string `json:"status"`
}

type UpdateAccountStatusRequest struct {
	Status   string `json:"status"`
	Revision int64  `json:"revision"`
}

type RotateAccountSecretRequest struct {
	Revision int64 `json:"revision"`
}

type DeleteAccountRequest struct {
	Revision int64 `json:"revision"`
}

type MatchSubjectRequest struct {
	Patterns []string `json:"patterns"`
	Samples  []string `json:"samples"`
}

func (h Handler) ListPools(ctx context.Context, q appw.ListPoolsQuery) ([]appw.PoolDTO, int64, error) {
	return h.Service.ListPools(ctx, q)
}
func (h Handler) CreatePool(ctx context.Context, orgID string, cmd appw.CreatePoolCommand) (*appw.PoolDTO, error) {
	cmd.OrganizationID = orgID
	return h.Service.CreatePool(ctx, cmd)
}
func (h Handler) GetPool(ctx context.Context, orgID, id string) (*appw.PoolDTO, error) {
	return h.Service.GetPool(ctx, orgID, id)
}
func (h Handler) ListTrusts(ctx context.Context, q appw.ListTrustsQuery) ([]appw.TrustDTO, int64, error) {
	return h.Service.ListTrusts(ctx, q)
}
func (h Handler) CreateTrust(ctx context.Context, orgID string, cmd appw.CreateTrustCommand) (*appw.TrustDTO, error) {
	cmd.OrganizationID = orgID
	return h.Service.CreateTrust(ctx, cmd)
}
func (h Handler) GetTrust(ctx context.Context, orgID, poolID, id string) (*appw.TrustDTO, error) {
	return h.Service.GetTrust(ctx, orgID, poolID, id)
}
func (h Handler) SetTrustStatus(ctx context.Context, orgID, poolID, id string, request UpdateTrustStatusRequest) (*appw.TrustDTO, error) {
	return h.Service.SetTrustStatus(ctx, appw.SetTrustStatusCommand{OrganizationID: orgID, PoolID: poolID, TrustID: id, Status: appw.FederationTrustStatus(request.Status)})
}
func (h Handler) DeleteTrust(ctx context.Context, orgID, poolID, id string) error {
	return h.Service.DeleteTrust(ctx, appw.DeleteTrustCommand{OrganizationID: orgID, PoolID: poolID, TrustID: id})
}
func (h Handler) PreviewSubjectMatches(request MatchSubjectRequest) ([]appw.SubjectMatchDTO, error) {
	return h.Service.PreviewSubjectMatches(request.Patterns, request.Samples)
}
func (h Handler) ListAccounts(ctx context.Context, q appw.ListAccountsQuery) ([]appw.AccountDTO, int64, error) {
	return h.Service.ListAccounts(ctx, q)
}
func (h Handler) CreateAccount(ctx context.Context, orgID string, cmd appw.CreateAccountCommand) (*appw.AccountDTO, error) {
	cmd.OrganizationID = orgID
	return h.Service.CreateAccount(ctx, cmd)
}
func (h Handler) GetAccount(ctx context.Context, orgID, poolID, id string) (*appw.AccountDTO, error) {
	return h.Service.GetAccount(ctx, orgID, poolID, id)
}
func (h Handler) SetAccountStatus(ctx context.Context, orgID, poolID, id string, request UpdateAccountStatusRequest) (*appw.AccountDTO, error) {
	return h.Service.SetAccountStatus(ctx, appw.SetAccountStatusCommand{OrganizationID: orgID, PoolID: poolID, AccountID: id, Status: appw.ServiceAccountStatus(request.Status), Revision: request.Revision})
}
func (h Handler) RotateAccountSecret(ctx context.Context, orgID, poolID, id string, request RotateAccountSecretRequest) (*appw.AccountDTO, error) {
	return h.Service.RotateAccountSecret(ctx, appw.RotateAccountSecretCommand{OrganizationID: orgID, PoolID: poolID, AccountID: id, Revision: request.Revision})
}
func (h Handler) GetAccountDeletePrecheck(ctx context.Context, orgID, poolID, id string) (*appw.AccountDeletePrecheckDTO, error) {
	return h.Service.GetAccountDeletePrecheck(ctx, orgID, poolID, id)
}
func (h Handler) DeleteAccount(ctx context.Context, orgID, poolID, id string, request DeleteAccountRequest) error {
	return h.Service.DeleteAccount(ctx, appw.DeleteAccountCommand{OrganizationID: orgID, PoolID: poolID, AccountID: id, Revision: request.Revision})
}
