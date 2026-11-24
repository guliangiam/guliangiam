import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SectionNavShell } from "./SectionNavShell";
import { useSectionNavStore } from "./section-nav-store";
import type { SectionNavItem } from "./types";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

const items: SectionNavItem[] = [
  { key: "basic", anchor: "sec-basic", label: "Basic" },
  { key: "credentials", anchor: "sec-credentials", label: "Credentials", disabled: true },
  { key: "advanced", anchor: "sec-advanced", label: "Advanced" },
];

function renderShell() {
  return render(
    <SectionNavShell items={items}>
      <div id="sec-basic">basic body</div>
      <div id="sec-credentials">credentials body</div>
      <div id="sec-advanced">advanced body</div>
    </SectionNavShell>,
  );
}

function layoutOf(container: HTMLElement): HTMLElement {
  const layout = container.querySelector<HTMLElement>(".gl-section-nav-layout");
  if (!layout) throw new Error("section nav layout not rendered");
  return layout;
}

describe("SectionNavShell", () => {
  beforeEach(() => {
    localStorage.clear();
    useSectionNavStore.setState({ scheme: null, updatedAt: null });
  });

  it("renders the content once and defaults to the right rail", () => {
    const { container } = renderShell();

    expect(screen.getByText("basic body")).toBeInTheDocument();
    expect(screen.getAllByText("basic body")).toHaveLength(1);
    expect(layoutOf(container).className).toContain("gl-section-nav-layout--rail-right");
  });

  it("renders nothing but the content when there are no sections", () => {
    const { container } = render(
      <SectionNavShell items={[]}>
        <div>only body</div>
      </SectionNavShell>,
    );

    expect(screen.getByText("only body")).toBeInTheDocument();
    expect(container.querySelector(".gl-section-nav")).toBeNull();
  });

  it("follows the persisted scheme without remounting the content", () => {
    useSectionNavStore.getState().setScheme("rail-left");
    const { container } = renderShell();

    expect(layoutOf(container).className).toContain("gl-section-nav-layout--rail-left");
    expect(screen.getAllByText("basic body")).toHaveLength(1);
  });

  it("reacts to a scheme change from the store", () => {
    const { container } = renderShell();

    act(() => {
      useSectionNavStore.getState().setScheme("rail-top");
    });

    expect(layoutOf(container).className).toContain("gl-section-nav-layout--rail-top");
  });

  it("marks disabled entries and keeps them out of the active state", async () => {
    const user = userEvent.setup();
    renderShell();

    const disabled = screen.getByText("Credentials").closest("a");
    expect(disabled).toHaveAttribute("aria-disabled", "true");
    expect(disabled).toHaveAttribute("tabindex", "-1");

    await user.click(screen.getByText("Credentials"));
    expect(disabled).not.toHaveAttribute("aria-current");
  });

  it("highlights the clicked section", async () => {
    const user = userEvent.setup();
    renderShell();

    const basic = screen.getByText("Basic").closest("a");
    await user.click(screen.getByText("Advanced"));

    expect(screen.getByText("Advanced").closest("a")).toHaveAttribute("aria-current", "location");
    expect(basic).not.toHaveAttribute("aria-current");
  });

  it("starts on the initial active key so deep-linked sections highlight on first paint", () => {
    render(
      <SectionNavShell items={items} initialActiveKey="advanced">
        <div id="sec-basic">basic body</div>
        <div id="sec-credentials">credentials body</div>
        <div id="sec-advanced">advanced body</div>
      </SectionNavShell>,
    );

    expect(screen.getByText("Advanced").closest("a")).toHaveAttribute("aria-current", "location");
    expect(screen.getByText("Basic").closest("a")).not.toHaveAttribute("aria-current");
  });

  it("keeps the clicked section highlighted until the user scrolls", async () => {
    const user = userEvent.setup();
    renderShell();

    await user.click(screen.getByText("Basic"));
    expect(screen.getByText("Basic").closest("a")).toHaveAttribute("aria-current", "location");

    // 点击后的程序化滚动阶段：Scrollspy 被锁定，不会把高亮切到别的标签。
    await act(async () => {
      window.dispatchEvent(new Event("scroll"));
      await new Promise((resolve) => requestAnimationFrame(() => resolve(null)));
    });

    expect(screen.getByText("Basic").closest("a")).toHaveAttribute("aria-current", "location");
  });

  it("scrolls to a deep-linked section after layout has settled", async () => {
    const targets: HTMLElement[] = [];
    const scrollIntoView = vi.fn<() => void>(function (this: HTMLElement) {
      targets.push(this);
    });
    Object.defineProperty(Element.prototype, "scrollIntoView", {
      configurable: true,
      writable: true,
      value: scrollIntoView,
    });

    render(
      <SectionNavShell items={items} initialActiveKey="advanced">
        <div id="sec-basic">basic body</div>
        <div id="sec-credentials">credentials body</div>
        <div id="sec-advanced">advanced body</div>
      </SectionNavShell>,
    );

    await act(async () => {
      await new Promise((resolve) => requestAnimationFrame(() => resolve(null)));
      await new Promise((resolve) => requestAnimationFrame(() => resolve(null)));
    });

    expect(targets.map((element) => element.id)).toContain("sec-advanced");

    Reflect.deleteProperty(Element.prototype, "scrollIntoView");
  });

  it("does not re-scroll when a parent render passes a new items array", async () => {
    const scrollIntoView = vi.fn<() => void>();
    Object.defineProperty(Element.prototype, "scrollIntoView", {
      configurable: true,
      writable: true,
      value: scrollIntoView,
    });

    const body = (
      <>
        <div id="sec-basic">basic body</div>
        <div id="sec-credentials">credentials body</div>
        <div id="sec-advanced">advanced body</div>
      </>
    );
    const { rerender } = render(
      <SectionNavShell items={items} initialActiveKey="advanced">
        {body}
      </SectionNavShell>,
    );
    await act(async () => {
      await new Promise((resolve) => requestAnimationFrame(() => resolve(null)));
      await new Promise((resolve) => requestAnimationFrame(() => resolve(null)));
    });
    const callsAfterMount = scrollIntoView.mock.calls.length;
    expect(callsAfterMount).toBeGreaterThan(0);

    // 父页面每次渲染都会新建数组；内容未变时不得再次触发深链滚动。
    rerender(
      <SectionNavShell items={items.map((item) => ({ ...item }))} initialActiveKey="advanced">
        {body}
      </SectionNavShell>,
    );
    await act(async () => {
      await new Promise((resolve) => requestAnimationFrame(() => resolve(null)));
      await new Promise((resolve) => requestAnimationFrame(() => resolve(null)));
    });

    expect(scrollIntoView.mock.calls.length).toBe(callsAfterMount);
    Reflect.deleteProperty(Element.prototype, "scrollIntoView");
  });

  it("forces the top bar on narrow viewports regardless of preference", () => {
    useSectionNavStore.getState().setScheme("rail-right");
    const matchMedia = vi.spyOn(window, "matchMedia").mockImplementation((query: string) => ({
      matches: query === "(max-width: 1080px)",
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }));

    const { container } = renderShell();
    expect(layoutOf(container).className).toContain("gl-section-nav-layout--rail-top");

    matchMedia.mockRestore();
  });
});
