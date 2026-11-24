import { useMemo } from "react";
import { Button, Modal, Space, Tabs } from "antd";
import { useTranslation } from "react-i18next";
import { sanitizeHtml } from "@/shared/lib/dom-purify";
import { defaultPrivacyPolicyHtml, defaultTermsOfUseHtml } from "@/shared/lib/legal-documents";

type SignupAgreementModalProps = {
  open: boolean;
  /** 应用自定义使用条款（HTML）；为空时回退平台默认条款。 */
  termsOfUse?: string | null;
  /** 应用自定义隐私政策（HTML）；为空时回退平台默认政策。 */
  privacyPolicy?: string | null;
  onAgree: () => void;
  onCancel: () => void;
};

/**
 * 注册协议确认弹窗：未勾选同意即点击注册时弹出，展示《使用条款》《隐私政策》，
 * 用户点击“同意并继续”后由调用方置勾选态并提交注册。正文经 sanitizeHtml 净化。
 */
export function SignupAgreementModal({
  open,
  termsOfUse,
  privacyPolicy,
  onAgree,
  onCancel,
}: SignupAgreementModalProps) {
  const { t, i18n } = useTranslation("auth");
  const { t: tc } = useTranslation("common");
  const lang = i18n.language;

  const termsHtml = useMemo(
    () => sanitizeHtml(termsOfUse?.trim() ? termsOfUse : defaultTermsOfUseHtml(lang)),
    [termsOfUse, lang],
  );
  const privacyHtml = useMemo(
    () => sanitizeHtml(privacyPolicy?.trim() ? privacyPolicy : defaultPrivacyPolicyHtml(lang)),
    [privacyPolicy, lang],
  );

  return (
    <Modal
      destroyOnHidden
      open={open}
      title={t("termsAndPrivacyTitle")}
      width={760}
      onCancel={onCancel}
      footer={
        <Space>
          <Button onClick={onCancel}>{tc("cancel")}</Button>
          <Button type="primary" onClick={onAgree}>
            {t("agreeAndContinue")}
          </Button>
        </Space>
      }
    >
      <Tabs
        defaultActiveKey="terms"
        items={[
          {
            key: "terms",
            label: t("termsOfUse"),
            children: <div dangerouslySetInnerHTML={{ __html: termsHtml }} />,
          },
          {
            key: "privacy",
            label: t("privacyTitle"),
            children: <div dangerouslySetInnerHTML={{ __html: privacyHtml }} />,
          },
        ]}
      />
    </Modal>
  );
}
