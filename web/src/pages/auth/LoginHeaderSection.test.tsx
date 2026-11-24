import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { LoginHeaderSection } from "./LoginHeaderSection";

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

vi.mock("@/shared/lib/auth-signin-items", () => ({
  isSigninItemShown: (item: { visible?: boolean }) => item.visible !== false,
  isCustomSigninItem: () => false,
}));

vi.mock("./LoginAuxiliarySigninItems", () => ({
  HeaderSigninItem: ({ itemKey, item }: { itemKey: string; item: { name: string } }) => (
    <div data-testid="header-signin-item" data-key={itemKey} data-name={item.name}>
      Header Item: {item.name}
    </div>
  ),
}));

// ─────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────

describe("LoginHeaderSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("renders header section", () => {
    it("renders with empty header items", () => {
      render(
        <LoginHeaderSection
          availableLanguages={["en"]}
          headerSigninItems={[]}
          logoAlt="Logo"
          onBack={vi.fn<(...args: unknown[]) => unknown>()}
          onChangeLanguage={vi.fn<(...args: unknown[]) => unknown>()}
          preferredLanguage="en"
          renderSigninItemCss={() => null}
          resolvedApplicationName="app"
        />,
      );
      // antd 6: Space returns null when it has zero children, so an empty
      // headerSigninItems list renders no container at all.
      expect(document.querySelector(".ant-space")).toBeNull();
    });

    it("renders header signin items", () => {
      render(
        <LoginHeaderSection
          availableLanguages={["en"]}
          headerSigninItems={[
            { name: "Logo", visible: true },
            { name: "Language", visible: true },
          ]}
          logoAlt="Logo"
          onBack={vi.fn<(...args: unknown[]) => unknown>()}
          onChangeLanguage={vi.fn<(...args: unknown[]) => unknown>()}
          preferredLanguage="en"
          renderSigninItemCss={() => null}
          resolvedApplicationName="app"
        />,
      );
      const items = screen.getAllByTestId("header-signin-item");
      expect(items.length).toBe(2);
    });

    it("does not render hidden header items", () => {
      render(
        <LoginHeaderSection
          availableLanguages={["en"]}
          headerSigninItems={[
            { name: "Logo", visible: true },
            { name: "Language", visible: false },
          ]}
          logoAlt="Logo"
          onBack={vi.fn<(...args: unknown[]) => unknown>()}
          onChangeLanguage={vi.fn<(...args: unknown[]) => unknown>()}
          preferredLanguage="en"
          renderSigninItemCss={() => null}
          resolvedApplicationName="app"
        />,
      );
      const items = screen.getAllByTestId("header-signin-item");
      expect(items.length).toBe(1);
    });

    it("passes correct props to HeaderSigninItem", () => {
      render(
        <LoginHeaderSection
          availableLanguages={["en", "zh"]}
          headerSigninItems={[{ name: "Logo", visible: true }]}
          logoAlt="App Logo"
          logoUrl="/logo.png"
          onBack={vi.fn<(...args: unknown[]) => unknown>()}
          onChangeLanguage={vi.fn<(...args: unknown[]) => unknown>()}
          preferredLanguage="en"
          renderSigninItemCss={() => null}
          resolvedApplicationName="myapp"
          resolvedApplicationOrganization="myorg"
        />,
      );
      const item = screen.getByTestId("header-signin-item");
      expect(item).toHaveAttribute("data-name", "Logo");
    });
  });

  describe("item key generation", () => {
    it("generates unique keys for items", () => {
      render(
        <LoginHeaderSection
          availableLanguages={["en"]}
          headerSigninItems={[
            { name: "Logo", visible: true },
            { name: "Back button", visible: true },
          ]}
          logoAlt="Logo"
          onBack={vi.fn<(...args: unknown[]) => unknown>()}
          onChangeLanguage={vi.fn<(...args: unknown[]) => unknown>()}
          preferredLanguage="en"
          renderSigninItemCss={() => null}
          resolvedApplicationName="app"
          resolvedApplicationOrganization="org"
        />,
      );
      const items = screen.getAllByTestId("header-signin-item");
      const key1 = items[0].getAttribute("data-key");
      const key2 = items[1].getAttribute("data-key");
      expect(key1).not.toBe(key2);
    });
  });

  describe("edge cases", () => {
    it("handles undefined organization", () => {
      render(
        <LoginHeaderSection
          availableLanguages={["en"]}
          headerSigninItems={[{ name: "Logo", visible: true }]}
          logoAlt="Logo"
          onBack={vi.fn<(...args: unknown[]) => unknown>()}
          onChangeLanguage={vi.fn<(...args: unknown[]) => unknown>()}
          preferredLanguage="en"
          renderSigninItemCss={() => null}
          resolvedApplicationName="app"
          resolvedApplicationOrganization={undefined}
        />,
      );
      expect(screen.getByTestId("header-signin-item")).toBeInTheDocument();
    });

    it("handles multiple languages", () => {
      render(
        <LoginHeaderSection
          availableLanguages={["en", "zh", "ja", "ko"]}
          headerSigninItems={[]}
          logoAlt="Logo"
          onBack={vi.fn<(...args: unknown[]) => unknown>()}
          onChangeLanguage={vi.fn<(...args: unknown[]) => unknown>()}
          preferredLanguage="en"
          renderSigninItemCss={() => null}
          resolvedApplicationName="app"
        />,
      );
      // Empty headerSigninItems: antd 6 Space renders nothing (no children).
      expect(document.querySelector(".ant-space")).toBeNull();
    });
  });
});
