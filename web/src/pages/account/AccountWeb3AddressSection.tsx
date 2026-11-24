import { Descriptions, Tag, Typography } from "antd";
import { SafetyOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import { FormSection } from "@/shared/ui/FormSection";

type AccountWeb3AddressSectionProps = {
  deviceInfo?: Record<string, unknown>;
};

/**
 * AccountWeb3AddressSection — Liquid Auth EVM 派生地址卡片。
 *
 * 来源：Passkey 注册时后端自动 keccak256(pubkey || salt)[12:] 得到确定性 EVM 地址，
 * 写入 Session.DeviceInfo.web3Address。前端只读展示，不做任何钱包交互。
 *
 * 无钱包依赖、无助记词、抗钓鱼（WebAuthn origin 全等匹配）。
 */
export function AccountWeb3AddressSection({ deviceInfo }: AccountWeb3AddressSectionProps) {
  const { t } = useTranslation("common");
  const web3Address =
    typeof deviceInfo?.web3Address === "string" ? deviceInfo.web3Address : undefined;
  const derivation =
    typeof deviceInfo?.web3Derivation === "string" ? deviceInfo.web3Derivation : undefined;
  const loginMethod =
    typeof deviceInfo?.loginMethod === "string" ? deviceInfo.loginMethod : undefined;

  const explorerUrl = web3Address ? `https://etherscan.io/address/${web3Address}` : undefined;

  if (!web3Address) {
    return (
      <FormSection title={t("web3SectionTitle")}>
        <Typography.Text type="secondary">{t("web3NoAddress")}</Typography.Text>
      </FormSection>
    );
  }

  return (
    <FormSection
      title={t("web3SectionTitle")}
      extra={
        <Tag color="green" icon={<SafetyOutlined />}>
          {t("web3AntiPhishing")}
        </Tag>
      }
    >
      <Descriptions
        size="small"
        column={1}
        labelStyle={{ width: 120, color: "var(--ant-color-text-secondary)" }}
      >
        <Descriptions.Item label={t("web3DerivedAddress")}>
          <Typography.Text
            code
            copyable={{ text: web3Address, tooltips: [t("copy"), t("web3Copied")] }}
          >
            {web3Address}
          </Typography.Text>
          {explorerUrl && (
            <Typography.Link
              type="secondary"
              href={explorerUrl}
              target="_blank"
              rel="noreferrer"
              style={{ marginLeft: 12, fontSize: 12 }}
            >
              Etherscan →
            </Typography.Link>
          )}
        </Descriptions.Item>
        <Descriptions.Item label={t("web3DerivationMethod")}>
          <Tag>{derivation ?? "keccak256-passkey"}</Tag>
        </Descriptions.Item>
        {loginMethod && (
          <Descriptions.Item label={t("web3LoginMethod")}>
            <Tag color="blue">{loginMethod}</Tag>
          </Descriptions.Item>
        )}
        <Descriptions.Item label={t("web3Features")}>
          <Tag color="gold">{t("web3Deterministic")}</Tag>
          <Tag color="gold">{t("web3OneWay")}</Tag>
          <Tag color="gold">{t("web3NoWallet")}</Tag>
        </Descriptions.Item>
      </Descriptions>
    </FormSection>
  );
}
