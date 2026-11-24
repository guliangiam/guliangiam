import { Button, Dropdown } from "antd";
import { EllipsisOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import type { ReactNode } from "react";

type QuickActionItem = {
  key: string;
  label: string;
  icon?: ReactNode;
  danger?: boolean;
  onClick: () => void;
};

type QuickActionsProps = {
  items: QuickActionItem[];
};

export function QuickActions({ items }: QuickActionsProps) {
  const { t } = useTranslation("common");

  const menuItems = items.map((item) => ({
    key: item.key,
    label: item.label,
    icon: item.icon,
    danger: item.danger,
    onClick: item.onClick,
  }));

  return (
    <Dropdown menu={{ items: menuItems }} trigger={["click"]}>
      <Button
        type="text"
        /* WCAG 4.1.2 — native button provides keyboard activation (A-03) */
        icon={<EllipsisOutlined style={{ fontSize: 18 }} />}
        aria-label={t("moreActions")}
      />
    </Dropdown>
  );
}
