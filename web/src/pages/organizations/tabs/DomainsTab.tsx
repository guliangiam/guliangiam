/**
 * Organization 详情页 · 内嵌域名治理
 *
 * 复用 B2B 域名 API，但不再跳转独立路由：认领、验证、删除都在详情页内完成。
 * 独立路由 `/organizations/:name/domains` 仍保留为兼容入口。
 */
import { useState } from "react";
import { App, Alert, Button, Input, Popconfirm, Space, Table, Tag, Typography } from "antd";
import type { ColumnsType } from "antd/es/table";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { queryKeys } from "@/shared/api/query-keys";
import {
  claimDomain,
  deleteDomain,
  listDomains,
  verifyDomain,
  type DomainResponse,
  type DomainStatus,
} from "@/shared/api/iam/b2b";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { handleMutationError } from "@/shared/hooks/useMutationError";
import { DOMAIN_REGEX, STATUS_COLOR, dnsTxtHost, normalizeDomain } from "@/shared/lib/b2b-domain";

type DomainsTabProps = {
  organizationId: string;
};

const { Text } = Typography;

export function DomainsTab({ organizationId }: DomainsTabProps) {
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const queryClient = useQueryClient();
  const [domain, setDomain] = useState("");
  const [domainError, setDomainError] = useState<string | null>(null);
  const [pendingClaimToken, setPendingClaimToken] = useState<{
    domain: string;
    token: string;
  } | null>(null);

  const domainsQuery = useQuery({
    queryKey: queryKeys.b2bDomains.list({
      organizationId,
      page: 1,
      pageSize: 100,
    }),
    queryFn: () => listDomains({ organizationId, page: 1, pageSize: 100 }),
    enabled: organizationId !== "",
  });

  const invalidateDomains = () =>
    queryClient.invalidateQueries({ queryKey: queryKeys.b2bDomains.all });

  const claimMutation = useMutation({
    mutationFn: claimDomain,
    onSuccess: (claimed) => {
      setPendingClaimToken({ domain: claimed.domain, token: claimed.verificationToken });
      setDomain("");
      setDomainError(null);
      message.success(tc("b2b.claimSuccess", { defaultValue: "Domain claimed" }));
      void invalidateDomains();
    },
    onError: (error) =>
      handleMutationError(error, tc("b2b.claimFailed", { defaultValue: "Failed to claim domain" })),
  });

  const verifyMutation = useMutation({
    mutationFn: (domainId: string) => verifyDomain(organizationId, domainId),
    onSuccess: (result) => {
      if (result.verified) {
        message.success(
          tc("b2b.verifySuccess", {
            domain: result.domain.domain,
            defaultValue: "Domain verified: {{domain}}",
          }),
        );
      } else {
        message.warning(
          tc("b2b.verifyNotVerified", {
            domain: result.domain.domain,
            defaultValue: "DNS verification did not pass for {{domain}}",
          }),
        );
      }
      void invalidateDomains();
    },
    onError: (error) =>
      handleMutationError(
        error,
        tc("b2b.verifyFailed", { defaultValue: "Failed to verify domain" }),
      ),
  });

  const deleteMutation = useMutation({
    mutationFn: (domainId: string) => deleteDomain(organizationId, domainId),
    onSuccess: () => {
      message.success(tc("b2b.deleteSuccess", { defaultValue: "Domain deleted" }));
      void invalidateDomains();
    },
    onError: (error) =>
      handleMutationError(
        error,
        tc("b2b.deleteFailed", { defaultValue: "Failed to delete domain" }),
      ),
  });

  const submitClaim = () => {
    const normalized = normalizeDomain(domain);
    if (!DOMAIN_REGEX.test(normalized)) {
      setDomainError(
        tc("b2b.domainInvalid", {
          defaultValue: "Enter a valid domain name (e.g. example.com)",
        }),
      );
      return;
    }
    setDomainError(null);
    claimMutation.mutate({ organizationId, domain: normalized });
  };

  const columns: ColumnsType<DomainResponse> = [
    {
      title: tc("b2b.domain", { defaultValue: "Domain" }),
      dataIndex: "domain",
      key: "domain",
      render: (value: string) => <Text code>{value}</Text>,
    },
    {
      title: tc("status", { defaultValue: "Status" }),
      dataIndex: "status",
      key: "status",
      width: 110,
      render: (value: DomainStatus) => (
        <Tag color={STATUS_COLOR[value]}>{tc(`b2b.status.${value}`, { defaultValue: value })}</Tag>
      ),
    },
    {
      title: tc("b2b.verificationToken", { defaultValue: "Verification Token" }),
      dataIndex: "verificationToken",
      key: "verificationToken",
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
      width: 170,
      render: (value?: string) => (value ? new Date(value).toLocaleString() : "-"),
    },
    {
      title: tc("actions", { defaultValue: "Actions" }),
      key: "actions",
      width: 150,
      render: (_value, record) => (
        <PermissionGuard action="b2b:write" fallback={null}>
          <Space size="small" style={{ flexWrap: "nowrap" }}>
            <Button
              size="small"
              type="link"
              disabled={record.status === "verified" || verifyMutation.isPending}
              loading={verifyMutation.isPending && verifyMutation.variables === record.id}
              onClick={() => verifyMutation.mutate(record.id)}
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
                loading: deleteMutation.isPending && deleteMutation.variables === record.id,
              }}
              onConfirm={() => deleteMutation.mutate(record.id)}
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
    <div style={{ marginTop: 16 }}>
      <Text strong style={{ display: "block", marginBottom: 8 }}>
        {tc("b2b.claimTitle", { defaultValue: "Claim New Domain" })}
      </Text>

      <PermissionGuard action="b2b:write" fallback={null}>
        <Space.Compact style={{ width: "100%" }}>
          <Input
            value={domain}
            status={domainError ? "error" : undefined}
            placeholder="example.com"
            aria-label={tc("b2b.domain", { defaultValue: "Domain" })}
            disabled={claimMutation.isPending || organizationId === ""}
            onChange={(event) => {
              setDomain(event.target.value);
              if (domainError) setDomainError(null);
            }}
            onPressEnter={(event) => {
              // 输入框位于组织主表单内：阻止 Enter 触发整页保存。
              event.preventDefault();
              submitClaim();
            }}
          />
          <Button
            type="primary"
            loading={claimMutation.isPending}
            disabled={organizationId === ""}
            onClick={submitClaim}
          >
            {tc("b2b.claim", { defaultValue: "Claim Domain" })}
          </Button>
        </Space.Compact>
        {domainError && (
          <Text type="danger" style={{ display: "block", marginTop: 4 }}>
            {domainError}
          </Text>
        )}
      </PermissionGuard>

      {pendingClaimToken && (
        <Alert
          type="info"
          showIcon
          closable
          style={{ marginTop: 12 }}
          onClose={() => setPendingClaimToken(null)}
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

      {domainsQuery.isError && (
        <Alert
          type="error"
          showIcon
          style={{ marginTop: 12 }}
          title={tr("organization.domainsLoadFailed")}
          action={
            <Button
              size="small"
              loading={domainsQuery.isFetching}
              onClick={() => void domainsQuery.refetch()}
            >
              {tc("retry")}
            </Button>
          }
        />
      )}

      <Table<DomainResponse>
        style={{ marginTop: 12 }}
        size="small"
        rowKey="id"
        columns={columns}
        dataSource={domainsQuery.data?.items ?? []}
        loading={domainsQuery.isLoading}
        pagination={false}
        scroll={{ x: 900 }}
        locale={{
          emptyText: tc("b2b.noDomains", { defaultValue: "No domains claimed yet" }),
        }}
      />
    </div>
  );
}
