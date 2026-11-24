import { lazy } from "react";
import type { RouteObject } from "react-router-dom";
import { lazyRouteLoader, withSuspense } from "./shared";
const organizationsListLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/identity-loaders"),
  "organizationsListLoader",
);
const organizationDetailLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/identity-loaders"),
  "organizationDetailLoader",
);
const groupsListLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/identity-loaders"),
  "groupsListLoader",
);
const invitationsListLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/identity-loaders"),
  "invitationsListLoader",
);

const OrganizationsPage = lazy(() => import("@/pages/organizations/OrganizationsPage"));
const OrganizationDetailPage = lazy(() => import("@/pages/organizations/OrganizationDetailPage"));
const OrganizationCreatePage = lazy(() => import("@/pages/organizations/OrganizationCreatePage"));
const OrgDomainsPage = lazy(() => import("@/pages/org-domains/OrgDomainsPage"));
const UsersPage = lazy(() => import("@/pages/users/UsersPage"));
const UserCreatePage = lazy(() => import("@/pages/users/UserCreatePage"));
const UserImportPage = lazy(() => import("@/pages/users/UserImportPage"));
const UserDetailPage = lazy(() => import("@/pages/users/UserDetailPage"));
const GroupsPage = lazy(() => import("@/pages/groups/GroupsPage"));
const GroupCreatePage = lazy(() => import("@/pages/groups/GroupCreatePage"));
const GroupDetailPage = lazy(() => import("@/pages/groups/GroupDetailPage"));
const GroupTreePage = lazy(() => import("@/pages/groups/GroupTreePage"));
const InvitationsPage = lazy(() => import("@/pages/invitations/InvitationsPage"));
const InvitationCreatePage = lazy(() => import("@/pages/invitations/InvitationCreatePage"));
const InvitationDetailPage = lazy(() => import("@/pages/invitations/InvitationDetailPage"));
const MfaSetupPage = lazy(() => import("@/pages/mfa/MfaSetupPage"));

/** Identity management routes — organizations, users, groups, invitations, MFA */
export const identityRoutes: RouteObject[] = [
  // ── Organizations ──
  {
    path: "organizations",
    element: withSuspense(<OrganizationsPage />),
    loader: organizationsListLoader,
  },
  { path: "organizations/new", element: withSuspense(<OrganizationCreatePage />) },
  {
    path: "organizations/:organizationName",
    element: withSuspense(<OrganizationDetailPage />),
    loader: organizationDetailLoader,
  },
  {
    path: "organizations/:organizationName/domains",
    element: withSuspense(<OrgDomainsPage />),
  },

  // ── Users ──
  { path: "users", element: withSuspense(<UsersPage />) },
  { path: "users/new", element: withSuspense(<UserCreatePage />) },
  { path: "users/import", element: withSuspense(<UserImportPage />) },
  { path: "users/:organizationName/:userName", element: withSuspense(<UserDetailPage />) },

  // ── Groups ──
  { path: "groups", element: withSuspense(<GroupsPage />), loader: groupsListLoader },
  { path: "groups/new", element: withSuspense(<GroupCreatePage />) },
  { path: "groups/tree", element: withSuspense(<GroupTreePage />) },
  {
    // 详情页不挂 loader：路由参数是 groupName（名称），而 GET /groups/{id}
    // 需要的是组 ID；把名称当 ID 传会让 loader 必然 404 并炸出 errorElement，
    // 详情页无法打开。页面组件自行经列表 owner+name 匹配后再按 ID 取详情。
    path: "groups/:organizationName/:groupName",
    element: withSuspense(<GroupDetailPage />),
  },

  // ── Invitations ──
  {
    path: "invitations",
    element: withSuspense(<InvitationsPage />),
    loader: invitationsListLoader,
  },
  { path: "invitations/new", element: withSuspense(<InvitationCreatePage />) },
  {
    // 同 groups 详情：路由参数是 invitationName，loader 会把名称当 ID 查询必
    // 然 404；页面组件自行按 owner+name 匹配列表后按 ID 取详情。
    path: "invitations/:organizationName/:invitationName",
    element: withSuspense(<InvitationDetailPage />),
  },

  // ── MFA ──
  { path: "mfa/setup", element: withSuspense(<MfaSetupPage />) },
];
