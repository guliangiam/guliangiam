import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MenuSearch } from "./MenuSearch";
import { globalSearch } from "@/shared/api/iam/search";
import type { SearchResult } from "@/shared/api/iam/search";

const navigateMock = vi.fn<(path: string) => void>();

vi.mock("react-router-dom", () => ({
  useNavigate: () => navigateMock,
  useLocation: () => ({ pathname: "/dashboard" }),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

// 批次9a：MenuSearch 内部改用 usePermissions 叠加授权菜单过滤；
// 本组用例聚焦搜索行为，模拟"权限加载中"（allowedMenus=null 不过滤）。
vi.mock("@/shared/hooks/usePermissions", () => ({
  usePermissions: () => ({
    permissions: { menus: [], actions: [], resources: [] },
    isLoading: true,
    isError: false,
    hasAction: () => true,
    hasAnyAction: () => true,
    hasMenu: () => true,
    hasResource: () => true,
    isAdmin: false,
  }),
}));

vi.mock("@/shared/api/iam/search", () => ({
  globalSearch: vi.fn<(keyword: string, limit?: number) => Promise<SearchResult>>(),
}));

const mockedGlobalSearch = vi.mocked(globalSearch);

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

function renderPalette() {
  return render(<MenuSearch account={null} organization={null} />);
}

/** 组件自管理开关：⌘K / Ctrl+K 打开命令面板。 */
function openPalette() {
  fireEvent.keyDown(window, { key: "k", metaKey: true });
}

function typeKeyword(value: string) {
  const input = screen.getByPlaceholderText("menuSearch.placeholder");
  fireEvent.change(input, { target: { value } });
}

/** 推进全部防抖定时器并冲刷 promise 微任务。 */
async function flush(ms = 350) {
  await act(async () => {
    vi.advanceTimersByTime(ms);
  });
}

describe("MenuSearch 数据结果分组", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    navigateMock.mockReset();
    mockedGlobalSearch.mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("输入防抖 300ms 后才调用聚合搜索接口", async () => {
    mockedGlobalSearch.mockResolvedValue({ query: "ali", total: 0, groups: [] });
    renderPalette();
    openPalette();
    typeKeyword("ali");

    await flush(200);
    expect(mockedGlobalSearch).not.toHaveBeenCalled();

    await flush(150);
    expect(mockedGlobalSearch).toHaveBeenCalledWith("ali");
  });

  it("按实体分组渲染数据结果并点击跳转详情路由", async () => {
    const result: SearchResult = {
      query: "ali",
      total: 2,
      groups: [
        {
          entity: "users",
          items: [
            {
              id: "user-1",
              title: "Alice Zhang",
              subtitle: "alice@acme.example",
              username: "alice",
              organizationId: "org-1",
              organizationSlug: "acme",
            },
          ],
        },
        {
          entity: "applications",
          items: [{ id: "app-1", title: "Portal", subtitle: "portal" }],
        },
      ],
    };
    mockedGlobalSearch.mockResolvedValue(result);
    renderPalette();
    openPalette();
    typeKeyword("zzz-no-menu-match");

    await flush();

    // 数据结果超组标题 + 实体分组标题 + 条目（标题/副标题）。
    expect(screen.getByText("menuSearch.dataResults")).toBeInTheDocument();
    expect(screen.getByText("nav.users")).toBeInTheDocument();
    expect(screen.getAllByText("nav.applications").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("Alice Zhang")).toBeInTheDocument();
    expect(screen.getByText("alice@acme.example")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Alice Zhang"));
    expect(navigateMock).toHaveBeenCalledWith("/users/acme/alice");

    fireEvent.click(screen.getByText("Portal"));
    expect(navigateMock).toHaveBeenCalledWith("/applications/app-1");
  });

  it("展示加载态并在完成后消失", async () => {
    const { promise, resolve } = deferred<SearchResult>();
    mockedGlobalSearch.mockReturnValue(promise);
    renderPalette();
    openPalette();
    typeKeyword("zzz-no-menu-match");

    await flush();
    expect(screen.getByText("menuSearch.dataLoading")).toBeInTheDocument();

    await act(async () => {
      resolve({ query: "zzz-no-menu-match", total: 0, groups: [] });
      await promise;
    });
    expect(screen.queryByText("menuSearch.dataLoading")).not.toBeInTheDocument();
    // 数据无命中且菜单也无命中 → 整体空态。
    expect(screen.getByText("menuSearch.noResults")).toBeInTheDocument();
  });

  it("接口失败时静默降级，不影响原菜单分组", async () => {
    mockedGlobalSearch.mockRejectedValue(new Error("403"));
    renderPalette();
    openPalette();
    typeKeyword("zzz-no-menu-match");

    await flush();

    expect(screen.queryByText("menuSearch.dataLoading")).not.toBeInTheDocument();
    // 数据分组不出现，但原有空态/菜单渲染不受影响。
    expect(screen.queryByText("menuSearch.dataResults")).not.toBeInTheDocument();
    expect(screen.getByText("menuSearch.noResults")).toBeInTheDocument();
  });

  it("空输入不触发数据搜索，保留原有分组行为", async () => {
    renderPalette();
    openPalette();
    await flush(400);

    expect(mockedGlobalSearch).not.toHaveBeenCalled();
    // 原 Create 分组（标题 + 条目副标题）正常渲染。
    expect(screen.getAllByText("menuSearch.section.create").length).toBeGreaterThan(0);
  });
});
