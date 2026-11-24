import {
  Alert,
  Button,
  Card,
  Col,
  Form,
  Input,
  Popconfirm,
  Row,
  Select,
  Space,
  Tag,
  Typography,
} from "antd";
import type { ColumnsType } from "antd/es/table";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { ResourceListCard } from "@/shared/ui/ResourceListCard";
import type { DomainResponse, DomainStatus } from "@/shared/api/iam/b2b";
import type { useOrgDomainsPageController } from "./useOrgDomainsPageController";
import { STATUS_COLOR, DOMAIN_REGEX, dnsTxtHost, normalizeDomain } from "@/shared/lib/b2b-domain";

type Props = { ctrl: ReturnType<typeof useOrgDomainsPageController> };

const { Text } = Typography;

export function OrgDomainsPageView({ ctrl }: Props) {
  const {
    tc,
    organizationId,
    setOrganizationId,
    statusFilter,
    setStatusFilter,
    query,
    total,
    page,
    pageSize,
    setPagination,
    claimForm,
    pendingClaimToken,
    setPendingClaimToken,
    claimMutation,
    deleteMutation,
    verifyMutation,
    handleClaim,
  } = ctrl;

  const columns: ColumnsType<DomainResponse> = [
    {
      title: tc("b2b.domain", { defaultValue: "Domain" }),
      dataIndex: "domain",
      key: "domain",
      width: 220,
      render: (value: string) => <Text code>{value}</Text>,
    },
    {
      title: tc("status", { defaultValue: "Status" }),
      dataIndex: "status",
      key: "status",
      width: 120,
      render: (value: DomainStatus) => (
        <Tag color={STATUS_COLOR[value]}>{tc(`b2b.status.${value}`, { defaultValue: value })}</Tag>
      ),
    },
    {
      title: tc("b2b.verificationToken", { defaultValue: "Verification Token" }),
      dataIndex: "verificationToken",
      key: "verificationToken",
      width: 240,
      render: (value: string) => (
        <Text code copyable>
          {value}
        </Text>
      ),
    },
    {
      title: tc("b2b.verifiedAt", { defaultValue: "Verified At" }),
      dataIndex: "verifiedAt",
      key: "verifiedAt",
      width: 180,
      render: (value?: string) => (value ? new Date(value).toLocaleString() : "-"),
    },
    {
      title: tc("createdAt", { defaultValue: "Created At" }),
      dataIndex: "createdAt",
      key: "createdAt",
      width: 180,
      render: (value: string) => new Date(value).toLocaleString(),
    },
    {
      title: tc("actions", { defaultValue: "Actions" }),
      key: "actions",
      width: 180,
      fixed: "right",
      render: (_value, record) => (
        <PermissionGuard action="b2b:write" fallback={null}>
          <Space size="small" style={{ flexWrap: "nowrap" }}>
            <Button
              size="small"
              type="link"
              disabled={record.status === "verified" || verifyMutation.isPending}
              loading={verifyMutation.isPending && verifyMutation.variables?.domainId === record.id}
              onClick={() =>
                organizationId &&
                verifyMutation.mutate({ orgId: organizationId, domainId: record.id })
              }
            >
              {tc("b2b.verify", { defaultValue: "Verify" })}
            </Button>
            <Popconfirm
              title={tc("b2b.deleteConfirm", {
                domain: record.domain,
                defaultValue: "Delete domain claim for {{domain}}?",
              })}
              okText={tc("delete", { defaultValue: "Delete" })}
              cancelText={tc("cancel", { defaultValue: "Cancel" })}
              okButtonProps={{
                danger: true,
                loading:
                  deleteMutation.isPending && deleteMutation.variables?.domainId === record.id,
              }}
              onConfirm={() =>
                organizationId &&
                deleteMutation.mutate({ orgId: organizationId, domainId: record.id })
              }
            >
              <Button size="small" type="link" danger disabled={deleteMutation.isPending}>
                {tc("delete", { defaultValue: "Delete" })}
              </Button>
            </Popconfirm>
          </Space>
        </PermissionGuard>
      ),
    },
  ];

  return (
    <div className="gl-resource-page">
      <Card size="small" className="gl-panel" style={{ marginBottom: 16 }}>
        <Row gutter={[16, 12]} align="middle">
          <Col xs={24} md={12} lg={8}>
            <Text strong style={{ display: "block", marginBottom: 8 }}>
              {tc("b2b.organization", { defaultValue: "Organization" })}
            </Text>
            <OrganizationSelect
              value={organizationId ?? undefined}
              onChange={(value) => setOrganizationId(value ?? null)}
              style={{ width: "100%" }}
              autoFillSessionOrg={false}
              allowClear
              ariaLabel={tc("b2b.organization", { defaultValue: "Organization" })}
            />
          </Col>
          <Col xs={24} md={12} lg={8}>
            <Text strong style={{ display: "block", marginBottom: 8 }}>
              {tc("status", { defaultValue: "Status" })}
            </Text>
            <Select<DomainStatus>
              value={statusFilter}
              onChange={setStatusFilter}
              allowClear
              placeholder={tc("b2b.allStatuses", { defaultValue: "All statuses" })}
              style={{ width: "100%" }}
              options={[
                {
                  value: "pending",
                  label: tc("b2b.status.pending", { defaultValue: "Pending" }),
                },
                {
                  value: "verified",
                  label: tc("b2b.status.verified", { defaultValue: "Verified" }),
                },
                { value: "failed", label: tc("b2b.status.failed", { defaultValue: "Failed" }) },
              ]}
            />
          </Col>
        </Row>
      </Card>

      <PermissionGuard action="b2b:write" fallback={null}>
        <Card
          title={tc("b2b.claimTitle", { defaultValue: "Claim New Domain" })}
          size="small"
          className="gl-panel"
          style={{ marginBottom: 16 }}
          extra={
            pendingClaimToken ? (
              <Button size="small" type="link" onClick={() => setPendingClaimToken(null)}>
                {tc("b2b.clearDnsInstructions", { defaultValue: "Clear DNS instructions" })}
              </Button>
            ) : null
          }
        >
          <Form
            form={claimForm}
            layout="inline"
            onFinish={handleClaim}
            disabled={claimMutation.isPending || !organizationId}
          >
            <Form.Item
              label={tc("b2b.domain", { defaultValue: "Domain" })}
              name="domain"
              rules={[
                {
                  required: true,
                  whitespace: true,
                  message: tc("b2b.domainRequired", { defaultValue: "Domain is required" }),
                },
                {
                  validator: (_rule, value: string) =>
                    !value || DOMAIN_REGEX.test(normalizeDomain(value))
                      ? Promise.resolve()
                      : Promise.reject(
                          new Error(
                            tc("b2b.domainInvalid", {
                              defaultValue: "Enter a valid domain name (e.g. example.com)",
                            }),
                          ),
                        ),
                },
              ]}
            >
              <Input placeholder="example.com" style={{ width: 280 }} />
            </Form.Item>
            <Form.Item>
              <Button type="primary" htmlType="submit" loading={claimMutation.isPending}>
                {tc("b2b.claim", { defaultValue: "Claim Domain" })}
              </Button>
            </Form.Item>
          </Form>

          {pendingClaimToken && (
            <Alert
              type="info"
              showIcon
              style={{ marginTop: 12 }}
              title={
                <Space orientation="vertical" size={4}>
                  <Text strong>
                    {tc("b2b.dnsInstructions", {
                      domain: pendingClaimToken.domain,
                      defaultValue: "Add this DNS TXT record to {{domain}}:",
                    })}
                  </Text>
                  <Text code copyable>
                    {tc("b2b.dnsHost", { defaultValue: "Host" })}:{" "}
                    {dnsTxtHost(pendingClaimToken.domain)}
                  </Text>
                  <Text code copyable>
                    {tc("b2b.dnsValue", { defaultValue: "Value" })}: {pendingClaimToken.token}
                  </Text>
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    {tc("b2b.dnsHint", {
                      defaultValue:
                        "After DNS propagates, click Verify on the domain row. The cron worker also verifies pending domains automatically.",
                    })}
                  </Text>
                </Space>
              }
            />
          )}
        </Card>
      </PermissionGuard>

      <ResourceListCard<DomainResponse>
        title={tc("b2b.title", { defaultValue: "B2B Domain Governance" })}
        description={tc("b2b.description", {
          defaultValue:
            "Claim organizational domains through DNS TXT verification for SSO federation routing.",
        })}
        query={query}
        columns={columns}
        rowKey="id"
        hideSearch
        showRefresh={Boolean(organizationId)}
        tableId="org-domains"
        tableScrollX={1220}
        emptyContent={
          organizationId
            ? tc("b2b.noDomains", { defaultValue: "No domains claimed yet" })
            : tc("b2b.selectOrganization", { defaultValue: "Select an organization" })
        }
        serverPagination={{
          page,
          pageSize,
          total,
          onChange: setPagination,
        }}
      />
    </div>
  );
}
