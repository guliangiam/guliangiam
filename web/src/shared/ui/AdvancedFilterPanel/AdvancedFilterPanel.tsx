import { Button, Card, Form } from "antd";
import { CloseOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import type { ReactNode } from "react";

type AdvancedFilterPanelProps = {
  visible: boolean;
  onClose: () => void;
  children: ReactNode;
};

export function AdvancedFilterPanel({ visible, onClose, children }: AdvancedFilterPanelProps) {
  const { t } = useTranslation("common");

  if (!visible) return null;

  return (
    <Card
      className="gl-filter-panel"
      variant="borderless"
      style={{ marginBottom: 16 }}
      title={t("search")}
      extra={
        <Button
          type="text"
          size="small"
          icon={<CloseOutlined />}
          onClick={onClose}
          aria-label={t("cancel")}
        />
      }
    >
      <Form layout="inline" style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
        {children}
      </Form>
    </Card>
  );
}
