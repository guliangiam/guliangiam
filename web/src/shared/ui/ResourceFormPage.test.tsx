import { afterEach, describe, expect, it, vi } from "vitest";
import { useState } from "react";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Form, Input } from "antd";
import { createMemoryRouter, Link, RouterProvider, useNavigate } from "react-router-dom";
import type { ReactElement } from "react";
import { HttpError } from "@/shared/api/http";
import { ResourceFormPage } from "./ResourceFormPage";
import { FormSection } from "./FormSection";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, options?: { field?: string }) =>
      options?.field ? `${key}:${options.field}` : key,
  }),
}));

const permissionState = vi.hoisted(() => ({ allowed: false }));

vi.mock("@/shared/hooks/usePermissions", () => ({
  usePermissions: () => ({
    permissions: { menus: [], actions: [], resources: [] },
    isLoading: false,
    hasMenu: () => false,
    hasAction: () => permissionState.allowed,
    hasResource: () => permissionState.allowed,
    refetch: vi.fn<(...args: unknown[]) => unknown>(),
  }),
}));

// ResourceFormPage 内部使用 useBlocker，必须挂在 data router 下；
// MemoryRouter 不满足要求，这里用 createMemoryRouter 提供同等上下文。
function renderInRouter(ui: ReactElement) {
  const router = createMemoryRouter([{ path: "/", element: ui }]);
  return render(<RouterProvider router={router} />);
}

type FormValues = { name: string };
type FinishHandler = (values: FormValues) => void | boolean | Promise<void | boolean>;

function TestForm({
  onFinish,
  onCancel = vi.fn<() => void>(),
  showSaveAndExit,
  stickyFooter,
  enableDraft,
  fieldLabels,
  submitPermission,
  loading,
}: {
  onFinish: FinishHandler;
  onCancel?: () => void;
  showSaveAndExit?: boolean;
  stickyFooter?: boolean;
  enableDraft?: boolean;
  fieldLabels?: Record<string, string>;
  submitPermission?: { permission: string; resource: string };
  loading?: boolean;
}) {
  const [form] = Form.useForm<FormValues>();

  return (
    <ResourceFormPage
      title="Create resource"
      description="Resource details"
      form={form}
      onFinish={onFinish}
      onCancel={onCancel}
      showSaveAndExit={showSaveAndExit}
      stickyFooter={stickyFooter}
      enableDraft={enableDraft}
      draftRoute="test"
      draftEntityId="1"
      fieldLabels={fieldLabels}
      submitPermission={submitPermission}
      loading={loading}
    >
      <Form.Item name="name" label="Name" rules={[{ required: true, message: "Name is required" }]}>
        <Input />
      </Form.Item>
    </ResourceFormPage>
  );
}

function LegacyTestForm({ onSubmit }: { onSubmit: () => void }) {
  const [form] = Form.useForm<FormValues>();

  return (
    <ResourceFormPage
      title="Create resource"
      description="Resource details"
      form={form}
      onSubmit={onSubmit}
      onCancel={vi.fn<() => void>()}
    >
      <Form.Item name="name" label="Name" rules={[{ required: true, message: "Name is required" }]}>
        <Input />
      </Form.Item>
    </ResourceFormPage>
  );
}

type NestedValues = { email: string; contact: { email: string } };

function NestedEmailForm({
  onFinish,
}: {
  onFinish: (values: NestedValues) => void | Promise<void>;
}) {
  const [form] = Form.useForm<NestedValues>();

  return (
    <ResourceFormPage
      title="Nested fields"
      description="Same leaf, different paths"
      form={form}
      onFinish={onFinish}
      onCancel={vi.fn<() => void>()}
      fieldLabels={{ email: "Email", "contact.email": "Contact email" }}
    >
      <Form.Item name="email" label="Email">
        <Input />
      </Form.Item>
      <Form.Item name={["contact", "email"]} label="Contact email">
        <Input />
      </Form.Item>
    </ResourceFormPage>
  );
}

function CollapsibleRequiredForm({ onFinish }: { onFinish: FinishHandler }) {
  const [form] = Form.useForm<FormValues>();

  return (
    <ResourceFormPage
      title="Collapsible form"
      description="Required field in a collapsed section"
      form={form}
      onFinish={onFinish}
      onCancel={vi.fn<() => void>()}
    >
      <FormSection title="Advanced" collapsible defaultCollapsed>
        <Form.Item
          name="name"
          label="Name"
          rules={[{ required: true, message: "Name is required" }]}
        >
          <Input />
        </Form.Item>
      </FormSection>
    </ResourceFormPage>
  );
}

/** 复刻真实页面形态：保存成功后在 onFinish 里直接 navigate 走人。 */
function NavigateAfterSaveForm({ onSaved }: { onSaved?: () => void }) {
  const [form] = Form.useForm<FormValues>();
  const navigate = useNavigate();

  return (
    <ResourceFormPage
      title="Create resource"
      description="Resource details"
      form={form}
      enableDraft
      draftRoute="test"
      draftEntityId="1"
      onFinish={async () => {
        onSaved?.();
        navigate("/done");
      }}
      onCancel={vi.fn<() => void>()}
    >
      <Form.Item name="name" label="Name">
        <Input />
      </Form.Item>
      <Link to="/done">leave</Link>
    </ResourceFormPage>
  );
}

function renderNavigationHarness() {
  const router = createMemoryRouter(
    [
      { path: "/", element: <NavigateAfterSaveForm /> },
      { path: "/done", element: <div data-testid="done" /> },
    ],
    { initialEntries: ["/"] },
  );
  return render(<RouterProvider router={router} />);
}

const DRAFT_KEY = "gl:draft:test:1";

function seedDraft() {
  localStorage.setItem(
    DRAFT_KEY,
    JSON.stringify({ v: 1, values: { name: "draft" }, savedAt: Date.now() }),
  );
}

afterEach(() => {
  permissionState.allowed = true;
  localStorage.clear();
});

describe("ResourceFormPage", () => {
  it("blocks submit on validation errors and displays the field error", async () => {
    const onFinish = vi.fn<FinishHandler>();
    const user = userEvent.setup();
    renderInRouter(<TestForm onFinish={onFinish} />);

    await user.click(screen.getByRole("button", { name: "save" }));

    expect(await screen.findByText("Name is required")).toBeInTheDocument();
    expect(onFinish).not.toHaveBeenCalled();
  });

  it("expands a collapsed section and focuses the first invalid field", async () => {
    const onFinish = vi.fn<FinishHandler>();
    const user = userEvent.setup();
    const { container } = renderInRouter(<CollapsibleRequiredForm onFinish={onFinish} />);

    const body = container.querySelector(".gl-form-section__body") as HTMLElement;
    expect(body).toHaveAttribute("hidden");

    await user.click(screen.getByRole("button", { name: "save" }));

    await waitFor(() => expect(body).not.toHaveAttribute("hidden"));
    // 错误文案不再处于任何 hidden 祖先之下，且字段获得焦点。
    expect(screen.getByText("Name is required").closest("[hidden]")).toBeNull();
    await waitFor(() => expect(screen.getByRole("textbox", { name: "Name" })).toHaveFocus());
    expect(onFinish).not.toHaveBeenCalled();
  });

  it("keeps duplicate submissions locked until the async save settles", async () => {
    let finishSave!: () => void;
    const onFinish = vi.fn<FinishHandler>(
      () =>
        new Promise<void>((resolve) => {
          finishSave = resolve;
        }),
    );
    const user = userEvent.setup();
    renderInRouter(<TestForm onFinish={onFinish} />);

    await user.type(screen.getByRole("textbox", { name: "Name" }), "resource");
    const saveButton = screen.getByRole("button", { name: "save" });
    await user.click(saveButton);
    await waitFor(() => expect(onFinish).toHaveBeenCalledOnce());

    await user.click(saveButton);
    expect(onFinish).toHaveBeenCalledOnce();

    finishSave();
    await waitFor(() => expect(saveButton).toBeEnabled());
  });

  it("submits on native form submission such as Enter from a field", async () => {
    const onFinish = vi.fn<FinishHandler>();
    const { container } = renderInRouter(<TestForm onFinish={onFinish} />);

    await userEvent.type(screen.getByRole("textbox", { name: "Name" }), "resource");
    fireEvent.submit(container.querySelector("form")!);

    await waitFor(() =>
      expect(onFinish).toHaveBeenCalledWith(expect.objectContaining({ name: "resource" })),
    );
  });

  it("clears the draft after a successful native submission", async () => {
    seedDraft();
    const onFinish = vi.fn<FinishHandler>().mockResolvedValue(true);
    const { container } = renderInRouter(<TestForm onFinish={onFinish} enableDraft />);

    expect(await screen.findByDisplayValue("draft")).toBeInTheDocument();
    expect(localStorage.getItem(DRAFT_KEY)).not.toBeNull();
    fireEvent.submit(container.querySelector("form")!);

    await waitFor(() => expect(onFinish).toHaveBeenCalledOnce());
    await waitFor(() => expect(localStorage.getItem(DRAFT_KEY)).toBeNull());
  });

  it("keeps input and draft after a failed Save & Exit without leaving", async () => {
    seedDraft();
    const onFinish = vi.fn<FinishHandler>().mockResolvedValue(false);
    const onCancel = vi.fn<() => void>();
    const user = userEvent.setup();
    renderInRouter(
      <TestForm onFinish={onFinish} onCancel={onCancel} showSaveAndExit enableDraft />,
    );

    const input = await screen.findByDisplayValue("draft");
    await user.clear(input);
    await user.type(input, "keep me");
    await user.click(screen.getByRole("button", { name: "saveAndExit" }));

    await waitFor(() => expect(onFinish).toHaveBeenCalledOnce());
    expect(input).toHaveValue("keep me");
    expect(localStorage.getItem(DRAFT_KEY)).not.toBeNull();
    expect(onCancel).not.toHaveBeenCalled();
  });

  it("keeps Save on the page and exits only after a successful Save & Exit", async () => {
    const onFinish = vi.fn<FinishHandler>().mockResolvedValue(true);
    const onCancel = vi.fn<() => void>();
    const user = userEvent.setup();
    renderInRouter(<TestForm onFinish={onFinish} onCancel={onCancel} showSaveAndExit />);

    await user.type(screen.getByRole("textbox", { name: "Name" }), "resource");
    await user.click(screen.getByRole("button", { name: "save" }));
    await waitFor(() => expect(onFinish).toHaveBeenCalledOnce());
    expect(onCancel).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "saveAndExit" }));
    await waitFor(() => expect(onCancel).toHaveBeenCalledOnce());
    expect(onFinish).toHaveBeenCalledTimes(2);
  });

  it("renders one primary action area when the sticky footer is enabled", () => {
    renderInRouter(<TestForm onFinish={vi.fn<FinishHandler>()} showSaveAndExit stickyFooter />);

    expect(screen.getAllByRole("button", { name: "save" })).toHaveLength(1);
    expect(screen.getAllByRole("button", { name: "saveAndExit" })).toHaveLength(1);
    expect(screen.getAllByRole("button", { name: "reset" })).toHaveLength(1);
    expect(
      screen.getByRole("button", { name: "save" }).closest(".gl-sticky-footer"),
    ).not.toBeNull();
  });

  it("maps an HttpError field to its Form.Item without rendering server prose", async () => {
    const httpError = new HttpError(422, {}, "Username is already taken");
    httpError.details = { field: "name" };
    const onFinish = vi.fn<FinishHandler>().mockRejectedValue(httpError);
    const user = userEvent.setup();
    const { container } = renderInRouter(
      <TestForm onFinish={onFinish} fieldLabels={{ name: "Name" }} />,
    );

    await user.type(screen.getByRole("textbox", { name: "Name" }), "resource");
    await user.click(screen.getByRole("button", { name: "save" }));

    await waitFor(() =>
      expect(container.querySelector(".ant-form-item-has-error")).toHaveTextContent(
        "common:serverError.invalidField:Name",
      ),
    );
    expect(container.querySelectorAll(".ant-alert")).toHaveLength(1);
    expect(container.querySelector(".ant-alert")).toHaveTextContent(
      "common:serverError.validation",
    );
    expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue("resource");
  });

  it("uses the full nested path so a same-leaf item stays clean", async () => {
    const httpError = new HttpError(422, {}, "server prose");
    httpError.details = { fields: [{ field: "contact.email" }] };
    const onFinish = vi.fn<(values: NestedValues) => Promise<void>>().mockRejectedValue(httpError);
    const user = userEvent.setup();
    renderInRouter(<NestedEmailForm onFinish={onFinish} />);

    await user.click(screen.getByRole("button", { name: "save" }));

    await waitFor(() => {
      const nestedItem = screen
        .getByRole("textbox", { name: "Contact email" })
        .closest(".ant-form-item");
      expect(nestedItem).toHaveTextContent("common:serverError.invalidField:Contact email");
    });
    const topLevelItem = screen.getByRole("textbox", { name: "Email" }).closest(".ant-form-item");
    expect(topLevelItem).not.toHaveClass("ant-form-item-has-error");
  });

  it("leaves both items clean when the leaf is ambiguous", async () => {
    const httpError = new HttpError(422, {}, "server prose");
    httpError.details = { fields: [{ field: "other.email" }] };
    const onFinish = vi.fn<(values: NestedValues) => Promise<void>>().mockRejectedValue(httpError);
    const user = userEvent.setup();
    const { container } = renderInRouter(<NestedEmailForm onFinish={onFinish} />);

    await user.click(screen.getByRole("button", { name: "save" }));

    await waitFor(() =>
      expect(container.querySelector(".ant-alert")).toHaveTextContent(
        "common:serverError.validation",
      ),
    );
    expect(container.querySelectorAll(".ant-form-item-has-error")).toHaveLength(0);
  });

  it("scrolls only inside the form that failed validation", async () => {
    const scrolledElements: Element[] = [];
    const originalScrollIntoView = Element.prototype.scrollIntoView;
    Object.defineProperty(Element.prototype, "scrollIntoView", {
      configurable: true,
      value: function (this: Element) {
        scrolledElements.push(this);
      },
    });

    try {
      const user = userEvent.setup();
      const { container } = renderInRouter(
        <>
          <TestForm onFinish={vi.fn<FinishHandler>()} />
          <TestForm onFinish={vi.fn<FinishHandler>()} />
        </>,
      );
      const forms = container.querySelectorAll("form");

      await user.click(screen.getAllByRole("button", { name: "save" })[1]);

      await waitFor(() => expect(scrolledElements.length).toBeGreaterThan(0));
      for (const element of scrolledElements) {
        expect(element.closest("form")).toBe(forms[1]);
      }
    } finally {
      if (originalScrollIntoView) {
        Object.defineProperty(Element.prototype, "scrollIntoView", {
          configurable: true,
          value: originalScrollIntoView,
        });
      } else {
        Reflect.deleteProperty(Element.prototype, "scrollIntoView");
      }
    }
  });

  it("keeps native submit behavior for legacy onSubmit pages", async () => {
    const onSubmit = vi.fn<() => void>();
    const { container } = renderInRouter(<LegacyTestForm onSubmit={onSubmit} />);

    await userEvent.type(screen.getByRole("textbox", { name: "Name" }), "resource");
    fireEvent.submit(container.querySelector("form")!);

    await waitFor(() => expect(onSubmit).toHaveBeenCalledOnce());
  });

  it("hides Save and Save & Exit when the submit permission is denied", () => {
    permissionState.allowed = false;

    renderInRouter(
      <TestForm
        onFinish={vi.fn<FinishHandler>()}
        showSaveAndExit
        submitPermission={{ permission: "resource:write", resource: "resources" }}
      />,
    );

    expect(screen.queryByRole("button", { name: "save" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "saveAndExit" })).not.toBeInTheDocument();
  });

  it("keeps Save and Save & Exit when the submit permission prop is omitted", () => {
    permissionState.allowed = false;

    renderInRouter(<TestForm onFinish={vi.fn<FinishHandler>()} showSaveAndExit />);

    expect(screen.getByRole("button", { name: "save" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "saveAndExit" })).toBeInTheDocument();
  });

  it("leaves without the unsaved-changes confirmation after a successful save navigates away", async () => {
    // 保存成功后导航发生在 setSubmitting 提交之前，守卫必须同步放行，不能弹确认框。
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(false);
    const user = userEvent.setup();
    renderNavigationHarness();

    await user.type(screen.getByRole("textbox", { name: "Name" }), "resource");
    await user.click(screen.getByRole("button", { name: "save" }));

    expect(await screen.findByTestId("done")).toBeInTheDocument();
    expect(confirmSpy).not.toHaveBeenCalled();
    confirmSpy.mockRestore();
  });

  it("still confirms before leaving when the draft has unsaved changes", async () => {
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(false);
    const user = userEvent.setup();
    renderNavigationHarness();

    await user.type(screen.getByRole("textbox", { name: "Name" }), "resource");
    await user.click(screen.getByRole("link", { name: "leave" }));

    await waitFor(() => expect(confirmSpy).toHaveBeenCalledTimes(1));
    expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue("resource");
    expect(screen.queryByTestId("done")).not.toBeInTheDocument();
    confirmSpy.mockRestore();
  });

  it("re-measures the sticky header height once the loading skeleton is replaced", async () => {
    // 回归防护：加载态渲染 Skeleton（没有真实 header），若测量 effect 只在挂载时
    // 执行一次，加载完成后变量会停留在 CSS 兜底 80px，header 变高时副导航会被遮住。
    const originalRect = Element.prototype.getBoundingClientRect;
    Element.prototype.getBoundingClientRect = function (this: Element) {
      const rect = originalRect.call(this);
      if (this instanceof HTMLElement && this.classList.contains("gl-resource-card__header")) {
        return { ...rect, height: 141, top: 0, bottom: 141 } as DOMRect;
      }
      return rect;
    };

    function LoadingThenReady() {
      const [loading, setLoading] = useState(true);
      return (
        <>
          <button type="button" onClick={() => setLoading(false)}>
            loaded
          </button>
          <TestForm onFinish={vi.fn<FinishHandler>()} loading={loading} />
        </>
      );
    }

    try {
      const user = userEvent.setup();
      const { container } = renderInRouter(<LoadingThenReady />);
      // 加载态下 .gl-resource-card 是 Skeleton，切换后会被真实 Card 替换，
      // 断言必须重新查询节点，不能复用切换前的引用。
      const skeletonCard = container.querySelector(".gl-resource-card") as HTMLElement;
      expect(skeletonCard.style.getPropertyValue("--gl-sticky-header-height")).toBe("");

      await user.click(screen.getByRole("button", { name: "loaded" }));

      await waitFor(() => {
        const card = container.querySelector(".gl-resource-card") as HTMLElement;
        expect(card).not.toBe(skeletonCard);
        expect(card.style.getPropertyValue("--gl-sticky-header-height")).toBe("141px");
      });
    } finally {
      Element.prototype.getBoundingClientRect = originalRect;
    }
  });
});
