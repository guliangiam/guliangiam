import { Card, Divider } from "antd";
import type { ReactNode } from "react";

/**
 * 子菜单标签页/分区的「整体卡片」容器。
 *
 * 规范：一个标签页只有一层卡片；卡内多个功能单元用细横线分隔
 * （多个 `FormSection` 由 `.gl-tab-card` 规则自动扁平化并用 border-top 分隔，
 * 非 FormSection 单元使用 `TabCardDivider`）。禁止在 `TabCard` 内再嵌套卡片。
 */
export interface TabCardProps {
  children: ReactNode;
  title?: ReactNode;
  extra?: ReactNode;
  className?: string;
}

export function TabCard({ children, title, extra, className }: TabCardProps) {
  return (
    <Card
      className={className ? `gl-tab-card ${className}` : "gl-tab-card"}
      title={title}
      extra={extra}
      variant="outlined"
    >
      {children}
    </Card>
  );
}

/** 卡片内功能单元之间的统一细横线。 */
export function TabCardDivider() {
  return <Divider className="gl-tab-card__divider" />;
}
