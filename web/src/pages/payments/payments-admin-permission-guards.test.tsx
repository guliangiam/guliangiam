import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { createTestWrapper } from "../../test-utils/render";
import { useSessionStore } from "@/features/session/session-store";
import PaymentsReconPage from "./PaymentsReconPage";
import PaymentsChannelConfigsPage from "./PaymentsChannelConfigsPage";

// ─────────────────────────────────────────────
// Mocks
// ─────────────────────────────────────────────

const mockHasAction = vi.fn<(action: string) => boolean>();
const mockHasResource = vi.fn<(resource: string) => boolean>();

const mockListReconBatches = vi.fn<() => Promise<unknown>>();
const mockListReconItems = vi.fn<() => Promise<unknown>>();
const mockListChannelConfigs = vi.fn<() => Promise<unknown>>();

vi.mock("@/shared/hooks/usePermissions", () => ({
  usePermissions: () => ({
    isLoading: false,
    hasAction: mockHasAction,
    hasResource: mockHasResource,
  }),
}));

vi.mock("@/shared/api/payments-admin", () => ({
  RECON_BATCH_STATUS: [
    { value: "pending", label: "pending", color: "default" },
    { value: "running", label: "running", color: "blue" },
    { value: "completed", label: "completed", color: "green" },
    { value: "failed", label: "failed", color: "red" },
  ],
  RECON_DIFF_TYPE: [
    { value: "local_only", label: "local_only", color: "gold" },
    { value: "channel_only", label: "channel_only", color: "purple" },
    { value: "amount_mismatch", label: "amount_mismatch", color: "volcano" },
    { value: "state_mismatch", label: "state_mismatch", color: "orange" },
    { value: "channel_unknown", label: "channel_unknown", color: "default" },
  ],
  CHANNEL_STATUS_OPTIONS: [
    { value: "draft", label: "Draft", color: "default" },
    { value: "testing", label: "Testing", color: "gold" },
    { value: "active", label: "Active", color: "green" },
    { value: "disabled", label: "Disabled", color: "red" },
  ],
  ENVIRONMENT_OPTIONS: [
    { value: "sandbox", label: "Sandbox", color: "gold" },
    { value: "production", label: "Production", color: "blue" },
  ],
  listReconBatches: () => mockListReconBatches(),
  listReconItems: () => mockListReconItems(),
  runRecon: vi.fn<() => Promise<unknown>>(),
  resolveReconItem: vi.fn<() => Promise<unknown>>(),
  listChannelConfigs: () => mockListChannelConfigs(),
  getChannelConfig: vi.fn<() => Promise<unknown>>(),
  createChannelConfig: vi.fn<() => Promise<unknown>>(),
  transitionChannelConfig: vi.fn<() => Promise<unknown>>(),
}));

vi.mock("./usePayChannels", () => ({
  usePayChannels: () => ({ channels: [], options: [], labels: {}, isLoading: false }),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => {
    const t = (key: string, fallback?: unknown) => (typeof fallback === "string" ? fallback : key);
    const i18n = { language: "en" };
    // i18next returns a tuple-like array that also carries t/i18n properties;
    // recon and channel-config pages destructure the tuple, so both shapes
    // must be available here.
    return Object.assign([t, i18n], { t, i18n });
  },
  Trans: ({ children }: { children: React.ReactNode }) => children,
}));

// ─────────────────────────────────────────────
// Fixtures
// ─────────────────────────────────────────────

const now = "2026-11-03T00:00:00Z";

const batch = {
  id: "batch-1",
  organizationId: "org-1",
  channel: "alipay",
  batchDate: "2026-11-01",
  status: "completed",
  diffCount: 1,
  totalLocal: 10,
  totalChannel: 9,
  createdAt: now,
  updatedAt: now,
};

const item = {
  id: "item-1",
  batchId: batch.id,
  diffType: "amount_mismatch",
  channelOrderNo: "CH-1",
  localPaymentId: "PAY-1",
  channelAmount: 9,
  localAmount: 10,
  channelState: "paid",
  localState: "paid",
  resolved: false,
  createdAt: now,
};

const channelConfig = {
  id: "config-1",
  organizationId: "org-1",
  channel: "alipay",
  environment: "sandbox",
  configFingerprint: "fp-1",
  configSet: [],
  versionNo: 1,
  status: "draft",
  createdAt: now,
  updatedAt: now,
  revision: 1,
};

function setSession() {
  useSessionStore.setState({
    account: { owner: "built-in", name: "admin", isAdmin: false },
    organization: {
      id: "org-1",
      name: "built-in",
      displayName: "Built-in",
      navItems: [],
      userNavItems: [],
    },
    impersonatedUser: null,
  });
}

// ─────────────────────────────────────────────
// Recon
// ─────────────────────────────────────────────

describe("PaymentsReconPage permission guards", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setSession();
    mockHasResource.mockReturnValue(true);
    mockListReconBatches.mockResolvedValue({
      items: [batch],
      page: { page: 1, pageSize: 100, total: 1 },
    });
    mockListReconItems.mockResolvedValue([item]);
  });

  it("hides run and resolve without the reconciliation write actions", async () => {
    mockHasAction.mockReturnValue(false);
    render(<PaymentsReconPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByText(batch.batchDate)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Run Reconciliation/ })).not.toBeInTheDocument();
  });

  it("shows run with pay-recon:write only", async () => {
    mockHasAction.mockImplementation((action) => action === "pay-recon:write");
    render(<PaymentsReconPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByText(batch.batchDate)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Run Reconciliation/ })).toBeInTheDocument();
  });

  it("shows resolve with pay-recon-items:write only", async () => {
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    mockHasAction.mockImplementation((action) => action === "pay-recon-items:write");
    render(<PaymentsReconPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByText(batch.batchDate)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Run Reconciliation/ })).not.toBeInTheDocument();

    const batchTable = screen.getAllByRole("table")[0];
    await user.click(within(batchTable).getAllByRole("radio")[0]);
    expect(await screen.findByRole("button", { name: "Resolve" })).toBeInTheDocument();
  });
});

// ─────────────────────────────────────────────
// Channel configs
// ─────────────────────────────────────────────

describe("PaymentsChannelConfigsPage permission guards", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setSession();
    mockHasResource.mockReturnValue(true);
    mockListChannelConfigs.mockResolvedValue({
      items: [channelConfig],
      total: 1,
      page: 1,
      pageSize: 50,
    });
  });

  it("renders 403 without pay-channel-configs:read", () => {
    mockHasAction.mockReturnValue(false);
    render(<PaymentsChannelConfigsPage />, { wrapper: createTestWrapper() });

    expect(screen.getByText("403")).toBeInTheDocument();
    expect(screen.queryByText("Channel Configs")).not.toBeInTheDocument();
  });

  it("shows the list but no write actions with read only", async () => {
    mockHasAction.mockImplementation((action) => action === "pay-channel-configs:read");
    render(<PaymentsChannelConfigsPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByText("Channel Configs")).toBeInTheDocument();
    expect(await screen.findByText("fp-1")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /New Channel Config/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Transition" })).not.toBeInTheDocument();
  });

  it("shows create and transition with read + write", async () => {
    mockHasAction.mockImplementation(
      (action) => action === "pay-channel-configs:read" || action === "pay-channel-configs:write",
    );
    render(<PaymentsChannelConfigsPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByRole("button", { name: /New Channel Config/ })).toBeInTheDocument();
    expect(await screen.findByRole("button", { name: "Transition" })).toBeInTheDocument();
  });
});
