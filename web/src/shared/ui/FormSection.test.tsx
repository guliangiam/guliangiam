import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FormSection } from "./FormSection";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

describe("FormSection", () => {
  describe("renders title and children", () => {
    it("renders title", () => {
      render(<FormSection title="Personal Info">content</FormSection>);
      expect(screen.getByText("Personal Info")).toBeInTheDocument();
    });

    it("renders description when provided", () => {
      render(
        <FormSection title="Info" description="Fill in details">
          content
        </FormSection>,
      );
      expect(screen.getByText("Fill in details")).toBeInTheDocument();
    });

    it("does not render description when not provided", () => {
      render(<FormSection title="Info">content</FormSection>);
      expect(screen.queryByText("Fill in details")).not.toBeInTheDocument();
    });

    it("renders children when not collapsed", () => {
      render(<FormSection title="Info">child content</FormSection>);
      expect(screen.getByText("child content")).toBeInTheDocument();
    });

    it("renders required asterisk when required is true", () => {
      render(
        <FormSection title="Info" required>
          content
        </FormSection>,
      );
      expect(screen.getByText("*")).toBeInTheDocument();
    });

    it("does not render required asterisk when required is not set", () => {
      render(<FormSection title="Info">content</FormSection>);
      expect(screen.queryByText("*")).not.toBeInTheDocument();
    });
  });

  describe("collapsible behavior", () => {
    it("hides children when defaultCollapsed is true and collapsible", () => {
      render(
        <FormSection title="Info" collapsible defaultCollapsed>
          hidden content
        </FormSection>,
      );
      // 折叠时保持挂载（hidden）以便字段参与校验/提交，只校验不可见。
      expect(screen.getByText("hidden content")).not.toBeVisible();
    });

    it("shows children when collapsible but not defaultCollapsed", () => {
      render(
        <FormSection title="Info" collapsible>
          visible content
        </FormSection>,
      );
      expect(screen.getByText("visible content")).toBeInTheDocument();
    });

    it("toggles children on click when collapsible", async () => {
      const user = userEvent.setup();
      render(
        <FormSection title="Info" collapsible defaultCollapsed>
          toggle content
        </FormSection>,
      );
      expect(screen.getByText("toggle content")).not.toBeVisible();

      await user.click(screen.getByRole("button", { name: "Info" }));
      expect(screen.getByText("toggle content")).toBeVisible();

      await user.click(screen.getByRole("button", { name: "Info" }));
      expect(screen.getByText("toggle content")).not.toBeVisible();
    });

    it("does not toggle when not collapsible", async () => {
      render(<FormSection title="Info">static content</FormSection>);
      expect(screen.getByText("static content")).toBeInTheDocument();
    });

    it("renders toggle icon when collapsible", () => {
      render(
        <FormSection title="Info" collapsible>
          content
        </FormSection>,
      );
      // The toggle icon should be present (RightOutlined or DownOutlined)
      const toggleIcon = document.querySelector(".gl-form-section__toggle");
      expect(toggleIcon).toBeInTheDocument();
    });

    it("does not render toggle icon when not collapsible", () => {
      render(<FormSection title="Info">content</FormSection>);
      const toggleIcon = document.querySelector(".gl-form-section__toggle");
      expect(toggleIcon).not.toBeInTheDocument();
    });

    it("header has pointer cursor when collapsible", () => {
      render(
        <FormSection title="Info" collapsible>
          content
        </FormSection>,
      );
      // pointer 光标落在折叠按钮上（标题 h3 不再被 button 包裹）。
      const toggle = document.querySelector(".gl-form-section__toggle");
      expect(toggle).toBeInTheDocument();
      expect((toggle as HTMLElement).style.cursor).toBe("pointer");
    });
  });

  describe("badge rendering", () => {
    it("renders badge count when provided", () => {
      render(
        <FormSection title="Info" badge={5}>
          content
        </FormSection>,
      );
      expect(screen.getByText("5")).toBeInTheDocument();
    });

    it("renders badge with string value", () => {
      render(
        <FormSection title="Info" badge={"new"}>
          content
        </FormSection>,
      );
      expect(screen.getByText("new")).toBeInTheDocument();
    });

    it("does not render badge when not provided", () => {
      render(<FormSection title="Info">content</FormSection>);
      // No badge element should be present
      const header = document.querySelector(".gl-form-section__header");
      expect(header).toBeInTheDocument();
    });
  });

  describe("extra content", () => {
    it("renders extra content when provided", () => {
      render(
        <FormSection title="Info" extra={<button type="button">Action</button>}>
          content
        </FormSection>,
      );
      expect(screen.getByText("Action")).toBeInTheDocument();
    });

    it("does not render extra content when not provided", () => {
      render(<FormSection title="Info">content</FormSection>);
      expect(screen.queryByText("Action")).not.toBeInTheDocument();
    });
  });

  describe("conditional visibility", () => {
    it("children are hidden when collapsed and shown when expanded", async () => {
      const user = userEvent.setup();
      render(
        <FormSection title="Section" collapsible defaultCollapsed>
          <div data-testid="child">Child Element</div>
        </FormSection>,
      );

      expect(screen.getByTestId("child")).not.toBeVisible();

      await user.click(screen.getByRole("button", { name: "Section" }));
      expect(screen.getByTestId("child")).toBeVisible();
    });

    it("non-collapsible section always shows children", () => {
      render(
        <FormSection title="Section">
          <div data-testid="child">Child Element</div>
        </FormSection>,
      );
      expect(screen.getByTestId("child")).toBeInTheDocument();
    });
  });

  describe("custom style", () => {
    it("applies custom style to the section container", () => {
      render(
        <FormSection title="Info" style={{ marginTop: 16 }}>
          content
        </FormSection>,
      );
      const section = document.querySelector(".gl-form-section");
      expect(section).toBeInTheDocument();
      expect((section as HTMLElement).style.marginTop).toBe("16px");
    });
  });
});
