import { requestDocumentJson } from "@/shared/api/http";
import {
  DASHBOARD_ACTIVITY_HEATMAP,
  DASHBOARD_AUTH,
  DASHBOARD_LOGIN_HEATMAP,
  DASHBOARD_MFA_COVERAGE,
  DASHBOARD_ORG_USER_DISTRIBUTION,
  DASHBOARD_PROVIDER_DISTRIBUTION,
  DASHBOARD_RESOURCES,
  DASHBOARD_STATS,
  DASHBOARD_TRENDS,
  DASHBOARD_USERS,
} from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Dashboard statistics & chart data
// Matches internal/app/dashboard/service.go types
// ─────────────────────────────────────────────

/** Aggregate counts from GET /api/v1/dashboard/stats */
export type DashboardStats = {
  totalUsers: number;
  totalOrganizations: number;
  totalApplications: number;
  totalRoles: number;
  totalGroups: number;
  totalPermissions: number;
};

/** User statistics from GET /api/v1/dashboard/users */
export type UserStats = {
  totalUsers: number;
  activeUsers: number;
  disabledUsers: number;
  newUsersToday: number;
  newUsers7Days: number;
  newUsers30Days: number;
};

/** Auth stats from GET /api/v1/dashboard/auth */
export type AuthStats = {
  totalSessions: number;
  activeSessions: number;
  totalTokens: number;
  activeTokens: number;
  failedLogins24h: number;
};

/** Resource stats from GET /api/v1/dashboard/resources */
export type ResourceStats = {
  totalApplications: number;
  totalRoles: number;
  totalGroups: number;
  totalPermissions: number;
  totalProviders: number;
};

/** Provider distribution item from GET /api/v1/dashboard/provider-distribution */
export type ProviderDistributionItem = {
  category: string;
  count: number;
};

/** MFA coverage from GET /api/v1/dashboard/mfa-coverage */
export type MfaCoverage = {
  usersWithMfa: number;
  usersWithoutMfa: number;
  coveragePercent: number;
};

/** Login heatmap point from GET /api/v1/dashboard/login-heatmap */
export type LoginHeatmapPoint = {
  hour: number;
  count: number;
};

/** 30-day trend data point from GET /api/v1/dashboard/trends */
export type TrendDataPoint = {
  date: string;
  users: number;
  applications: number;
  providers: number;
  logins: number;
};

/** Activity heatmap point from GET /api/v1/dashboard/activity-heatmap */
export type ActivityHeatmapPoint = {
  date: string;
  count: number;
};

/** Organization user distribution from GET /api/v1/dashboard/org-user-distribution */
export type OrgUserDistributionItem = {
  organizationName: string;
  userCount: number;
};

// ─── API functions (errors propagate to TanStack Query) ──────────────

export async function getDashboardStats(): Promise<DashboardStats> {
  return (await requestDocumentJson(DASHBOARD_STATS)) as DashboardStats;
}

export async function getDashboardUsers(): Promise<UserStats> {
  return (await requestDocumentJson(DASHBOARD_USERS)) as UserStats;
}

export async function getAuthStats(): Promise<AuthStats> {
  return (await requestDocumentJson(DASHBOARD_AUTH)) as AuthStats;
}

export async function getResourceStats(): Promise<ResourceStats> {
  return (await requestDocumentJson(DASHBOARD_RESOURCES)) as ResourceStats;
}

export async function getProviderDistribution(): Promise<ProviderDistributionItem[]> {
  const res = await requestDocumentJson(DASHBOARD_PROVIDER_DISTRIBUTION);
  const data = (res as { data?: ProviderDistributionItem[] }).data;
  return data ?? (res as ProviderDistributionItem[]);
}

export async function getMfaCoverage(): Promise<MfaCoverage> {
  return (await requestDocumentJson(DASHBOARD_MFA_COVERAGE)) as MfaCoverage;
}

export async function getLoginHeatmap(): Promise<LoginHeatmapPoint[]> {
  const res = await requestDocumentJson(DASHBOARD_LOGIN_HEATMAP);
  const data = (res as { data?: LoginHeatmapPoint[] }).data;
  return data ?? (res as LoginHeatmapPoint[]);
}

export async function getTrends(): Promise<TrendDataPoint[]> {
  const res = await requestDocumentJson(DASHBOARD_TRENDS);
  const data = (res as { data?: TrendDataPoint[] }).data;
  return data ?? (res as TrendDataPoint[]);
}

export async function getOrgUserDistribution(): Promise<OrgUserDistributionItem[]> {
  const res = await requestDocumentJson(DASHBOARD_ORG_USER_DISTRIBUTION);
  const data = (res as { data?: OrgUserDistributionItem[] }).data;
  return data ?? (res as OrgUserDistributionItem[]);
}

export async function getActivityHeatmap(): Promise<ActivityHeatmapPoint[]> {
  const res = await requestDocumentJson(DASHBOARD_ACTIVITY_HEATMAP);
  const data = (res as { data?: ActivityHeatmapPoint[] }).data;
  return data ?? (res as ActivityHeatmapPoint[]);
}
