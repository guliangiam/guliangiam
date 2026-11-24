import { memo } from "react";
import { Button, Space } from "antd";
import { useTranslation } from "react-i18next";

type StickyFormFooterProps = {
  loading?: boolean;
  onSave?: () => void;
  onSaveAndExit?: () => void;
  onReset?: () => void;
  saveText?: string;
  saveAndExitText?: string;
  resetText?: string;
  extra?: React.ReactNode;
};

export const StickyFormFooter = memo(function StickyFormFooter({
  loading,
  onSave,
  onSaveAndExit,
  onReset,
  saveText,
  saveAndExitText,
  resetText,
  extra,
}: StickyFormFooterProps) {
  const { t } = useTranslation("common");

  return (
    <div className="gl-sticky-footer">
      <div className="gl-sticky-footer__inner">
        {extra}
        <Space style={{ marginLeft: "auto" }}>
          {onReset && <Button onClick={onReset}>{resetText ?? t("reset")}</Button>}
          {onSave && (
            <Button type="primary" loading={loading} onClick={onSave} disabled={loading}>
              {saveText ?? t("save")}
            </Button>
          )}
          {onSaveAndExit && (
            <Button loading={loading} onClick={onSaveAndExit} disabled={loading}>
              {saveAndExitText ?? t("saveAndExit")}
            </Button>
          )}
        </Space>
      </div>
    </div>
  );
});
