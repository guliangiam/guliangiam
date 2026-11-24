import { Modal, Typography } from "antd";
import { useTranslation } from "react-i18next";
import { sanitizeHtml } from "@/shared/lib/dom-purify";

type LoginTermsModalProps = {
  open: boolean;
  termsOfUse?: string | null;
  onCancel: () => void;
};

export function LoginTermsModal(props: LoginTermsModalProps) {
  const { onCancel, open, termsOfUse } = props;
  const { t } = useTranslation("auth");

  return (
    <Modal
      destroyOnHidden
      open={open}
      title={t("termsTitle")}
      width={760}
      footer={null}
      onCancel={onCancel}
    >
      {termsOfUse ? (
        <div dangerouslySetInnerHTML={{ __html: sanitizeHtml(termsOfUse) }} />
      ) : (
        <Typography.Paragraph style={{ marginBottom: 0 }} type="secondary">
          {t("noTermsConfigured")}
        </Typography.Paragraph>
      )}
    </Modal>
  );
}
