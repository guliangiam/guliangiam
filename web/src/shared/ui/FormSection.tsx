import { useState, useEffect, useId, useRef, memo } from "react";
import DownOutlined from "@ant-design/icons/DownOutlined";
import RightOutlined from "@ant-design/icons/RightOutlined";
import { Badge } from "antd";

/**
 * 由表单壳在「校验失败且首个错误字段位于折叠分区内」时派发到分区根节点，
 * 折叠分区收到后展开自身，使错误字段可见、可聚焦、可滚动。
 */
export const FORM_SECTION_EXPAND_EVENT = "gl-form-section-expand";

interface FormSectionProps {
  title: string;
  description?: string;
  collapsible?: boolean;
  defaultCollapsed?: boolean;
  required?: boolean;
  badge?: string | number;
  style?: React.CSSProperties;
  extra?: React.ReactNode;
  /**
   * 字段列数。默认 1（纵向堆叠）；2 表示在宽屏下把本区块内的 Form.Item
   * 排成两栏，单行文本域与 `gl-form-item-full` 字段仍独占整行。
   */
  columns?: 1 | 2;
  children: React.ReactNode;
}

type HeaderContentProps = Pick<
  FormSectionProps,
  "title" | "description" | "required" | "badge" | "extra"
> & {
  toggle?: React.ReactNode;
};

function HeaderContent({ title, description, required, badge, extra, toggle }: HeaderContentProps) {
  return (
    <>
      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
        <div>
          {/* 标题是 h3，折叠按钮只包图标放在标题内，避免 button 包裹 h3 的非法内容模型。 */}
          <h3 className="gl-form-section__title">
            {toggle}
            {title}
            {required && (
              <span style={{ color: "var(--gl-color-error)", marginLeft: 4, fontSize: 12 }}>*</span>
            )}
          </h3>
          {description && <p className="gl-form-section__desc">{description}</p>}
        </div>
        {badge !== undefined && (
          <Badge
            count={badge}
            size="small"
            style={{ backgroundColor: "var(--gl-color-primary)", marginLeft: 4 }}
          />
        )}
      </div>
      {extra && <div style={{ marginLeft: "auto" }}>{extra}</div>}
    </>
  );
}

export const FormSection = memo(function FormSection({
  title,
  description,
  collapsible = false,
  defaultCollapsed = false,
  required,
  badge,
  style,
  extra,
  columns = 1,
  children,
}: FormSectionProps) {
  const [collapsed, setCollapsed] = useState(defaultCollapsed);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const root = rootRef.current;
    if (!root || !collapsible) return;
    const expand = () => setCollapsed(false);
    root.addEventListener(FORM_SECTION_EXPAND_EVENT, expand);
    return () => root.removeEventListener(FORM_SECTION_EXPAND_EVENT, expand);
  }, [collapsible]);

  /* WCAG 4.1.2 Name, Role, Value — stable id for aria-controls (A-05).
     useId() is stable across renders and SSR-safe, unlike a random value. */
  const bodyId = `form-section-body-${useId()}`;

  const toggle = collapsible ? (
    <button
      type="button"
      className="gl-form-section__toggle"
      aria-expanded={!collapsed}
      aria-controls={bodyId}
      aria-label={title}
      onClick={() => setCollapsed((v) => !v)}
      style={{
        cursor: "pointer",
        border: "none",
        background: "none",
        padding: 0,
        marginRight: 4,
        color: "inherit",
      }}
    >
      {collapsed ? <RightOutlined /> : <DownOutlined />}
    </button>
  ) : undefined;

  const header = (
    <HeaderContent
      title={title}
      description={description}
      required={required}
      badge={badge}
      extra={extra}
      toggle={toggle}
    />
  );

  return (
    <div ref={rootRef} className="gl-form-section" style={style}>
      {/* 非折叠时标题仍是 h3；折叠时 h3 在外、按钮只包图标（合法内容模型）。 */}
      <div className="gl-form-section__header">{header}</div>
      {/* 折叠时保留 DOM 节点，`aria-controls` 始终指向存在的合法节点。 */}
      <div
        id={bodyId}
        hidden={collapsed}
        className={
          columns === 2
            ? "gl-form-section__body gl-form-section__body--grid"
            : "gl-form-section__body"
        }
      >
        {children}
      </div>
    </div>
  );
});
