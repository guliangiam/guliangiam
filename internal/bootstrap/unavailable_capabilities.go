// unavailable_capabilities.go — MySQL 部署形态下「能力明确不支持」的显式实现。
//
// 背景（OPEN-API-36）：dashboard / ldapsync 的 PostgreSQL 仓储直接把 *pgxpool.Pool
// 注入实现；MySQL 分支的 pool 为 nil，构造能通过但首个请求会在 nil 连接上 panic，
// 属于运行期崩溃而不是受控降级。这里在组合根按 driver 选择「能力不可用」实现：
// 服务照常装配（启动不失败、健康探针为绿），调用方得到 503 + unavailable 语义，
// 而不是 nil-pointer panic。
//
// SCIM 不在此列：它走 appscim.NewService(unitOfWork)，MySQL 走 mysqluow，
// Users()/Groups() 两个仓储都具备，因此 MySQL 下可用（见 wire_transport.go）。
package bootstrap

import (
	"context"

	domaindashboard "github.com/guliangiam/guliangiam/internal/domain/dashboard"
	"github.com/guliangiam/guliangiam/internal/domain/ldaps"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	dashboardUnsupported = "dashboard statistics are not available on this database driver (MySQL); " +
		"use PostgreSQL or run dashboard queries from a PostgreSQL deployment"
	ldapSyncUnsupported = "LDAP sync is not available on this database driver (MySQL); " +
		"use PostgreSQL for LDAP directory synchronization"
)

func errUnavailable(msg string) error { return apperrors.NewUnavailable(msg) }

var (
	_ domaindashboard.StatsRepository = unavailableDashboardRepository{}
	_ ldaps.SyncRepository            = unavailableLDAPSyncRepository{}
)

// ─── Dashboard ───────────────────────────────────────────────────────────────

type unavailableDashboardRepository struct{}

func (unavailableDashboardRepository) CountUsers(context.Context, string) (*domaindashboard.UserCountResult, error) {
	return nil, errUnavailable(dashboardUnsupported)
}

func (unavailableDashboardRepository) CountOrganizations(context.Context) (int64, error) {
	return 0, errUnavailable(dashboardUnsupported)
}

func (unavailableDashboardRepository) CountApplications(context.Context, string) (int64, error) {
	return 0, errUnavailable(dashboardUnsupported)
}

func (unavailableDashboardRepository) CountRoles(context.Context, string) (int64, error) {
	return 0, errUnavailable(dashboardUnsupported)
}

func (unavailableDashboardRepository) CountGroups(context.Context, string) (int64, error) {
	return 0, errUnavailable(dashboardUnsupported)
}

func (unavailableDashboardRepository) CountPermissions(context.Context, string) (int64, error) {
	return 0, errUnavailable(dashboardUnsupported)
}

func (unavailableDashboardRepository) CountProviders(context.Context, string) (int64, error) {
	return 0, errUnavailable(dashboardUnsupported)
}

func (unavailableDashboardRepository) CountActiveSessions(context.Context, string) (*domaindashboard.SessionCountResult, error) {
	return nil, errUnavailable(dashboardUnsupported)
}

func (unavailableDashboardRepository) CountTokens(context.Context, string) (*domaindashboard.TokenCountResult, error) {
	return nil, errUnavailable(dashboardUnsupported)
}

func (unavailableDashboardRepository) CountFailedLogins24h(context.Context, string) (int64, error) {
	return 0, errUnavailable(dashboardUnsupported)
}

func (unavailableDashboardRepository) GetLoginHeatmap(context.Context, string) ([]domaindashboard.HeatmapPoint, error) {
	return nil, errUnavailable(dashboardUnsupported)
}

func (unavailableDashboardRepository) GetMfaCoverageStats(context.Context, string) (*domaindashboard.MfaCoverageResult, error) {
	return nil, errUnavailable(dashboardUnsupported)
}

func (unavailableDashboardRepository) GetProviderDistribution(context.Context, string) ([]domaindashboard.ProviderDistPoint, error) {
	return nil, errUnavailable(dashboardUnsupported)
}

func (unavailableDashboardRepository) GetTrends(context.Context, string) ([]domaindashboard.TrendPoint, error) {
	return nil, errUnavailable(dashboardUnsupported)
}

func (unavailableDashboardRepository) GetActivityHeatmap(context.Context, string) ([]domaindashboard.ActivityPoint, error) {
	return nil, errUnavailable(dashboardUnsupported)
}

func (unavailableDashboardRepository) GetOrgUserDistribution(context.Context) ([]domaindashboard.OrgUserDistPoint, error) {
	return nil, errUnavailable(dashboardUnsupported)
}

// ─── LDAP Sync ───────────────────────────────────────────────────────────────

type unavailableLDAPSyncRepository struct{}

func (unavailableLDAPSyncRepository) GetSyncConfigByID(context.Context, string) (*ldaps.LdapSyncConfig, error) {
	return nil, errUnavailable(ldapSyncUnsupported)
}

func (unavailableLDAPSyncRepository) GetSyncConfigByIDForOrganization(context.Context, string, string) (*ldaps.LdapSyncConfig, error) {
	return nil, errUnavailable(ldapSyncUnsupported)
}

func (unavailableLDAPSyncRepository) ListSyncConfigs(context.Context, string) ([]*ldaps.LdapSyncConfig, error) {
	return nil, errUnavailable(ldapSyncUnsupported)
}

func (unavailableLDAPSyncRepository) CreateSyncConfig(context.Context, *ldaps.LdapSyncConfig) error {
	return errUnavailable(ldapSyncUnsupported)
}

func (unavailableLDAPSyncRepository) UpdateSyncConfig(context.Context, *ldaps.LdapSyncConfig) error {
	return errUnavailable(ldapSyncUnsupported)
}

func (unavailableLDAPSyncRepository) DeleteSyncConfig(context.Context, string, string) error {
	return errUnavailable(ldapSyncUnsupported)
}

func (unavailableLDAPSyncRepository) CreateSyncTask(context.Context, *ldaps.LdapSyncTask) error {
	return errUnavailable(ldapSyncUnsupported)
}

func (unavailableLDAPSyncRepository) UpdateSyncTask(context.Context, *ldaps.LdapSyncTask) error {
	return errUnavailable(ldapSyncUnsupported)
}

func (unavailableLDAPSyncRepository) ListSyncTasks(context.Context, string, int) ([]*ldaps.LdapSyncTask, error) {
	return nil, errUnavailable(ldapSyncUnsupported)
}

func (unavailableLDAPSyncRepository) ListSyncTasksForOrganization(context.Context, string, string, int) ([]*ldaps.LdapSyncTask, error) {
	return nil, errUnavailable(ldapSyncUnsupported)
}

func (unavailableLDAPSyncRepository) GetLatestSyncTask(context.Context, string) (*ldaps.LdapSyncTask, error) {
	return nil, errUnavailable(ldapSyncUnsupported)
}

func (unavailableLDAPSyncRepository) ListDueSyncConfigs(context.Context) ([]*ldaps.LdapSyncConfig, error) {
	return nil, errUnavailable(ldapSyncUnsupported)
}

func (unavailableLDAPSyncRepository) MarkSyncRunning(context.Context, string) error {
	return errUnavailable(ldapSyncUnsupported)
}

func (unavailableLDAPSyncRepository) MarkSyncComplete(context.Context, string, string) error {
	return errUnavailable(ldapSyncUnsupported)
}
