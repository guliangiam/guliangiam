import "./GlForm.css";

import React, { useCallback, useEffect, useRef, useState } from "react";
import { Form, Card, Space, Button, Spin, Alert, Modal } from "antd";
import type { FormInstance } from "antd";
import { useTranslation } from "react-i18next";

/**
 * GlEditShell — 编辑页母版。
 *
 * 超越 Casdoor 的核心承诺：内置草稿自动保存（debounce 2s → localStorage）
 * + 离开页面确认。Casdoor 只有布局壳，没有状态持久化。
 *
 * 布局结构：
 *   ┌─────────────────────────────────────┐
 *   │ Card title + Extra actions          │
 *   │ ┌─────────────────────────────────┐ │
 *   │ │ Form children (GlFormGrid等)     │ │
 *   │ └─────────────────────────────────┘ │
 *   │ Sticky action bar (底部固定)        │
 *   │   Save | Cancel | (Dirty draft tag) │
 *   └─────────────────────────────────────┘
 */
export interface GlEditShellProps {
  /** 草稿持久化 key：默认 `gl:draft:{route}:{entityId}` */
  draftKey?: string;
  /** entityId — 和 route 组成默认 draftKey，undefined 时不保存草稿 */
  entityId?: string;
  /** route name — 组成默认 draftKey */
  route?: string;
  /** 是否允许草稿持久化（设 false 或 entityId undefined 时禁用） */
  enableDraft?: boolean;
  /** 表单实例（antd Form.useForm） */
  form: FormInstance;
  /** 保存函数 */
  onSave: () => Promise<void> | void;
  /** 取消函数 */
  onCancel: () => void;
  /** Card title */
  title?: React.ReactNode;
  /** Card 右上角额外 actions（在 Save/Cancel 之外） */
  extra?: React.ReactNode;
  /** 提交按钮 loading（受控） */
  loading?: boolean;
  /** 表单 children */
  children: React.ReactNode;
}

const DRAFT_DEBOUNCE_MS = 2000;

export const GlEditShell: React.FC<GlEditShellProps> = ({
  draftKey,
  entityId,
  route,
  enableDraft = true,
  form,
  onSave,
  onCancel,
  title,
  extra,
  loading = false,
  children,
}) => {
  const { t } = useTranslation("common");
  const [hasDraft, setHasDraft] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [touched, setTouched] = useState(false); // antd isFieldsTouched() 非响应式，手动维护
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // 计算最终草稿 key
  const resolvedDraftKey = React.useMemo(() => {
    if (draftKey) return draftKey;
    if (!enableDraft || !entityId) return null;
    return `gl:draft:${route ?? "unknown"}:${entityId}`;
  }, [draftKey, enableDraft, entityId, route]);

  // 挂载时尝试恢复草稿
  useEffect(() => {
    if (!resolvedDraftKey) return;
    try {
      const raw = localStorage.getItem(resolvedDraftKey);
      if (raw) {
        const draft = JSON.parse(raw);
        if (draft && typeof draft === "object" && Object.keys(draft).length > 0) {
          form.setFieldsValue(draft);
          // oxlint-disable-next-line react/set-state-in-effect -- 从 localStorage 恢复草稿是外部存储同步点
          setHasDraft(true);
        }
      }
    } catch {
      // 坏草稿 → 丢弃，不阻塞页面
      localStorage.removeItem(resolvedDraftKey);
    }
  }, [resolvedDraftKey, form]);

  // 监听 Form 变化，debounce 后存草稿
  const handleValuesChange = useCallback(
    (_changed: unknown, allValues: Record<string, unknown>) => {
      setTouched(true); // antd isFieldsTouched() 非响应式，手动维护
      if (!resolvedDraftKey) {
        setDirty(true);
        return;
      }
      setDirty(true);
      if (debounceRef.current) clearTimeout(debounceRef.current);
      debounceRef.current = setTimeout(() => {
        try {
          localStorage.setItem(resolvedDraftKey, JSON.stringify(allValues));
          setHasDraft(true);
        } catch {
          // quota 超限等 → 静默失败
        }
      }, DRAFT_DEBOUNCE_MS);
    },
    [resolvedDraftKey],
  );

  // 组件卸载时清理 debounce timer
  useEffect(
    () => () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    },
    [],
  );

  // 离开确认：dirty 状态下阻止路由切换
  useEffect(() => {
    if (!dirty) return;
    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = "";
    };
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [dirty]);

  const handleSave = useCallback(async () => {
    await onSave();
    // 保存成功 → 清草稿 + 清 dirty
    if (resolvedDraftKey) {
      localStorage.removeItem(resolvedDraftKey);
      setHasDraft(false);
    }
    setDirty(false);
    setTouched(false); // 保存成功 → Save 按钮重新 disabled
  }, [onSave, resolvedDraftKey]);

  const handleCancel = useCallback(() => {
    if (dirty) {
      Modal.confirm({
        title: t("discardChangesTitle", "放弃未保存的修改？"),
        content: t("discardChangesContent", "草稿已自动保存到本地，下次进入时可恢复。"),
        okText: t("discard", "放弃"),
        cancelText: t("stay", "留下继续编辑"),
        okButtonProps: { danger: true },
        onOk: () => {
          setDirty(false);
          onCancel();
        },
      });
    } else {
      onCancel();
    }
  }, [dirty, onCancel, t]);

  return (
    <div className="gl-edit-shell">
      <Card
        className="gl-edit-shell__card"
        title={title}
        extra={
          <Space>
            {hasDraft && (
              <Alert
                type="info"
                title={t("autoSavedDraft", "草稿已自动保存")}
                showIcon
                style={{ paddingInline: 8 }}
              />
            )}
            {extra}
          </Space>
        }
      >
        <Spin spinning={loading}>
          <Form form={form} layout="vertical" onValuesChange={handleValuesChange}>
            {children}
          </Form>
        </Spin>
      </Card>

      {/* Sticky action bar */}
      <div className="gl-edit-shell__action-bar">
        <Space>
          <Button onClick={handleCancel} disabled={loading}>
            {t("cancel", "取消")}
          </Button>
          <Button type="primary" onClick={handleSave} loading={loading} disabled={!touched}>
            {t("save", "保存")}
            {dirty && <span className="gl-edit-shell__dirty-dot" aria-hidden />}
          </Button>
        </Space>
      </div>
    </div>
  );
};
