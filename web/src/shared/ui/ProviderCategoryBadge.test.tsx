import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { ProviderCategoryBadge } from "./ProviderCategoryBadge";

vi.mock("@/shared/hooks/useProviderCategories", () => ({
  getCategoryColor: (category: string) => {
    const colors: Record<string, string> = {
      OAuth2: "blue",
      SAML: "green",
      OIDC: "purple",
      LDAP: "orange",
      Web3: "cyan",
    };
    return colors[category] ?? "default";
  },
}));

describe("ProviderCategoryBadge", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("renders category badge", () => {
    it("renders OAuth2 category", () => {
      render(<ProviderCategoryBadge category="OAuth2" />);
      expect(screen.getByText("OAuth2")).toBeInTheDocument();
    });

    it("renders SAML category", () => {
      render(<ProviderCategoryBadge category="SAML" />);
      expect(screen.getByText("SAML")).toBeInTheDocument();
    });

    it("renders OIDC category", () => {
      render(<ProviderCategoryBadge category="OIDC" />);
      expect(screen.getByText("OIDC")).toBeInTheDocument();
    });

    it("renders LDAP category", () => {
      render(<ProviderCategoryBadge category="LDAP" />);
      expect(screen.getByText("LDAP")).toBeInTheDocument();
    });

    it("renders Web3 category", () => {
      render(<ProviderCategoryBadge category="Web3" />);
      expect(screen.getByText("Web3")).toBeInTheDocument();
    });

    it("renders unknown category with default color", () => {
      render(<ProviderCategoryBadge category="Unknown" />);
      expect(screen.getByText("Unknown")).toBeInTheDocument();
    });
  });

  describe("edge cases", () => {
    it("renders dash when category is empty string", () => {
      const { container } = render(<ProviderCategoryBadge category="" />);
      expect(container.textContent).toBe("-");
    });

    it("renders dash when category is not provided", () => {
      const { container } = render(
        <ProviderCategoryBadge category={undefined as unknown as string} />,
      );
      expect(container.textContent).toBe("-");
    });
  });

  describe("tag color", () => {
    it("renders with correct color for OAuth2", () => {
      render(<ProviderCategoryBadge category="OAuth2" />);
      const tag = screen.getByText("OAuth2").closest(".ant-tag");
      expect(tag).toBeInTheDocument();
    });

    it("renders with correct color for SAML", () => {
      render(<ProviderCategoryBadge category="SAML" />);
      const tag = screen.getByText("SAML").closest(".ant-tag");
      expect(tag).toBeInTheDocument();
    });
  });
});
