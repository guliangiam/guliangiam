import { Button, Input } from "antd";
import { PlusOutlined, SearchOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import type { ReactNode } from "react";

type ListToolbarProps = {
  searchValue?: string;
  onSearchChange?: (value: string) => void;
  createLabel?: string;
  onCreate?: () => void;
  extra?: ReactNode;
  loading?: boolean;
};

export function ListToolbar({
  searchValue,
  onSearchChange,
  createLabel,
  onCreate,
  extra,
  loading,
}: ListToolbarProps) {
  const { t } = useTranslation("common");

  return (
    <div
      className="gl-list-toolbar"
      style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 16, flexWrap: "wrap" }}
    >
      <Input.Search
        prefix={<SearchOutlined />}
        placeholder={t("search")}
        aria-label={t("search")}
        value={searchValue}
        onChange={(e) => onSearchChange?.(e.target.value)}
        onSearch={(value) => onSearchChange?.(value)}
        allowClear
        style={{ maxWidth: 320, flex: "1 1 240px" }}
        loading={loading}
      />
      <div style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 8 }}>
        {extra}
        {onCreate && (
          <Button type="primary" icon={<PlusOutlined />} onClick={onCreate}>
            {createLabel ?? t("create")}
          </Button>
        )}
      </div>
    </div>
  );
}
