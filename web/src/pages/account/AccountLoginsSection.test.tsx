import { describe, it, expect, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { UseMutationResult } from "@tanstack/react-query";

import { AccountLoginsSection } from "./AccountLoginsSection";
import type { UserIdentity } from "@/shared/api/iam/user-types";
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

const mockIdentities: UserIdentity[] = [
  {
    id: "id-1",
    providerId: "provider-google",
    identityType: "OIDC",
    subject: "google-12345",
    providerAccount: "user@gmail.com",
    linkedAt: "2024-01-15T00:00:00Z",
  },
  {
    id: "id-2",
    providerId: "provider-github",
    identityType: "OIDC",
    subject: "github-67890",
    providerAccount: "testuser",
    linkedAt: "2024-02-20T00:00:00Z",
  },
];

const mockProviders: Array<{
  id?: string;
  name?: string;
  displayName?: string;
  category?: string;
  type?: string;
  clientId?: string;
}> = [
  { id: "provider-google", name: "google", displayName: "Google" },
  { id: "provider-github", name: "github", displayName: "GitHub" },
];

// ─────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────

describe("AccountLoginsSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("renders with no identities", () => {
    it("shows no linked providers alert", () => {
      render(<AccountLoginsSection identities={[]} providers={undefined} />, {
        wrapper: createWrapper(),
      });
      expect(screen.getByText("account.noLinkedProviders")).toBeInTheDocument();
    });

    it("shows no linked providers description", () => {
      render(<AccountLoginsSection identities={[]} providers={undefined} />, {
        wrapper: createWrapper(),
      });
      expect(screen.getByText("account.noLinkedProvidersDesc")).toBeInTheDocument();
    });

    it("shows linked providers note", () => {
      render(<AccountLoginsSection identities={[]} providers={undefined} />, {
        wrapper: createWrapper(),
      });
      expect(screen.getByText("account.linkedProvidersNote")).toBeInTheDocument();
    });
  });

  describe("renders with identities", () => {
    it("renders table with identities", () => {
      render(<AccountLoginsSection identities={mockIdentities} providers={mockProviders} />, {
        wrapper: createWrapper(),
      });
      expect(screen.getByText("providerName")).toBeInTheDocument();
      expect(screen.getByText("user.identityType")).toBeInTheDocument();
      expect(screen.getByText("user.providerAccount")).toBeInTheDocument();
      expect(screen.getByText("user.linkedAt")).toBeInTheDocument();
    });

    it("renders identity type tags", () => {
      render(<AccountLoginsSection identities={mockIdentities} providers={mockProviders} />, {
        wrapper: createWrapper(),
      });
      const oidcTags = screen.getAllByText("OIDC");
      expect(oidcTags.length).toBe(2);
    });

    it("renders provider account info", () => {
      render(<AccountLoginsSection identities={mockIdentities} providers={mockProviders} />, {
        wrapper: createWrapper(),
      });
      expect(screen.getByText("user@gmail.com")).toBeInTheDocument();
      expect(screen.getByText("testuser")).toBeInTheDocument();
    });

    it("shows linked providers note", () => {
      render(<AccountLoginsSection identities={mockIdentities} providers={mockProviders} />, {
        wrapper: createWrapper(),
      });
      expect(screen.getByText("account.linkedProvidersNote")).toBeInTheDocument();
    });
  });

  describe("provider display name resolution", () => {
    it("shows provider displayName when available", () => {
      render(<AccountLoginsSection identities={mockIdentities} providers={mockProviders} />, {
        wrapper: createWrapper(),
      });
      expect(screen.getByText("Google")).toBeInTheDocument();
      expect(screen.getByText("GitHub")).toBeInTheDocument();
    });

    it("falls back to providerId when provider not found", () => {
      render(<AccountLoginsSection identities={mockIdentities} providers={undefined} />, {
        wrapper: createWrapper(),
      });
      expect(screen.getByText("provider-google")).toBeInTheDocument();
      expect(screen.getByText("provider-github")).toBeInTheDocument();
    });

    it("falls back to provider name when displayName is null", () => {
      const providersWithNullName: Array<{
        id?: string;
        name?: string;
        displayName?: string | null;
        category?: string;
        type?: string;
        clientId?: string;
      }> = [{ id: "provider-google", name: "google", displayName: null }];
      render(
        <AccountLoginsSection
          identities={[mockIdentities[0]]}
          providers={
            providersWithNullName as Array<{
              id?: string;
              name?: string;
              displayName?: string;
              category?: string;
              type?: string;
              clientId?: string;
            }>
          }
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.getByText("google")).toBeInTheDocument();
    });
  });

  describe("edge cases", () => {
    it("renders dash for missing provider account", () => {
      const identityNoAccount: UserIdentity[] = [
        {
          id: "id-3",
          providerId: "provider-test",
          identityType: "SAML",
          subject: "saml-user",
          providerAccount: "",
          linkedAt: null,
        },
      ];
      render(<AccountLoginsSection identities={identityNoAccount} providers={undefined} />, {
        wrapper: createWrapper(),
      });
      // Empty string should render as "-"
      const dashElements = screen.getAllByText("-");
      expect(dashElements.length).toBeGreaterThanOrEqual(1);
    });

    it("renders single identity", () => {
      render(<AccountLoginsSection identities={[mockIdentities[0]]} providers={mockProviders} />, {
        wrapper: createWrapper(),
      });
      expect(screen.getByText("Google")).toBeInTheDocument();
    });
  });

  describe("self-service unlink", () => {
    function makeMutation(mutate: (identityId: string) => void) {
      return { mutate, isPending: false } as unknown as UseMutationResult<void, Error, string>;
    }

    it("shows an unlink button per identity when the mutation is provided", () => {
      const mutate = vi.fn<(identityId: string) => void>();
      render(
        <AccountLoginsSection
          identities={mockIdentities}
          providers={mockProviders}
          deleteIdentityMutation={makeMutation(mutate)}
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.getAllByRole("button", { name: "user.unlink" })).toHaveLength(2);
    });

    it("calls the mutation with the identity id after confirming", async () => {
      const mutate = vi.fn<(identityId: string) => void>();
      render(
        <AccountLoginsSection
          identities={[mockIdentities[0]]}
          providers={mockProviders}
          deleteIdentityMutation={makeMutation(mutate)}
        />,
        { wrapper: createWrapper() },
      );

      fireEvent.click(screen.getByRole("button", { name: "user.unlink" }));

      // Popconfirm 的确认按钮与触发按钮同名，确认后追加到 portal 中。
      await waitFor(() => {
        expect(screen.getAllByRole("button", { name: "user.unlink" }).length).toBeGreaterThan(1);
      });
      const buttons = screen.getAllByRole("button", { name: "user.unlink" });
      fireEvent.click(buttons[buttons.length - 1]);

      expect(mutate).toHaveBeenCalledWith("id-1");
    });

    it("disables unlink when the identity has no persisted id", () => {
      const mutate = vi.fn<(identityId: string) => void>();
      const noId: UserIdentity[] = [{ ...mockIdentities[0], id: undefined }];
      render(
        <AccountLoginsSection
          identities={noId}
          providers={mockProviders}
          deleteIdentityMutation={makeMutation(mutate)}
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.getByRole("button", { name: "user.unlink" })).toBeDisabled();
    });

    it("does not render unlink buttons without the mutation", () => {
      render(<AccountLoginsSection identities={mockIdentities} providers={mockProviders} />, {
        wrapper: createWrapper(),
      });
      expect(screen.queryByRole("button", { name: "user.unlink" })).not.toBeInTheDocument();
    });
  });

  describe("self-service link", () => {
    const loginProviders = [
      { id: "provider-google", name: "google", displayName: "Google", category: "oidc" },
      { id: "provider-github", name: "github", displayName: "GitHub", category: "oauth" },
      { id: "provider-ldap", name: "ldap", displayName: "LDAP", category: "ldap" },
    ];

    it("lists oauth/oidc/saml providers with a link button for unlinked ones", () => {
      const onLinkIdentity = vi.fn<(providerId: string) => void>();
      render(
        <AccountLoginsSection
          identities={[mockIdentities[0]]}
          providers={loginProviders}
          onLinkIdentity={onLinkIdentity}
        />,
        { wrapper: createWrapper() },
      );
      // GitHub 未绑定 → 绑定按钮；LDAP 非登录类 provider → 不展示
      expect(screen.getByRole("button", { name: /user\.link$/ })).toBeInTheDocument();
      expect(screen.queryByText("LDAP")).not.toBeInTheDocument();
    });

    it("calls onLinkIdentity with the provider id", () => {
      const onLinkIdentity = vi.fn<(providerId: string) => void>();
      render(
        <AccountLoginsSection
          identities={[mockIdentities[0]]}
          providers={loginProviders}
          onLinkIdentity={onLinkIdentity}
        />,
        { wrapper: createWrapper() },
      );
      fireEvent.click(screen.getByRole("button", { name: /user\.link$/ }));
      expect(onLinkIdentity).toHaveBeenCalledWith("provider-github");
    });

    it("shows unlink instead of link for an already linked provider", () => {
      const mutate = vi.fn<(identityId: string) => void>();
      const onLinkIdentity = vi.fn<(providerId: string) => void>();
      render(
        <AccountLoginsSection
          identities={[mockIdentities[0]]}
          providers={[loginProviders[0]]}
          deleteIdentityMutation={
            { mutate, isPending: false } as unknown as UseMutationResult<void, Error, string>
          }
          onLinkIdentity={onLinkIdentity}
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.queryByRole("button", { name: /user\.link$/ })).not.toBeInTheDocument();
      // 表格行 + provider 行各一个解绑入口
      expect(screen.getAllByRole("button", { name: "user.unlink" })).toHaveLength(2);
    });
  });
});
