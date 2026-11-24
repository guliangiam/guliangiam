import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { App } from "antd";
import { QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createTestQueryClient } from "@/test-utils/render";
import { useSessionStore } from "@/features/session/session-store";
import UserCreatePage from "./UserCreatePage";

const { mockCreateUser } = vi.hoisted(() => ({
  mockCreateUser: vi.fn<(payload: Record<string, unknown>) => Promise<unknown>>(),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: "en" },
  }),
}));

vi.mock("@/shared/api/iam/user", () => ({
  createUser: mockCreateUser,
}));

vi.mock("@/shared/api/iam/role", () => ({
  getRoleOptions: vi.fn<() => Promise<unknown[]>>(async () => []),
}));

vi.mock("@/shared/api/iam/group", () => ({
  getGroupOptions: vi.fn<() => Promise<unknown[]>>(async () => []),
}));

vi.mock("@/shared/api/iam/application", () => ({
  getApplicationOptions: vi.fn<() => Promise<unknown[]>>(async () => []),
}));

type OrganizationOptionFixture = {
  id: string;
  name: string;
  slug: string;
  displayName: string;
  label: string;
  value: string;
};

vi.mock("@/shared/api/iam/organization", () => ({
  getOrganizationOptions: vi.fn<() => Promise<OrganizationOptionFixture[]>>(async () => [
    {
      id: "org-1",
      name: "built-in",
      slug: "built-in",
      displayName: "Built-in",
      label: "Built-in",
      value: "org-1",
    },
  ]),
}));

vi.mock("@/shared/ui/PasswordChecker", () => {
  const rules: unknown[] = [];
  return {
    PasswordChecker: () => null,
    checkPasswordComplexity: () => null,
    usePasswordRules: () => rules,
  };
});

vi.mock("@/shared/ui/ImageCropUpload", () => ({
  ImageCropUpload: ({ id }: { id?: string }) => <input id={id} readOnly />,
}));
vi.mock("@/shared/ui/CountryCodeSelect", () => ({
  CountryCodeSelect: ({ id }: { id?: string }) => <input id={id} readOnly />,
}));
vi.mock("@/shared/ui/RegionSelect", () => ({
  RegionSelect: ({ id }: { id?: string }) => <input id={id} readOnly />,
}));

function renderPage() {
  const router = createMemoryRouter([{ path: "/users", element: <UserCreatePage /> }], {
    initialEntries: ["/users"],
  });

  return render(
    <QueryClientProvider client={createTestQueryClient()}>
      <App component={false}>
        <RouterProvider router={router} />
      </App>
    </QueryClientProvider>,
  );
}

async function fillRequiredFields(username: string, displayName: string) {
  const user = userEvent.setup();
  await user.type(await screen.findByPlaceholderText("enterUsername"), username);
  await user.type(screen.getByPlaceholderText("displayName"), displayName);
  return user;
}

describe("UserCreatePage password", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    mockCreateUser.mockResolvedValue({ id: "user-1" });
    useSessionStore.setState({
      account: null,
      organization: {
        id: "org-1",
        name: "built-in",
        displayName: "Built-in",
        navItems: [],
        userNavItems: [],
      },
      impersonatedUser: null,
    });
  });

  it("submits the password chosen in the modal and flips the button to change password", async () => {
    renderPage();
    const user = await fillRequiredFields("alice", "Alice Smith");

    await user.click(screen.getByRole("button", { name: "user.groups.security" }));
    await user.click(await screen.findByRole("button", { name: "setPassword" }));

    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByPlaceholderText("enterNewPassword"), "Str0ng!Pass");
    await user.type(within(dialog).getByPlaceholderText("enterConfirmPassword"), "Str0ng!Pass");
    await user.click(within(dialog).getByRole("button", { name: "setPassword" }));

    expect(await screen.findByRole("button", { name: "changePassword" })).toBeInTheDocument();
    expect(screen.getByText("✓")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "save" }));

    await waitFor(() => expect(mockCreateUser).toHaveBeenCalledTimes(1));
    expect(mockCreateUser.mock.calls[0][0]).toMatchObject({
      username: "alice",
      password: "Str0ng!Pass",
    });
  });

  it("does not submit a password when the modal was never used", async () => {
    renderPage();
    const user = await fillRequiredFields("bob", "Bob Jones");

    expect(screen.queryByText("changePassword")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "save" }));

    await waitFor(() => expect(mockCreateUser).toHaveBeenCalledTimes(1));
    expect(mockCreateUser.mock.calls[0][0]).toMatchObject({
      username: "bob",
      password: "",
    });
  });

  it("shows a visible required hint when confirm password is empty instead of returning silently", async () => {
    renderPage();
    const user = await fillRequiredFields("carol", "Carol Smith");

    await user.click(screen.getByRole("button", { name: "user.groups.security" }));
    await user.click(await screen.findByRole("button", { name: "setPassword" }));

    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByPlaceholderText("enterNewPassword"), "Str0ng!Pass");
    await user.click(within(dialog).getByRole("button", { name: "setPassword" }));

    expect(within(dialog).getByText("enterConfirmPassword")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "changePassword" })).not.toBeInTheDocument();
  });

  it("shows required hints for both password fields when OK is clicked while empty", async () => {
    renderPage();
    const user = await fillRequiredFields("dave", "Dave Jones");

    await user.click(screen.getByRole("button", { name: "user.groups.security" }));
    await user.click(await screen.findByRole("button", { name: "setPassword" }));

    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: "setPassword" }));

    expect(within(dialog).getByText("enterNewPassword")).toBeInTheDocument();
    expect(within(dialog).getByText("enterConfirmPassword")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "changePassword" })).not.toBeInTheDocument();
  });
});

/** 创建页现有字段全集（按 name 断言，防止分区调整时漏字段）。 */
const CREATE_FORM_FIELDS = [
  // 账号
  "organizationId",
  "username",
  "displayName",
  "userType",
  "status",
  "signupApplicationId",
  "externalId",
  // 资料
  "avatarUrl",
  "firstName",
  "lastName",
  "email",
  "phone",
  "countryCode",
  "region",
  "location",
  "gender",
  "birthday",
  "language",
  "homepage",
  "bio",
  "education",
  "tag",
  "realName",
  "idCardType",
  "idCard",
  // 组织与角色
  "isAdmin",
  "roleBindingIds",
  "groupIds",
  "affiliation",
  "title",
  "score",
  "karma",
  "ranking",
  "balance",
  "balanceCredit",
  "balanceCurrency",
  // 安全
  "isForbidden",
  "isDeleted",
  "needUpdatePassword",
  "ipWhitelist",
];

describe("UserCreatePage sections", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    mockCreateUser.mockResolvedValue({ id: "user-1" });
    useSessionStore.setState({
      account: null,
      organization: {
        id: "org-1",
        name: "built-in",
        displayName: "Built-in",
        navItems: [],
        userNavItems: [],
      },
      impersonatedUser: null,
    });
  });

  it("renders the four sections in account / profile / organization-role / security order", async () => {
    const { container } = renderPage();
    await screen.findByPlaceholderText("enterUsername");

    const titles = [
      "user.basicSection",
      "user.tabs.profile",
      "user.roleSection",
      "user.groups.security",
    ];
    const headings = Array.from(container.querySelectorAll(".gl-form-section__title")).map((node) =>
      node.textContent?.trim().replace(/\*$/, ""),
    );
    expect(headings).toEqual(titles);
  });

  it("keeps every create field mounted", async () => {
    const { container } = renderPage();
    await screen.findByPlaceholderText("enterUsername");

    for (const name of CREATE_FORM_FIELDS) {
      expect(container.querySelector(`#${name}`), `field ${name}`).not.toBeNull();
    }
  });

  it("keeps the create-time defaults unchanged", async () => {
    renderPage();
    await screen.findByPlaceholderText("enterUsername");

    const selectText = (id: string) =>
      document.getElementById(id)?.closest(".ant-select")?.textContent?.trim() ?? "";
    // OrganizationSelect 经 AsyncSearchSelect 异步加载（250ms debounce），标签须等待解析。
    await waitFor(() => expect(selectText("organizationId")).toBe("Built-in (built-in)"));
    expect(selectText("userType")).toBe("resource:user.userTypeNormal");
    expect(selectText("status")).toBe("resource:user.statusActive");
    expect(selectText("balanceCurrency")).toBe("common:currency.USD");

    for (const id of ["isForbidden", "isDeleted", "needUpdatePassword", "isAdmin"]) {
      expect(document.getElementById(id)).toHaveAttribute("aria-checked", "false");
    }
    for (const id of ["score", "karma", "ranking", "balance", "balanceCredit"]) {
      expect((document.getElementById(id) as HTMLInputElement).value).toBe("0");
    }
  });
});
