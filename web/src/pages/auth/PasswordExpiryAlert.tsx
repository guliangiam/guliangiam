import React from "react";
import { Alert, Button, Modal, Form, Input } from "antd";
import { useTranslation } from "react-i18next";

interface PasswordExpiryAlertProps {
  isExpired: boolean;
  isExpiringSoon: boolean;
  daysRemaining: number;
  forceChange: boolean;
  onChangePassword: (oldPassword: string, newPassword: string) => Promise<boolean>;
}

const PasswordExpiryAlert: React.FC<PasswordExpiryAlertProps> = ({
  isExpired,
  isExpiringSoon,
  daysRemaining,
  forceChange,
  onChangePassword,
}) => {
  const { t } = useTranslation("auth");
  const [modalOpen, setModalOpen] = React.useState(false);
  const [loading, setLoading] = React.useState(false);
  const [form] = Form.useForm();

  if (!isExpired && !isExpiringSoon) {
    return null;
  }

  const handleChangePassword = async (values: {
    oldPassword: string;
    newPassword: string;
    confirmPassword: string;
  }) => {
    if (values.newPassword !== values.confirmPassword) {
      form.setFields([{ name: "confirmPassword", errors: [t("passwordExpiry.passwordMismatch")] }]);
      return;
    }

    setLoading(true);
    try {
      const ok = await onChangePassword(values.oldPassword, values.newPassword);
      if (ok) {
        setModalOpen(false);
        form.resetFields();
      }
    } finally {
      setLoading(false);
    }
  };

  if (isExpired && forceChange) {
    return (
      <Modal
        open
        title={t("passwordExpiry.forceChangeTitle")}
        closable={false}
        mask={{ closable: false }}
        footer={null}
      >
        <Alert
          type="error"
          title={t("passwordExpiry.expiredMessage")}
          style={{ marginBottom: 16 }}
        />
        <Form form={form} layout="vertical" onFinish={handleChangePassword}>
          <Form.Item
            name="oldPassword"
            label={t("passwordExpiry.oldPassword")}
            rules={[{ required: true, message: t("passwordExpiry.oldPasswordRequired") }]}
          >
            <Input.Password />
          </Form.Item>
          <Form.Item
            name="newPassword"
            label={t("passwordExpiry.newPassword")}
            rules={[{ required: true, message: t("passwordExpiry.newPasswordRequired") }]}
          >
            <Input.Password />
          </Form.Item>
          <Form.Item
            name="confirmPassword"
            label={t("passwordExpiry.confirmPassword")}
            rules={[{ required: true, message: t("passwordExpiry.confirmPasswordRequired") }]}
          >
            <Input.Password />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading} block>
              {t("passwordExpiry.changePassword")}
            </Button>
          </Form.Item>
        </Form>
      </Modal>
    );
  }

  return (
    <>
      <Alert
        type={isExpired ? "error" : "warning"}
        title={
          isExpired
            ? t("passwordExpiry.expiredMessage")
            : t("passwordExpiry.expiringSoonMessage", { days: daysRemaining })
        }
        action={
          <Button size="small" type="primary" onClick={() => setModalOpen(true)}>
            {t("passwordExpiry.changeNow")}
          </Button>
        }
        showIcon
        style={{ marginBottom: 16 }}
      />
      <Modal
        open={modalOpen}
        title={t("passwordExpiry.changePasswordTitle")}
        onCancel={() => setModalOpen(false)}
        footer={null}
      >
        <Form form={form} layout="vertical" onFinish={handleChangePassword}>
          <Form.Item
            name="oldPassword"
            label={t("passwordExpiry.oldPassword")}
            rules={[{ required: true, message: t("passwordExpiry.oldPasswordRequired") }]}
          >
            <Input.Password />
          </Form.Item>
          <Form.Item
            name="newPassword"
            label={t("passwordExpiry.newPassword")}
            rules={[{ required: true, message: t("passwordExpiry.newPasswordRequired") }]}
          >
            <Input.Password />
          </Form.Item>
          <Form.Item
            name="confirmPassword"
            label={t("passwordExpiry.confirmPassword")}
            rules={[{ required: true, message: t("passwordExpiry.confirmPasswordRequired") }]}
          >
            <Input.Password />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading} block>
              {t("passwordExpiry.changePassword")}
            </Button>
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
};

export default PasswordExpiryAlert;
