import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { ThemeTokenEditor } from "./ThemeTokenEditor";

// Mock react-i18next
vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: "en" },
  }),
  Trans: ({ children }: { children: React.ReactNode }) => children,
}));

describe("ThemeTokenEditor", () => {
  it("renders token fields from default configuration", () => {
    render(<ThemeTokenEditor />);
    expect(screen.getByText("organization.themeTokens.colorPrimary")).toBeInTheDocument();
    expect(screen.getByText("organization.themeTokens.colorInfo")).toBeInTheDocument();
    expect(screen.getByText("organization.themeTokens.colorSuccess")).toBeInTheDocument();
    expect(screen.getByText("organization.themeTokens.colorWarning")).toBeInTheDocument();
    expect(screen.getByText("organization.themeTokens.colorError")).toBeInTheDocument();
  });

  it("renders number fields", () => {
    render(<ThemeTokenEditor />);
    expect(screen.getByText("organization.themeTokens.borderRadius")).toBeInTheDocument();
    expect(screen.getByText("organization.themeTokens.fontSize")).toBeInTheDocument();
  });

  it("renders switch fields", () => {
    render(<ThemeTokenEditor />);
    expect(screen.getByText("organization.themeTokens.wireframe")).toBeInTheDocument();
  });

  it("shows parse error for invalid JSON", () => {
    render(<ThemeTokenEditor value="not-valid-json" />);
    expect(screen.getByText("theme.tokenParseError")).toBeInTheDocument();
  });

  it("shows parse error for JSON array", () => {
    render(<ThemeTokenEditor value="[1,2,3]" />);
    expect(screen.getByText("theme.tokenParseError")).toBeInTheDocument();
  });

  it("renders without error for valid JSON", () => {
    render(<ThemeTokenEditor value='{"colorPrimary":"#1677ff"}' />);
    expect(screen.queryByText("theme.tokenParseError")).not.toBeInTheDocument();
  });

  it("renders without error for empty value", () => {
    render(<ThemeTokenEditor value="" />);
    expect(screen.queryByText("theme.tokenParseError")).not.toBeInTheDocument();
  });

  it("renders without error for undefined value", () => {
    render(<ThemeTokenEditor value={undefined} />);
    expect(screen.queryByText("theme.tokenParseError")).not.toBeInTheDocument();
  });

  it("renders all color fields", () => {
    render(<ThemeTokenEditor />);
    expect(screen.getByText("organization.themeTokens.colorBgBase")).toBeInTheDocument();
    expect(screen.getByText("organization.themeTokens.colorTextBase")).toBeInTheDocument();
    expect(screen.getByText("organization.themeTokens.colorBgContainer")).toBeInTheDocument();
    expect(screen.getByText("organization.themeTokens.colorBorder")).toBeInTheDocument();
  });
});
