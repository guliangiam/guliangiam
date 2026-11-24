import { useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useLocation } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { HelpTip } from "@/shared/ui/FieldHelp";

/**
 * 全局字段帮助注入器。
 *
 * 在 AppShell 挂载一次，为页面上所有表单字段 label 追加统一的问号图标：
 *  - `GlFormRow` 的 label（`label.gl-form-row__label`）
 *  - antd `Form.Item` 的 label（`.ant-form-item-label > label`）
 *  - 任意显式标记 `data-gl-help-key="<scope>.<field>"` 的元素
 *
 * 说明文本按「当前路由 scope + 字段名」从 `help` 命名空间解析；未登记字段使用
 * `help:fallback.field` 的本地化兜底文案，保证每个字段都有图标且有说明。
 * 图标通过 React portal 注入，主题与语言切换即时生效。
 */
const LABEL_SELECTOR = "label.gl-form-row__label, .ant-form-item-label > label";

type InjectionTarget = {
  id: string;
  slot: HTMLElement;
  helpKey: string;
  label: string;
};

function camelize(value: string): string {
  return value
    .replace(/[-_\s]+([a-zA-Z0-9])/g, (_m, ch: string) => ch.toUpperCase())
    .replace(/^[A-Z]/, (ch) => ch.toLowerCase());
}

function scopeFromPath(pathname: string): string {
  const segment = pathname.split("/").filter(Boolean)[0];
  return segment ? camelize(segment) : "dashboard";
}

/**
 * 由控件 id 推导候选 help key。antd 的 id 可能是：
 *  - 字段名（`email`）
 *  - 嵌套路径以 `_` 连接（`profile_email`）
 *  - 表单名 + 字段名（`user_email`）
 * 因此从完整 id 开始逐段去掉前缀，任一候选命中即采用。
 */
function candidateKeys(scope: string, rawId: string): string[] {
  const parts = rawId.split("_").filter(Boolean);
  if (parts.length === 0) return [];
  const keys: string[] = [];
  for (let i = 0; i < parts.length; i += 1) {
    const candidate = camelize(parts.slice(i).join("_"));
    if (!candidate) continue;
    // 业务命名空间已占用的同名键（如 resource:account.email）在 help 命名空间
    // 追加 Help 后缀，避免跨命名空间重复归属门禁失败。
    keys.push(`${scope}.${candidate}`, `${scope}.${candidate}Help`);
  }
  // 概念层兜底：同一语义字段（name/status/description…）在不同页面复用同一条
  // 真实说明，避免为每个页面重复登记，也避免落回通用占位文案。
  const leaf = camelize(parts[parts.length - 1]);
  if (leaf) keys.push(`concept.${leaf}`);
  return keys;
}

function fieldIdOf(label: HTMLLabelElement): string {
  const explicit = label.dataset.helpField;
  if (explicit) return explicit;
  const forId = label.getAttribute("for");
  if (!forId) return "";
  const control = document.getElementById(forId);
  const name = control?.getAttribute("name");
  return name || forId;
}

export function FieldHelpInjector() {
  const { pathname } = useLocation();
  const { t, i18n } = useTranslation("help");
  const [targets, setTargets] = useState<InjectionTarget[]>([]);
  const sequence = useRef(0);

  useEffect(() => {
    const scope = scopeFromPath(pathname);
    const createdSlots: HTMLElement[] = [];
    let timer: number | undefined;

    const labelText = (element: HTMLElement) =>
      (element.textContent ?? "").replace(/\*/g, "").replace(/\s+/g, " ").trim();

    const resolveKey = (element: HTMLElement): string | null => {
      const explicit =
        element.dataset.glHelpKey ??
        element.closest<HTMLElement>("[data-gl-help-key]")?.dataset.glHelpKey;
      if (explicit) return explicit;
      if (!(element instanceof HTMLLabelElement)) return null;
      const rawId = fieldIdOf(element);
      if (rawId) {
        const candidates = candidateKeys(scope, rawId);
        for (const key of candidates) {
          if (i18n.exists(key, { ns: "help" })) return key;
        }
        return candidates[candidates.length - 1] ?? null;
      }
      // 自定义控件没有 for/id（例如全局抽屉里的表单），退化为按标签文本匹配概念键。
      const fromLabel = camelize(labelText(element));
      if (!fromLabel) return null;
      const concept = `concept.${fromLabel}`;
      if (i18n.exists(concept, { ns: "help" })) return concept;
      return concept;
    };

    const scan = () => {
      // 直接用 document.body 作为根，确保 Antd Modal / Drawer portal（渲染在 body 下）也能被扫描到。
      const root = document.body;
      const next: InjectionTarget[] = [];

      const attach = (element: HTMLElement, helpKey: string, label: string) => {
        if (element.dataset.glHelpDone || !label) return;
        element.dataset.glHelpDone = "1";
        const slot = document.createElement("span");
        slot.className = "gl-help-slot";
        // 审计钩子：渲染结果可直接读出命中的 help 键，用于覆盖度巡检与 E2E 断言。
        slot.dataset.glHelpResolvedKey = helpKey;
        if (!i18n.exists(helpKey, { ns: "help" })) slot.dataset.glHelpFallback = "1";
        element.appendChild(slot);
        createdSlots.push(slot);
        sequence.current += 1;
        next.push({ id: `gl-help-${sequence.current}`, slot, helpKey, label });
      };

      root.querySelectorAll<HTMLElement>(LABEL_SELECTOR).forEach((label) => {
        if (label.dataset.glHelpDone) return;
        const helpKey = resolveKey(label);
        if (!helpKey) return;
        attach(label, helpKey, labelText(label));
      });

      root.querySelectorAll<HTMLElement>("[data-gl-help-key]").forEach((element) => {
        if (element.dataset.glHelpDone) return;
        if (element.matches(LABEL_SELECTOR)) return;
        const helpKey = element.dataset.glHelpKey;
        if (!helpKey) return;
        attach(element, helpKey, labelText(element));
      });

      if (next.length > 0) {
        setTargets((previous) => [...previous, ...next]);
      }
    };

    const schedule = () => {
      if (timer !== undefined) window.clearTimeout(timer);
      timer = window.setTimeout(scan, 150);
    };

    // 图标自身的 portal、Tooltip 浮层与下拉浮层会产生大量 DOM 变更；这些变更
    // 不会引入新的字段 label，直接跳过，避免每个动画帧都触发全页扫描。
    const IGNORED =
      ".gl-help-slot, .ant-tooltip, .ant-select-dropdown, .ant-picker-dropdown, .ant-dropdown";
    const isRelevantMutation = (record: MutationRecord): boolean => {
      if (record.type !== "childList") return false;
      const nodes = [...Array.from(record.addedNodes), ...Array.from(record.removedNodes)];
      if (nodes.length === 0) return false;
      return nodes.some((node) => {
        if (!(node instanceof HTMLElement)) return false;
        return !node.closest(IGNORED) && !node.querySelector(IGNORED);
      });
    };

    scan();
    const observer = new MutationObserver((records) => {
      if (records.some(isRelevantMutation)) schedule();
    });
    observer.observe(document.body, { childList: true, subtree: true });

    return () => {
      observer.disconnect();
      if (timer !== undefined) window.clearTimeout(timer);
      createdSlots.forEach((slot) => {
        slot.parentElement?.removeAttribute("data-gl-help-done");
        slot.remove();
      });
      setTargets((previous) => previous.filter((target) => !createdSlots.includes(target.slot)));
    };
  }, [pathname, i18n]);

  const portals = useMemo(
    () =>
      targets.map((target) =>
        createPortal(
          <HelpTip
            text={
              i18n.exists(target.helpKey, { ns: "help" })
                ? t(target.helpKey)
                : t("fallback.field", { label: target.label })
            }
            ariaLabel={t("fallback.ariaLabel", { label: target.label })}
          />,
          target.slot,
          target.id,
        ),
      ),
    [targets, i18n, t],
  );

  return <>{portals}</>;
}
