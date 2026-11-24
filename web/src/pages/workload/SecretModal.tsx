import { Alert, App, Button, Descriptions, Modal, Space, Typography } from "antd";
import CopyOutlined from "@ant-design/icons/CopyOutlined";
import { useTranslation } from "react-i18next";
import type { ServiceAccountCredentials } from "@/shared/api/iam/identity-pools";

type SecretModalProps = {
  credentials: ServiceAccountCredentials | null;
  onClose: () => void;
};

export function SecretModal({ credentials, onClose }: SecretModalProps) {
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();

  const copy = async (value: string, label: string) => {
    try {
      await navigator.clipboard.writeText(value);
      message.success(tc("workload.copied", { label, defaultValue: `${label} copied` }));
    } catch {
      message.error(tc("workload.copyFailed", { defaultValue: "Failed to copy credentials" }));
    }
  };

  if (!credentials) return null;

  return (
    <Modal
      open
      title={tc("workload.oneTimeCredentials", { defaultValue: "Service account credentials" })}
      onCancel={onClose}
      mask={{ closable: false }}
      destroyOnHidden
      footer={
        <Button type="primary" onClick={onClose}>
          {tc("workload.credentialsSaved", { defaultValue: "I saved the credentials" })}
        </Button>
      }
    >
      {credentials && (
        <Space orientation="vertical" size="large" style={{ display: "flex" }}>
          <Alert
            type="warning"
            showIcon
            title={tc("workload.copyCredentialsNow", {
              defaultValue: "Copy these credentials now",
            })}
            description={tc("workload.oneTimeSecretWarning", {
              defaultValue:
                "The client secret is shown only once and cannot be retrieved after this dialog closes.",
            })}
          />
          <Descriptions bordered size="small" column={1}>
            <Descriptions.Item label={tc("workload.clientId", { defaultValue: "Client ID" })}>
              <Space>
                <Typography.Text code>{credentials.clientId}</Typography.Text>
                <Button
                  size="small"
                  icon={<CopyOutlined />}
                  aria-label={tc("workload.copyClientId", { defaultValue: "Copy Client ID" })}
                  onClick={() => void copy(credentials.clientId, tc("workload.clientId"))}
                />
              </Space>
            </Descriptions.Item>
            <Descriptions.Item
              label={tc("workload.clientSecret", { defaultValue: "Client Secret" })}
            >
              <Space>
                <Typography.Text code>{credentials.clientSecretPlain}</Typography.Text>
                <Button
                  size="small"
                  icon={<CopyOutlined />}
                  aria-label={tc("workload.copyClientSecret", {
                    defaultValue: "Copy Client Secret",
                  })}
                  onClick={() =>
                    void copy(credentials.clientSecretPlain, tc("workload.clientSecret"))
                  }
                />
              </Space>
            </Descriptions.Item>
          </Descriptions>
        </Space>
      )}
    </Modal>
  );
}
