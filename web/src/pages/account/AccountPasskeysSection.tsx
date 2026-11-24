import { useState } from "react";
import { Alert, App, Button, Empty, Popconfirm, Space, Table, Tag, Typography } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { FormSection } from "@/shared/ui/FormSection";
import { registrationErrorMessageKey } from "@/shared/lib/webauthn-error";
import {
  base64UrlToArrayBuffer,
  credentialToRegistrationFinishRequest,
  isWebAuthnSupported,
} from "@/shared/lib/webauthn-b64";
import {
  finishOwnWebAuthnRegistration,
  listOwnWebAuthnCredentials,
  revokeOwnWebAuthnCredential,
  webAuthnRegistrationBegin,
  type WebAuthnCredentialSummary,
} from "@/shared/api/iam/auth-webauthn";

type AccountPasskeysSectionProps = {
  organizationSlug: string;
  username: string;
};

const STATUS_COLOR: Record<string, string> = {
  active: "green",
  revoked: "red",
  deactivated: "orange",
};

const STATUS_LABEL_KEY: Record<string, string> = {
  active: "user.webauthn.statusActive",
  revoked: "user.webauthn.statusRevoked",
  deactivated: "user.webauthn.statusDeactivated",
};

export function AccountPasskeysSection({
  organizationSlug,
  username,
}: AccountPasskeysSectionProps) {
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const [registering, setRegistering] = useState(false);

  const credentialsQuery = useQuery({
    queryKey: ["auth", "webauthn", "credentials"],
    queryFn: () => listOwnWebAuthnCredentials(),
    enabled: !!organizationSlug && !!username,
  });
  const credentials = credentialsQuery.data ?? [];

  async function invalidateAfterWrite() {
    // 凭据写入级联到本人会话与令牌；finish 还会轮换当前会话。
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["auth", "webauthn", "credentials"] }),
      queryClient.invalidateQueries({ queryKey: ["user"] }),
      queryClient.invalidateQueries({ queryKey: ["session"] }),
      queryClient.invalidateQueries({ queryKey: ["auth", "account"] }),
    ]);
  }

  const revokeMutation = useMutation({
    mutationFn: (id: string) => revokeOwnWebAuthnCredential(id),
    onSuccess: async () => {
      message.success(t("user.webauthn.updated"));
      await invalidateAfterWrite();
    },
    onError: () => message.error(tc("operationFailed")),
  });

  async function registerPasskey() {
    if (!isWebAuthnSupported()) {
      message.error(t("user.webauthn.unsupported"));
      return;
    }
    setRegistering(true);
    try {
      const begin = await webAuthnRegistrationBegin(organizationSlug, username);
      const publicKey: PublicKeyCredentialCreationOptions = {
        rp: { id: begin.rpId, name: begin.rpName },
        user: {
          id: base64UrlToArrayBuffer(begin.userId),
          name: begin.userDisplayName || username,
          displayName: begin.userDisplayName || username,
        },
        // 后端比对 clientDataJSON.challenge 与 base64url(utf8(challengeString))，
        // 故挑战取 base64url 字符串的 UTF-8 字节，而非其解码字节序列。
        challenge: new TextEncoder().encode(begin.challenge),
        // 后端 COSE 解析仅接受 EC2/P-256/ES256，只 advertised ES256。
        pubKeyCredParams: [{ type: "public-key", alg: -7 }],
        timeout: begin.timeout || 60_000,
        attestation: "none",
        authenticatorSelection: {
          residentKey: "preferred",
          requireResidentKey: false,
          userVerification: begin.userVerification,
        },
        ...(begin.excludeCredentials?.length
          ? {
              excludeCredentials: begin.excludeCredentials.map((id) => ({
                type: "public-key" as const,
                id: base64UrlToArrayBuffer(id),
              })),
            }
          : {}),
      };

      const created = (await navigator.credentials.create({
        publicKey,
      })) as PublicKeyCredential | null;
      if (!created) throw new Error("registration cancelled without a credential");

      const body = credentialToRegistrationFinishRequest(
        created,
        begin.challengeId,
        begin.challenge,
        organizationSlug,
        username,
      );
      const saved = await finishOwnWebAuthnRegistration(body);
      if (saved.credentialId !== created.id) {
        message.warning(t("user.webauthn.credentialMismatch"));
      } else {
        message.success(t("user.webauthn.registered"));
      }
      await invalidateAfterWrite();
    } catch (err) {
      message.error(t(registrationErrorMessageKey(err)));
    } finally {
      setRegistering(false);
    }
  }

  const columns = [
    { title: t("user.webauthn.name"), dataIndex: "name", key: "name" },
    {
      title: t("user.webauthn.status"),
      dataIndex: "status",
      key: "status",
      width: 130,
      render: (status: string) => (
        <Tag color={STATUS_COLOR[status] ?? "default"}>{t(STATUS_LABEL_KEY[status] ?? status)}</Tag>
      ),
    },
    {
      title: t("user.webauthn.aaguid"),
      dataIndex: "aaguid",
      key: "aaguid",
      width: 130,
      render: (value: string) => <Tag>{value ? `${value.slice(0, 8)}...` : "-"}</Tag>,
    },
    { title: t("user.webauthn.signCount"), dataIndex: "signCount", key: "signCount", width: 100 },
    {
      title: t("user.webauthn.createdAt"),
      dataIndex: "createdAt",
      key: "createdAt",
      render: (value: string) => (value ? new Date(value).toLocaleString() : "-"),
    },
    {
      title: tc("actions"),
      key: "actions",
      width: 120,
      render: (_: unknown, record: WebAuthnCredentialSummary) =>
        record.status === "active" ? (
          <Popconfirm
            title={t("user.webauthn.revokeConfirm")}
            okText={t("user.webauthn.revoke")}
            cancelText={tc("cancel")}
            okButtonProps={{ danger: true }}
            onConfirm={() => revokeMutation.mutate(record.id)}
          >
            <Button type="link" size="small" danger loading={revokeMutation.isPending}>
              {t("user.webauthn.revoke")}
            </Button>
          </Popconfirm>
        ) : null,
    },
  ];

  return (
    <FormSection title={t("account.passkeySection")}>
      <Space orientation="vertical" size="middle" style={{ width: "100%" }}>
        <Space style={{ width: "100%", justifyContent: "space-between" }}>
          <Typography.Text type="secondary">{t("user.webauthn.addHint")}</Typography.Text>
          <Button type="primary" loading={registering} onClick={() => void registerPasskey()}>
            {t("user.webauthn.add")}
          </Button>
        </Space>

        {!isWebAuthnSupported() && (
          <Alert type="warning" showIcon title={t("user.webauthn.unsupported")} />
        )}

        {!credentialsQuery.isLoading && credentials.length === 0 ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={
              <Space orientation="vertical" size={4}>
                <Typography.Text strong>{t("user.noWebauthn")}</Typography.Text>
                <Typography.Text type="secondary">
                  {t("user.webauthn.emptyDescription")}
                </Typography.Text>
              </Space>
            }
          >
            <Button type="primary" loading={registering} onClick={() => void registerPasskey()}>
              {t("user.webauthn.add")}
            </Button>
          </Empty>
        ) : (
          <Table<WebAuthnCredentialSummary>
            dataSource={credentials}
            columns={columns}
            loading={credentialsQuery.isLoading}
            pagination={false}
            size="small"
            rowKey="id"
          />
        )}
      </Space>
    </FormSection>
  );
}
