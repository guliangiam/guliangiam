import { Card, Col, Empty, Progress, Row, Skeleton, Statistic, theme, Typography } from "antd";
import {
  AppstoreOutlined,
  BankOutlined,
  ClockCircleOutlined,
  CloudServerOutlined,
  DashboardOutlined,
  HistoryOutlined,
  KeyOutlined,
  LoginOutlined,
  SafetyCertificateOutlined,
  SafetyOutlined,
  TeamOutlined,
  UserOutlined,
} from "@ant-design/icons";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import {
  getDashboardStats,
  getDashboardUsers,
  getAuthStats,
  getResourceStats,
  getLoginHeatmap,
  getProviderDistribution,
  getMfaCoverage,
  getTrends,
  getOrgUserDistribution,
  getActivityHeatmap,
} from "@/shared/api/iam/dashboard";
import { getSystemMetrics } from "@/shared/api/iam/system";
import { StatCard } from "@/shared/ui/charts/StatCard";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { QueryErrorResult } from "@/shared/ui/QueryErrorResult";
import { BarChart } from "@/shared/ui/charts/BarChart";
import { PieChart } from "@/shared/ui/charts/PieChart";
import { LineChart } from "@/shared/ui/charts/LineChart";
import { ActivityHeatmap } from "@/shared/ui/charts/ActivityHeatmap";

export default function DashboardPage() {
  // 批次9a：首页内嵌数据看板区块由 dashboard:read 授权守卫；
  // 未授权时整体隐藏（首页 "/" 本身恒可达），而不是渲染 403。
  return (
    <PermissionGuard action="dashboard:read" fallback={null}>
      <DashboardContent />
    </PermissionGuard>
  );
}

function DashboardContent() {
  const { t } = useTranslation("common");
  const { token } = theme.useToken();
  const navigate = useNavigate();
  const account = useSessionStore((state) => state.account);
  const enabled = account !== null;

  const statsQuery = useQuery({
    queryKey: ["dashboard", "stats"],
    queryFn: () => getDashboardStats(),
    enabled,
  });

  const userQuery = useQuery({
    queryKey: ["dashboard", "users"],
    queryFn: () => getDashboardUsers(),
    enabled,
  });

  const authQuery = useQuery({
    queryKey: ["dashboard", "auth"],
    queryFn: () => getAuthStats(),
    enabled,
  });

  const resourceQuery = useQuery({
    queryKey: ["dashboard", "resources"],
    queryFn: () => getResourceStats(),
    enabled,
  });

  const heatmapQuery = useQuery({
    queryKey: ["dashboard", "heatmap"],
    queryFn: () => getLoginHeatmap(),
    enabled,
  });

  const providerDistQuery = useQuery({
    queryKey: ["dashboard", "provider-distribution"],
    queryFn: () => getProviderDistribution(),
    enabled,
  });

  const mfaQuery = useQuery({
    queryKey: ["dashboard", "mfa-coverage"],
    queryFn: () => getMfaCoverage(),
    enabled,
  });

  const trendsQuery = useQuery({
    queryKey: ["dashboard", "trends"],
    queryFn: () => getTrends(),
    enabled,
  });

  const orgDistQuery = useQuery({
    queryKey: ["dashboard", "org-user-distribution"],
    queryFn: () => getOrgUserDistribution(),
    enabled,
  });

  const activityHeatmapQuery = useQuery({
    queryKey: ["dashboard", "activity-heatmap"],
    queryFn: () => getActivityHeatmap(),
    enabled,
  });

  const metricsQuery = useQuery({
    queryKey: ["system", "metrics"],
    queryFn: () => getSystemMetrics(),
    enabled,
  });

  const isLoading = statsQuery.isLoading || userQuery.isLoading || authQuery.isLoading;
  const mainStatsErrorQuery = [statsQuery, userQuery, authQuery].find((query) => query.isError);
  const retryMainStats = () => {
    void statsQuery.refetch();
    void userQuery.refetch();
    void authQuery.refetch();
  };

  return (
    <div className="gl-resource-page">
      {/* 列表/表单页都有 H2 页面标题，首页保持同一结构，避免落地页只有卡片无从定位。 */}
      <Typography.Title level={2} style={{ margin: 0 }}>
        {t("dashboard.title")}
      </Typography.Title>
      {isLoading ? (
        <Skeleton active paragraph={{ rows: 8 }} className="gl-panel" />
      ) : mainStatsErrorQuery ? (
        <QueryErrorResult
          error={mainStatsErrorQuery.error}
          onRetry={retryMainStats}
          notFoundTitle={t("resourceNotFound")}
          notFoundDescription={t("dashboard.noData")}
        />
      ) : !statsQuery.data ? (
        <Card variant="borderless">
          <Empty description={t("dashboard.noData")} />
        </Card>
      ) : (
        <>
          <section className="gl-metrics">
            <StatCard
              icon={<BankOutlined />}
              title={t("dashboard.totalOrganizations")}
              value={statsQuery.data.totalOrganizations.toLocaleString()}
              loading={false}
              onClick={() => navigate("/organizations")}
            />
            <StatCard
              icon={<UserOutlined />}
              title={t("dashboard.totalUsers")}
              value={statsQuery.data.totalUsers.toLocaleString()}
              loading={false}
              onClick={() => navigate("/users")}
            />
            <StatCard
              icon={<AppstoreOutlined />}
              title={t("dashboard.totalApplications")}
              value={statsQuery.data.totalApplications.toLocaleString()}
              loading={false}
              onClick={() => navigate("/applications")}
            />
            <StatCard
              icon={<CloudServerOutlined />}
              title={t("dashboard.totalProviders")}
              value={(resourceQuery.data?.totalProviders || 0).toLocaleString()}
              loading={resourceQuery.isLoading}
              onClick={() => navigate("/providers")}
            />
          </section>

          <section className="gl-metrics">
            <StatCard
              icon={<SafetyCertificateOutlined />}
              title={t("dashboard.totalRoles")}
              value={statsQuery.data.totalRoles.toLocaleString()}
              loading={false}
              onClick={() => navigate("/roles")}
            />
            <StatCard
              icon={<TeamOutlined />}
              title={t("dashboard.totalGroups")}
              value={statsQuery.data.totalGroups.toLocaleString()}
              loading={false}
              onClick={() => navigate("/groups")}
            />
            <StatCard
              icon={<KeyOutlined />}
              title={t("dashboard.totalPermissions")}
              value={statsQuery.data.totalPermissions.toLocaleString()}
              loading={false}
              onClick={() => navigate("/permissions")}
            />
            <StatCard
              icon={<LoginOutlined />}
              title={t("dashboard.activeSessions")}
              value={(authQuery.data?.activeSessions || 0).toLocaleString()}
              loading={authQuery.isLoading}
              onClick={() => navigate("/sessions")}
            />
          </section>

          <section className="gl-metrics">
            <StatCard
              icon={<ClockCircleOutlined />}
              title={t("dashboard.todayNewUsers")}
              value={(userQuery.data?.newUsersToday || 0).toLocaleString()}
              loading={userQuery.isLoading}
              onClick={() => navigate("/users")}
            />
            <StatCard
              icon={<HistoryOutlined />}
              title={t("dashboard.recent7DaysNewUsers")}
              value={(userQuery.data?.newUsers7Days || 0).toLocaleString()}
              loading={userQuery.isLoading}
              onClick={() => navigate("/users")}
            />
            <StatCard
              icon={<HistoryOutlined />}
              title={t("dashboard.recent30DaysNewUsers")}
              value={(userQuery.data?.newUsers30Days || 0).toLocaleString()}
              loading={userQuery.isLoading}
              onClick={() => navigate("/users")}
            />
            <StatCard
              icon={<KeyOutlined />}
              title={t("dashboard.totalTokens")}
              value={(authQuery.data?.totalTokens || 0).toLocaleString()}
              loading={authQuery.isLoading}
              onClick={() => navigate("/tokens")}
            />
          </section>
        </>
      )}

      {/* Activity Heatmap - GitHub-style contribution graph */}
      <section>
        <ActivityHeatmap
          title={t("dashboard.activityHeatmap")}
          data={activityHeatmapQuery.data ?? []}
          loading={activityHeatmapQuery.isLoading}
        />
      </section>

      <section>
        <Row gutter={[20, 20]}>
          <Col xs={24} lg={16}>
            <BarChart
              title={t("dashboard.recentLogins")}
              data={heatmapQuery.data ?? []}
              xKey="hour"
              yKeys={[{ key: "count", color: "var(--gl-color-primary)", name: "Logins" }]}
              loading={heatmapQuery.isLoading}
            />
          </Col>
          <Col xs={24} lg={8}>
            <PieChart
              title={t("dashboard.totalProviders")}
              data={(providerDistQuery.data ?? []).map((d) => ({
                name: d.category,
                value: d.count,
              }))}
              loading={providerDistQuery.isLoading}
            />
          </Col>
        </Row>
      </section>

      <section>
        <Row gutter={[20, 20]}>
          <Col xs={24} lg={16}>
            <LineChart
              title={t("dashboard.trends30d")}
              data={(trendsQuery.data ?? []) as Array<Record<string, unknown>>}
              xKey="date"
              lines={[
                { key: "logins", color: "#fa541c", name: t("dashboard.logins") },
                { key: "users", color: token.colorPrimary, name: t("dashboard.totalUsers") },
                {
                  key: "applications",
                  color: token.colorSuccess,
                  name: t("dashboard.totalApplications"),
                },
                { key: "providers", color: "#722ed1", name: t("dashboard.totalProviders") },
              ]}
              loading={trendsQuery.isLoading}
            />
          </Col>
          <Col xs={24} lg={8}>
            <PieChart
              title={t("dashboard.orgUserDistribution")}
              data={(orgDistQuery.data ?? []).map((d) => ({
                name: d.organizationName,
                value: d.userCount,
              }))}
              loading={orgDistQuery.isLoading}
            />
          </Col>
        </Row>
      </section>

      <section>
        <Card variant="borderless" loading={metricsQuery.isLoading}>
          {/* WCAG 1.3.1 Info and Relationships — fix heading level skip h2→h5 to h2→h3 (D-01) */}
          <Typography.Title level={3} style={{ fontSize: 16 }}>
            <DashboardOutlined style={{ marginRight: 8 }} />
            {t("dashboard.runtimeMetrics")}
          </Typography.Title>
          {metricsQuery.data ? (
            <Row gutter={[20, 20]}>
              <Col xs={12} sm={6}>
                <Statistic
                  title={t("dashboard.goroutines")}
                  value={metricsQuery.data.numGoroutine}
                />
              </Col>
              <Col xs={12} sm={6}>
                <Statistic
                  title={t("dashboard.heapAlloc")}
                  value={metricsQuery.data.heapAllocMB.toFixed(1)}
                />
              </Col>
              <Col xs={12} sm={6}>
                <Statistic title={t("dashboard.numGC")} value={metricsQuery.data.numGC} />
              </Col>
              <Col xs={12} sm={6}>
                <Statistic title={t("dashboard.uptime")} value={metricsQuery.data.uptimeSeconds} />
              </Col>
            </Row>
          ) : !metricsQuery.isLoading ? (
            <Empty description={t("dashboard.noMetrics")} image={Empty.PRESENTED_IMAGE_SIMPLE} />
          ) : null}
        </Card>
      </section>

      <section>
        <Card variant="borderless" loading={mfaQuery.isLoading}>
          {/* WCAG 1.3.1 Info and Relationships — fix heading level skip h2→h5 to h2→h3 (D-01) */}
          <Typography.Title level={3} style={{ fontSize: 16 }}>
            <SafetyOutlined style={{ marginRight: 8 }} />
            {t("dashboard.mfaCoverage")}
          </Typography.Title>
          {mfaQuery.data ? (
            <Row gutter={[20, 20]} align="middle">
              <Col xs={24} sm={8} style={{ textAlign: "center" }}>
                <Progress
                  type="dashboard"
                  aria-label={t("dashboard.mfaCoverage")}
                  percent={Math.round(mfaQuery.data.coveragePercent)}
                  format={(pct) => `${pct}%`}
                  strokeColor={{
                    "0%": token.colorSuccess,
                    "100%": token.colorPrimary,
                  }}
                />
                <Typography.Text type="secondary">{t("dashboard.mfaCoverage")}</Typography.Text>
              </Col>
              <Col xs={24} sm={16}>
                <Typography.Paragraph>
                  {mfaQuery.data.usersWithMfa} {t("dashboard.usersWithMfa")} /{" "}
                  {mfaQuery.data.usersWithoutMfa} {t("dashboard.usersWithoutMfa")}
                </Typography.Paragraph>
              </Col>
            </Row>
          ) : !mfaQuery.isLoading ? (
            <Empty
              description="MFA coverage data not available"
              image={Empty.PRESENTED_IMAGE_SIMPLE}
            />
          ) : null}
        </Card>
      </section>
    </div>
  );
}
