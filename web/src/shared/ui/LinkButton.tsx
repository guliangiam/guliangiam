import type { ReactNode } from "react";
import { Button, type ButtonProps } from "antd";
import { useNavigate } from "react-router-dom";

type LinkButtonProps = Omit<ButtonProps, "href" | "onClick"> & {
  /** Router destination. Rendered as a single button so no interactive element nests inside another. */
  to: string;
  children: ReactNode;
};

/**
 * LinkButton — 单个可聚焦元素完成「导航 + 按钮外观」。
 *
 * 直接写 `<Button><Link to=...>…</Link></Button>` 会把可聚焦的 <a> 嵌进 <button>，
 * 产生嵌套交互控件（axe `nested-interactive`，serious），键盘与读屏都会遇到
 * 双层激活语义。本组件用 useNavigate 保留 SPA 跳转，同时只渲染一个 <button>。
 */
export function LinkButton({ to, children, ...rest }: LinkButtonProps) {
  const navigate = useNavigate();
  return (
    <Button {...rest} onClick={() => navigate(to)}>
      {children}
    </Button>
  );
}
