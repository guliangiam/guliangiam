import { Alert, Button, Card, Form, Skeleton, Space, Typography } from "antd";
import type { FormInstance } from "antd";
import { useEffect, useRef, useState, type PropsWithChildren, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { StickyFormFooter } from "@/shared/ui/StickyFormFooter";
import { FORM_SECTION_EXPAND_EVENT } from "@/shared/ui/FormSection";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { useGlDraft } from "@/shared/hooks/useGlDraft";
import { useUnsavedChangesGuard } from "@/shared/hooks/useUnsavedChangesGuard";
import { isHttpError } from "@/shared/api/http";
import {
  applyServerFieldErrors,
  buildServerFieldResolver,
  localizedError,
  serverErrorMessage,
} from "@/shared/lib/server-error";

const FOCUSABLE_FIELD_SELECTOR =
  'input:not([type="hidden"]), textarea, select, [tabindex]:not([tabindex="-1"])';

/** 展开包含 target 的所有折叠分区，返回实际展开的分区数量。 */
function expandCollapsedSectionsAround(container: HTMLElement, target: Element): number {
  let expanded = 0;
  let node: Element | null = target.parentElement;
  while (node && node !== container) {
    if (node.classList.contains("gl-form-section")) {
      const body = Array.from(node.children).find((child) =>
        child.classList.contains("gl-form-section__body"),
      );
      if (body?.hasAttribute("hidden")) {
        node.dispatchEvent(new CustomEvent(FORM_SECTION_EXPAND_EVENT));
        expanded += 1;
      }
    }
    node = node.parentElement;
  }
  return expanded;
}

/** 滚动到错误项并聚焦其首个可聚焦控件，确保折叠分区内的错误可被用户发现。 */
function focusFirstError(element: Element) {
  const field = element.querySelector<HTMLElement>(FOCUSABLE_FIELD_SELECTOR);
  field?.focus({ preventScroll: true });
  if ("scrollIntoView" in element) {
    element.scrollIntoView({ behavior: "smooth", block: "center" });
  }
}

type ResourceFormPageProps<TValues> = PropsWithChildren<{
  title: string;
  description: string;
  loading?: boolean;
  saving?: boolean;
  error?: string | null;
  form?: FormInstance<TValues>;
  /** Called after the form passes validation. Return a promise to keep submission locked until it settles. */
  onFinish?: (values: TValues) => void | boolean | Promise<void | boolean>;
  /** @deprecated Use onFinish instead. Kept for backward compatibility. */
  onSubmit?: () => void | boolean | Promise<void | boolean>;
  onCancel: () => void;
  cancelText?: string;
  submitText?: string;
  /** Custom reset behaviour. Defaults to `form.resetFields()`. */
  onReset?: () => void;
  /** Show "Save & Exit" button that saves then calls onCancel */
  showSaveAndExit?: boolean;
  saveAndExitText?: string;
  /**
   * 保存动作的权限声明。传入时用 PermissionGuard 包裹 Save / Save&Exit
   * （含 loading 态与 sticky footer 中的同名按钮）；未传时保持既有渲染，
   * 不做权限判断。
   */
  submitPermission?: { permission: string; resource: string };
  /** Extra content rendered at the start of the header action bar (e.g. Delete button). */
  extra?: ReactNode;
  /** Render content inside children instead of form wrapper */
  noForm?: boolean;
  /**
   * Show a sticky footer action bar at the bottom of the card.
   * OFF by default — all primary actions (Reset / Cancel / Save / Save&Exit)
   * live in the card header so users don't need to scroll. Set true only for
   * exceptionally long forms where the footer provides useful persistence.
   */
  stickyFooter?: boolean;
  /**
   * Enable auto-save draft to localStorage. OFF by default (CreatePage usually
   * has no entityId to key on). DetailPage callers should set:
   *   enableDraft + route + entityId
   * to get debounced localStorage persistence + dirty-discard guard.
   */
  enableDraft?: boolean;
  /** Additional dirty state owned by a sibling form (e.g. provider bindings). */
  extraDirty?: boolean;
  /** Route segment for the draft key. Auto-derived from URL if omitted. */
  draftRoute?: string;
  /** Entity ID for the draft key. Required to actually persist. */
  draftEntityId?: string | number;
  /**
   * 不进入草稿的字段（乐观锁 revision、凭据、密钥等）。
   * 这些字段必须始终以服务端当前值为准。
   */
  draftIgnoreKeys?: string[];
  /**
   * Localized labels for 422 field messages, keyed by full form path
   * (`profile.email`) or by unique leaf (`email`). Fields without a label
   * still get the generic "check the marked fields" message.
   */
  fieldLabels?: Record<string, string>;
  /** Map backend field names to form field paths (`name` → `username`). */
  fieldAliases?: Record<string, string>;
  /** 表单值变化回调（与草稿绑定并行触发）；供页面级脏检查等消费。 */
  onValuesChange?: (changedValues: Partial<TValues>, values: TValues) => void;
  /**
   * Offer a Retry action on the submit-error alert for transient failures
   * (5xx / 429 / 408 / network). Validation errors (422) never offer a blind
   * retry — the user must fix the marked fields first.
   */
  retryable?: boolean;
}>;

export function ResourceFormPage<TValues = Record<string, unknown>>({
  title,
  description,
  loading,
  saving,
  error,
  form,
  onFinish,
  onSubmit,
  onCancel,
  onReset,
  cancelText,
  submitText,
  showSaveAndExit,
  saveAndExitText,
  submitPermission,
  extra,
  noForm,
  stickyFooter,
  enableDraft,
  extraDirty = false,
  draftRoute,
  draftEntityId,
  draftIgnoreKeys,
  fieldLabels,
  fieldAliases,
  onValuesChange,
  retryable,
  children,
}: ResourceFormPageProps<TValues>) {
  const { t } = useTranslation("common");
  const resolvedCancelText = cancelText ?? t("backToList");
  const resolvedSubmitText = submitText ?? t("save");
  const resolvedSaveAndExitText = saveAndExitText ?? t("saveAndExit");

  // --- Draft auto-save + dirty guard ---------------------------------------
  const routeForDraft = draftRoute ?? undefined;
  const idForDraft =
    draftEntityId !== undefined && draftEntityId !== null ? String(draftEntityId) : undefined;

  const draft = useGlDraft(form ?? null, {
    enable: enableDraft,
    route: routeForDraft,
    entityId: idForDraft,
    ignoreKeys: draftIgnoreKeys,
  });

  // Keep the guard for the full async submission, including the mutation request.
  const isSubmittingRef = useRef(false);
  // 提交期间必须放行导航：成功回调通常直接 navigate 到详情/列表页，
  // 而此刻草稿仍是 dirty，若不抑制就会弹出「未保存修改」确认框并取消跳转。
  const [submitting, setSubmitting] = useState(false);
  useUnsavedChangesGuard({
    enabled: (draft.dirty || extraDirty) && !submitting,
    // 成功回调里 navigate 时 submitting 的 setState 可能尚未提交，用 ref 同步放行。
    bypassRef: isSubmittingRef,
  });

  const formContainerRef = useRef<HTMLDivElement>(null);
  const cardHeaderRef = useRef<HTMLDivElement>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [canRetry, setCanRetry] = useState(false);
  const [errorFocusRequest, setErrorFocusRequest] = useState(0);

  // 动态测量 sticky header 高度：写入 CSS 变量 ——gl-sticky-header-height，
  // 供分区副导航 / 分区标题 / 锚点滚动偏移实时计算，避免固定常量在不同
  // 断点、缩放、字体加载、header 内容（extra 按钮 / 返回键）变化时出现遮挡。
  // 依赖 loading：加载态渲染的是 Skeleton（没有真实 header），首次挂载时
  // cardHeaderRef 为空；加载完成后必须重新绑定，否则变量会一直停留在
  // CSS 兜底值 80px，header 变高时副导航就会钻到 header 底下。
  useEffect(() => {
    if (loading) return;
    const header = cardHeaderRef.current;
    if (!header) return;
    const card = header.closest<HTMLElement>(".gl-resource-card");
    if (!card) return;

    const writeHeight = () => {
      const rect = header.getBoundingClientRect();
      // 必须取 Math.ceil，否则浏览器亚像素渲染可能让 sticky 元素与 header 间
      // 出现可见细线或重叠。
      card.style.setProperty("--gl-sticky-header-height", `${Math.ceil(rect.height)}px`);
    };

    // 首次布局后立即写入，避免 CSS fallback 80px 与真实高度闪烁。
    const raf = requestAnimationFrame(writeHeight);

    const ro = new ResizeObserver(writeHeight);
    ro.observe(header);

    // header 内部内容变化（extra 按钮出现/消失、描述文本换行、loading→ready）
    // 也会改变 header 高度，MutationObserver 捕获这类无法被 ResizeObserver
    // 覆盖的变更（ResizeObserver 只观察盒模型尺寸变化）。
    const mo = new MutationObserver(writeHeight);
    mo.observe(header, { childList: true, subtree: true, characterData: true });

    const onWindowResize = () => writeHeight();
    window.addEventListener("resize", onWindowResize);

    return () => {
      cancelAnimationFrame(raf);
      ro.disconnect();
      mo.disconnect();
      window.removeEventListener("resize", onWindowResize);
    };
  }, [loading]);

  const scrollToFirstError = () => setErrorFocusRequest((request) => request + 1);

  useEffect(() => {
    if (errorFocusRequest === 0) return;
    const container = formContainerRef.current;
    const firstError = container?.querySelector(".ant-form-item-has-error");
    if (!container || !firstError) return;

    const expanded = expandCollapsedSectionsAround(container, firstError);
    if (expanded === 0) {
      focusFirstError(firstError);
      return;
    }
    // 折叠分区展开后才移除 hidden；等一帧再滚动/聚焦，避免定位到不可见节点。
    const frame = requestAnimationFrame(() => {
      const next = container.querySelector(".ant-form-item-has-error");
      if (next) focusFirstError(next);
    });
    return () => cancelAnimationFrame(frame);
  }, [errorFocusRequest]);

  if (loading) {
    // 占位与成品同宽（表单/详情档 1200px），避免加载完成时主显示区宽度跳变。
    return (
      <div className="gl-content__inner--form">
        <Skeleton active paragraph={{ rows: 12 }} className="gl-panel gl-resource-card" />
      </div>
    );
  }

  const runSubmission = async (operation: () => void | boolean | Promise<void | boolean>) => {
    if (isSubmittingRef.current || saving) return false;
    isSubmittingRef.current = true;
    setSubmitting(true);
    setSubmitError(null);
    setCanRetry(false);
    try {
      const ok = (await operation()) !== false;
      if (ok) draft.clearDraft();
      return ok;
    } catch (cause) {
      let fieldErrorsApplied = false;
      if (form && isHttpError(cause)) {
        // Full-path matching first; ambiguous leaves stay unmatched instead of
        // writing the error onto the wrong control (profile.email vs email).
        fieldErrorsApplied = applyServerFieldErrors(
          form,
          cause,
          buildServerFieldResolver(form, { labels: fieldLabels, aliases: fieldAliases }),
          t,
        );
      }
      const message = isHttpError(cause)
        ? serverErrorMessage(cause, t, "error")
        : localizedError(cause, t, "error");
      setSubmitError(message);
      // Transient failures are worth retrying; a 422 field error is not.
      setCanRetry(
        isHttpError(cause)
          ? cause.status >= 500 || cause.status === 429 || cause.status === 408
          : true,
      );
      if (fieldErrorsApplied) scrollToFirstError();
      return false;
    } finally {
      isSubmittingRef.current = false;
      setSubmitting(false);
    }
  };

  const submitValues = (values: TValues) => {
    if (!onFinish) return Promise.resolve(false);
    return runSubmission(() => onFinish(values));
  };

  const submitLegacy = () => {
    if (!onSubmit) return Promise.resolve(false);
    return runSubmission(onSubmit);
  };

  const handleSubmit = async () => {
    if (isSubmittingRef.current || saving) return false;

    if (onFinish && form) {
      let values: TValues;
      try {
        values = await form.validateFields();
      } catch {
        scrollToFirstError();
        return false;
      }
      return submitValues(values);
    }

    return submitLegacy();
  };

  const content = noForm ? (
    children
  ) : (
    <Form<TValues>
      form={form}
      layout="vertical"
      className="gl-form-grid"
      onFinish={(values) => (onFinish ? void submitValues(values as TValues) : void submitLegacy())}
      onFinishFailed={scrollToFirstError}
      onValuesChange={
        form
          ? (changed, values) => {
              draft.bindValuesChange(changed, values as unknown as Record<string, unknown>);
              onValuesChange?.(changed as Partial<TValues>, values as TValues);
            }
          : undefined
      }
    >
      {children}
    </Form>
  );

  const showSticky = stickyFooter === true; // opt-in only; header now holds all primary actions

  const submitActions = (
    <>
      {!showSticky && (onFinish || onSubmit) && (
        <Button type="primary" loading={saving} onClick={handleSubmit} disabled={saving}>
          {resolvedSubmitText}
        </Button>
      )}
      {!showSticky && showSaveAndExit && (onFinish || onSubmit) && (
        <Button
          loading={saving}
          onClick={async () => {
            if (await handleSubmit()) onCancel();
          }}
          disabled={saving}
        >
          {resolvedSaveAndExitText}
        </Button>
      )}
    </>
  );

  // 未配置权限时原样输出；配置后 denial 直接不渲染保存入口（loading 态随之消失）。
  // sticky 模式下保存按钮在底栏，由下方 StickyFormFooter 的 guard 负责。
  const guardedSubmitActions =
    submitPermission && !showSticky && (onFinish || onSubmit) ? (
      <PermissionGuard
        action={submitPermission.permission}
        resource={submitPermission.resource}
        fallback={null}
      >
        {submitActions}
      </PermissionGuard>
    ) : (
      submitActions
    );

  const stickyFooterFields = {
    loading: saving,
    onReset: form ? () => (onReset ? onReset() : form.resetFields()) : undefined,
    onSave: handleSubmit,
    onSaveAndExit:
      showSaveAndExit && (onFinish || onSubmit)
        ? async () => {
            if (await handleSubmit()) onCancel();
          }
        : undefined,
    saveText: resolvedSubmitText,
    saveAndExitText: resolvedSaveAndExitText,
  };

  return (
    // 表单/详情页统一内容宽度档（1200px）：所有 ResourceFormPage 页面共享同一骨架宽度。
    <div ref={formContainerRef} className="gl-content__inner--form">
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <div ref={cardHeaderRef} className="gl-resource-card__header">
          <div>
            <Typography.Title level={2}>{title}</Typography.Title>
            <Typography.Paragraph>{description}</Typography.Paragraph>
          </div>
          <Space>
            {extra}
            {!showSticky && (onFinish || onSubmit) && form && (
              <Button onClick={() => (onReset ? onReset() : form.resetFields())}>
                {t("reset")}
              </Button>
            )}
            <Button onClick={onCancel}>{resolvedCancelText}</Button>
            {guardedSubmitActions}
          </Space>
        </div>

        {(submitError ?? error) && (
          <Alert
            type="error"
            title={submitError ?? error}
            showIcon
            closable
            style={{ marginBottom: 16 }}
            action={
              retryable && canRetry && submitError ? (
                <Button size="small" onClick={() => void handleSubmit()}>
                  {t("retry")}
                </Button>
              ) : undefined
            }
          />
        )}

        {content}
      </Card>
      {showSticky &&
        (submitPermission ? (
          <PermissionGuard
            action={submitPermission.permission}
            resource={submitPermission.resource}
            fallback={<StickyFormFooter loading={saving} onReset={stickyFooterFields.onReset} />}
          >
            <StickyFormFooter {...stickyFooterFields} />
          </PermissionGuard>
        ) : (
          <StickyFormFooter {...stickyFooterFields} />
        ))}
    </div>
  );
}

/** CSS class to mark a form item as full-width (spanning both grid columns). */
export const FORM_ITEM_FULL_WIDTH_CLASS = "gl-form-item-full";
