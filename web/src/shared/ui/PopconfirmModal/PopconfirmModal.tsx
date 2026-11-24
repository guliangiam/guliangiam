import { useState } from "react";
import { Button, Popconfirm, Alert } from "antd";
import { ExclamationCircleOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";

type PopconfirmModalProps = {
  /** Main confirmation text (required). */
  title: string;
  /** Optional secondary description paragraph. */
  description?: React.ReactNode;
  /** Handler — supports async (Promise). Shows loading state while running. */
  onConfirm: () => void | Promise<void>;
  /** Disables the button entirely. */
  disabled?: boolean;
  /** Button label (default: i18n "delete"). */
  text?: string;
  /** Button size (default: "middle"). */
  size?: "small" | "middle" | "large";
  /** Inline style override. */
  style?: React.CSSProperties;
  /** Confirm button text override (default: i18n "ok"). */
  confirmText?: string;
  /** Cancel button text override (default: i18n "cancel"). */
  cancelText?: string;
  /** Show warning alert above the confirm text (default: true for destructive). */
  showWarning?: boolean;
  /** Button type — "primary" danger for destructive, "default" for plain confirm. */
  variant?: "danger" | "default";
};

/**
 * PopconfirmModal – enhanced danger-action confirmation button.
 *
 * A5-B3 enhancement over original:
 *   ✅ async onConfirm with built-in loading state (legacy ConfirmButton parity)
 *   ✅ description prop for multi-paragraph confirmation
 *   ✅ confirmText / cancelText overrides
 *   ✅ optional warning Alert above title
 *   ✅ variant: "danger" (default) or "default"
 *
 * Usage:
 *   <PopconfirmModal
 *     title="Delete these 23 users?"
 *     description="This action is irreversible."
 *     onConfirm={async () => await batchDelete(ids)}
 *   >
 *     Delete 23 Users
 *   </PopconfirmModal>
 */
export const PopconfirmModal = ({
  title,
  description,
  onConfirm,
  disabled,
  text,
  size = "middle",
  style,
  confirmText,
  cancelText,
  showWarning = true,
  variant = "danger",
}: PopconfirmModalProps) => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);

  const handleConfirm = async () => {
    setLoading(true);
    try {
      const result = onConfirm();
      if (result instanceof Promise) {
        await result;
      }
      setOpen(false);
    } finally {
      setLoading(false);
    }
  };

  const buttonType = variant === "danger" ? ("primary" as const) : ("default" as const);
  const buttonDanger = variant === "danger";

  // Popconfirm title — combine optional warning + title + description
  const titleContent = (
    <div style={{ maxWidth: 320 }}>
      {showWarning && variant === "danger" && (
        <Alert
          type="warning"
          showIcon
          icon={<ExclamationCircleOutlined />}
          title={title}
          style={{ marginBottom: description ? 8 : 0, padding: "4px 8px", fontSize: 13 }}
        />
      )}
      {(!showWarning || variant !== "danger") && <div style={{ fontWeight: 500 }}>{title}</div>}
      {description && (
        <div
          style={{
            fontSize: 13,
            color: "var(--gl-color-text-tertiary)",
            marginTop: 6,
          }}
        >
          {description}
        </div>
      )}
    </div>
  );

  return (
    <Popconfirm
      open={open}
      onOpenChange={setOpen}
      title={titleContent}
      onConfirm={handleConfirm}
      disabled={disabled}
      okText={confirmText ?? t("ok")}
      cancelText={cancelText ?? t("cancel")}
      okButtonProps={{
        danger: buttonDanger,
        loading,
      }}
    >
      <Button style={style} size={size} disabled={disabled} type={buttonType} danger={buttonDanger}>
        {text ?? t("delete")}
      </Button>
    </Popconfirm>
  );
};

export default PopconfirmModal;
