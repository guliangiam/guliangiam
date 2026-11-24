import {
  App,
  Button,
  Card,
  Descriptions,
  Form,
  Input,
  InputNumber,
  Popconfirm,
  Select,
  Skeleton,
  Space,
  Tag,
  Typography,
} from "antd";
import { Link, useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { AuditSectionNav } from "@/features/navigation/AuditSectionNav";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { NotFoundResult, QueryErrorResult } from "@/shared/ui/QueryErrorResult";
import JwtDecodeView from "./JwtDecodeView";
import {
  useTokenDetailPageController,
  type TokenCreateFormValues,
} from "./useTokenDetailPageController";
import type { TokenDetail } from "@/shared/api/iam/token-extended";

function formatDateTime(value?: string | null) {
  if (!value) return "-";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString();
}
function formatJson(value: Record<string, unknown>) {
  const entries = Object.keys(value);
  if (entries.length === 0) return "{}";
  return JSON.stringify(value, null, 2);
}
function getTokenStatus(token?: TokenDetail | null, t?: (key: string) => string) {
  if (!token) return { text: t ? t("token.statusUnknown") : "unknown", color: "default" as const };
  if (token.revokedAt)
    return { text: t ? t("token.statusRevoked") : "revoked", color: "red" as const };
  const expiresAt = new Date(token.expiresAt);
  if (!Number.isNaN(expiresAt.getTime()) && expiresAt.getTime() <= Date.now()) {
    return { text: t ? t("token.statusExpired") : "expired", color: "orange" as const };
  }
  return { text: t ? t("token.statusActive") : "active", color: "green" as const };
}

export default function TokenDetailPage() {
  const { tokenName = "new" } = useParams();

  if (tokenName === "new") {
    return (
      <PermissionGuard action="token:write" resource="tokens">
        <TokenDetailContent />
      </PermissionGuard>
    );
  }

  return (
    <PermissionGuard action="token:read" resource="tokens">
      <TokenDetailContent />
    </PermissionGuard>
  );
}

function TokenDetailContent() {
  const navigate = useNavigate();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const {
    tokenName,
    isNew,
    orgNameMap,
    form,
    createdToken,
    showFullTokenValue,
    setShowFullTokenValue,
    usersQuery,
    applicationsQuery,
    detailQuery,
    createMutation,
    deleteMutation,
    revokeMutation,
  } = useTokenDetailPageController();

  if (isNew) {
    return (
      <div className="gl-resource-page">
        <AuditSectionNav />
        <ResourceFormPage
          title={tr("token.createNew")}
          description={tr("token.createDescription")}
          saving={createMutation.isPending}
          form={form}
          onCancel={() => navigate("/tokens")}
          onFinish={async (values: TokenCreateFormValues) => {
            await createMutation.mutateAsync(values);
          }}
        >
          <Form.Item
            label={tr("token.organizationId")}
            name="organizationId"
            rules={[{ required: true, message: tr("token.validation.missingOrganization") }]}
          >
            <Input disabled />
          </Form.Item>
          <Form.Item
            label={tr("token.user")}
            name="userId"
            rules={[{ required: true, message: tr("token.validation.selectUser") }]}
          >
            <Select
              showSearch
              loading={usersQuery.isLoading}
              placeholder={tr("token.placeholder.selectUser")}
              options={(usersQuery.data ?? []).map((item) => ({
                label: item.displayName ? `${item.displayName} (${item.username})` : item.username,
                value: item.id,
              }))}
              optionFilterProp="label"
            />
          </Form.Item>
          <Form.Item
            label={tr("token.application")}
            name="applicationId"
            rules={[{ required: true, message: tr("token.validation.selectApplication") }]}
          >
            <Select
              showSearch
              loading={applicationsQuery.isLoading}
              placeholder={tr("token.placeholder.selectApplication")}
              options={(applicationsQuery.data ?? []).map((item) => ({
                label: item.displayName ? `${item.displayName} (${item.slug})` : item.slug,
                value: item.id,
              }))}
              optionFilterProp="label"
            />
          </Form.Item>
          <Form.Item
            label={tr("token.scopes")}
            name="scopes"
            rules={[{ required: true, message: tr("token.validation.scopeRequired") }]}
          >
            <Select
              mode="tags"
              tokenSeparators={[","]}
              placeholder={tr("token.placeholder.scopes")}
            />
          </Form.Item>
          <Form.Item label={tr("token.audience")} name="audience">
            <Select
              mode="tags"
              tokenSeparators={[","]}
              placeholder={tr("token.placeholder.audience")}
            />
          </Form.Item>
          <Form.Item
            label={tr("token.expiresInSeconds")}
            name="expiresInSeconds"
            rules={[{ required: true, message: tr("token.validation.expiresRequired") }]}
          >
            <InputNumber min={1} step={300} style={{ width: "100%" }} />
          </Form.Item>
          <Form.Item label={tr("token.metadataJson")} name="metadataText">
            <Input.TextArea rows={8} placeholder='{"source":"auth-ui"}' />
          </Form.Item>
        </ResourceFormPage>

        {createdToken && (
          <Card
            className="gl-panel gl-resource-card"
            variant="borderless"
            style={{ marginTop: 16 }}
          >
            <div className="gl-resource-card__header">
              <div>
                <Typography.Title level={3}>{tr("token.oneTimeResult")}</Typography.Title>
                <Typography.Paragraph>{tr("token.oneTimeResultDesc")}</Typography.Paragraph>
              </div>
              <Space>
                <Button onClick={() => navigate(`/tokens/${createdToken.token.id}`)}>
                  {tr("token.viewDetail")}
                </Button>
                <Button onClick={() => navigate("/tokens")}>{tc("backToList")}</Button>
              </Space>
            </div>
            <Descriptions bordered size="small" column={1}>
              <Descriptions.Item label={tr("token.tokenValue")}>
                <Space>
                  <Typography.Text code>
                    {showFullTokenValue
                      ? createdToken.tokenValue
                      : `${createdToken.tokenValue.slice(0, 4)}${"*".repeat(8)}${createdToken.tokenValue.length > 8 ? createdToken.tokenValue.slice(-4) : ""}`}
                  </Typography.Text>
                  <Button type="link" size="small" onClick={() => setShowFullTokenValue((v) => !v)}>
                    {showFullTokenValue
                      ? tr("token.hideValue", "Hide")
                      : tr("token.showValue", "Show")}
                  </Button>
                  {showFullTokenValue && (
                    <Button
                      type="link"
                      size="small"
                      onClick={() => {
                        void navigator.clipboard.writeText(createdToken.tokenValue).then(
                          () => message.success(tc("copySuccess")),
                          () => message.error(tc("copyFailed")),
                        );
                      }}
                    >
                      {tr("token.copyValue", "Copy")}
                    </Button>
                  )}
                </Space>
              </Descriptions.Item>
              <Descriptions.Item label={tr("token.prefix")}>
                {createdToken.token.tokenPrefix}
              </Descriptions.Item>
              <Descriptions.Item label={tr("token.user")}>
                {createdToken.token.username}
              </Descriptions.Item>
              <Descriptions.Item label={tr("token.application")}>
                {createdToken.token.applicationSlug}
              </Descriptions.Item>
              <Descriptions.Item label={tr("token.scopes")}>
                {createdToken.token.scopes.length > 0
                  ? createdToken.token.scopes.map((s) => <Tag key={s}>{s}</Tag>)
                  : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={tr("token.audience")}>
                {createdToken.token.audience.length > 0
                  ? createdToken.token.audience.map((a) => <Tag key={a}>{a}</Tag>)
                  : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={tr("token.expiresAt")}>
                {formatDateTime(createdToken.token.expiresAt)}
              </Descriptions.Item>
            </Descriptions>
          </Card>
        )}
      </div>
    );
  }

  const token = detailQuery.data as TokenDetail | undefined;
  const status = getTokenStatus(token, tr);

  return (
    <div className="gl-resource-page">
      <AuditSectionNav />
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <div className="gl-resource-card__header">
          <div>
            <Typography.Title level={2}>{tr("token.detailTitle")}</Typography.Title>
            <Typography.Paragraph>{tr("token.detailDescription")}</Typography.Paragraph>
          </div>
          <Space>
            {!token?.revokedAt && (
              <PermissionGuard action="token:write" resource="tokens" fallback={null}>
                <Popconfirm
                  title={tr("token.revokeConfirm")}
                  description={tokenName}
                  okText={tr("token.revoke")}
                  cancelText={tc("cancel")}
                  okButtonProps={{ loading: revokeMutation.isPending }}
                  onConfirm={() => revokeMutation.mutate()}
                >
                  <Button disabled={!token}>{tr("token.revoke")}</Button>
                </Popconfirm>
              </PermissionGuard>
            )}
            <PermissionGuard action="token:delete" resource="tokens" fallback={null}>
              <Popconfirm
                title={tr("token.deleteConfirm")}
                description={tokenName}
                okText={tc("delete")}
                cancelText={tc("cancel")}
                okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
                onConfirm={() => deleteMutation.mutate()}
              >
                <Button danger disabled={!token}>
                  {tc("delete")}
                </Button>
              </Popconfirm>
            </PermissionGuard>
            <Button onClick={() => navigate("/tokens")}>{tc("backToList")}</Button>
          </Space>
        </div>

        {detailQuery.isLoading ? (
          <Skeleton active paragraph={{ rows: 6 }} />
        ) : detailQuery.isError ? (
          <QueryErrorResult
            error={detailQuery.error}
            onRetry={() => detailQuery.refetch()}
            notFoundTitle={tr("token.notFound")}
            notFoundDescription={tr("token.tokenNotExist")}
          />
        ) : !token ? (
          <NotFoundResult title={tr("token.notFound")} description={tr("token.tokenNotExist")} />
        ) : (
          <Descriptions bordered size="small" column={1}>
            <Descriptions.Item label={tr("token.status")}>
              <Tag color={status.color}>{status.text}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label={tr("token.tokenId")}>{token.id}</Descriptions.Item>
            <Descriptions.Item label={tr("token.prefix")}>
              <Typography.Text copyable={{ text: token.tokenPrefix }}>
                {token.tokenPrefix}
              </Typography.Text>
            </Descriptions.Item>
            <Descriptions.Item label={tr("token.hash")}>
              {/* BE-D-19: reads only expose the masked projection, never the raw hash. */}
              {token.tokenHashSet ? (
                <Typography.Text code>{token.tokenHashMasked}</Typography.Text>
              ) : (
                <Typography.Text type="secondary">-</Typography.Text>
              )}
            </Descriptions.Item>
            <Descriptions.Item label={tr("token.user")}>
              {token.username && token.organizationSlug ? (
                <Link to={`/users/${token.organizationSlug}/${token.username}`}>
                  {token.username}
                </Link>
              ) : (
                token.username || "-"
              )}
            </Descriptions.Item>
            <Descriptions.Item label={tr("token.userId")}>{token.userId}</Descriptions.Item>
            <Descriptions.Item label={tr("token.organization")}>
              {token.organizationSlug}
            </Descriptions.Item>
            <Descriptions.Item label={tr("token.organizationId")}>
              {orgNameMap.get(token.organizationId) || token.organizationId}
            </Descriptions.Item>
            <Descriptions.Item label={tr("token.application")}>
              {token.applicationSlug && token.applicationId ? (
                <Link to={`/applications/${token.applicationId}`}>{token.applicationSlug}</Link>
              ) : (
                token.applicationSlug || "-"
              )}
            </Descriptions.Item>
            <Descriptions.Item label={tr("token.applicationId")}>
              {token.applicationId}
            </Descriptions.Item>
            <Descriptions.Item label={tr("token.scopes")}>
              {token.scopes.length > 0 ? token.scopes.map((s) => <Tag key={s}>{s}</Tag>) : "-"}
            </Descriptions.Item>
            <Descriptions.Item label={tr("token.audience")}>
              {token.audience.length > 0 ? token.audience.map((a) => <Tag key={a}>{a}</Tag>) : "-"}
            </Descriptions.Item>
            <Descriptions.Item label={tr("token.expiresAt")}>
              {formatDateTime(token.expiresAt)}
            </Descriptions.Item>
            <Descriptions.Item label={tr("token.revokedAt")}>
              {formatDateTime(token.revokedAt)}
            </Descriptions.Item>
            <Descriptions.Item label={tr("token.createdAt")}>
              {formatDateTime(token.createdAt)}
            </Descriptions.Item>
            <Descriptions.Item label={tr("token.updatedAt")}>
              {formatDateTime(token.updatedAt)}
            </Descriptions.Item>
            <Descriptions.Item label={tr("token.revision")}>{token.revision}</Descriptions.Item>
            <Descriptions.Item label={tr("token.metadata")}>
              <Typography.Paragraph
                style={{ marginBottom: 0, whiteSpace: "pre-wrap", fontFamily: "monospace" }}
              >
                {formatJson(token.metadata)}
              </Typography.Paragraph>
            </Descriptions.Item>
          </Descriptions>
        )}
      </Card>
      <JwtDecodeView />
    </div>
  );
}
