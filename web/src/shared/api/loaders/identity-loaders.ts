/**
 * Identity 域 Loaders — organizations / users / groups / invitations
 *
 * 每个 loader 3-5 行，业务逻辑复用 API client。
 * 路由层加 `loader:` 属性，Page 组件用 `useLoaderData()` 取数据。
 */

import { createDetailLoader, createListLoader } from "@/shared/api/loaders/factory";
import {
  getOrganizationIdentityBySlug,
  getOrganizationsPaginated,
  type OrganizationListParams,
} from "@/shared/api/iam/organization";
import { getGroupsPaginated } from "@/shared/api/iam/group";
import { getInvitationsPaginated } from "@/shared/api/iam/invitation";

// ── Organizations ──

export const organizationsListLoader = createListLoader({
  fetch: async (ctx, params) => {
    // OrganizationListParams 只接受 page/pageSize/search/status；
    // 组织作用域由请求头与 RLS 决定，排序不在该契约内。
    const listParams: OrganizationListParams = {
      search: params.keyword,
      page: params.page ?? 1,
      pageSize: params.pageSize ?? 10,
    };
    return getOrganizationsPaginated(listParams);
  },
});

export const organizationDetailLoader = createDetailLoader({
  fetch: async (_ctx, slug) => getOrganizationIdentityBySlug(slug),
  idParam: "organizationName",
});

// ── Groups ──

export const groupsListLoader = createListLoader({
  fetch: async (_ctx, params) => {
    return getGroupsPaginated({
      search: params.keyword,
      page: params.page ?? 1,
      pageSize: params.pageSize ?? 10,
    });
  },
});

// ── Invitations ──

export const invitationsListLoader = createListLoader({
  fetch: async (ctx, params) => {
    return getInvitationsPaginated({
      search: params.keyword,
      page: params.page ?? 1,
      pageSize: params.pageSize ?? 10,
      organizationId: ctx.orgId ?? undefined,
    });
  },
});
