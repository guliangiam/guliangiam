import { memo } from "react";
import { Button, Dropdown, Tooltip } from "antd";
import DownOutlined from "@ant-design/icons/DownOutlined";
import type { ColumnsType } from "antd/es/table";
import type { ReactNode } from "react";

type ColumnMenuProps = {
  // Each page supplies its own row type; this menu only reads dataIndex/title.
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  columns: ColumnsType<any>;
  visible: Set<string>;
  toggle: (dataIndex: string) => void;
};

/**
 * Dropdown that lets the user toggle column visibility.
 * Columns without a dataIndex (selection, expandIcon, etc.) are excluded.
 */
export const ColumnMenu = memo(function ColumnMenu({ columns, visible, toggle }: ColumnMenuProps) {
  const columnItems = columns
    .filter((c) => "dataIndex" in c && c.dataIndex !== undefined && String(c.dataIndex).length > 0)
    .map((c) => {
      const di = String((c as { dataIndex?: unknown }).dataIndex);
      const title = (c as { title?: ReactNode }).title;
      const label = typeof title === "string" ? title : di;
      return {
        key: di,
        label,
        checked: visible.has(di),
        onClick: () => toggle(di),
      };
    });
  const menuLabel = "Manage columns";
  return (
    <Dropdown
      menu={{
        selectable: true,
        items: columnItems,
      }}
      trigger={["click"]}
      aria-label={menuLabel}
    >
      <Tooltip title={menuLabel}>
        <Button aria-label={menuLabel} icon={<DownOutlined />} />
      </Tooltip>
    </Dropdown>
  );
});
