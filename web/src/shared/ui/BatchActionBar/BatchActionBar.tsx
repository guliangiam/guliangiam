import { Button, Space, Tag } from "antd";
import { CloseOutlined } from "@ant-design/icons";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

type BatchActionBarProps = {
  /** Number of rows currently selected. The bar renders nothing at zero. */
  selectedCount: number;
  /** Action buttons shown next to the count (delete, enable, export, …). */
  children?: ReactNode;
  /** Clear the selection. Rendered as the trailing icon button. */
  onClearSelection: () => void;
};

/**
 * BatchActionBar — 列表批量操作栏。
 *
 * 只负责布局与"已选 N 项 / 取消选择"两件事；具体动作由调用方以 children 传入，
 * 这样破坏性动作可以使用自己的确认弹窗（PopconfirmModal），非破坏性动作用普通按钮。
 */
export function BatchActionBar({ selectedCount, children, onClearSelection }: BatchActionBarProps) {
  const { t } = useTranslation("common");

  if (selectedCount === 0) return null;

  return (
    <div
      className="gl-batch-action-bar"
      role="toolbar"
      aria-label={t("selected", { count: selectedCount })}
    >
      <Tag color="blue">{t("selected", { count: selectedCount })}</Tag>
      <Space size="small" wrap>
        {children}
      </Space>
      <Button
        type="text"
        size="small"
        className="gl-batch-action-bar__clear"
        icon={<CloseOutlined />}
        onClick={onClearSelection}
        aria-label={t("cancel")}
      />
    </div>
  );
}
