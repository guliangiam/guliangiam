import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { CasbinPolicyEditor, type Policy } from "./CasbinPolicyEditor";

// Mock react-i18next
vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, options?: { defaultValue?: string }) => options?.defaultValue ?? key,
    i18n: { language: "en" },
  }),
  Trans: ({ children }: { children: React.ReactNode }) => children,
}));

const samplePolicies: Policy[] = [
  { ptype: "p", subject: "admin", object: "resource1", action: "read" },
  { ptype: "g", subject: "alice", object: "managers", action: "" },
];

describe("CasbinPolicyEditor", () => {
  it("renders policy rows in table", () => {
    render(
      <CasbinPolicyEditor
        policies={samplePolicies}
        onAdd={vi.fn<(...args: unknown[]) => unknown>()}
        onRemove={vi.fn<(...args: unknown[]) => unknown>()}
        onUpdate={vi.fn<(...args: unknown[]) => unknown>()}
      />,
    );

    // Use getAllByText for values that may appear multiple times
    expect(screen.getAllByText("admin").length).toBeGreaterThan(0);
    expect(screen.getByText("alice")).toBeInTheDocument();
    expect(screen.getByText("resource1")).toBeInTheDocument();
  });

  it("renders p type as Policy label", () => {
    render(
      <CasbinPolicyEditor
        policies={[samplePolicies[0]]}
        onAdd={vi.fn<(...args: unknown[]) => unknown>()}
        onRemove={vi.fn<(...args: unknown[]) => unknown>()}
        onUpdate={vi.fn<(...args: unknown[]) => unknown>()}
      />,
    );

    expect(screen.getByText("p (Policy)")).toBeInTheDocument();
  });

  it("renders g type as Grouping label", () => {
    render(
      <CasbinPolicyEditor
        policies={[samplePolicies[1]]}
        onAdd={vi.fn<(...args: unknown[]) => unknown>()}
        onRemove={vi.fn<(...args: unknown[]) => unknown>()}
        onUpdate={vi.fn<(...args: unknown[]) => unknown>()}
      />,
    );

    expect(screen.getByText("g (Grouping)")).toBeInTheDocument();
  });

  it("renders Add Policy button", () => {
    render(
      <CasbinPolicyEditor
        policies={[]}
        onAdd={vi.fn<(...args: unknown[]) => unknown>()}
        onRemove={vi.fn<(...args: unknown[]) => unknown>()}
        onUpdate={vi.fn<(...args: unknown[]) => unknown>()}
      />,
    );

    // Button text comes from i18n key "policy.addRow"
    expect(screen.getByText("policy.addRow")).toBeInTheDocument();
  });

  it("renders table headers", () => {
    render(
      <CasbinPolicyEditor
        policies={[]}
        onAdd={vi.fn<(...args: unknown[]) => unknown>()}
        onRemove={vi.fn<(...args: unknown[]) => unknown>()}
        onUpdate={vi.fn<(...args: unknown[]) => unknown>()}
      />,
    );

    expect(screen.getByText("Type")).toBeInTheDocument();
    expect(screen.getByText("Subject")).toBeInTheDocument();
    expect(screen.getByText("Object")).toBeInTheDocument();
    expect(screen.getByText("Action")).toBeInTheDocument();
  });

  it("renders action column values", () => {
    render(
      <CasbinPolicyEditor
        policies={samplePolicies}
        onAdd={vi.fn<(...args: unknown[]) => unknown>()}
        onRemove={vi.fn<(...args: unknown[]) => unknown>()}
        onUpdate={vi.fn<(...args: unknown[]) => unknown>()}
      />,
    );

    expect(screen.getByText("read")).toBeInTheDocument();
  });

  it("renders empty table with no policies", () => {
    const { container } = render(
      <CasbinPolicyEditor
        policies={[]}
        onAdd={vi.fn<(...args: unknown[]) => unknown>()}
        onRemove={vi.fn<(...args: unknown[]) => unknown>()}
        onUpdate={vi.fn<(...args: unknown[]) => unknown>()}
      />,
    );

    // Table exists but with no data rows
    expect(container.querySelector(".ant-table")).toBeInTheDocument();
    expect(screen.getByText("policy.addRow")).toBeInTheDocument();
  });

  it("renders with loading prop", () => {
    const { container } = render(
      <CasbinPolicyEditor
        policies={[]}
        onAdd={vi.fn<(...args: unknown[]) => unknown>()}
        onRemove={vi.fn<(...args: unknown[]) => unknown>()}
        onUpdate={vi.fn<(...args: unknown[]) => unknown>()}
        loading
      />,
    );

    // Loading state is applied via Table's loading prop (renders ant-spin or ant-skeleton)
    expect(
      container.querySelector(".ant-spin") || container.querySelector(".ant-skeleton"),
    ).toBeInTheDocument();
  });
});
