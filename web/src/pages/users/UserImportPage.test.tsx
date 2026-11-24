import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { MemoryRouter } from "react-router-dom";
import { useSessionStore } from "@/features/session/session-store";
import { queryKeys } from "@/shared/api/query-keys";
import { createTestQueryClient, createTestWrapper } from "@/test-utils/render";
import UserImportPage from "./UserImportPage";

const { mockImportUsers, mockPreview, mockMessage } = vi.hoisted(() => ({
  mockImportUsers: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockPreview: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockMessage: {
    success: vi.fn<(...args: unknown[]) => void>(),
    error: vi.fn<(...args: unknown[]) => void>(),
    warning: vi.fn<(...args: unknown[]) => void>(),
    info: vi.fn<(...args: unknown[]) => void>(),
  },
}));

vi.mock("@/shared/api/iam/user", () => ({
  importUsersFromExcel: (...args: unknown[]) => mockImportUsers(...args),
}));

vi.mock("@/shared/api/http", () => ({
  requestDocumentJson: (...args: unknown[]) => mockPreview(...args),
}));

vi.mock("antd", async (importOriginal) => {
  const actual = await importOriginal<typeof import("antd")>();
  const AppWithMock = Object.assign(actual.App, {
    useApp: () => ({ message: mockMessage, notification: {}, modal: {} }),
  });
  return { ...actual, App: AppWithMock };
});

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

vi.mock("@/shared/ui/PermissionGuard", () => ({
  PermissionGuard: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

const previewResult = {
  totalRows: 2,
  users: [
    {
      row: 1,
      username: "alice",
      displayName: "Alice",
      email: "alice@example.com",
      phone: "100",
      userType: "normal-user",
      status: "active",
    },
    {
      row: 2,
      username: "bob",
      displayName: "Bob",
      email: "not-an-email",
      phone: "",
      userType: "normal-user",
      status: "active",
      error: "invalid email",
    },
  ],
};

function renderPage(queryClient = createTestQueryClient(), entry = "/users/import") {
  return render(
    <MemoryRouter initialEntries={[entry]}>
      <UserImportPage />
    </MemoryRouter>,
    { wrapper: createTestWrapper({ queryClient }) },
  );
}

async function chooseFile(container: HTMLElement) {
  const input = container.querySelector<HTMLInputElement>('input[type="file"]');
  if (!input) throw new Error("file input not found");
  const file = new File(["username,email"], "users.xlsx", {
    type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
  });
  await userEvent.setup().upload(input, file);
  return file;
}

describe("UserImportPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    useSessionStore.setState({
      account: {
        owner: "built-in",
        name: "admin",
        isAdmin: true,
        organization: { id: "org-1", name: "built-in", displayName: "Built-in" },
      },
      organization: {
        id: "org-1",
        name: "built-in",
        displayName: "Built-in",
        navItems: [],
        userNavItems: [],
      },
    });
  });

  it("previews and imports with the session organization, then invalidates the user list", async () => {
    const user = userEvent.setup();
    mockPreview.mockResolvedValue(previewResult);
    mockImportUsers.mockResolvedValue({
      total: 2,
      succeeded: 1,
      failed: 1,
      errors: ["invalid email"],
    });
    const queryClient = createTestQueryClient();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { container } = renderPage(queryClient);
    const file = await chooseFile(container);

    await user.click(screen.getByRole("button", { name: "userImport.next" }));

    await waitFor(() => expect(mockPreview).toHaveBeenCalledTimes(1));
    const [previewUrl, previewInit] = mockPreview.mock.calls[0] as [string, RequestInit];
    expect(previewUrl).toContain("organizationId=org-1");
    expect(previewInit.method).toBe("POST");
    expect((previewInit.body as FormData).get("file")).toBe(file);

    const importButton = await screen.findByRole("button", { name: "action.import" });
    expect(importButton).toBeEnabled();
    await user.click(importButton);

    await waitFor(() => expect(mockImportUsers).toHaveBeenCalledTimes(1));
    expect(mockImportUsers.mock.calls[0][0]).toBe(file);
    expect(mockImportUsers.mock.calls[0][1]).toBe("org-1");
    expect(await screen.findByText("userImport.complete")).toBeInTheDocument();
    expect(screen.getByText("userImport.succeeded: 1, userImport.failed: 1")).toBeInTheDocument();
    expect(screen.getByText("invalid email")).toBeInTheDocument();
    await waitFor(() =>
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.users.all }),
    );
  });

  it("imports into the organization handed over by the list page instead of the session organization", async () => {
    const user = userEvent.setup();
    mockPreview.mockResolvedValue(previewResult);
    mockImportUsers.mockResolvedValue({ total: 2, succeeded: 2, failed: 0 });

    const { container } = renderPage(createTestQueryClient(), "/users/import?organizationId=org-9");
    await chooseFile(container);

    await user.click(screen.getByRole("button", { name: "userImport.next" }));
    await waitFor(() => expect(mockPreview).toHaveBeenCalledTimes(1));
    expect(mockPreview.mock.calls[0][0]).toContain("organizationId=org-9");

    const importButton = await screen.findByRole("button", { name: "action.import" });
    await user.click(importButton);

    await waitFor(() => expect(mockImportUsers).toHaveBeenCalledTimes(1));
    expect(mockImportUsers.mock.calls[0][1]).toBe("org-9");
  });

  it("shows the backend error, keeps the preview and skips invalidation when import fails", async () => {
    const user = userEvent.setup();
    mockPreview.mockResolvedValue(previewResult);
    mockImportUsers.mockRejectedValue(new Error("import backend down"));
    const queryClient = createTestQueryClient();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { container } = renderPage(queryClient);
    await chooseFile(container);

    await user.click(screen.getByRole("button", { name: "userImport.next" }));
    const importButton = await screen.findByRole("button", { name: "action.import" });
    await user.click(importButton);

    await waitFor(() => expect(mockMessage.error).toHaveBeenCalledWith("import backend down"));
    expect(screen.queryByText("userImport.complete")).not.toBeInTheDocument();
    // The button keeps its loading prefix until the rejected promise settles.
    expect(await screen.findByRole("button", { name: /action.import/ })).toBeInTheDocument();
    expect(invalidateSpy).not.toHaveBeenCalled();
  });

  it("shows the preview error and stays on the upload step when preview fails", async () => {
    const user = userEvent.setup();
    mockPreview.mockRejectedValue(new Error("preview backend down"));

    const { container } = renderPage();
    await chooseFile(container);

    await user.click(screen.getByRole("button", { name: "userImport.next" }));

    await waitFor(() => expect(mockMessage.error).toHaveBeenCalledWith("preview backend down"));
    expect(screen.queryByRole("button", { name: "action.import" })).not.toBeInTheDocument();
    // The button keeps its loading prefix until the rejected promise settles.
    expect(await screen.findByRole("button", { name: /userImport.next/ })).toBeInTheDocument();
    expect(mockImportUsers).not.toHaveBeenCalled();
  });
});
