import { describe, it, expect, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { FieldHelpInjector } from "./FieldHelpInjector";

/**
 * 全局注入器的行为契约：
 * 1. 表单 label 自动追加问号图标，说明文本按「路由 scope + 字段名」解析；
 * 2. 未登记字段使用本地化兜底文案，仍保证有图标；
 * 3. 显式 `data-gl-help-key` 标记的功能点（Tabs 标签等）同样获得图标；
 * 4. 异步渲染出的新字段会被 MutationObserver 补注入。
 */
function mountForm(extra?: React.ReactNode) {
  return (
    <form>
      <label className="gl-form-row__label" htmlFor="username">
        Username
      </label>
      <input id="username" name="username" />
      <label className="gl-form-row__label" htmlFor="nickname">
        Nickname
      </label>
      <input id="nickname" name="nickname" />
      {extra}
    </form>
  );
}

function renderInjector(path: string, children: React.ReactNode) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <main>{children}</main>
      <FieldHelpInjector />
    </MemoryRouter>,
  );
}

describe("FieldHelpInjector", () => {
  beforeEach(() => {
    document.body.innerHTML = "";
  });

  it("为已登记字段追加图标并解析 help 文案", async () => {
    renderInjector("/users", mountForm());

    const label = screen.getByText("Username").closest("label")!;
    await waitFor(() => expect(label.querySelector(".gl-help-tip")).not.toBeNull());
    expect(label).toHaveAttribute("data-gl-help-done", "1");

    const user = userEvent.setup();
    await user.hover(label.querySelector(".gl-help-tip") as HTMLElement);
    expect(await screen.findByText(/Login username/)).toBeInTheDocument();
  });

  it("未登记字段使用本地化兜底文案", async () => {
    renderInjector("/users", mountForm());

    const label = screen.getByText("Nickname").closest("label")!;
    await waitFor(() => expect(label.querySelector(".gl-help-tip")).not.toBeNull());

    const user = userEvent.setup();
    await user.hover(label.querySelector(".gl-help-tip") as HTMLElement);
    expect(await screen.findByText(/"Nickname"/)).toBeInTheDocument();
  });

  it("为显式标记的功能点注入图标", async () => {
    renderInjector(
      "/rebac",
      <span data-gl-help-key="tabs.rebac.tuples" data-testid="tab-label">
        Relation Tuples
      </span>,
    );

    const target = screen.getByTestId("tab-label");
    await waitFor(() => expect(target.querySelector(".gl-help-tip")).not.toBeNull());
  });

  it("按字段名命中概念层说明", async () => {
    renderInjector(
      "/users",
      <form>
        <label className="gl-form-row__label" htmlFor="description">
          Description
        </label>
        <input id="description" name="description" />
      </form>,
    );

    const label = screen.getByText("Description").closest("label")!;
    await waitFor(() => expect(label.querySelector(".gl-help-slot")).not.toBeNull());
    expect(label.querySelector(".gl-help-slot")).toHaveAttribute(
      "data-gl-help-resolved-key",
      "concept.description",
    );
    expect(label.querySelector(".gl-help-slot")).not.toHaveAttribute("data-gl-help-fallback");
  });

  it("无 for/id 的自定义控件按标签文本命中概念层", async () => {
    renderInjector(
      "/account",
      <form>
        {/* 刻意模拟没有 for/id 的自定义控件 label，验证按文本推导概念键的路径 */}
        {/* eslint-disable-next-line jsx-a11y/label-has-associated-control */}
        <label className="gl-form-row__label">Username</label>
      </form>,
    );

    const label = screen.getByText("Username").closest("label")!;
    await waitFor(() => expect(label.querySelector(".gl-help-slot")).not.toBeNull());
    expect(label.querySelector(".gl-help-slot")).toHaveAttribute(
      "data-gl-help-resolved-key",
      "concept.username",
    );
    expect(label.querySelector(".gl-help-slot")).not.toHaveAttribute("data-gl-help-fallback");
  });

  it("为异步插入的字段补注入图标", async () => {
    const { container } = renderInjector("/users", mountForm());

    const late = document.createElement("label");
    late.className = "gl-form-row__label";
    late.htmlFor = "email";
    late.textContent = "Email";
    const input = document.createElement("input");
    input.id = "email";
    input.name = "email";
    container.querySelector("form")!.append(late, input);

    await waitFor(() => expect(late.querySelector(".gl-help-tip")).not.toBeNull());
  });
});
