import {
  Alert,
  App,
  Button,
  Descriptions,
  Drawer,
  Empty,
  Form,
  Input,
  Modal,
  Popconfirm,
  Space,
  Table,
  Tag,
  Typography,
} from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { queryKeys } from "@/shared/api/query-keys";
import { registrationErrorMessageKey } from "@/shared/lib/webauthn-error";
import {
  beginWebAuthnCredentialRegistration,
  deactivateWebAuthnCredential,
  deleteWebAuthnCredential,
  finishWebAuthnCredentialRegistration,
  getWebAuthnCredential,
  listWebAuthnCredentials,
  renameWebAuthnCredential,
  restoreWebAuthnCredential,
  revokeWebAuthnCredential,
  type WebAuthnCredentialItem,
  type WebAuthnCredentialStatus,
} from "@/shared/api/iam/user";
import {
  arrayBufferToBase64Url,
  base64UrlToArrayBuffer,
  isWebAuthnSupported,
} from "@/shared/lib/webauthn-b64";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";

interface WebAuthnCredentialsTabProps {
  userId: string;
  /** 目标用户所属组织；平台管理员跨租户操作时必须显式携带，否则后端回退会话租户 404。 */
  organizationId?: string;
}

const STATUS_COLOR: Record<WebAuthnCredentialStatus, string> = {
  active: "green",
  revoked: "red",
  deactivated: "orange",
};

const STATUS_LABEL_KEY: Record<WebAuthnCredentialStatus, string> = {
  active: "user.webauthn.statusActive",
  revoked: "user.webauthn.statusRevoked",
  deactivated: "user.webauthn.statusDeactivated",
};

export default function WebAuthnCredentialsTab({
  userId,
  organizationId,
}: WebAuthnCredentialsTabProps) {
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { message } = App.useApp();
  const queryClient = useQueryClient();

  const [detailTarget, setDetailTarget] = useState<WebAuthnCredentialItem | null>(null);
  const [renameTarget, setRenameTarget] = useState<WebAuthnCredentialItem | null>(null);
  const [registerOpen, setRegisterOpen] = useState(false);
  const [registering, setRegistering] = useState(false);
  const [renameForm] = Form.useForm<{ name: string }>();
  const [registerForm] = Form.useForm<{ name?: string }>();

  const { data: credentials = [], isLoading } = useQuery({
    queryKey: queryKeys.users.webauthnCredentials(userId, organizationId),
    queryFn: () => listWebAuthnCredentials(userId, organizationId),
    enabled: !!userId,
  });

  const detailQuery = useQuery({
    queryKey: queryKeys.users.webauthnCredential(
      userId,
      detailTarget?.credentialId ?? "",
      organizationId,
    ),
    queryFn: () => getWebAuthnCredential(userId, detailTarget!.credentialId, organizationId),
    enabled: !!detailTarget,
  });

  async function invalidateAfterWrite() {
    // AC-AUTH-009: credential writes cascade to the owner's sessions and tokens.
    await Promise.all([
      queryClient.invalidateQueries({
        queryKey: queryKeys.users.webauthnCredentials(userId, organizationId),
      }),
      queryClient.invalidateQueries({ queryKey: queryKeys.users.detail(userId) }),
      // 用户详情主查询键为 ["user","path",slug,username]，按前缀失效才能刷新摘要。
      queryClient.invalidateQueries({ queryKey: ["user"] }),
      queryClient.invalidateQueries({ queryKey: ["session"] }),
    ]);
  }

  const deleteMutation = useMutation({
    mutationFn: (credentialId: string) =>
      deleteWebAuthnCredential(userId, credentialId, organizationId),
    onSuccess: async () => {
      message.success(tr("user.webauthn.deleted"));
      setDetailTarget(null);
      await invalidateAfterWrite();
    },
    onError: () => message.error(tr("user.webauthn.deleteFailed")),
  });

  const statusMutation = useMutation({
    mutationFn: ({
      credentialId,
      action,
    }: {
      credentialId: string;
      action: "revoke" | "deactivate" | "restore";
    }) => {
      if (action === "revoke")
        return revokeWebAuthnCredential(userId, credentialId, organizationId);
      if (action === "deactivate")
        return deactivateWebAuthnCredential(userId, credentialId, organizationId);
      return restoreWebAuthnCredential(userId, credentialId, organizationId);
    },
    onSuccess: async () => {
      message.success(tr("user.webauthn.updated"));
      setDetailTarget(null);
      await invalidateAfterWrite();
    },
    onError: () => message.error(tc("operationFailed")),
  });

  const renameMutation = useMutation({
    mutationFn: ({ credentialId, name }: { credentialId: string; name: string }) =>
      renameWebAuthnCredential(userId, credentialId, name, organizationId),
    onSuccess: async () => {
      message.success(tr("user.webauthn.updated"));
      setRenameTarget(null);
      await invalidateAfterWrite();
    },
    onError: () => message.error(tc("operationFailed")),
  });

  async function submitRegistration() {
    const values = await registerForm.validateFields();
    const name = values.name?.trim();
    if (!isWebAuthnSupported()) {
      message.error(tr("user.webauthn.unsupported"));
      return;
    }

    setRegistering(true);
    try {
      const begin = await beginWebAuthnCredentialRegistration(userId, name, organizationId);
      const publicKey: PublicKeyCredentialCreationOptions = {
        rp: { id: begin.rpId, name: begin.rpName },
        user: {
          id: base64UrlToArrayBuffer(begin.userId),
          name: begin.userDisplayName || userId,
          displayName: begin.userDisplayName || userId,
        },
        // The backend compares clientDataJSON.challenge with
        // base64url(utf8(challengeString)), so the ceremony challenge is the
        // base64url string's UTF-8 bytes — not its decoded byte sequence.
        challenge: new TextEncoder().encode(begin.challenge),
        // The backend COSE parser only accepts EC2/P-256/ES256; advertising
        // RS256 lets such authenticators register a credential that finish
        // then rejects, so only ES256 is offered here.
        pubKeyCredParams: [{ type: "public-key", alg: -7 }],
        timeout: begin.timeout || 60_000,
        attestation: "none",
        authenticatorSelection: {
          residentKey: "preferred",
          requireResidentKey: false,
          // 由 begin 下发：组织/部署策略要求 UV 时必须是 "required"，不能再硬编码。
          userVerification: begin.userVerification,
        },
        // 由 begin 下发：该用户已有凭据。浏览器在客户端直接拒绝同一认证器重复注册。
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

      const attestation = created.response as AuthenticatorAttestationResponse;
      const transports =
        typeof attestation.getTransports === "function" ? attestation.getTransports() : undefined;

      // The server derives credentialId/publicKey/aaguid from attestationObject;
      // `created.id` is sent only so the backend can reject a mismatch.
      const saved = await finishWebAuthnCredentialRegistration(
        userId,
        {
          challengeId: begin.challengeId,
          challenge: begin.challenge,
          credentialId: created.id,
          clientDataJSON: arrayBufferToBase64Url(attestation.clientDataJSON),
          attestationObject: arrayBufferToBase64Url(attestation.attestationObject),
          transports,
          name: name || undefined,
        },
        organizationId,
      );

      if (saved.credentialId !== created.id) {
        message.warning(tr("user.webauthn.credentialMismatch"));
      } else {
        message.success(tr("user.webauthn.registered"));
      }
      setRegisterOpen(false);
      registerForm.resetFields();
      await invalidateAfterWrite();
    } catch (err) {
      message.error(tr(registrationErrorMessageKey(err)));
    } finally {
      setRegistering(false);
    }
  }

  const columns = [
    { title: tr("user.webauthn.name"), dataIndex: "name", key: "name" },
    {
      title: tr("user.webauthn.status"),
      dataIndex: "status",
      key: "status",
      width: 130,
      render: (status: WebAuthnCredentialStatus) => (
        <Tag color={STATUS_COLOR[status] ?? "default"}>
          {tr(STATUS_LABEL_KEY[status] ?? status)}
        </Tag>
      ),
    },
    {
      title: tr("user.webauthn.aaguid"),
      dataIndex: "aaguid",
      key: "aaguid",
      width: 130,
      render: (value: string) => <Tag>{value ? `${value.slice(0, 8)}...` : "-"}</Tag>,
    },
    {
      title: tr("user.webauthn.attestationType"),
      dataIndex: "attestationType",
      key: "attestationType",
    },
    {
      title: tr("user.webauthn.transports"),
      dataIndex: "transport",
      key: "transport",
      render: (transports: string[]) =>
        transports?.length ? transports.map((item) => <Tag key={item}>{item}</Tag>) : "-",
    },
    { title: tr("user.webauthn.signCount"), dataIndex: "signCount", key: "signCount", width: 100 },
    {
      title: tr("user.webauthn.createdAt"),
      dataIndex: "createdAt",
      key: "createdAt",
      render: (value: string) => (value ? new Date(value).toLocaleString() : "-"),
    },
    {
      title: tc("actions"),
      key: "actions",
      width: 280,
      render: (_: unknown, record: WebAuthnCredentialItem) => (
        <Space size={0} wrap>
          <Button type="link" size="small" onClick={() => setDetailTarget(record)}>
            {tr("user.webauthn.detail")}
          </Button>
          <PermissionGuard action="users:write" resource="users" fallback={null}>
            <Button type="link" size="small" onClick={() => setRenameTarget(record)}>
              {tr("user.webauthn.rename")}
            </Button>
            {record.status === "active" && (
              <>
                <Popconfirm
                  title={tr("user.webauthn.deactivateConfirm")}
                  okText={tr("user.webauthn.deactivate")}
                  cancelText={tc("cancel")}
                  onConfirm={() =>
                    statusMutation.mutate({
                      credentialId: record.credentialId,
                      action: "deactivate",
                    })
                  }
                >
                  <Button type="link" size="small">
                    {tr("user.webauthn.deactivate")}
                  </Button>
                </Popconfirm>
                <Popconfirm
                  title={tr("user.webauthn.revokeConfirm")}
                  okText={tr("user.webauthn.revoke")}
                  cancelText={tc("cancel")}
                  okButtonProps={{ danger: true }}
                  onConfirm={() =>
                    statusMutation.mutate({ credentialId: record.credentialId, action: "revoke" })
                  }
                >
                  <Button type="link" size="small" danger>
                    {tr("user.webauthn.revoke")}
                  </Button>
                </Popconfirm>
              </>
            )}
            {record.status === "deactivated" && (
              <Popconfirm
                title={tr("user.webauthn.restoreConfirm")}
                okText={tr("user.webauthn.restore")}
                cancelText={tc("cancel")}
                onConfirm={() =>
                  statusMutation.mutate({ credentialId: record.credentialId, action: "restore" })
                }
              >
                <Button type="link" size="small">
                  {tr("user.webauthn.restore")}
                </Button>
              </Popconfirm>
            )}
            <Popconfirm
              title={tr("user.webauthn.deleteConfirm")}
              okText={tc("delete")}
              cancelText={tc("cancel")}
              okButtonProps={{ danger: true }}
              onConfirm={() => deleteMutation.mutate(record.credentialId)}
            >
              <Button type="link" size="small" danger loading={deleteMutation.isPending}>
                {tc("delete")}
              </Button>
            </Popconfirm>
          </PermissionGuard>
        </Space>
      ),
    },
  ];

  return (
    <>
      <Space orientation="vertical" size="middle" style={{ width: "100%" }}>
        <Space style={{ width: "100%", justifyContent: "space-between" }}>
          <Typography.Text type="secondary">{tr("user.webauthn.selfServiceHint")}</Typography.Text>
          <PermissionGuard action="users:write" resource="users" fallback={null}>
            <Button type="primary" onClick={() => setRegisterOpen(true)}>
              {tr("user.webauthn.add")}
            </Button>
          </PermissionGuard>
        </Space>

        {!isLoading && credentials.length === 0 ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={
              <Space orientation="vertical" size={4}>
                <Typography.Text strong>{tr("user.noWebauthn")}</Typography.Text>
                <Typography.Text type="secondary">
                  {tr("user.webauthn.emptyDescription")}
                </Typography.Text>
              </Space>
            }
          >
            <PermissionGuard action="users:write" resource="users" fallback={null}>
              <Button type="primary" onClick={() => setRegisterOpen(true)}>
                {tr("user.webauthn.add")}
              </Button>
            </PermissionGuard>
          </Empty>
        ) : (
          <Table<WebAuthnCredentialItem>
            dataSource={credentials}
            columns={columns}
            loading={isLoading}
            pagination={false}
            size="small"
            rowKey="id"
          />
        )}
      </Space>

      <Drawer
        open={!!detailTarget}
        title={tr("user.webauthn.detail")}
        size={480}
        onClose={() => setDetailTarget(null)}
        destroyOnHidden
      >
        {detailQuery.isLoading ? (
          <Typography.Text type="secondary">{tc("loading")}</Typography.Text>
        ) : detailQuery.data ? (
          <Descriptions column={1} size="small" bordered>
            <Descriptions.Item label={tr("user.webauthn.name")}>
              {detailQuery.data.name}
            </Descriptions.Item>
            <Descriptions.Item label={tr("user.webauthn.status")}>
              <Tag color={STATUS_COLOR[detailQuery.data.status] ?? "default"}>
                {tr(STATUS_LABEL_KEY[detailQuery.data.status] ?? detailQuery.data.status)}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label={tr("user.webauthn.credentialId")}>
              <Typography.Text copyable>{detailQuery.data.credentialId}</Typography.Text>
            </Descriptions.Item>
            <Descriptions.Item label={tr("user.webauthn.aaguid")}>
              {detailQuery.data.aaguid || "-"}
            </Descriptions.Item>
            <Descriptions.Item label={tr("user.webauthn.attestationType")}>
              {detailQuery.data.attestationType || "-"}
            </Descriptions.Item>
            <Descriptions.Item label={tr("user.webauthn.transports")}>
              {detailQuery.data.transport?.join(", ") || "-"}
            </Descriptions.Item>
            <Descriptions.Item label={tr("user.webauthn.signCount")}>
              {detailQuery.data.signCount}
            </Descriptions.Item>
            <Descriptions.Item label={tr("user.webauthn.createdAt")}>
              {new Date(detailQuery.data.createdAt).toLocaleString()}
            </Descriptions.Item>
            <Descriptions.Item label={tr("user.webauthn.updatedAt")}>
              {new Date(detailQuery.data.updatedAt).toLocaleString()}
            </Descriptions.Item>
          </Descriptions>
        ) : null}
      </Drawer>

      <Modal
        open={!!renameTarget}
        title={tr("user.webauthn.renameTitle")}
        okText={tr("user.webauthn.rename")}
        cancelText={tc("cancel")}
        confirmLoading={renameMutation.isPending}
        destroyOnHidden
        onCancel={() => setRenameTarget(null)}
        onOk={async () => {
          const values = await renameForm.validateFields();
          if (!renameTarget) return;
          renameMutation.mutate({
            credentialId: renameTarget.credentialId,
            name: values.name.trim(),
          });
        }}
      >
        <Form
          form={renameForm}
          layout="vertical"
          preserve={false}
          initialValues={{ name: renameTarget?.name }}
        >
          <Form.Item
            name="name"
            label={tr("user.webauthn.name")}
            rules={[
              { required: true, message: tr("user.webauthn.nameInvalid") },
              { max: 255, message: tr("user.webauthn.nameInvalid") },
            ]}
          >
            <Input placeholder={tr("user.webauthn.namePlaceholder")} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        open={registerOpen}
        title={tr("user.webauthn.addTitle")}
        okText={tr("user.webauthn.add")}
        cancelText={tc("cancel")}
        confirmLoading={registering}
        destroyOnHidden
        onCancel={() => setRegisterOpen(false)}
        onOk={submitRegistration}
      >
        <Space orientation="vertical" size="middle" style={{ width: "100%" }}>
          {!isWebAuthnSupported() && (
            <Alert type="warning" showIcon title={tr("user.webauthn.unsupported")} />
          )}
          <Typography.Text type="secondary">{tr("user.webauthn.addHint")}</Typography.Text>
          <Form form={registerForm} layout="vertical" preserve={false}>
            <Form.Item
              name="name"
              label={tr("user.webauthn.name")}
              rules={[{ max: 255, message: tr("user.webauthn.nameInvalid") }]}
            >
              <Input placeholder={tr("user.webauthn.namePlaceholder")} />
            </Form.Item>
          </Form>
        </Space>
      </Modal>
    </>
  );
}
