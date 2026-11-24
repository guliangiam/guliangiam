import { useCallback, useState } from "react";
import { App } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import {
  getInvitationsPaginated,
  resendInvitation,
  revokeInvitation,
  sendInvitation,
  type InvitationDetail,
  type InvitationListParams,
} from "@/shared/api/iam/invitation";
import { getGroupOptions } from "@/shared/api/iam/group";
import { getRoleOptions } from "@/shared/api/iam/role";
import { getOrganizations } from "@/shared/api/iam/organization";
import { queryKeys } from "@/shared/api/query-keys";
import { useCrudList } from "@/shared/hooks/useCrudList";
import { useOrganizationNames, useUserNames } from "@/shared/hooks/useResourceNames";
import { localizedError } from "@/shared/lib/server-error";

export type InvitationBulkAction = "send" | "revoke" | "resend";

const BULK_ACTION_FN = {
  send: sendInvitation,
  revoke: revokeInvitation,
  resend: resendInvitation,
} as const;

const BULK_SUCCESS_KEY = {
  send: "invitation.bulkSendSuccess",
  revoke: "invitation.bulkRevokeSuccess",
  resend: "invitation.bulkResendSuccess",
} as const;

// 与壳层 bulkDelete 一致的分批并发上限，避免百条选中瞬时百并发触发限流。
const BULK_ACTION_BATCH_SIZE = 10;

export function useInvitationsPageController() {
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);
  const organization = useSessionStore((state) => state.organization);
  const queryClient = useQueryClient();

  const { nameMap: orgNameMap } = useOrganizationNames();

  // 组织列表行的「邀请」快捷入口会带 ?organizationId=<id>，进入即建立组织上下文。
  // URL 是唯一可信来源：显式选择记录在 orgSelection 中，且只在它绑定的 URL 上下文
  // 仍然成立时生效；前进/后退或同页跳转改变 URL 后它会自动失效，不会覆盖 URL。
  // 保留显式清除（值为 ""）语义，避免回落到会话组织而无法查看全部组织。
  const [searchParams, setSearchParams] = useSearchParams();
  const orgContext = searchParams.get("organizationId") ?? "";
  const [orgSelection, setOrgSelection] = useState<{ context: string; value: string } | null>(null);
  const orgFilter =
    orgSelection && orgSelection.context === orgContext
      ? orgSelection.value
      : orgContext || (organization?.id ?? "");

  // 选择/清除组织时同步 URL：清除即移除 organizationId，避免上下文被静默丢弃。
  const handleOrgChange = useCallback(
    (value: string) => {
      const next = value || "";
      setOrgSelection({ context: next, value: next });
      setSearchParams(
        (prev) => {
          const params = new URLSearchParams(prev);
          if (next) params.set("organizationId", next);
          else params.delete("organizationId");
          return params;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

  // ── 服务端分页：后端 GET /api/v1/invitations 支持 page/pageSize/keyword/status ──
  // 查询键保持 ["invitations","auth",…] 前缀，refetch() 的整族失效仍覆盖详情页订阅。
  const list = useCrudList<InvitationDetail, InvitationListParams, string>({
    buildQueryKey: (p) =>
      [
        "invitations",
        "auth",
        "paginated",
        p.page,
        p.pageSize,
        p.search,
        p.status,
        p.organizationId,
      ] as const,
    buildParams: ({ page, pageSize, search, filters }) => ({
      page,
      pageSize,
      search: search || undefined,
      status: filters.status || undefined,
      organizationId: orgFilter || undefined,
    }),
    paginatedFn: getInvitationsPaginated,
    enabled: account !== null,
  });

  // 详情页订阅 ["invitations","auth"]（全量列表，SEMI_STATIC）与
  // ["invitation","auth","detail",id]（单数前缀，DYNAMIC）；列表自身键带 paginated
  // 后缀，只 refetch 本键无法失效它们——动作后必须一并失效，否则详情页在
  // staleTime 窗口内显示陈旧状态（已撤销仍显待接受、Revoke 可点）。
  const refetch = () => {
    queryClient.invalidateQueries({ queryKey: ["invitations", "auth"] });
    queryClient.invalidateQueries({ queryKey: ["invitation"] });
  };

  const revokeMutation = useMutation({
    mutationFn: (id: string) => revokeInvitation(id),
    onSuccess: () => {
      message.success(tr("invitation.revoked"));
      refetch();
    },
    onError: (error) => {
      message.error(localizedError(error, t, "operationFailed"));
    },
  });

  const resendMutation = useMutation({
    mutationFn: (id: string) => resendInvitation(id),
    onSuccess: () => {
      message.success(tr("invitation.resent"));
      refetch();
    },
    onError: (error) => {
      message.error(localizedError(error, t, "operationFailed"));
    },
  });

  const sendMutation = useMutation({
    mutationFn: (id: string) => sendInvitation(id),
    onSuccess: () => {
      message.success(tr("invitation.sendSuccess", "Invitation sent"));
      refetch();
    },
    onError: (error) => {
      message.error(localizedError(error, t, "operationFailed"));
    },
  });

  // ── 批量操作：后端只有单条 send/revoke/resend 端点，逐条并发调用后汇总 ──
  const bulkActionMutation = useMutation({
    mutationFn: async ({ action, ids }: { action: InvitationBulkAction; ids: string[] }) => {
      const fn = BULK_ACTION_FN[action];
      let succeeded = 0;
      const failedIds: string[] = [];
      for (let i = 0; i < ids.length; i += BULK_ACTION_BATCH_SIZE) {
        const batch = ids.slice(i, i + BULK_ACTION_BATCH_SIZE);
        const results = await Promise.allSettled(batch.map((id) => fn(id)));
        results.forEach((result, index) => {
          if (result.status === "fulfilled") {
            succeeded += 1;
          } else {
            failedIds.push(batch[index]);
          }
        });
      }
      return { total: ids.length, succeeded, failedIds };
    },
    onSuccess: (summary, { action }) => {
      if (summary.failedIds.length > 0) {
        message.warning(
          tr("invitation.batchPartial", {
            success: summary.succeeded,
            failed: summary.failedIds.length,
          }),
        );
        // 失败行保留选中，便于直接重试；成功行已随 refetch 更新状态。
        list.setSelectedKeys(summary.failedIds);
      } else {
        message.success(tr(BULK_SUCCESS_KEY[action], { count: summary.total }));
        list.setSelectedKeys([]);
      }
      refetch();
    },
    onError: (error) => {
      message.error(localizedError(error, t, "operationFailed"));
    },
  });

  // ── 详情抽屉：按选中行的组织懒加载角色/分组名，组织列表用于接受链接的 slug ──
  const [detailTarget, setDetailTarget] = useState<InvitationDetail | null>(null);
  const detailOrgId = detailTarget?.organizationId;

  const { data: roleOptions = [] } = useQuery({
    queryKey: queryKeys.roles.options(detailOrgId),
    queryFn: () => getRoleOptions(detailOrgId),
    enabled: Boolean(detailOrgId),
  });
  const { data: groupOptions = [] } = useQuery({
    queryKey: ["group-options", detailOrgId],
    queryFn: () => getGroupOptions(detailOrgId),
    enabled: Boolean(detailOrgId),
  });
  const { data: organizations = [] } = useQuery({
    queryKey: [...queryKeys.organizations.all, "slug-map"],
    queryFn: () => getOrganizations(),
    staleTime: 60_000,
  });

  const organizationSlug = (organizationId?: string) =>
    organizations.find((org) => org.id === organizationId)?.slug ?? "";

  /** 接受链接指向邀请注册页，携带邀请码与组织 slug（注册接口按 slug 定位组织）。 */
  const acceptLink = (invitation: InvitationDetail) => {
    const params = new URLSearchParams();
    if (invitation.code) params.set("code", invitation.code);
    const slug = organizationSlug(invitation.organizationId);
    if (slug) params.set("organization", slug);
    return `${window.location.origin}/register/invitation?${params.toString()}`;
  };

  const copyAcceptLink = async (invitation: InvitationDetail) => {
    try {
      await navigator.clipboard.writeText(acceptLink(invitation));
      message.success(tr("invitation.acceptLinkCopied", "Accept link copied"));
    } catch {
      message.error(tr("invitation.copyFailed", "Copy failed"));
    }
  };

  const roleName = (id: string) => roleOptions.find((role) => role.id === id)?.name ?? id;
  const groupName = (id: string) =>
    groupOptions.find((group) => group.id === id)?.displayName ?? id;

  // 邀请人列：UUID → 可读用户名；解析不到时由视图降级为缩短 id。
  const { nameMap: userNameMap } = useUserNames();

  return {
    t,
    tr,
    account,
    orgFilter,
    orgContext,
    orgNameMap,
    handleOrgChange,
    list,
    revokeMutation,
    resendMutation,
    sendMutation,
    bulkActionMutation,
    detailTarget,
    setDetailTarget,
    acceptLink,
    copyAcceptLink,
    roleName,
    groupName,
    userNameMap,
  };
}
