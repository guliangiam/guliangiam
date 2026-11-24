import {
  Alert,
  App,
  Button,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Tooltip,
  Typography,
} from "antd";
import { EditOutlined } from "@ant-design/icons";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { FormSection } from "@/shared/ui/FormSection";
import WebAuthnCredentialsTab from "./WebAuthnCredentialsTab";
import { listUserTrustedDevices, revokeUserTrustedDevice } from "@/shared/api/iam/user";
import { serverErrorMessage } from "@/shared/lib/server-error";
import { formatDateTime, MFA_TYPE_OPTIONS } from "./types";
import type { UserDetail } from "./types";
import type { TrustedDeviceItem, UserMfaMethod } from "@/shared/api/iam/user";

interface UserMfaTabProps {
  user: UserDetail | undefined;
  onUpdateMfaMethods: (
    methods: { methodType: string; enabled: boolean; displayName?: string }[],
  ) => void;
  onSetPreferredMfa: (mfaType: string) => void;
  /** users:write：无写权限时所有写入口只读化，而不是等后端 403。 */
  canWrite?: boolean;
  /** MFA 方法替换请求进行中：禁用所有开关，避免基于旧快照的快速连点丢失上一次修改。 */
  mfaPending?: boolean;
}

export default function UserMfaTab({
  user,
  onUpdateMfaMethods,
  onSetPreferredMfa,
  canWrite = true,
  mfaPending = false,
}: UserMfaTabProps) {
  const { t } = useTranslation();
  const [renameTarget, setRenameTarget] = useState<UserMfaMethod | null>(null);
  const [renameForm] = Form.useForm<{ displayName: string }>();

  // 首选因子只能从「已启用且已验证」的方法中选择；后端同样拒绝其它类型。
  const verifiedMethods = (user?.mfaMethods ?? []).filter((m) => m.enabled && m.verified);
  const preferredOptions = MFA_TYPE_OPTIONS.filter(
    (option) =>
      option.value !== "none" && verifiedMethods.some((m) => m.methodType === option.value),
  );

  const submitRename = async () => {
    const values = await renameForm.validateFields();
    const methods = (user?.mfaMethods ?? []).map((m: UserMfaMethod) => ({
      methodType: m.methodType,
      enabled: m.enabled,
      displayName: m.methodType === renameTarget?.methodType ? values.displayName : m.displayName,
    }));
    onUpdateMfaMethods(methods);
    setRenameTarget(null);
  };

  return (
    <Space orientation="vertical" style={{ width: "100%" }} size="large">
      {/* ─── 1. MFA 方法配置 ─── */}
      <FormSection
        title={t("resource:user.mfaSection")}
        extra={
          <Space>
            <Typography.Text type="secondary">{t("resource:user.preferredMfa")}:</Typography.Text>
            <Select
              value={user?.preferredMfaType || undefined}
              options={preferredOptions}
              placeholder={t("resource:user.mfaPreferredUnavailable")}
              size="small"
              style={{ width: 160 }}
              disabled={!canWrite || mfaPending || preferredOptions.length === 0}
              onChange={(value) => onSetPreferredMfa(value)}
            />
          </Space>
        }
      >
        <Table
          dataSource={(user?.mfaMethods ?? []).map((m: UserMfaMethod, i: number) => ({
            ...m,
            _idx: i,
          }))}
          columns={[
            {
              title: t("resource:user.mfaType", "Type"),
              dataIndex: "methodType",
              key: "methodType",
            },
            {
              title: t("enabled", "Enabled"),
              dataIndex: "enabled",
              key: "enabled",
              render: (v: boolean, record: UserMfaMethod, idx: number) => (
                <Tooltip
                  title={!record.verified && !v ? t("resource:user.mfaVerifyFirst") : undefined}
                >
                  <Switch
                    size="small"
                    checked={v}
                    disabled={!canWrite || mfaPending || (!record.verified && !v)}
                    onChange={(checked) => {
                      const methods = (user?.mfaMethods ?? []).map(
                        (m: UserMfaMethod, i: number) => ({
                          methodType: m.methodType,
                          enabled: i === idx ? checked : m.enabled,
                          displayName: m.displayName,
                        }),
                      );
                      onUpdateMfaMethods(methods);
                    }}
                  />
                </Tooltip>
              ),
            },
            {
              title: t("verified", "Verified"),
              dataIndex: "verified",
              key: "verified",
              render: (v: boolean) =>
                v ? <Tag color="green">{t("yes")}</Tag> : <Tag>{t("no")}</Tag>,
            },
            { title: t("displayName"), dataIndex: "displayName", key: "displayName" },
            {
              title: "",
              key: "edit",
              width: 64,
              render: (_: unknown, record: UserMfaMethod) =>
                canWrite ? (
                  <Tooltip title={t("common:edit", { defaultValue: "Edit" })}>
                    <Button
                      type="text"
                      size="small"
                      icon={<EditOutlined />}
                      disabled={mfaPending}
                      onClick={() => {
                        renameForm.setFieldsValue({
                          displayName: record.displayName ?? record.methodType,
                        });
                        setRenameTarget(record);
                      }}
                    />
                  </Tooltip>
                ) : null,
            },
            {
              title: t("resource:user.lastUsed", "Last Used"),
              dataIndex: "lastUsedAt",
              key: "lastUsedAt",
              render: (v: string) => formatDateTime(v),
            },
            {
              title: "",
              key: "preferred",
              width: 100,
              render: (_: unknown, record: UserMfaMethod) => (
                <Button
                  size="small"
                  type={user?.preferredMfaType === record.methodType ? "primary" : "default"}
                  disabled={!canWrite || mfaPending || !record.enabled || !record.verified}
                  onClick={() => onSetPreferredMfa(String(record.methodType))}
                >
                  {t("resource:user.setPreferred", "Set Preferred")}
                </Button>
              ),
            },
          ]}
          pagination={false}
          size="small"
          rowKey="_idx"
        />
      </FormSection>

      {/* ─── 2. 通行密钥 / WebAuthn ─── */}
      <FormSection title={t("resource:user.webauthnCredentials", "Passkeys / WebAuthn")}>
        {user?.id ? (
          <WebAuthnCredentialsTab userId={user.id} organizationId={user.organizationId} />
        ) : (
          <Typography.Text type="secondary">
            {t("resource:user.noWebauthn", "No WebAuthn credentials registered")}
          </Typography.Text>
        )}
      </FormSection>

      {/* ─── 3. 受信设备 ─── */}
      <TrustedDevicesFormSection
        userId={user?.id}
        organizationId={user?.organizationId}
        canWrite={canWrite}
      />

      {/* ─── 重命名 Modal ─── */}
      <Modal
        forceRender
        open={!!renameTarget}
        title={t("resource:user.editMfaDisplayName", "Edit MFA display name")}
        onCancel={() => setRenameTarget(null)}
        onOk={submitRename}
        confirmLoading={mfaPending}
        okText={t("common:save", { defaultValue: "Save" })}
        cancelText={t("common:cancel", { defaultValue: "Cancel" })}
      >
        <Form form={renameForm} layout="vertical">
          <Form.Item
            label={t("displayName", { defaultValue: "Display name" })}
            name="displayName"
            rules={[{ required: true, whitespace: true }]}
          >
            <Input />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  );
}

/**
 * 受信设备列表与撤销（FormSection 版本）。
 *
 * 数据源 GET /users/{id}/trusted-devices，撤销 DELETE 同源；
 * 撤销后下一次登录该设备不再跳过 MFA（登录流程每次从库读取受信设备，不受 principal 缓存影响）。
 */
function TrustedDevicesFormSection({
  userId,
  organizationId,
  canWrite,
}: {
  userId?: string;
  organizationId?: string;
  canWrite: boolean;
}) {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const queryKey = ["user", userId, "trusted-devices"] as const;
  const { data, isLoading, isError, error } = useQuery({
    queryKey,
    // 详情级端点要求可信租户：跨组织查看用户时必须显式携带所属组织。
    queryFn: () => listUserTrustedDevices(userId!, organizationId),
    enabled: !!userId,
  });
  const revoke = useMutation({
    mutationFn: (deviceId: string) => revokeUserTrustedDevice(userId!, deviceId, organizationId),
    onSuccess: () => {
      message.success(t("resource:user.trustedDeviceRevoked", "Trusted device revoked"));
      queryClient.invalidateQueries({ queryKey });
    },
    onError: (err) => message.error(serverErrorMessage(err, t, "common:loadFailed")),
  });

  return (
    <FormSection title={t("resource:user.trustedDevices", "Trusted Devices")}>
      {isError ? (
        <Alert type="error" showIcon title={serverErrorMessage(error, t, "common:loadFailed")} />
      ) : (
        <Table
          dataSource={data ?? []}
          loading={isLoading}
          rowKey="id"
          size="small"
          pagination={false}
          locale={{ emptyText: t("resource:user.noTrustedDevices", "No trusted devices") }}
          columns={[
            { title: "ID", dataIndex: "id", key: "id", ellipsis: true },
            {
              title: t("resource:user.trustedDeviceStatus", "Status"),
              dataIndex: "status",
              key: "status",
              render: (value: string) => (
                <Tag color={value === "active" ? "green" : value === "revoked" ? "red" : "default"}>
                  {value === "active"
                    ? t("resource:user.trustedDeviceStatusActive", "Active")
                    : value === "revoked"
                      ? t("resource:user.trustedDeviceStatusRevoked", "Revoked")
                      : t("resource:user.trustedDeviceStatusExpired", "Expired")}
                </Tag>
              ),
            },
            {
              title: t("resource:user.trustedDeviceLastSeen", "Last Seen"),
              dataIndex: "lastSeenAt",
              key: "lastSeenAt",
              render: (value: string) => formatDateTime(value),
            },
            {
              title: t("resource:user.trustedDeviceExpiresAt", "Expires At"),
              dataIndex: "expiresAt",
              key: "expiresAt",
              render: (value: string) => formatDateTime(value),
            },
            {
              title: "",
              key: "actions",
              width: 100,
              render: (_: unknown, record: TrustedDeviceItem) =>
                canWrite && record.status === "active" ? (
                  <Popconfirm
                    title={t(
                      "resource:user.trustedDeviceRevokeConfirm",
                      "Revoke this trusted device?",
                    )}
                    onConfirm={() => revoke.mutate(record.id)}
                  >
                    <Button
                      size="small"
                      danger
                      loading={revoke.isPending && revoke.variables === record.id}
                    >
                      {t("resource:user.trustedDeviceRevoke", "Revoke")}
                    </Button>
                  </Popconfirm>
                ) : null,
            },
          ]}
        />
      )}
    </FormSection>
  );
}
