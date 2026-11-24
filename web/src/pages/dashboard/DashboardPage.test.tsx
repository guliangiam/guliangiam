import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HttpError } from "@/shared/api/http";
import { useSessionStore } from "@/features/session/session-store";
import { createTestWrapper } from "@/test-utils/render";
import DashboardPage from "./DashboardPage";

const mockGetDashboardStats = vi.fn<(...args: unknown[]) => unknown>();
const mockGetDashboardUsers = vi.fn<(...args: unknown[]) => unknown>();
const mockGetAuthStats = vi.fn<(...args: unknown[]) => unknown>();
const mockGetResourceStats = vi.fn<(...args: unknown[]) => unknown>();
const mockGetLoginHeatmap = vi.fn<(...args: unknown[]) => unknown>();
const mockGetProviderDistribution = vi.fn<(...args: unknown[]) => unknown>();
const mockGetMfaCoverage = vi.fn<(...args: unknown[]) => unknown>();
const mockGetTrends = vi.fn<(...args: unknown[]) => unknown>();
const mockGetOrgUserDistribution = vi.fn<(...args: unknown[]) => unknown>();
const mockGetActivityHeatmap = vi.fn<(...args: unknown[]) => unknown>();
const mockGetSystemMetrics = vi.fn<(...args: unknown[]) => unknown>();
const mockHasAction = vi.fn<(...args: unknown[]) => unknown>();
const mockHasResource = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("@/shared/api/iam/dashboard", () => ({
  getDashboardStats: () => mockGetDashboardStats(),
  getDashboardUsers: () => mockGetDashboardUsers(),
  getAuthStats: () => mockGetAuthStats(),
  getResourceStats: () => mockGetResourceStats(),
  getLoginHeatmap: () => mockGetLoginHeatmap(),
  getProviderDistribution: () => mockGetProviderDistribution(),
  getMfaCoverage: () => mockGetMfaCoverage(),
  getTrends: () => mockGetTrends(),
  getOrgUserDistribution: () => mockGetOrgUserDistribution(),
  getActivityHeatmap: () => mockGetActivityHeatmap(),
}));

vi.mock("@/shared/api/iam/system", () => ({
  getSystemMetrics: () => mockGetSystemMetrics(),
}));

vi.mock("@/shared/hooks/usePermissions", () => ({
  usePermissions: () => ({
    isLoading: false,
    hasAction: mockHasAction,
    hasResource: mockHasResource,
  }),
}));

vi.mock("react-router-dom", () => ({
  useNavigate: () => vi.fn<() => void>(),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: "en" },
  }),
}));

vi.mock("@/shared/ui/charts/StatCard", () => ({
  StatCard: ({ title, value }: { title: React.ReactNode; value: React.ReactNode }) => (
    <div data-testid="main-stat">
      {title}:{value}
    </div>
  ),
}));
vi.mock("@/shared/ui/charts/BarChart", () => ({ BarChart: () => null }));
vi.mock("@/shared/ui/charts/PieChart", () => ({ PieChart: () => null }));
vi.mock("@/shared/ui/charts/LineChart", () => ({ LineChart: () => null }));
vi.mock("@/shared/ui/charts/ActivityHeatmap", () => ({ ActivityHeatmap: () => null }));

const stats = {
  totalUsers: 10,
  totalOrganizations: 6,
  totalApplications: 4,
  totalRoles: 3,
  totalGroups: 2,
  totalPermissions: 9,
};

describe("DashboardPage main stats states", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useSessionStore.setState({
      account: {
        id: "user-1",
        owner: "org-1",
        name: "admin",
        displayName: "Admin",
        avatar: null,
        isAdmin: true,
      },
      organization: null,
      impersonatedUser: null,
    });
    mockHasAction.mockReturnValue(true);
    mockHasResource.mockReturnValue(true);
    mockGetDashboardStats.mockResolvedValue(stats);
    mockGetDashboardUsers.mockResolvedValue({
      newUsersToday: 1,
      newUsers7Days: 2,
      newUsers30Days: 3,
    });
    mockGetAuthStats.mockResolvedValue({ activeSessions: 5, totalTokens: 7 });
    mockGetResourceStats.mockResolvedValue({ totalProviders: 8 });
    mockGetLoginHeatmap.mockResolvedValue([]);
    mockGetProviderDistribution.mockResolvedValue([]);
    mockGetMfaCoverage.mockResolvedValue({
      coveragePercent: 50,
      usersWithMfa: 1,
      usersWithoutMfa: 1,
    });
    mockGetTrends.mockResolvedValue([]);
    mockGetOrgUserDistribution.mockResolvedValue([]);
    mockGetActivityHeatmap.mockResolvedValue([]);
    mockGetSystemMetrics.mockResolvedValue({
      numGoroutine: 1,
      heapAllocMB: 1,
      numGC: 1,
      uptimeSeconds: 1,
    });
  });

  // 批次9a：未授权时首页看板区块整体隐藏（不再渲染 403），且不发起看板请求。
  it("hides the dashboard block without querying data or rendering 403 when permission is denied", () => {
    mockHasAction.mockReturnValue(false);

    const { container } = render(<DashboardPage />, { wrapper: createTestWrapper() });

    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.queryByText("403")).not.toBeInTheDocument();
    expect(container).toBeEmptyDOMElement();
    expect(mockGetDashboardStats).not.toHaveBeenCalled();
  });

  it("shows Skeleton without mixing in stats or Empty while loading", () => {
    mockGetDashboardStats.mockImplementation(() => new Promise(() => {}));
    const { container } = render(<DashboardPage />, { wrapper: createTestWrapper() });

    expect(container.querySelector(".gl-panel.ant-skeleton")).toBeInTheDocument();
    expect(screen.queryByTestId("main-stat")).not.toBeInTheDocument();
    expect(screen.queryByText("dashboard.noData")).not.toBeInTheDocument();
  });

  it("shows stats errors instead of Empty and retries all main stats queries", async () => {
    const user = userEvent.setup();
    mockGetDashboardStats
      .mockRejectedValueOnce(new Error("stats unavailable"))
      .mockResolvedValue(stats);

    render(<DashboardPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByText("loadFailed")).toBeInTheDocument();
    expect(screen.queryByText("dashboard.noData")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "retry" }));

    expect(await screen.findByText("dashboard.totalOrganizations:6")).toBeInTheDocument();
    expect(mockGetDashboardStats).toHaveBeenCalledTimes(2);
    expect(mockGetDashboardUsers).toHaveBeenCalledTimes(2);
    expect(mockGetAuthStats).toHaveBeenCalledTimes(2);
  });

  it("surfaces errors from secondary main stats queries", async () => {
    mockGetAuthStats.mockRejectedValue(new Error("auth stats unavailable"));

    render(<DashboardPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByText("loadFailed")).toBeInTheDocument();
    expect(screen.queryByTestId("main-stat")).not.toBeInTheDocument();
    expect(screen.queryByText("dashboard.noData")).not.toBeInTheDocument();
  });

  it("shows Empty only when stats are missing without an error", async () => {
    mockGetDashboardStats.mockResolvedValue(null);

    render(<DashboardPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByText("dashboard.noData")).toBeInTheDocument();
    expect(screen.queryByText("loadFailed")).not.toBeInTheDocument();
    expect(screen.queryByTestId("main-stat")).not.toBeInTheDocument();
  });

  it("shows the same 403 for a server-side permission rejection", async () => {
    mockGetDashboardStats.mockRejectedValue(new HttpError(403, null, "Forbidden"));

    render(<DashboardPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByText("common:forbidden")).toBeInTheDocument();
    expect(screen.getByText("403")).toBeInTheDocument();
    expect(screen.queryByText("loadFailed")).not.toBeInTheDocument();
    expect(screen.queryByText("dashboard.noData")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "retry" })).not.toBeInTheDocument();
  });

  it("renders stats without the main Skeleton or Empty", async () => {
    const { container } = render(<DashboardPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByText("dashboard.totalOrganizations:6")).toBeInTheDocument();
    expect(container.querySelector(".gl-panel.ant-skeleton")).not.toBeInTheDocument();
    expect(screen.queryByText("dashboard.noData")).not.toBeInTheDocument();
    expect(screen.queryByText("loadFailed")).not.toBeInTheDocument();
  });
});
