import { useState } from "react";
import { App, Button } from "antd";
import { ApiOutlined, CheckCircleOutlined, CloseCircleOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import { testProviderConnection } from "@/shared/api/iam/provider-extended";
import { localizedError } from "@/shared/lib/server-error";

type TestConnectionButtonProps = {
  providerId: string;
  /** 提供商所属组织：详情级端点要求可信租户，平台管理员必须显式携带。 */
  organizationId?: string;
  disabled?: boolean;
};

export function TestConnectionButton({
  providerId,
  organizationId,
  disabled,
}: TestConnectionButtonProps) {
  const { t } = useTranslation("common");
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);

  const handleTest = async () => {
    setLoading(true);
    try {
      const result = await testProviderConnection(providerId, organizationId);
      if (result.connected) {
        message.success({
          content: t("provider.testConnectionSuccess"),
          icon: <CheckCircleOutlined />,
        });
      } else {
        message.error({
          content: result.error || t("provider.testConnectionFailed"),
          icon: <CloseCircleOutlined />,
          duration: 6,
        });
      }
    } catch (err) {
      message.error({
        content: localizedError(err, t, "provider.testConnectionFailed"),
        icon: <CloseCircleOutlined />,
        duration: 6,
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Button
      icon={<ApiOutlined />}
      loading={loading}
      onClick={handleTest}
      disabled={disabled || !providerId}
      title={t("providerTestConnectionProbeNote")}
    >
      {t("provider.testConnection")}
    </Button>
  );
}
