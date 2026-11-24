import { createDetailLoader, createListLoader } from "@/shared/api/loaders/factory";
import { getApplication, getApplicationsPaginated } from "@/shared/api/iam/application";
import { getRolesPaginated, getRole } from "@/shared/api/iam/role";
import { getPermissionsPaginated, getPermission } from "@/shared/api/iam/permission";
import { getSessionsPaginated } from "@/shared/api/iam/session";
import { getTokensPaginated } from "@/shared/api/iam/token-extended";
import { getProvider, getProvidersPaginated } from "@/shared/api/iam/provider-extended";

export const applicationsListLoader = createListLoader({
  fetch: async (ctx, params) =>
    getApplicationsPaginated({
      organizationId: ctx.orgId ?? undefined,
      search: params.keyword,
      page: params.page ?? 1,
      pageSize: params.pageSize ?? 10,
    }),
});
export const applicationDetailLoader = createDetailLoader({
  fetch: async (_ctx, id) => getApplication(id),
});

export const rolesListLoader = createListLoader({
  fetch: async (_ctx, params) =>
    getRolesPaginated({
      search: params.keyword,
      page: params.page ?? 1,
      pageSize: params.pageSize ?? 10,
    }),
});
export const roleDetailLoader = createDetailLoader({ fetch: async (_ctx, id) => getRole(id) });

export const permissionsListLoader = createListLoader({
  fetch: async (_ctx, params) =>
    getPermissionsPaginated({
      search: params.keyword,
      page: params.page ?? 1,
      pageSize: params.pageSize ?? 10,
    }),
});
export const permissionDetailLoader = createDetailLoader({
  fetch: async (_ctx, id) => getPermission(id),
});

export const sessionsListLoader = createListLoader({
  fetch: async (_ctx, params) =>
    getSessionsPaginated({
      search: params.keyword,
      page: params.page ?? 1,
      pageSize: params.pageSize ?? 10,
    }),
});

export const tokensListLoader = createListLoader({
  fetch: async (_ctx, params) =>
    getTokensPaginated({
      search: params.keyword,
      page: params.page ?? 1,
      pageSize: params.pageSize ?? 10,
    }),
});

export const providersListLoader = createListLoader({
  fetch: async (_ctx, params) =>
    getProvidersPaginated({
      search: params.keyword,
      page: params.page ?? 1,
      pageSize: params.pageSize ?? 10,
    }),
});
export const providerDetailLoader = createDetailLoader({
  fetch: async (_ctx, id) => getProvider(id),
});
