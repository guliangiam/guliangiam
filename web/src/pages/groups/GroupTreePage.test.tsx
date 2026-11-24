import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { App } from "antd";
import { QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createTestQueryClient } from "@/test-utils/render";
import i18n, { ensureResourceNamespace } from "@/shared/i18n";
import enResource from "@/shared/i18n/locales/en/resource.json";
import zhCNResource from "@/shared/i18n/locales/zh-CN/resource.json";
import GroupTreePage from "./GroupTreePage";

const api = vi.hoisted(() => ({
  getGroups: vi.fn<() => Promise<unknown[]>>(),
  getGroup: vi.fn<(id: string) => Promise<unknown>>(),
}));

vi.mock("@/shared/api/iam", () => api);

vi.mock("@/shared/ui/PermissionGuard", () => ({
  PermissionGuard: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

const groups = [
  {
    id: "root",
    owner: "built-in",
    name: "root-group",
    displayName: "Root Group",
    type: "physical",
    parentId: "",
    isTopGroup: true,
    isEnabled: true,
  },
  {
    id: "child",
    owner: "built-in",
    name: "child-group",
    displayName: "Child Group",
    type: "virtual",
    parentId: "root",
    isTopGroup: false,
    isEnabled: true,
  },
  {
    id: "leaf",
    owner: "built-in",
    name: "leaf-team",
    displayName: "Leaf Squad",
    type: "virtual",
    parentId: "child",
    isTopGroup: false,
    isEnabled: false,
  },
  {
    id: "other",
    owner: "built-in",
    name: "other-group",
    displayName: "Other Group",
    type: "virtual",
    parentId: "",
    isTopGroup: true,
    isEnabled: true,
  },
];

function groupDetail(id: string) {
  return {
    id,
    organizationId: "built-in",
    name: id,
    displayName: id,
    type: "virtual",
    parentId: "",
    isTopGroup: false,
    isEnabled: true,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    manager: "",
    contactEmail: "",
    homepage: "",
    users: ["alice"],
    revision: 1,
  };
}

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location-search">{location.search}</div>;
}

function renderPage(entry: string) {
  const queryClient = createTestQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      <App component={false}>
        <MemoryRouter initialEntries={[entry]}>
          <Routes>
            <Route path="/groups/tree" element={<GroupTreePage />} />
          </Routes>
          <LocationProbe />
        </MemoryRouter>
      </App>
    </QueryClientProvider>,
  );
}

beforeEach(async () => {
  vi.clearAllMocks();
  api.getGroups.mockResolvedValue(groups);
  api.getGroup.mockImplementation(async (id: string) => groupDetail(id));
  await i18n.changeLanguage("en");
});

afterEach(async () => {
  await i18n.changeLanguage("en");
});

describe("GroupTreePage deep link selection", () => {
  it("selects the group from ?group= and renders its detail", async () => {
    renderPage("/groups/tree?group=child");

    expect(await screen.findByText("Group: Child Group")).toBeInTheDocument();
    await waitFor(() => expect(api.getGroup).toHaveBeenCalledWith("child", "built-in"));
    expect(await screen.findByText("alice")).toBeInTheDocument();
    expect(screen.getByTestId("location-search")).toHaveTextContent("group=child");
  });

  it("falls back to no selection for an unknown ?group= id without crashing or fetching", async () => {
    renderPage("/groups/tree?group=does-not-exist");

    // 树本身仍然渲染，说明无效深链没有破坏页面。
    expect(await screen.findByText("Root Group")).toBeInTheDocument();
    expect(screen.getByText(enResource.group.treeSelectPrompt)).toBeInTheDocument();
    expect(api.getGroup).not.toHaveBeenCalled();
  });
});

describe("GroupTreePage URL synchronization", () => {
  it("writes the selected group to ?group= while preserving ?organizationId=", async () => {
    const user = userEvent.setup();
    renderPage("/groups/tree?organizationId=built-in");

    await user.click(await screen.findByText("Leaf Squad"));

    await waitFor(() =>
      expect(screen.getByTestId("location-search")).toHaveTextContent("group=leaf"),
    );
    const search = screen.getByTestId("location-search").textContent ?? "";
    expect(search).toContain("organizationId=built-in");
    await waitFor(() => expect(api.getGroup).toHaveBeenCalledWith("leaf", "built-in"));
  });
});

describe("GroupTreePage search filter", () => {
  it("keeps matches and their ancestors, issues no extra request, and restores on clear", async () => {
    const user = userEvent.setup();
    renderPage("/groups/tree");

    expect(await screen.findByText("Other Group")).toBeInTheDocument();
    expect(api.getGroups).toHaveBeenCalledTimes(1);

    const search = screen.getByPlaceholderText(enResource.group.searchPlaceholder);

    // 命中 name（leaf-team）而非 displayName（Leaf Squad）。
    await user.type(search, "team");
    await waitFor(() => expect(screen.queryByText("Other Group")).not.toBeInTheDocument());
    expect(screen.getByText("Root Group")).toBeInTheDocument();
    expect(screen.getByText("Child Group")).toBeInTheDocument();
    expect(screen.getByText("Leaf Squad")).toBeInTheDocument();
    expect(api.getGroups).toHaveBeenCalledTimes(1);

    // 命中 displayName（Leaf Squad）而非 name（leaf-team）。
    await user.clear(search);
    await user.type(search, "squad");
    await waitFor(() => expect(screen.queryByText("Other Group")).not.toBeInTheDocument());
    expect(screen.getByText("Leaf Squad")).toBeInTheDocument();
    expect(api.getGroups).toHaveBeenCalledTimes(1);

    await user.clear(search);
    expect(await screen.findByText("Other Group")).toBeInTheDocument();
    expect(api.getGroups).toHaveBeenCalledTimes(1);
  });
});

describe("GroupTreePage locale keys", () => {
  it("renders the tree meta suffixes and read-only hint from the en pack", async () => {
    renderPage("/groups/tree?group=leaf");

    expect(await screen.findByText("Group: Leaf Squad")).toBeInTheDocument();
    expect(screen.getByText(enResource.group.treeDisabledSuffix.trim())).toBeInTheDocument();
    expect(
      screen.getAllByText(enResource.group.treeTypeSuffix.replace("{{type}}", "virtual").trim())
        .length,
    ).toBeGreaterThan(0);
    expect(await screen.findByText(enResource.group.treeReadOnlyHint)).toBeInTheDocument();
  });

  it("renders the same keys from the zh-CN pack", async () => {
    await ensureResourceNamespace("zh-CN");
    await i18n.changeLanguage("zh-CN");

    renderPage("/groups/tree?group=leaf");

    expect(
      await screen.findByText(zhCNResource.group.detailTitle.replace("{{name}}", "Leaf Squad")),
    ).toBeInTheDocument();
    expect(screen.getByText(zhCNResource.group.treeDisabledSuffix)).toBeInTheDocument();
    expect(
      screen.getAllByText(zhCNResource.group.treeTypeSuffix.replace("{{type}}", "virtual").trim())
        .length,
    ).toBeGreaterThan(0);
    expect(await screen.findByText(zhCNResource.group.treeReadOnlyHint)).toBeInTheDocument();
  });
});
