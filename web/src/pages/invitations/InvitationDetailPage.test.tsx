import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HttpError } from "@/shared/api/http";
import { useSessionStore } from "@/features/session/session-store";
import { createTestWrapper } from "@/test-utils/render";
import InvitationDetailPage from "./InvitationDetailPage";

const mockGetInvitations = vi.fn<(...args: unknown[]) => unknown>();
const mockGetInvitation = vi.fn<(...args: unknown[]) => unknown>();
const mockRevokeInvitation = vi.fn<(...args: unknown[]) => unknown>();
const mockResendInvitation = vi.fn<(...args: unknown[]) => unknown>();
const mockHasAction = vi.fn<(...args: unknown[]) => unknown>();
const mockHasResource = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("@/shared/api/iam/invitation", () => ({
  getInvitations: (...args: unknown[]) => mockGetInvitations(...args),
  getInvitation: (id: string) => mockGetInvitation(id),
  revokeInvitation: (id: string) => mockRevokeInvitation(id),
  resendInvitation: (id: string) => mockResendInvitation(id),
}));

vi.mock("@/shared/api/iam/role", () => ({
  getRoleOptions: vi.fn<() => Promise<unknown[]>>().mockResolvedValue([]),
}));

vi.mock("@/shared/api/iam/group", () => ({
  getGroupOptions: vi.fn<() => Promise<unknown[]>>().mockResolvedValue([]),
}));

vi.mock("@/shared/api/iam/organization", () => ({
  getOrganizations: vi.fn<() => Promise<unknown[]>>().mockResolvedValue([]),
}));

vi.mock("@/shared/hooks/usePermissions", () => ({
  usePermissions: () => ({
    isLoading: false,
    hasAction: mockHasAction,
    hasResource: mockHasResource,
  }),
}));

vi.mock("@/shared/hooks/useResourceNames", () => ({
  useOrganizationNames: () => ({ nameMap: new Map() }),
}));

vi.mock("react-router-dom", () => ({
  useNavigate: () => vi.fn<() => void>(),
  useParams: () => ({
    organizationName: "org-1",
    invitationName: "alice@example.com",
  }),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: "en" },
  }),
}));

vi.mock("@/shared/ui/ResourceFormPage", () => ({
  ResourceFormPage: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="invitation-form">{children}</div>
  ),
}));

vi.mock("@/shared/ui/FormSection", () => ({
  FormSection: ({ children }: { children: React.ReactNode }) => <section>{children}</section>,
}));

vi.mock("@/shared/ui/GlForm", () => ({
  GlFormRow: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));

const invitation = {
  id: "invitation-1",
  organizationId: "org-1",
  email: "alice@example.com",
  code: "code-123",
  roleIds: [],
  groupIds: ["group-1"],
  status: "pending",
  expiresAt: "2026-12-01T00:00:00Z",
  invitedBy: "admin",
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
  revision: 1,
};

describe("InvitationDetailPage states", () => {
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
    mockGetInvitations.mockResolvedValue([invitation]);
    mockGetInvitation.mockResolvedValue(invitation);
  });

  it("shows page-level 403 without querying invitations when permission is denied", () => {
    mockHasAction.mockReturnValue(false);

    render(<InvitationDetailPage />, { wrapper: createTestWrapper() });

    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.getByText("403")).toBeInTheDocument();
    expect(screen.getByText("common:forbidden")).toBeInTheDocument();
    expect(mockGetInvitations).not.toHaveBeenCalled();
  });

  it("shows Skeleton without mixing in the detail form while loading", () => {
    mockGetInvitations.mockImplementation(() => new Promise(() => {}));
    const { container } = render(<InvitationDetailPage />, { wrapper: createTestWrapper() });

    expect(container.querySelector(".ant-skeleton")).toBeInTheDocument();
    expect(screen.queryByTestId("invitation-form")).not.toBeInTheDocument();
    expect(screen.queryByText("invitation.notFound")).not.toBeInTheDocument();
  });

  it("retries a recoverable list error without presenting it as NotFound", async () => {
    const user = userEvent.setup();
    mockGetInvitations
      .mockRejectedValueOnce(new Error("list unavailable"))
      .mockResolvedValue([invitation]);

    render(<InvitationDetailPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByText("loadFailed")).toBeInTheDocument();
    expect(screen.queryByText("invitation.notFound")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "retry" }));

    expect(await screen.findByTestId("invitation-form")).toBeInTheDocument();
    expect(mockGetInvitations).toHaveBeenCalledTimes(2);
  });

  it("retries a recoverable detail error", async () => {
    const user = userEvent.setup();
    mockGetInvitation
      .mockRejectedValueOnce(new Error("detail unavailable"))
      .mockResolvedValue(invitation);

    render(<InvitationDetailPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByText("loadFailed")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "retry" }));

    expect(await screen.findByTestId("invitation-form")).toBeInTheDocument();
    expect(mockGetInvitation).toHaveBeenCalledTimes(2);
  });

  it("shows an explicit NotFound result when the invitation is absent", async () => {
    mockGetInvitations.mockResolvedValue([]);

    render(<InvitationDetailPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByText("invitation.notFound")).toBeInTheDocument();
    expect(screen.getByText("invitation.notFoundDesc")).toBeInTheDocument();
    expect(screen.queryByText("loadFailed")).not.toBeInTheDocument();
    expect(mockGetInvitation).not.toHaveBeenCalled();
  });

  it("maps a detail 404 response to NotFound instead of a retryable error", async () => {
    mockGetInvitation.mockRejectedValue(new HttpError(404, null, "Missing"));

    render(<InvitationDetailPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByText("invitation.notFound")).toBeInTheDocument();
    expect(screen.queryByText("loadFailed")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "retry" })).not.toBeInTheDocument();
  });

  it("renders detail data without Skeleton or NotFound", async () => {
    const { container } = render(<InvitationDetailPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByTestId("invitation-form")).toBeInTheDocument();
    expect(screen.getByText("alice@example.com")).toBeInTheDocument();
    expect(container.querySelector(".ant-skeleton")).not.toBeInTheDocument();
    expect(screen.queryByText("invitation.notFound")).not.toBeInTheDocument();
  });

  // 定向解析回归：按路由组织 + email 关键词服务端过滤，不再依赖全量列表前 50 行。
  it("resolves the invitation with a targeted org + keyword query", async () => {
    render(<InvitationDetailPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByTestId("invitation-form")).toBeInTheDocument();
    expect(mockGetInvitations).toHaveBeenCalledWith("org-1", undefined, "alice@example.com");
  });

  it("renders invitation code and groups in the detail view", async () => {
    render(<InvitationDetailPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByText("code-123")).toBeInTheDocument();
    expect(screen.getByText("group-1")).toBeInTheDocument();
    expect(screen.getByText("invitation.acceptedAt")).toBeInTheDocument();
  });

  // INC-FE-03 回归：详情 revoke 失败必须给出可见错误提示，不得静默。
  it("surfaces an error message when revoke fails", async () => {
    mockRevokeInvitation.mockRejectedValue(new Error("boom"));
    const user = userEvent.setup();
    render(<InvitationDetailPage />, { wrapper: createTestWrapper() });

    await user.click(await screen.findByRole("button", { name: "invitation.revoke" }));
    await user.click(await screen.findByRole("button", { name: "confirm" }));

    expect(await screen.findByText("boom")).toBeInTheDocument();
  });
});
