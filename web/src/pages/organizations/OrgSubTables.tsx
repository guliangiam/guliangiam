/**
 * Organization 详情页内嵌子表组件
 *
 * 从 OrganizationDetailPage.tsx 拆分而来。各自独立发起查询，
 * 仅通过 `organizationId` prop 与主页面耦合，保持可单独测试与复用。
 */
import { useState } from "react";
import { Alert, Button, Skeleton } from "antd";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { LdapTable } from "@/shared/ui/LdapTable";
import { TransactionTable } from "@/shared/ui/TransactionTable";
import { getIamLdaps } from "@/shared/api/iam/ldap";
import { getIamTransactionsPaginated } from "@/shared/api/iam/transaction";
import { queryKeys } from "@/shared/api/query-keys";

const TRANSACTION_PAGE_SIZE = 10;

export function OrgLdapSubTable({ organizationId }: { organizationId: string }) {
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { data, isLoading, isError, refetch, isFetching } = useQuery({
    queryKey: queryKeys.ldaps.org(organizationId),
    queryFn: () => getIamLdaps({ owner: organizationId }),
    enabled: !!organizationId,
  });
  if (isLoading) {
    return <Skeleton active paragraph={{ rows: 3 }} title={false} />;
  }
  if (isError) {
    return (
      <Alert
        type="error"
        showIcon
        title={tr("organization.ldapLoadFailed")}
        action={
          <Button size="small" loading={isFetching} onClick={() => void refetch()}>
            {t("retry")}
          </Button>
        }
      />
    );
  }
  // 空态由 LdapTable 统一渲染（Empty + 说明），这里只负责加载/错误。
  return <LdapTable value={data ?? []} organizationId={organizationId} readOnly />;
}

export function OrgTransactionSubTable({ organizationId }: { organizationId: string }) {
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const [page, setPage] = useState(1);
  const { data, isLoading, isError, refetch, isFetching } = useQuery({
    queryKey: [...queryKeys.transactions.org(organizationId), page],
    queryFn: () =>
      getIamTransactionsPaginated({
        owner: organizationId,
        page,
        pageSize: TRANSACTION_PAGE_SIZE,
      }),
    enabled: !!organizationId,
  });
  if (isLoading) {
    return <Skeleton active paragraph={{ rows: 3 }} title={false} />;
  }
  if (isError) {
    return (
      <Alert
        type="error"
        showIcon
        title={tr("organization.transactionsLoadFailed")}
        action={
          <Button size="small" loading={isFetching} onClick={() => void refetch()}>
            {t("retry")}
          </Button>
        }
      />
    );
  }
  return (
    <TransactionTable
      value={data?.items ?? []}
      readOnly
      pagination={{
        current: page,
        pageSize: TRANSACTION_PAGE_SIZE,
        total: data?.total ?? 0,
        showSizeChanger: false,
        onChange: (nextPage) => setPage(nextPage),
      }}
    />
  );
}
