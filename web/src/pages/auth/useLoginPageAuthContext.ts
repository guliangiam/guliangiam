import { Form, type FormInstance } from "antd";
import { type LoginMethodKey } from "@/shared/lib/auth-signin-items";
import { usePublishDiscoveredOrg } from "@/features/theme/discovered-org-store";
import { useLoginPageAuthQueries } from "./useLoginPageAuthQueries";
import { useLoginPageRouteContext } from "./useLoginPageRouteContext";

type LoginFormLikeValues = {
  organization: string;
  application: string;
  username: string;
};

type LocationStateLike = {
  from?: {
    pathname?: string;
    search?: string;
  };
} | null;

type SessionAccountLike = {
  owner?: string | null;
} | null;

type UseLoginPageAuthContextValues = {
  form: FormInstance<LoginFormLikeValues>;
  loginMethod: LoginMethodKey;
  pathname: string;
  search: string;
  state: LocationStateLike;
  organizationName?: string;
  owner?: string;
  casApplicationName?: string;
  applicationName?: string;
  sessionAccount: SessionAccountLike;
};

export function useLoginPageAuthContext(values: UseLoginPageAuthContextValues) {
  const {
    applicationName,
    casApplicationName,
    form,
    loginMethod,
    organizationName,
    owner,
    pathname,
    search,
    sessionAccount,
    state,
  } = values;

  const routeContext = useLoginPageRouteContext({
    loginMethod,
    pathname,
    search,
    state,
    organizationName,
    owner,
    casApplicationName,
    applicationName,
  });
  const watchedOrganization =
    Form.useWatch("organization", form) || routeContext.loginDefaults.organization;
  const watchedApplication =
    Form.useWatch("application", form) || routeContext.loginDefaults.application;
  const watchedUsername = Form.useWatch("username", form) || "";
  const queryContext = useLoginPageAuthQueries({
    routeContext,
    pathname,
    search,
    loginMethod,
    watchedOrganization,
    watchedApplication,
    sessionAccount,
  });

  // 组织主题匿名跟随：优先解析出的应用所属组织，否则用页面呈现的组织上下文
  // （组织选择器/路由默认值，也是 resolve 查询的 ownerSlug）。
  usePublishDiscoveredOrg(queryContext.resolvedApplication?.organization || watchedOrganization);

  return {
    ...queryContext,
    ...routeContext,
    watchedApplication,
    watchedOrganization,
    watchedUsername,
  };
}

export type LoginPageAuthContextState = ReturnType<typeof useLoginPageAuthContext>;
