import { useState } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  ServiceAccountCredentials,
  ServiceAccountDeletePrecheck,
  ServiceAccountItem,
} from "@/shared/api/iam/identity-pools";
import { createTestWrapper } from "@/test-utils/render";
import { ServiceAccountFormModal } from "./ServiceAccountFormModal";
import { SecretModal } from "./SecretModal";
import { AccountDeleteModal } from "./AccountDeleteModal";

const mockCreateMutateAsync = vi.fn<(...args: unknown[]) => unknown>();
const mockCreateReset = vi.fn<(...args: unknown[]) => unknown>();
const mockStatusMutateAsync = vi.fn<(...args: unknown[]) => unknown>();
const mockDeleteMutateAsync = vi.fn<(...args: unknown[]) => unknown>();
const mockPrecheckRefetch = vi.fn<(...args: unknown[]) => unknown>();
const mockUsePrecheck = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, options?: { defaultValue?: string }) => options?.defaultValue ?? key,
    i18n: { language: "en" },
  }),
}));

vi.mock("@/shared/hooks/use-identity-pools", () => ({
  useCreateServiceAccount: () => ({
    mutateAsync: mockCreateMutateAsync,
    reset: mockCreateReset,
    isPending: false,
  }),
  useSetServiceAccountStatus: () => ({
    mutateAsync: mockStatusMutateAsync,
    isPending: false,
  }),
  useDeleteServiceAccount: () => ({
    mutateAsync: mockDeleteMutateAsync,
    isPending: false,
  }),
  useServiceAccountDeletePrecheck: (...args: unknown[]) => mockUsePrecheck(...args),
}));

const account: ServiceAccountItem = {
  id: "account-1",
  organizationId: "org-1",
  poolId: "pool-1",
  name: "deploy-runner",
  description: "",
  clientId: "gl_cid_test",
  clientSecretPlain: "gl_cs_one_time",
  scopes: ["workload:deploy"],
  accessTokenTtlSeconds: 900,
  lastRotatedAt: null,
  status: "active",
  secretSet: true,
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
  revision: 3,
};

function precheck(
  overrides: Partial<ServiceAccountDeletePrecheck> = {},
): ServiceAccountDeletePrecheck {
  return {
    id: account.id,
    organizationId: account.organizationId,
    poolId: account.poolId,
    activeTokenCount: 2,
    scopes: account.scopes,
    status: "active",
    canDelete: false,
    revision: 3,
    ...overrides,
  };
}

function precheckQuery(value: ServiceAccountDeletePrecheck | undefined) {
  return {
    data: value,
    isLoading: false,
    isFetching: false,
    isError: false,
    error: null,
    refetch: mockPrecheckRefetch,
  };
}

function SecretHarness() {
  const [credentials, setCredentials] = useState<ServiceAccountCredentials | null>({
    clientId: "gl_cid_test",
    clientSecretPlain: "gl_cs_one_time",
  });
  return <SecretModal credentials={credentials} onClose={() => setCredentials(null)} />;
}

describe("service account one-time secret", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCreateMutateAsync.mockResolvedValue(account);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText: vi.fn<(...args: unknown[]) => unknown>().mockResolvedValue(undefined) },
    });
  });

  it("returns only the one-time credentials from creation and clears mutation data", async () => {
    const user = userEvent.setup();
    const onCreated = vi.fn<(...args: unknown[]) => unknown>();
    const onClose = vi.fn<(...args: unknown[]) => unknown>();
    render(
      <ServiceAccountFormModal open poolId="pool-1" onClose={onClose} onCreated={onCreated} />,
      { wrapper: createTestWrapper() },
    );

    await user.type(screen.getByLabelText("Name"), "deploy-runner");
    await user.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() =>
      expect(onCreated).toHaveBeenCalledWith({
        clientId: "gl_cid_test",
        clientSecretPlain: "gl_cs_one_time",
      }),
    );
    expect(mockCreateMutateAsync).toHaveBeenCalledWith(
      expect.objectContaining({
        name: "deploy-runner",
        scopes: [],
        accessTokenTtlSeconds: 900,
      }),
    );
    expect(mockCreateReset).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("shows and copies the secret once, then removes it when the dialog closes", async () => {
    const user = userEvent.setup();
    const writeText = vi.fn<(...args: unknown[]) => unknown>().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    render(<SecretHarness />, { wrapper: createTestWrapper() });

    expect(screen.getByText("gl_cid_test")).toBeInTheDocument();
    expect(screen.getByText("gl_cs_one_time")).toBeInTheDocument();
    expect(screen.getByText(/shown only once/)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Copy Client Secret" }));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith("gl_cs_one_time"));

    await user.click(screen.getByRole("button", { name: "I saved the credentials" }));
    await waitFor(() => expect(screen.queryByText("gl_cs_one_time")).not.toBeInTheDocument());
  });
});

describe("AccountDeleteModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUsePrecheck.mockReturnValue(precheckQuery(precheck()));
    mockStatusMutateAsync.mockResolvedValue({ ...account, status: "disabled", revision: 4 });
    mockDeleteMutateAsync.mockResolvedValue(undefined);
    mockPrecheckRefetch.mockResolvedValue({ data: precheck() });
  });

  it("blocks deletion with active tokens, disables, refreshes, and uses the new revision", async () => {
    const user = userEvent.setup();
    const onDeleted = vi.fn<(...args: unknown[]) => unknown>();
    const view = render(
      <AccountDeleteModal
        open
        poolId="pool-1"
        account={account}
        onClose={vi.fn<(...args: unknown[]) => unknown>()}
        onDeleted={onDeleted}
      />,
      { wrapper: createTestWrapper() },
    );

    expect(screen.getByText("2")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Delete" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Disable first" }));

    await waitFor(() =>
      expect(mockStatusMutateAsync).toHaveBeenCalledWith({
        id: "account-1",
        status: "disabled",
        revision: 3,
      }),
    );
    expect(mockPrecheckRefetch).toHaveBeenCalledTimes(1);

    mockUsePrecheck.mockReturnValue(
      precheckQuery(
        precheck({ activeTokenCount: 0, status: "disabled", canDelete: true, revision: 4 }),
      ),
    );
    view.rerender(
      <AccountDeleteModal
        open
        poolId="pool-1"
        account={account}
        onClose={vi.fn<(...args: unknown[]) => unknown>()}
        onDeleted={onDeleted}
      />,
    );

    const deleteButton = screen.getByRole("button", { name: "Delete" });
    expect(deleteButton).toBeEnabled();
    await user.click(deleteButton);

    await waitFor(() =>
      expect(mockDeleteMutateAsync).toHaveBeenCalledWith({ id: "account-1", revision: 4 }),
    );
    expect(onDeleted).toHaveBeenCalledTimes(1);
  });

  it("allows retrying a failed precheck without enabling deletion", async () => {
    const user = userEvent.setup();
    mockUsePrecheck.mockReturnValue({
      ...precheckQuery(undefined),
      isError: true,
      error: new Error("precheck unavailable"),
    });
    render(
      <AccountDeleteModal
        open
        poolId="pool-1"
        account={account}
        onClose={vi.fn<(...args: unknown[]) => unknown>()}
        onDeleted={vi.fn<(...args: unknown[]) => unknown>()}
      />,
      { wrapper: createTestWrapper() },
    );

    expect(screen.getByText("precheck unavailable")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Delete" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(mockPrecheckRefetch).toHaveBeenCalledTimes(1);
  });
});
