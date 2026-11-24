import {
  Button,
  Card,
  Descriptions,
  Divider,
  Form,
  Input,
  Popconfirm,
  Result,
  Select,
  Skeleton,
  Switch,
  InputNumber,
  Tag,
  Table,
  Space,
} from "antd";
import { SectionNavShell } from "@/features/section-nav/SectionNavShell";
import type { SectionNavItem } from "@/features/section-nav/types";
import { GlFormRow } from "@/shared/ui/GlForm";
import { FormSection } from "@/shared/ui/FormSection";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { NotFoundResult } from "@/shared/ui/QueryErrorResult";
import type { CertStatusResult, HealthCheckResult } from "@/shared/api/iam/gateway";
import type { SiteFormValues } from "./useSiteDetailPageController";

type Props = {
  ctrl: ReturnType<typeof import("./useSiteDetailPageController").useSiteDetailPageController>;
};

export function SiteDetailPageView({ ctrl }: Props) {
  const {
    tc,
    tr,
    navigate,
    form,
    orgNameMap,
    listQuery,
    detailQuery,
    certQuery,
    healthQuery,
    site,
    updateMutation,
    deleteMutation,
    provisionCertMutation,
    renewCertMutation,
    checkHealthMutation,
  } = ctrl;

  if (listQuery.isLoading || detailQuery.isLoading) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Skeleton active paragraph={{ rows: 6 }} />
      </Card>
    );
  }

  if (!site) {
    return <NotFoundResult title={tc("notFound")} description={tc("notFoundDesc")} />;
  }

  if (listQuery.isError) {
    return (
      <Result
        status="error"
        title={tc("loadFailed")}
        subTitle={String(listQuery.error)}
        extra={
          <Button type="primary" onClick={() => listQuery.refetch()}>
            {tc("retry")}
          </Button>
        }
      />
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

  const renderGeneralTab = () => (
    <>
      {site && (
        <>
          <Descriptions bordered column={1} size="small" style={{ marginBottom: 16 }}>
            <Descriptions.Item label="ID">{site.id}</Descriptions.Item>
            <Descriptions.Item label={tc("organizationId")}>
              {orgNameMap.get(site.organizationId) || site.organizationId}
            </Descriptions.Item>
            <Descriptions.Item label={tr("gateway.revision")}>{site.revision}</Descriptions.Item>
          </Descriptions>
          <Divider style={{ margin: "12px 0" }} />
          <Form.Item name="name" initialValue={site.name} rules={[{ required: true }]}>
            <GlFormRow label={tc("name")} required>
              <Input />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="host" initialValue={site.host} rules={[{ required: true }]}>
            <GlFormRow label={tc("host")} required>
              <Input />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="status" initialValue={site.status}>
            <GlFormRow label={tc("statusLabel")}>
              <Select>
                <Select.Option value="active">{tr("gateway.active")}</Select.Option>
                <Select.Option value="disabled">{tc("disabled")}</Select.Option>
              </Select>
            </GlFormRow>
          </Form.Item>
          <Divider style={{ margin: "12px 0" }} />
          <PermissionGuard action="site:write" resource="sites" fallback={null}>
            <Popconfirm
              title={tr("gateway.confirmDeleteSite")}
              onConfirm={() => deleteMutation.mutate()}
              okText={tc("confirmDelete")}
              cancelText={tc("cancel")}
            >
              <Button danger loading={deleteMutation.isPending}>
                {tr("gateway.deleteSite")}
              </Button>
            </Popconfirm>
          </PermissionGuard>
        </>
      )}
    </>
  );

  const renderSslTab = () => {
    const cert = certQuery.data as CertStatusResult | undefined;
    return (
      <div>
        <Descriptions bordered column={1} size="small" style={{ marginBottom: 16 }}>
          <Descriptions.Item label={tr("site.certStatus", "Status")}>
            {cert ? (
              <Tag color={cert.status === "valid" ? "green" : "red"}>{cert.status}</Tag>
            ) : (
              "-"
            )}
          </Descriptions.Item>
          <Descriptions.Item label={tr("site.certIssuer", "Issuer")}>
            {cert?.issuer ?? "-"}
          </Descriptions.Item>
          <Descriptions.Item label={tr("site.certExpiry", "Expiry")}>
            {cert?.expireTime ?? "-"}
          </Descriptions.Item>
          <Descriptions.Item label={tr("site.certDaysRemaining", "Days Remaining")}>
            {cert?.daysRemaining ?? "-"}
          </Descriptions.Item>
        </Descriptions>
        <Form.Item name="sslMode" initialValue={site.sslMode || "off"}>
          <GlFormRow label={tr("site.sslMode", "SSL Mode")}>
            <Select>
              <Select.Option value="off">off</Select.Option>
              <Select.Option value="auto">auto</Select.Option>
              <Select.Option value="manual">manual</Select.Option>
            </Select>
          </GlFormRow>
        </Form.Item>
        <Form.Item name="forceHttps" valuePropName="checked" initialValue={site.forceHttps}>
          <GlFormRow label={tr("site.forceHttps", "Force HTTPS")}>
            <Switch />
          </GlFormRow>
        </Form.Item>
        <Divider style={{ margin: "12px 0" }} />
        <Space>
          <PermissionGuard action="site:write" resource="sites" fallback={null}>
            <Button
              type="primary"
              loading={provisionCertMutation.isPending}
              onClick={() => provisionCertMutation.mutate()}
            >
              {tr("site.provisionCert", "Provision Certificate")}
            </Button>
          </PermissionGuard>
          <PermissionGuard action="site:write" resource="sites" fallback={null}>
            <Button
              loading={renewCertMutation.isPending}
              onClick={() => renewCertMutation.mutate()}
            >
              {tr("site.renewCert", "Renew Certificate")}
            </Button>
          </PermissionGuard>
        </Space>
      </div>
    );
  };

  const renderHealthTab = () => {
    // 健康检查接口返回单个快照对象（契约 ResourceDetail），表格需要数组。
    const healthSnapshot = healthQuery.data;
    const healthHistory: HealthCheckResult[] = healthSnapshot
      ? Array.isArray(healthSnapshot)
        ? healthSnapshot
        : [healthSnapshot]
      : [];
    return (
      <div>
        <Form.Item
          name="enableHealthCheck"
          valuePropName="checked"
          initialValue={site.enableHealthCheck}
        >
          <GlFormRow label={tr("site.enableHealthCheck", "Enable Health Check")}>
            <Switch />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="healthCheckInterval" initialValue={site.healthCheckInterval}>
          <GlFormRow label={tr("site.healthCheckInterval", "Health Check Interval (seconds)")}>
            <InputNumber min={0} />
          </GlFormRow>
        </Form.Item>
        <Divider style={{ margin: "12px 0" }} />
        <div style={{ marginBottom: 16 }}>
          <PermissionGuard action="site:write" resource="sites" fallback={null}>
            <Button
              type="primary"
              loading={checkHealthMutation.isPending}
              onClick={() => checkHealthMutation.mutate()}
            >
              {tr("site.checkHealth", "Check Health Now")}
            </Button>
          </PermissionGuard>
        </div>
        <Table
          dataSource={healthHistory}
          rowKey="checkedAt"
          size="small"
          pagination={false}
          columns={[
            {
              title: tr("site.healthStatus", "Status"),
              dataIndex: "status",
              render: (v: string) => <Tag color={v === "healthy" ? "green" : "red"}>{v}</Tag>,
            },
            { title: tr("site.statusCode", "Status Code"), dataIndex: "statusCode" },
            { title: tr("site.latency", "Latency (ms)"), dataIndex: "latencyMs" },
            { title: tr("site.checkedAt", "Checked At"), dataIndex: "checkedAt" },
            { title: tr("site.error", "Error"), dataIndex: "error", ellipsis: true },
          ]}
        />
      </div>
    );
  };

  const renderAlertTab = () => (
    <div>
      <Form.Item
        label={tr("site.enableAlert", "Enable Alert")}
        name="enableAlert"
        valuePropName="checked"
        initialValue={site.enableAlert}
      >
        <Switch />
      </Form.Item>
      <Form.Item
        label={tr("site.alertInterval", "Alert Interval (seconds)")}
        name="alertInterval"
        initialValue={site.alertInterval}
      >
        <InputNumber min={60} />
      </Form.Item>
      <Form.Item
        label={tr("site.alertTryTimes", "Alert Try Times")}
        name="alertTryTimes"
        initialValue={site.alertTryTimes}
      >
        <InputNumber min={1} max={10} />
      </Form.Item>
    </div>
  );

  const sectionItems: SectionNavItem[] = [
    { key: "general", anchor: "site-section-general", label: tr("site.general", "General") },
    { key: "ssl", anchor: "site-section-ssl", label: tr("site.ssl", "SSL") },
    { key: "health", anchor: "site-section-health", label: tr("site.health", "Health") },
    { key: "alert", anchor: "site-section-alert", label: tr("site.alert", "Alert") },
  ];

  return (
    <ResourceFormPage
      title={tr("gateway.editSite")}
      description={tr("gateway.editSiteDescription")}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/sites")}
      onFinish={async (values: SiteFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      submitPermission={{ permission: "site:write", resource: "sites" }}
    >
      <SectionNavShell items={sectionItems}>
        <div className="gl-section-anchors">
          <div id="site-section-general" className="gl-section-anchor">
            <FormSection title={tr("site.general", "General")}>{renderGeneralTab()}</FormSection>
          </div>
          <div id="site-section-ssl" className="gl-section-anchor">
            <FormSection title={tr("site.ssl", "SSL")}>{renderSslTab()}</FormSection>
          </div>
          <div id="site-section-health" className="gl-section-anchor">
            <FormSection title={tr("site.health", "Health")}>{renderHealthTab()}</FormSection>
          </div>
          <div id="site-section-alert" className="gl-section-anchor">
            <FormSection title={tr("site.alert", "Alert")}>{renderAlertTab()}</FormSection>
          </div>
        </div>
      </SectionNavShell>
    </ResourceFormPage>
  );
}
