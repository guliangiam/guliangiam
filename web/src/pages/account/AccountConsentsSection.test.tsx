import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { UseQueryResult } from "@tanstack/react-query";

import { AccountConsentsSection } from "./AccountConsentsSection";
import type { UserConsentItem } from "@/shared/api/iam/user-types";
import { createTestWrapper } from "../../test-utils/render";

// ─────────────────────────────────────────────
// Mocks
// ─────────────────────────────────────────────

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: "en" },
  }),
  Trans: ({ children }: { children: React.ReactNode }) => children,
}));

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

const createWrapper = createTestWrapper;

const consents: UserConsentItem[] = [
  {
    id: "consent-1",
    applicationId: "app-1",
    applicationSlug: "portal",
    grantedScopes: ["openid", "profile"],
    createdAt: "2024-01-01T00:00:00Z",
  },
  {
    id: "consent-2",
    applicationId: "app-2",
    grantedScopes: ["email"],
    createdAt: "2024-02-01T00:00:00Z",
    revokedAt: "2024-03-01T00:00:00Z",
  },
];

function makeQuery(data: UserConsentItem[], isLoading = false) {
  return { data, isLoading } as unknown as UseQueryResult<UserConsentItem[], Error>;
}

// ─────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────

describe("AccountConsentsSection", () => {
  it("renders the section title and consent rows", () => {
    render(<AccountConsentsSection consentsQuery={makeQuery(consents)} />, {
      wrapper: createWrapper(),
    });
    expect(screen.getByText("user.tabs.consents")).toBeInTheDocument();
    expect(screen.getByText("portal")).toBeInTheDocument();
    // applicationSlug 缺失时回退 applicationId
    expect(screen.getByText("app-2")).toBeInTheDocument();
    expect(screen.getByText("openid")).toBeInTheDocument();
    expect(screen.getByText("profile")).toBeInTheDocument();
    expect(screen.getByText("email")).toBeInTheDocument();
  });

  it("renders active and revoked status tags", () => {
    render(<AccountConsentsSection consentsQuery={makeQuery(consents)} />, {
      wrapper: createWrapper(),
    });
    expect(screen.getByText("user.consents.active")).toBeInTheDocument();
    expect(screen.getByText("user.consents.revoked")).toBeInTheDocument();
  });

  it("is read-only: no revoke entry is rendered", () => {
    render(<AccountConsentsSection consentsQuery={makeQuery(consents)} />, {
      wrapper: createWrapper(),
    });
    expect(screen.queryByText("user.consents.revoke")).not.toBeInTheDocument();
    expect(screen.queryByText("user.consents.actions")).not.toBeInTheDocument();
  });

  it("shows an empty hint when there are no consents", () => {
    render(<AccountConsentsSection consentsQuery={makeQuery([])} />, {
      wrapper: createWrapper(),
    });
    expect(screen.getByText("noConsents")).toBeInTheDocument();
  });
});
