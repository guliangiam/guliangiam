import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Popconfirm,
  Result,
  Select,
  Skeleton,
  Space,
  Tag,
  Typography,
} from "antd";
import { CopyOutlined, DownloadOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { Link, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { getApplications } from "@/shared/api/iam/application";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { GlFormRow } from "@/shared/ui/GlForm";
import { useCertDetailPageController, type CertFormValues } from "./useCertDetailPageController";

/** Days between an ISO timestamp and a reference "now" (ms since epoch). */
function daysUntil(isoTime: string, nowMs: number) {
  const exp = new Date(isoTime);
  if (Number.isNaN(exp.getTime())) return null;
  return Math.ceil((exp.getTime() - nowMs) / 86400000);
}

function downloadText(filename: string, content: string) {
  const blob = new Blob([content], { type: "text/plain" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

// 后端 app/certs/validation.go：scope ∈ {JWT, SSL}，type ∈ {x509, SSL}。
const SCOPE_OPTIONS = [
  { label: "JWT", value: "JWT" },
  { label: "SSL", value: "SSL" },
];
const CERT_TYPE_OPTIONS = [
  { label: "x509", value: "x509" },
  { label: "SSL", value: "SSL" },
];
const JWT_ALGORITHM_OPTIONS = [
  { label: "RS256", value: "RS256" },
  { label: "RS384", value: "RS384" },
  { label: "RS512", value: "RS512" },
  { label: "ES256", value: "ES256" },
  { label: "ES384", value: "ES384" },
  { label: "ES512", value: "ES512" },
  { label: "PS256", value: "PS256" },
  { label: "PS384", value: "PS384" },
  { label: "PS512", value: "PS512" },
];
// 后端只接受 RS/ES/PS 系列 cryptoAlgorithm（RSA/ECC 会被 validation 拒绝）。
const BIT_SIZE_OPTIONS = [
  { label: "1024", value: 1024 },
  { label: "2048", value: 2048 },
  { label: "4096", value: 4096 },
];
const SSL_PROVIDER_OPTIONS = [
  { label: "GoDaddy", value: "GoDaddy" },
  { label: "Aliyun", value: "Aliyun" },
];

export default function CertDetailPage() {
  const navigate = useNavigate();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const {
    id,
    form,
    orgNameMap,
    isSSL,
    isES,
    detail,
    certValue,
    keyValue,
    detailQuery,
    updateMutation,
    deleteMutation,
    checkDomainMutation,
    handleCopy,
  } = useCertDetailPageController();

  const relatedAppsQuery = useQuery({
    queryKey: ["applications", "cert-related", detailQuery.data?.organizationId ?? ""],
    queryFn: () => getApplications(detailQuery.data?.organizationId || undefined),
    enabled: Boolean(detailQuery.data),
  });
  const relatedApps = (relatedAppsQuery.data ?? []).filter(
    (app) => app.certificate && app.certificate === detailQuery.data?.name,
  );

  if (detailQuery.isLoading) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Skeleton active paragraph={{ rows: 6 }} />
      </Card>
    );
  }

  if (detailQuery.isError) {
    return (
      <Result
        status="error"
        title={tc("loadFailed")}
        subTitle={String(detailQuery.error)}
        extra={
          <Button type="primary" onClick={() => detailQuery.refetch()}>
            {tc("retry")}
          </Button>
        }
      />
    );
  }

  if (!detailQuery.data) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Alert
          type="error"
          showIcon
          title={tr("cert.notFound")}
          description={tr("cert.notFoundDesc", { id })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/certs")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  return (
    <ResourceFormPage
      title={tr("cert.detailTitle", {
        name: detailQuery.data.displayName || detailQuery.data.name,
      })}
      description={tr("cert.editDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/certs")}
      onFinish={async (values: CertFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      extra={
        <Popconfirm
          title={tc("deleteConfirm")}
          description={tr("cert.deleteDesc")}
          onConfirm={() => deleteMutation.mutate()}
          okText={tc("delete")}
          cancelText={tc("cancel")}
          okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
        >
          <Button danger>{tc("delete")}</Button>
        </Popconfirm>
      }
    >
      <FormSection title={tc("overview", "Overview")}>
        <Form.Item label={tc("name")}>
          <span>{detailQuery.data.name}</span>
        </Form.Item>
        <Form.Item label={tc("certType")}>
          <Tag color="blue">{detail.type || "-"}</Tag>
        </Form.Item>
        {detail.expireTime ? (
          <Form.Item label={tc("expireAt")}>
            <span>{String(detail.expireTime)}</span>
          </Form.Item>
        ) : null}
        {/* PatchCertRequest 不含 organizationId：归属组织创建后不可变，仅展示。 */}
        <Form.Item label={tc("organization")}>
          <span>
            {orgNameMap.get(detailQuery.data.organizationId) || detailQuery.data.organizationId}
          </span>
        </Form.Item>
      </FormSection>

      <FormSection title={tc("certBasicInfo")}>
        <Form.Item name="scope">
          <GlFormRow label="Scope">
            <Select options={SCOPE_OPTIONS} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="type">
          <GlFormRow label={tc("type")}>
            <Select options={CERT_TYPE_OPTIONS} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="algorithm">
          <GlFormRow label={tc("algorithm")}>
            <Select options={JWT_ALGORITHM_OPTIONS} />
          </GlFormRow>
        </Form.Item>
        {!isES && !isSSL && (
          <Form.Item name="bitSize">
            <GlFormRow label="Bit Size">
              <Select options={BIT_SIZE_OPTIONS} />
            </GlFormRow>
          </Form.Item>
        )}
        {!isSSL && (
          <Form.Item name="expireInYears">
            <GlFormRow label="Expire In Years">
              <InputNumber min={1} max={30} style={{ width: "100%" }} />
            </GlFormRow>
          </Form.Item>
        )}
        <Form.Item
          name="displayName"
          rules={[{ required: true, message: tc("validation.displayNameRequired") }]}
        >
          <GlFormRow label={tc("displayName")} required>
            <Input />
          </GlFormRow>
        </Form.Item>
      </FormSection>

      <FormSection title={tr("cert.relatedApplications")}>
        {relatedApps.length > 0 ? (
          <Space orientation="vertical" size={4}>
            {relatedApps.map((app) => (
              <Link key={app.id} to={`/applications/${app.id}`}>
                {app.displayName || app.slug || app.id}
              </Link>
            ))}
          </Space>
        ) : (
          <Typography.Text type="secondary">{tr("cert.noRelatedApplications")}</Typography.Text>
        )}
      </FormSection>

      {isSSL && (
        <FormSection title={tc("certSslConfig")}>
          {detailQuery.data.expireTime && (
            <Form.Item label={tc("expireTime")}>
              <Space>
                <span>{detailQuery.data.expireTime}</span>
                {(() => {
                  const daysLeft = daysUntil(
                    detailQuery.data.expireTime,
                    detailQuery.dataUpdatedAt,
                  );
                  if (daysLeft === null) return null;
                  if (daysLeft < 0) return <Tag color="red">{tc("expired")}</Tag>;
                  if (daysLeft <= 30)
                    return <Tag color="orange">{tc("daysLeft", { days: daysLeft })}</Tag>;
                  return <Tag color="green">{tc("daysLeft", { days: daysLeft })}</Tag>;
                })()}
              </Space>
            </Form.Item>
          )}
          {detailQuery.data.domainExpireTime && (
            <Form.Item label={tc("domainExpireTime")}>
              <Space>
                <span>{detailQuery.data.domainExpireTime}</span>
                {(() => {
                  const daysLeft = daysUntil(
                    detailQuery.data.domainExpireTime,
                    detailQuery.dataUpdatedAt,
                  );
                  if (daysLeft === null) return null;
                  if (daysLeft < 0) return <Tag color="red">{tc("expired")}</Tag>;
                  if (daysLeft <= 30)
                    return <Tag color="orange">{tc("daysLeft", { days: daysLeft })}</Tag>;
                  return <Tag color="green">{tc("daysLeft", { days: daysLeft })}</Tag>;
                })()}
              </Space>
            </Form.Item>
          )}
          <Form.Item>
            <Button
              icon={<SafetyCertificateOutlined />}
              loading={checkDomainMutation.isPending}
              onClick={() => checkDomainMutation.mutate()}
            >
              {tc("checkDomainExpire")}
            </Button>
          </Form.Item>
          <Form.Item name="provider">
            <GlFormRow label="Provider">
              <Select
                options={SSL_PROVIDER_OPTIONS}
                allowClear
                placeholder={tc("selectSslProvider")}
              />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="account">
            <GlFormRow label="Account">
              <Input />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="accessKey">
            <GlFormRow label="Access Key">
              <Input />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="accessSecret">
            <GlFormRow label="Access Secret">
              <Input.Password />
            </GlFormRow>
          </Form.Item>
        </FormSection>
      )}

      <FormSection title={tc("certificateContent")}>
        <Form.Item name="certificate">
          <GlFormRow label={tc("certificate")}>
            <Input.TextArea
              rows={8}
              style={{ fontFamily: "monospace", fontSize: 12 }}
              placeholder={tc("certificatePlaceholder")}
            />
          </GlFormRow>
        </Form.Item>
        <Space style={{ marginBottom: 16 }}>
          <Button
            size="small"
            icon={<CopyOutlined />}
            onClick={() => handleCopy(certValue)}
            disabled={!certValue}
          >
            {tc("copyCertificate")}
          </Button>
          <Button
            size="small"
            icon={<DownloadOutlined />}
            onClick={() => downloadText(`${detailQuery.data.name || "cert"}.pem`, certValue)}
            disabled={!certValue}
          >
            {tc("downloadCertificate")}
          </Button>
        </Space>

        <Form.Item name="privateKey">
          <GlFormRow label={tc("privateKey")}>
            <Input.TextArea
              rows={8}
              style={{ fontFamily: "monospace", fontSize: 12 }}
              placeholder={tc("privateKeyPlaceholder")}
            />
          </GlFormRow>
        </Form.Item>
        <Space style={{ marginBottom: 16 }}>
          <Button
            size="small"
            icon={<CopyOutlined />}
            onClick={() => handleCopy(keyValue)}
            disabled={!keyValue}
          >
            {tc("copyPrivateKey")}
          </Button>
          <Button
            size="small"
            icon={<DownloadOutlined />}
            onClick={() => downloadText(`${detailQuery.data.name || "cert"}.key`, keyValue)}
            disabled={!keyValue}
          >
            {tc("downloadPrivateKey")}
          </Button>
        </Space>
      </FormSection>

      <FormSection title={tc("certAuthority")}>
        <Form.Item name="authorityPublicKey">
          <GlFormRow label={tc("authorityPublicKey")}>
            <Input.TextArea
              rows={4}
              style={{ fontFamily: "monospace", fontSize: 12 }}
              placeholder={tc("authorityPublicKeyPlaceholder")}
            />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="authorityRootPublicKey">
          <GlFormRow label={tc("authorityRootPublicKey")}>
            <Input.TextArea
              rows={4}
              style={{ fontFamily: "monospace", fontSize: 12 }}
              placeholder={tc("authorityRootPublicKeyPlaceholder")}
            />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="acmeIssuer">
          <GlFormRow label="ACME Issuer">
            <Input />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="acmeEndpoint">
          <GlFormRow label="ACME Endpoint">
            <Input />
          </GlFormRow>
        </Form.Item>
      </FormSection>
    </ResourceFormPage>
  );
}
