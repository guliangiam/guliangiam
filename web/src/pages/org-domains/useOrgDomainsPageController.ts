import { useCallback, useEffect, useRef, useState } from "react";
import { App, Form } from "antd";
import { useTranslation } from "react-i18next";
import { useParams, useSearchParams } from "react-router-dom";
import { useMutation, useQuery, useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { useSessionStore } from "@/features/session/session-store";
import { queryKeys } from "@/shared/api/query-keys";
import { getOrganizationIdentityBySlug } from "@/shared/api/iam/organization";
import {
  listDomains,
  claimDomain,
  deleteDomain,
  verifyDomain,
  domainStatusSchema,
  type DomainResponse,
  type DomainStatus,
} from "@/shared/api/iam/b2b";
import { handleMutationError } from "@/shared/hooks/useMutationError";
import { normalizeDomain } from "@/shared/lib/b2b-domain";

const DEFAULT_PAGE_SIZE = 20;

function positiveIntegerParam(value: string | null, fallback: number, maximum?: number): number {
  if (!value) return fallback;
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 1 || (maximum !== undefined && parsed > maximum)) {
    return fallback;
  }
  return parsed;
}

export function useOrgDomainsPageController() {
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const { organizationName } = useParams<{ organizationName?: string }>();
  const sessionOrganizationId = useSessionStore((state) => state.organization?.id ?? undefined);
  const previousOrganizationId = useRef(sessionOrganizationId);

  const orgIdParam = searchParams.get("orgId");
  // 兼容路由 organizations/:organizationName/domains 的深链可能不带 ?orgId
  // （组织详情入口）：用路由 slug 经 identity resolver 兜底，否则整页禁用。
  const routeOrganizationQuery = useQuery({
    queryKey: ["organizations", "identity", organizationName],
    queryFn: () => getOrganizationIdentityBySlug(organizationName ?? ""),
    enabled: !orgIdParam && Boolean(organizationName),
  });
  const organizationId = orgIdParam || routeOrganizationQuery.data?.id || null;
  const page = positiveIntegerParam(searchParams.get("page"), 1);
  const pageSize = positiveIntegerParam(searchParams.get("pageSize"), DEFAULT_PAGE_SIZE, 100);
  const parsedStatus = domainStatusSchema.safeParse(searchParams.get("status"));
  const statusFilter = parsedStatus.success ? parsedStatus.data : undefined;

  const [claimForm] = Form.useForm<{ domain: string }>();
  const [pendingClaimToken, setPendingClaimToken] = useState<{
    domain: string;
    token: string;
  } | null>(null);

  const updateParams = useCallback(
    (updates: Record<string, string | null>) => {
      setSearchParams(
        (current) => {
          const next = new URLSearchParams(current);
          Object.entries(updates).forEach(([key, value]) => {
            if (value) next.set(key, value);
            else next.delete(key);
          });
          return next;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

  useEffect(() => {
    if (previousOrganizationId.current === sessionOrganizationId) return;
    const hadOrganization = Boolean(previousOrganizationId.current);
    previousOrganizationId.current = sessionOrganizationId;
    if (!hadOrganization) return;

    updateParams({ orgId: null, page: null, pageSize: null, status: null });
    setPendingClaimToken(null);
    claimForm.resetFields();
  }, [sessionOrganizationId, updateParams, claimForm]);

  const setOrganizationId = useCallback(
    (nextOrganizationId: string | null) => {
      setPendingClaimToken(null);
      claimForm.resetFields();
      updateParams({ orgId: nextOrganizationId, page: null });
    },
    [claimForm, setPendingClaimToken, updateParams],
  );

  const setStatusFilter = useCallback(
    (status?: DomainStatus) => updateParams({ status: status ?? null, page: null }),
    [updateParams],
  );

  const setPagination = useCallback(
    (nextPage: number, nextPageSize: number) => {
      const sizeChanged = nextPageSize !== pageSize;
      updateParams({
        page: sizeChanged || nextPage <= 1 ? null : String(nextPage),
        pageSize: nextPageSize === DEFAULT_PAGE_SIZE ? null : String(nextPageSize),
      });
    },
    [pageSize, updateParams],
  );

  const domainsQuery = useQuery({
    queryKey: queryKeys.b2bDomains.list({
      organizationId,
      page,
      pageSize,
      status: statusFilter ?? null,
    }),
    queryFn: () =>
      listDomains({
        organizationId: organizationId!,
        page,
        pageSize,
        status: statusFilter,
      }),
    enabled: Boolean(organizationId),
  });
  const listQuery = {
    ...domainsQuery,
    data: domainsQuery.data?.items ?? [],
  } as UseQueryResult<DomainResponse[], Error>;

  const invalidateDomains = useCallback(
    () => queryClient.invalidateQueries({ queryKey: queryKeys.b2bDomains.all }),
    [queryClient],
  );

  const claimMutation = useMutation({
    mutationFn: claimDomain,
    onSuccess: (domain) => {
      setPendingClaimToken({ domain: domain.domain, token: domain.verificationToken });
      message.success(tc("b2b.claimSuccess", { defaultValue: "Domain claimed" }));
      claimForm.resetFields();
      void invalidateDomains();
    },
    onError: (error) =>
      handleMutationError(error, tc("b2b.claimFailed", { defaultValue: "Failed to claim domain" })),
  });

  const deleteMutation = useMutation({
    mutationFn: async (variables: { orgId: string; domainId: string }) => {
      await deleteDomain(variables.orgId, variables.domainId);
    },
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

  const verifyMutation = useMutation({
    mutationFn: (variables: { orgId: string; domainId: string }) =>
      verifyDomain(variables.orgId, variables.domainId),
    onSuccess: (data) => {
      if (data.verified) {
        message.success(
          tc("b2b.verifySuccess", {
            domain: data.domain.domain,
            defaultValue: "Domain verified: {{domain}}",
          }),
        );
      } else {
        message.warning(
          tc("b2b.verifyNotVerified", {
            domain: data.domain.domain,
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

  const handleClaim = (values: { domain: string }) => {
    if (!organizationId) return;
    claimMutation.mutate({ organizationId, domain: normalizeDomain(values.domain) });
  };

  return {
    tc,
    organizationId,
    setOrganizationId,
    statusFilter,
    setStatusFilter,
    query: listQuery,
    total: domainsQuery.data?.page.total ?? 0,
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
  };
}
