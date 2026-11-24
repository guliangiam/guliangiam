import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { createTestWrapper } from "../../test-utils/render";
import I18nLanguagesPage from "./I18nLanguagesPage";
import I18nTranslationsPage from "./I18nTranslationsPage";

// ─────────────────────────────────────────────
// Mocks
// ─────────────────────────────────────────────

const mockHasAction = vi.fn<(action: string) => boolean>();
const mockHasResource = vi.fn<(resource: string) => boolean>();

const mockIamGet = vi.fn<() => Promise<unknown>>();
const mockIamList = vi.fn<() => Promise<unknown>>();

vi.mock("@/shared/hooks/usePermissions", () => ({
  usePermissions: () => ({
    isLoading: false,
    hasAction: mockHasAction,
    hasResource: mockHasResource,
  }),
}));

vi.mock("@/shared/api/iam/i18n", () => ({
  iamGet: () => mockIamGet(),
  iamUpdate: vi.fn<() => Promise<unknown>>(),
  iamList: () => mockIamList(),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: unknown) => (typeof fallback === "string" ? fallback : key),
    i18n: { language: "en" },
  }),
  Trans: ({ children }: { children: React.ReactNode }) => children,
}));

function permissions(...actions: string[]) {
  return (action: string) => actions.includes(action);
}

// ─────────────────────────────────────────────
// Languages
// ─────────────────────────────────────────────

describe("I18nLanguagesPage permission guard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockHasResource.mockReturnValue(true);
    mockIamList.mockResolvedValue([{ code: "en", name: "English", nativeName: "English (US)" }]);
  });

  it("renders 403 without i18n:read", () => {
    mockHasAction.mockReturnValue(false);
    render(<I18nLanguagesPage />, { wrapper: createTestWrapper() });

    expect(screen.getByText("403")).toBeInTheDocument();
    expect(screen.queryByText("en")).not.toBeInTheDocument();
  });

  it("renders the language list with i18n:read", async () => {
    mockHasAction.mockImplementation(permissions("i18n:read"));
    render(<I18nLanguagesPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByText("English")).toBeInTheDocument();
    expect(screen.getByText("en")).toBeInTheDocument();
  });
});

// ─────────────────────────────────────────────
// Translations
// ─────────────────────────────────────────────

describe("I18nTranslationsPage permission guards", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockHasResource.mockReturnValue(true);
    mockIamGet.mockResolvedValue({ greeting: "Hello" });
  });

  it("renders 403 without i18n:read", () => {
    mockHasAction.mockReturnValue(false);
    render(<I18nTranslationsPage />, { wrapper: createTestWrapper() });

    expect(screen.getByText("403")).toBeInTheDocument();
    expect(screen.queryByText("Hello")).not.toBeInTheDocument();
  });

  it("shows translations without an edit affordance with read only", async () => {
    mockHasAction.mockImplementation(permissions("i18n:read"));
    render(<I18nTranslationsPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByText("Hello")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Hello" })).not.toBeInTheDocument();
  });

  it("makes the translation editable with read + write", async () => {
    const user = userEvent.setup();
    mockHasAction.mockImplementation(permissions("i18n:read", "i18n:write"));
    render(<I18nTranslationsPage />, { wrapper: createTestWrapper() });

    const trigger = await screen.findByRole("button", { name: "Hello" });
    await user.click(trigger);

    expect(screen.getByRole("button", { name: "Save" })).toBeInTheDocument();
  });
});
