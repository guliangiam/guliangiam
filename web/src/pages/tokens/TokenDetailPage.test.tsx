import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HttpError } from "@/shared/api/http";
import { createTestWrapper } from "@/test-utils/render";
import TokenDetailPage from "./TokenDetailPage";

const mockUseTokenDetailPageController = vi.fn<(...args: unknown[]) => unknown>();
const mockHasAction = vi.fn<(...args: unknown[]) => unknown>();
const mockHasResource = vi.fn<(...args: unknown[]) => unknown>();
const mockUseParams = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("./useTokenDetailPageController", () => ({
  useTokenDetailPageController: () => mockUseTokenDetailPageController(),
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
  useParams: () => mockUseParams(),
  Link: ({ children }: { children?: React.ReactNode }) => children ?? null,
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: "en" },
  }),
}));

vi.mock("@/features/navigation/AuditSectionNav", () => ({
  AuditSectionNav: () => null,
}));

vi.mock("./JwtDecodeView", () => ({
  default: () => null,
}));

const token = {
  id: "token-1",
  tokenPrefix: "glt_prefix",
  tokenHashSet: true,
  tokenHashMasked: "******",
  username: "alice",
  userId: "user-1",
  organizationId: "org-1",
  organizationSlug: "default",
  applicationId: "app-1",
  applicationSlug: "console",
  scopes: ["read"],
  audience: [],
  expiresAt: "2999-01-01T00:00:00Z",
  revokedAt: null,
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
  revision: 1,
  metadata: {},
};

function mockController(detailOverrides: Record<string, unknown> = {}) {
  const detailQuery = {
    isLoading: false,
    isError: false,
    error: null,
    data: token,
    refetch: vi.fn<() => void>(),
    ...detailOverrides,
  };

  mockUseTokenDetailPageController.mockReturnValue({
    tokenName: "token-1",
    isNew: false,
    orgNameMap: new Map(),
    form: {},
    createdToken: null,
    showFullTokenValue: false,
    setShowFullTokenValue: vi.fn<() => void>(),
    usersQuery: { isLoading: false, data: [] },
    applicationsQuery: { isLoading: false, data: [] },
    detailQuery,
    createMutation: { isPending: false, mutate: vi.fn<() => void>() },
    deleteMutation: { isPending: false, mutate: vi.fn<() => void>() },
    revokeMutation: { isPending: false, mutate: vi.fn<() => void>() },
  });

  return detailQuery;
}

describe("TokenDetailPage states", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseParams.mockReturnValue({ tokenName: "token-1" });
    mockHasAction.mockReturnValue(true);
    mockHasResource.mockReturnValue(true);
    mockController();
  });

  it("shows page-level 403 without loading token details when permission is denied", () => {
    mockHasAction.mockReturnValue(false);

    render(<TokenDetailPage />, { wrapper: createTestWrapper() });

    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.getByText("403")).toBeInTheDocument();
    expect(screen.getByText("common:forbidden")).toBeInTheDocument();
    expect(mockUseTokenDetailPageController).not.toHaveBeenCalled();
  });

  it("shows Skeleton without mixing in detail data while loading", () => {
    mockController({ isLoading: true, data: token });
    const { container } = render(<TokenDetailPage />, { wrapper: createTestWrapper() });

    expect(container.querySelector(".ant-skeleton")).toBeInTheDocument();
    expect(screen.queryByText("token-1")).not.toBeInTheDocument();
    expect(screen.queryByText("token.notFound")).not.toBeInTheDocument();
  });

  it("shows server errors separately from NotFound and retries the detail query", async () => {
    const user = userEvent.setup();
    const detailQuery = mockController({
      isError: true,
      error: new Error("backend unavailable"),
      data: undefined,
    });

    render(<TokenDetailPage />, { wrapper: createTestWrapper() });

    expect(screen.getByText("loadFailed")).toBeInTheDocument();
    expect(screen.queryByText("token.notFound")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "retry" }));
    expect(detailQuery.refetch).toHaveBeenCalledTimes(1);
  });

  it("shows an explicit NotFound result for missing data", () => {
    mockController({ data: undefined });

    render(<TokenDetailPage />, { wrapper: createTestWrapper() });

    expect(screen.getByText("token.notFound")).toBeInTheDocument();
    expect(screen.getByText("token.tokenNotExist")).toBeInTheDocument();
    expect(screen.queryByText("loadFailed")).not.toBeInTheDocument();
  });

  it("maps a 404 response to NotFound instead of retryable server error", () => {
    mockController({
      isError: true,
      error: new HttpError(404, null, "Missing"),
      data: undefined,
    });

    render(<TokenDetailPage />, { wrapper: createTestWrapper() });

    expect(screen.getByText("token.notFound")).toBeInTheDocument();
    expect(screen.queryByText("loadFailed")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "retry" })).not.toBeInTheDocument();
  });

  it("renders detail data without Skeleton or NotFound", () => {
    const { container } = render(<TokenDetailPage />, { wrapper: createTestWrapper() });

    expect(screen.getByText("token-1")).toBeInTheDocument();
    expect(screen.getByText("alice")).toBeInTheDocument();
    expect(container.querySelector(".ant-skeleton")).not.toBeInTheDocument();
    expect(screen.queryByText("token.notFound")).not.toBeInTheDocument();
  });

  it("renders the masked token hash instead of a raw hash value", () => {
    render(<TokenDetailPage />, { wrapper: createTestWrapper() });

    expect(screen.getByText("token.hash")).toBeInTheDocument();
    expect(screen.getByText("******")).toBeInTheDocument();
    expect(screen.queryByText("hash-1")).not.toBeInTheDocument();
  });

  it("renders a placeholder when the token hash is not set", () => {
    mockController({ data: { ...token, tokenHashSet: false, tokenHashMasked: "" } });

    render(<TokenDetailPage />, { wrapper: createTestWrapper() });

    expect(screen.getByText("token.hash")).toBeInTheDocument();
    expect(screen.queryByText("******")).not.toBeInTheDocument();
  });
});
