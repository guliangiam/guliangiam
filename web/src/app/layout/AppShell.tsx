import App from "antd/es/app";
import Avatar from "antd/es/avatar";
import Badge from "antd/es/badge";
import Breadcrumb from "antd/es/breadcrumb";
import Button from "antd/es/button";
import Drawer from "antd/es/drawer";
import Dropdown from "antd/es/dropdown";
import Layout from "antd/es/layout";
import Menu from "antd/es/menu";
import Select from "antd/es/select";
import Skeleton from "antd/es/skeleton";
import Tooltip from "antd/es/tooltip";
import Alert from "antd/es/alert";
import Result from "antd/es/result";
import {
  BellOutlined,
  GlobalOutlined,
  LaptopOutlined,
  LogoutOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  MenuOutlined,
  MoonOutlined,
  SunOutlined,
  SwapOutlined,
  StopOutlined,
  UserOutlined,
  UserSwitchOutlined,
} from "@ant-design/icons";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Navigate, Outlet, useLocation, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useCallback, useEffect, useMemo, useState } from "react";
import { logoutSession, ssoLogout } from "@/shared/api/iam/auth";
import { exitImpersonationSession } from "@/shared/api/iam/user";
import { getOrganizations, getOrganization } from "@/shared/api/iam/organization";
import { useSessionStore } from "@/features/session/session-store";
import { useThemeStore } from "@/features/theme/theme-store";
import { PasswordChangeBanner } from "./PasswordChangeBanner";
import { useLayoutStore } from "@/features/layout/layout-store";
import { useSessionBootstrap } from "@/features/session/useSessionBootstrap";
import { queryKeys } from "@/shared/api/query-keys";
import {
  buildBreadcrumbs,
  buildMenuItems,
  findSelectedKey,
  isRouteKeyVisible,
  menuGroups,
} from "@/features/navigation/menu";
import { MenuSearch } from "@/features/navigation/MenuSearch";
import { useUnreadNotificationCount } from "@/features/notifications/useUnreadNotificationCount";
import { useMediaQuery } from "@/shared/hooks/useMediaQuery";
import { usePermissions } from "@/shared/hooks/usePermissions";
import { useAccessibleTableRegions } from "@/shared/hooks/useAccessibleTableRegions";
import { SessionTimeoutDialog } from "@/shared/ui/SessionTimeoutDialog";
import { FieldHelpInjector } from "@/shared/ui/FieldHelpInjector";
import { FeedbackLauncher } from "@/features/feedback/FeedbackLauncher";
import { loadLanguageResources } from "@/shared/i18n";
import { setAppMessage } from "@/shared/lib/app-message";
import { localizedError } from "@/shared/lib/server-error";

const { Sider, Header, Content } = Layout;

const DRAWER_BREAKPOINT = 1024;
const SIDEBAR_WIDTH = 240;
const SIDEBAR_COLLAPSED_WIDTH = 64;

export function AppShell() {
  const { message } = App.useApp();
  useEffect(() => {
    setAppMessage(message);
  }, [message]);
  const navigate = useNavigate();
  const location = useLocation();
  const { t, i18n } = useTranslation("common");
  const { t: tAuth } = useTranslation("auth");
  const queryClient = useQueryClient();
  const setSession = useSessionStore((state) => state.setSession);
  const needUpdatePassword = useSessionStore((state) => state.needUpdatePassword);
  const impersonatedUser = useSessionStore((state) => state.impersonatedUser);
  const setImpersonatedUser = useSessionStore((state) => state.setImpersonatedUser);
  const themeMode = useThemeStore((s) => s.themeMode);
  const resolvedTheme = useThemeStore((s) => s.resolvedTheme);
  const setThemeMode = useThemeStore((s) => s.setThemeMode);
  const layoutMode = useLayoutStore((s) => s.layoutMode);
  const toggleLayout = useLayoutStore((s) => s.toggleLayout);
  const sidebarCollapsed = useLayoutStore((s) => s.sidebarCollapsed);
  const toggleSidebar = useLayoutStore((s) => s.toggleSidebar);
  const { isLoading, account, organization, isBackendUnavailable, refetch } = useSessionBootstrap();
  const unreadNotifications = useUnreadNotificationCount(Boolean(account));

  // 表格滚动区键盘可达（a11y）：覆盖 React 内联 Table 与 ResourceListCard。
  useAccessibleTableRegions();

  // Organizations list for header switcher (admin can see all orgs they manage).
  const orgsQuery = useQuery({
    queryKey: ["organizations", "list"],
    queryFn: getOrganizations,
    enabled: account !== null,
    staleTime: 60_000,
  });

  // ── Organization switch: Dropdown on brand removed, Select in header is the single switch UI.
  // Switching = fetch target org detail → replace session → invalidate queries → refresh.
  const switchOrgMutation = useMutation({
    mutationFn: async (orgId: string) => getOrganization(orgId),
    onSuccess: (nextOrg) => {
      const { account: sessionAccount, organization: previousOrg } = useSessionStore.getState();
      const orgName = nextOrg.displayName || nextOrg.slug;
      // OrganizationDetail → SessionOrganization：会话里的组织只保留菜单/品牌所需字段。
      // 菜单项沿用当前组织的值，待 session 上下文重新拉取后由服务端覆盖。
      setSession({
        account: sessionAccount,
        organization: {
          id: nextOrg.id,
          name: orgName,
          displayName: nextOrg.displayName,
          logoUrl: nextOrg.logoUrl,
          faviconUrl: nextOrg.faviconUrl,
          navItems: previousOrg?.navItems ?? [],
          userNavItems: previousOrg?.userNavItems ?? [],
          navGroupOrder: previousOrg?.navGroupOrder ?? [],
        },
      });
      // Invalidate everything that depends on organization context so pages refetch with new org
      queryClient.invalidateQueries({ queryKey: queryKeys.session.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.organizations.lists() });
      queryClient.invalidateQueries({ queryKey: queryKeys.users.all });
      message.success(t("organizationSwitched", { org: orgName }));
    },
    onError: (err) => {
      message.error(localizedError(err, t, "error"));
    },
  });

  const orgSelectOptions = useMemo(() => {
    const orgs = orgsQuery.data ?? [];
    return orgs.map((org) => ({
      value: org.id,
      label:
        org.id === organization?.id ? (
          <span style={{ fontWeight: 600 }}>
            {org.displayName || org.slug}
            <span style={{ color: "var(--gl-color-success)", marginLeft: 8, fontSize: 12 }}>●</span>
          </span>
        ) : (
          org.displayName || org.slug
        ),
    }));
  }, [orgsQuery.data, organization?.id]);

  const isDrawerMode = useMediaQuery(`(max-width: ${DRAWER_BREAKPOINT}px)`);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [openKeys, setOpenKeys] = useState<string[]>([]);

  // The sidebar is force-collapsed in drawer/top layout, and the group that owns
  // the current route is always expanded. Both are derived during render instead
  // of being pushed into state by effects.
  const sidebarSuppressed = sidebarCollapsed || layoutMode === "top";
  const routeGroupKey = useMemo(
    () =>
      menuGroups.find((g) =>
        g.routeKeys.some(
          (rk) => location.pathname === rk || location.pathname.startsWith(`${rk}/`),
        ),
      )?.key,
    [location.pathname],
  );
  const effectiveOpenKeys = useMemo(() => {
    if (sidebarSuppressed) return [];
    if (routeGroupKey && !openKeys.includes(routeGroupKey)) {
      return [routeGroupKey, ...openKeys];
    }
    return openKeys;
  }, [sidebarSuppressed, routeGroupKey, openKeys]);

  // Close the drawer when the viewport grows past the drawer breakpoint so that
  // reopening it later doesn't surprise the user, and on every in-app navigation
  // (browser back/forward, direct URL edit, programmatic navigate) — the mobile
  // convention is "pick a destination from the menu → get out of the way".
  // Reset during render instead of in an effect: React re-renders before
  // committing, so the stale "open" never reaches a paint.
  const drawerResetKey = `${isDrawerMode}|${location.pathname}`;
  const [lastDrawerResetKey, setLastDrawerResetKey] = useState(drawerResetKey);
  if (lastDrawerResetKey !== drawerResetKey) {
    setLastDrawerResetKey(drawerResetKey);
    if (drawerOpen) setDrawerOpen(false);
  }

  // Listen for auth:unauthorized events dispatched by http.ts
  // and redirect to login via React Router (preserves SPA state).
  useEffect(() => {
    const handleAuthUnauthorized = (e: Event) => {
      const detail = (e as CustomEvent).detail;
      navigate("/login", { replace: true, state: { from: detail?.from } });
    };
    window.addEventListener("auth:unauthorized", handleAuthUnauthorized);
    return () => window.removeEventListener("auth:unauthorized", handleAuthUnauthorized);
  }, [navigate]);

  // B 侧首屏直达：登录响应/session 已带 needUpdatePassword 时立即落到 /account，
  // 不依赖任何业务接口先返回 403。服务端的 403 → event → navigate 仍是兜底通道，
  // 两条路径相互独立（去掉其中任一条，另一条都能完成自救）。
  useEffect(() => {
    if (needUpdatePassword !== true) return;
    if (location.pathname === "/account") return;
    navigate("/account", {
      replace: true,
      state: { from: location.pathname, passwordGate: "password_change_required" },
    });
  }, [needUpdatePassword, location.pathname, navigate]);

  // password_change_required: an administrator forced the change (initial or
  // reset password). This is a distinct notice from password_expired; the copy,
  // event, and message severity must not be shared.
  useEffect(() => {
    const handlePasswordChangeRequired = (e: Event) => {
      const detail = (e as CustomEvent).detail;
      if (window.location.pathname === "/account") return;
      message.warning(tAuth("passwordChangeRequiredNotice"));
      navigate("/account", {
        replace: true,
        state: { from: detail?.from, passwordGate: "password_change_required" },
      });
    };
    window.addEventListener("auth:password-change-required", handlePasswordChangeRequired);
    return () =>
      window.removeEventListener("auth:password-change-required", handlePasswordChangeRequired);
  }, [navigate, message, tAuth]);

  // password_expired: the password aged out of the organization max-age policy.
  // Separate copy and severity from password_change_required.
  useEffect(() => {
    const handlePasswordExpired = (e: Event) => {
      const detail = (e as CustomEvent).detail;
      if (window.location.pathname === "/account") return;
      message.error(tAuth("passwordExpiredNotice"));
      navigate("/account", {
        replace: true,
        state: { from: detail?.from, passwordGate: "password_expired" },
      });
    };
    window.addEventListener("auth:password-expired", handlePasswordExpired);
    return () => window.removeEventListener("auth:password-expired", handlePasswordExpired);
  }, [navigate, message, tAuth]);

  // Dynamically update favicon and page title to match current organization.
  useEffect(() => {
    if (organization?.faviconUrl) {
      const link =
        document.querySelector<HTMLLinkElement>('link[rel="icon"]') ||
        document.createElement("link");
      link.rel = "icon";
      link.href = organization.faviconUrl;
      if (!link.isConnected) document.head.appendChild(link);
    }
    if (organization?.displayName) {
      document.title = organization.displayName;
    }
  }, [organization?.faviconUrl, organization?.displayName]);

  const logoutMutation = useMutation({
    mutationFn: async () => {
      await logoutSession();
      // Perform SSO front-channel logout: open each app's logout URL in a hidden iframe.
      try {
        const result = await ssoLogout();
        if (result.logoutUrls && result.logoutUrls.length > 0) {
          await Promise.all(
            result.logoutUrls.map(
              (url) =>
                new Promise<void>((resolve) => {
                  const iframe = document.createElement("iframe");
                  iframe.style.display = "none";
                  iframe.src = url;
                  iframe.onload = () => {
                    document.body.removeChild(iframe);
                    resolve();
                  };
                  iframe.onerror = () => {
                    document.body.removeChild(iframe);
                    resolve();
                  };
                  // Timeout fallback: remove iframe after 5s.
                  setTimeout(() => {
                    if (iframe.parentNode) {
                      document.body.removeChild(iframe);
                    }
                    resolve();
                  }, 5000);
                  document.body.appendChild(iframe);
                }),
            ),
          );
        }
      } catch {
        // SSO logout is best-effort; don't block the main logout flow.
      }
    },
    onSettled: () => {
      // Always forget the local session state — regardless of whether the
      // server acknowledged the logout (CSRF expiry, network error, ...).
      // The user's intent is "I want to be logged out" and we must honour it
      // client-side even if the request itself failed. Otherwise a stale CSRF
      // token causes logoutMutation.onError → session stays → next bootstrap
      // picks up the still-valid session cookie → "退出后自动重新登录".
      setSession({ account: null, organization: null });
      // 登出必须同时清空冒充态（含内存 imp token），避免下次登录残留注入。
      setImpersonatedUser(null);
      queryClient.clear();
      navigate("/login", { replace: true });
    },
    onError: (error) => {
      message.error(localizedError(error, t, "error"));
    },
  });

  const exitImpersonateMutation = useMutation({
    // auth 面退出：imp 优先下请求身份是被冒充用户，users 面 exit 会因
    // users:write 403；auth 面按会话自身身份撤销 imp token。
    mutationFn: async () => {
      await exitImpersonationSession();
    },
    onSuccess: () => {
      setImpersonatedUser(null);
      // 冒充期间的列表/权限缓存是按被冒充用户可见性取的，退出后全部失效重取。
      void queryClient.invalidateQueries();
      message.success(t("exitImpersonateSuccess"));
      navigate("/users");
    },
    onError: (error) => {
      // Even if the API call fails, clear local impersonation state
      setImpersonatedUser(null);
      void queryClient.invalidateQueries();
      message.error(localizedError(error, t, "error"));
    },
  });

  // Accordion: only one group open at a time
  const handleOpenChange = (keys: string[]) => {
    const latestOpenKey = keys.find((key) => !effectiveOpenKeys.includes(key));
    setOpenKeys(latestOpenKey ? [latestOpenKey] : []);
  };

  const handleMenuClick = useCallback(
    ({ key }: { key: string }) => {
      navigate(key);
      setDrawerOpen(false);
    },
    [navigate],
  );

  // Compute menu items and breadcrumbs even during loading (hooks must not be conditional)
  // 批次9a：侧栏菜单叠加服务端授权过滤（/auth/permissions 派生的 menus）。
  // 权限加载中传 null 不过滤，避免首屏闪烁；加载完成后未授权菜单完全隐藏。
  const { permissions, isLoading: permissionsLoading } = usePermissions();
  const menuItems = buildMenuItems(
    account,
    organization,
    t,
    permissionsLoading ? null : permissions.menus,
  );
  const selectedKey = findSelectedKey(menuItems, location.pathname);
  const breadcrumbs = buildBreadcrumbs(account, organization, location.pathname, t);

  const breadcrumbItems = useMemo(
    () =>
      breadcrumbs.map((crumb, idx) => ({
        key: idx,
        title:
          idx < breadcrumbs.length - 1 ? (
            <a
              href={crumb.path}
              onClick={(e) => {
                e.preventDefault();
                navigate(crumb.path);
              }}
            >
              {crumb.label}
            </a>
          ) : (
            crumb.label
          ),
      })),
    [breadcrumbs, navigate],
  );

  if (isLoading) {
    // 会话引导占位与正式内容同宽（.gl-content__inner），壳层挂载时不产生宽度跳变。
    return (
      <div className="gl-content gl-content__inner">
        <div className="gl-empty-state">
          <Skeleton active paragraph={{ rows: 10 }} />
        </div>
      </div>
    );
  }

  // 后端不可用（5xx/网络中断）且尚无会话：展示可重试的错误态，而不是把用户
  // 静默重定向到登录页（RT-09）。真正的未登录（401/404）仍走下方 Navigate。
  if (isBackendUnavailable && !account) {
    return (
      <div className="gl-content gl-content__inner">
        <Result
          status="500"
          title={t("serviceUnavailableTitle", "服务暂时不可用")}
          subTitle={t(
            "serviceUnavailableDesc",
            "无法连接到服务器，可能是网络中断或服务正在维护。请稍后重试。",
          )}
          extra={
            <Button type="primary" onClick={() => refetch()}>
              {t("retry", "重试")}
            </Button>
          }
        />
      </div>
    );
  }

  if (!account || !organization) {
    return <Navigate to="/login" replace state={{ from: location }} />;
  }

  // 直达守卫：URL 直达/刷新时按组织导航配置裁决可见性，与侧边栏过滤同源。
  const routeVisible = isRouteKeyVisible(account, organization, location.pathname);

  const themeIcon =
    themeMode === "light" ? (
      <SunOutlined />
    ) : themeMode === "dark" ? (
      <MoonOutlined />
    ) : (
      <LaptopOutlined />
    );

  const themeDropdownItems = [
    { key: "light", icon: <SunOutlined />, label: t("lightMode") },
    { key: "dark", icon: <MoonOutlined />, label: t("darkMode") },
    { key: "system", icon: <LaptopOutlined />, label: t("systemMode") },
  ];

  const sidebarBrand = (
    <div className="gl-sidebar__brand">
      {sidebarCollapsed && <div className="gl-sidebar__brand-collapsed">GuliangIAM</div>}
    </div>
  );

  const sidebarContent = (
    <>
      {sidebarBrand}
      <Menu
        mode="inline"
        theme={resolvedTheme === "dark" ? "dark" : "light"}
        selectedKeys={[selectedKey]}
        openKeys={effectiveOpenKeys}
        onOpenChange={handleOpenChange}
        inlineCollapsed={sidebarCollapsed}
        items={menuItems}
        onClick={handleMenuClick}
      />
    </>
  );

  const topNavContent = (
    <div className="gl-top-nav">
      <Menu
        mode="horizontal"
        selectedKeys={[selectedKey]}
        items={menuItems}
        onClick={handleMenuClick}
      />
    </div>
  );

  const headerContent = (
    <Header className="gl-header gl-panel">
      <div className="gl-header__left">
        {isDrawerMode && (
          <Button
            type="text"
            className="gl-header__drawer-btn"
            icon={<MenuOutlined />}
            onClick={() => setDrawerOpen(true)}
          />
        )}
        <div
          className="gl-header__brand"
          title={organization?.displayName || organization?.name || t("companyName")}
        >
          {/* Brand logo: rectangular (not circular), height bounded to the header so it never
              exceeds the top bar, width derived from the image's intrinsic ratio, whole logo
              shown via object-fit: contain. Square single-letter badge when there is no logo. */}
          <div className="gl-header__brand-icon">
            {organization?.logoUrl ? (
              <img
                className="gl-header__brand-logo"
                src={organization.logoUrl}
                alt={organization?.displayName || organization?.name || t("companyName")}
              />
            ) : (
              <Avatar shape="square" alt={organization?.displayName || "Organization"}>
                {organization?.displayName?.slice(0, 1) || organization?.name?.slice(0, 1)}
              </Avatar>
            )}
          </div>
          <div className="gl-header__brand-text">
            <div className="gl-header__brand-eyebrow">
              {organization?.displayName || organization?.name || t("companyName")}
            </div>
          </div>
        </div>
      </div>
      <div className="gl-header__center">
        <MenuSearch account={account} organization={organization} />
      </div>
      <div className="gl-header__meta">
        <Tooltip title={t("nav.notifications")}>
          <Badge count={unreadNotifications.data ?? 0} size="small" overflowCount={99}>
            <Button
              type="text"
              icon={<BellOutlined />}
              aria-label={t("nav.notifications")}
              onClick={() => navigate("/notifications")}
            />
          </Badge>
        </Tooltip>
        <Tooltip
          title={layoutMode === "sidebar" ? t("nav.switchToTopNav") : t("nav.switchToSidebar")}
        >
          <Button type="text" icon={<SwapOutlined />} onClick={toggleLayout} />
        </Tooltip>
        <Dropdown
          menu={{
            items: themeDropdownItems,
            onClick: ({ key }) => setThemeMode(key as "light" | "dark" | "system"),
          }}
        >
          <Tooltip title={t("theme.label")}>
            <Button type="text" icon={themeIcon} />
          </Tooltip>
        </Dropdown>
        <Dropdown
          menu={{
            items: [
              { key: "zh-CN", label: t("lang.zhCN") },
              { key: "zh-TW", label: t("lang.zhTW") },
              { key: "en", label: t("lang.en") },
              { key: "ru", label: t("lang.ru") },
              { key: "ja", label: t("lang.ja") },
              { key: "ko", label: t("lang.ko") },
              { key: "de", label: t("lang.de") },
              { key: "fr", label: t("lang.fr") },
              { key: "es", label: t("lang.es") },
              { key: "pt", label: t("lang.pt") },
              { key: "vi", label: t("lang.vi") },
              { key: "pl", label: t("lang.pl") },
              { key: "tr", label: t("lang.tr") },
              { key: "uk", label: t("lang.uk") },
            ],
            selectedKeys: [i18n.language],
            onClick: async ({ key }) => {
              await loadLanguageResources(key);
              await i18n.changeLanguage(key);
            },
          }}
        >
          <Button type="text" icon={<GlobalOutlined />} />
        </Dropdown>
        <Select
          value={organization?.id}
          style={{ minWidth: 200 }}
          options={orgSelectOptions}
          loading={orgsQuery.isLoading || switchOrgMutation.isPending}
          disabled={orgSelectOptions.length <= 1}
          onChange={(orgId) => switchOrgMutation.mutate(orgId)}
          placeholder={t("selectOrganization")}
          title={organization?.displayName || organization?.name}
          aria-label={t("switchOrganization")}
        />
        <Dropdown
          menu={{
            items: [
              {
                key: "profile",
                icon: <UserOutlined />,
                label: t("profile") || "个人信息",
                onClick: () => navigate("/account"),
              },
              { type: "divider" },
              {
                key: "logout",
                icon: <LogoutOutlined />,
                label: logoutMutation.isPending ? t("loggingOut") : t("logout"),
                disabled: logoutMutation.isPending,
                onClick: () => logoutMutation.mutate(),
              },
            ],
          }}
          trigger={["click"]}
        >
          {/* 用 Button 而不是裸 Space 作触发区：键盘可聚焦、有 role=button 语义，
              昵称菜单（个人信息/退出）才能被键盘和 E2E 稳定访问。 */}
          <Button type="text" className="gl-header__user">
            <Avatar src={account?.avatar} alt={account?.displayName || "User avatar"}>
              {account?.displayName?.slice(0, 1) || account?.name?.slice(0, 1)}
            </Avatar>
            <span className="gl-header__user-name">
              {account?.displayName || account?.name || t("guest")}
            </span>
          </Button>
        </Dropdown>
      </div>
    </Header>
  );

  // 主显示区唯一出口：侧栏/顶栏两种布局共用同一层宽度容器与面包屑，
  // 避免其中一种布局单独漂移出第二套 max-width。
  const mainContent = (
    <Content className="gl-content" id="main-content" tabIndex={-1}>
      <div className="gl-content__inner">
        <PasswordChangeBanner />
        {breadcrumbItems.length > 0 && (
          <nav aria-label={t("breadcrumb")}>
            <Breadcrumb
              style={{ marginBottom: "var(--gl-space-4)" }}
              items={breadcrumbItems.map((item) => ({ title: item.title }))}
            />
          </nav>
        )}
        {routeVisible ? <Outlet /> : <Result status="403" title="403" subTitle={t("forbidden")} />}
      </div>
      <FieldHelpInjector />
      <FeedbackLauncher />
    </Content>
  );

  /* WCAG 2.4.1 Bypass Blocks — skip-to-content link for keyboard users (C-01) */
  const skipLink = (
    <a
      href="#main-content"
      className="gl-skip-link"
      style={{
        position: "absolute",
        left: "-9999px",
        top: 0,
        zIndex: 9999,
        padding: "var(--gl-space-2) var(--gl-space-4)",
        background: "var(--gl-color-primary)",
        color: "var(--gl-color-on-primary)",
        textDecoration: "none",
        borderRadius: "0 0 8px 0",
      }}
      onFocus={(e) => {
        e.currentTarget.style.left = "0";
      }}
      onBlur={(e) => {
        e.currentTarget.style.left = "-9999px";
      }}
    >
      {t("skipToContent")}
    </a>
  );

  if (layoutMode === "top" && !isDrawerMode) {
    return (
      <Layout className="gl-app-shell gl-app-shell--top">
        {skipLink}
        <SessionTimeoutDialog />
        {headerContent}
        {topNavContent}
        {impersonatedUser && (
          <Alert
            type="warning"
            showIcon
            icon={<UserSwitchOutlined />}
            title={t("impersonatingUser", {
              name: impersonatedUser.displayName || impersonatedUser.username,
            })}
            action={
              <Button
                size="small"
                danger
                icon={<StopOutlined />}
                onClick={() => exitImpersonateMutation.mutate()}
                loading={exitImpersonateMutation.isPending}
              >
                {t("exitImpersonate")}
              </Button>
            }
            style={{ borderRadius: 0, textAlign: "center" }}
          />
        )}
        {mainContent}
      </Layout>
    );
  }

  return (
    <Layout className="gl-app-shell">
      {skipLink}
      <SessionTimeoutDialog />
      {headerContent}
      {impersonatedUser && (
        <Alert
          type="warning"
          showIcon
          icon={<UserSwitchOutlined />}
          title={t("impersonatingUser", {
            name: impersonatedUser.displayName || impersonatedUser.username,
          })}
          action={
            <Button
              size="small"
              danger
              icon={<StopOutlined />}
              onClick={() => exitImpersonateMutation.mutate()}
              loading={exitImpersonateMutation.isPending}
            >
              {t("exitImpersonate")}
            </Button>
          }
          style={{ borderRadius: 0, textAlign: "center" }}
        />
      )}

      <Layout className="gl-body" hasSider>
        {!isDrawerMode && (
          <Sider
            width={SIDEBAR_WIDTH}
            collapsedWidth={SIDEBAR_COLLAPSED_WIDTH}
            collapsed={sidebarCollapsed}
            className="gl-sidebar gl-sidebar--inline"
            trigger={null}
            aria-label={t("nav.sidebar")}
          >
            <nav aria-label={t("nav.sidebarMenu")}>{sidebarContent}</nav>
            <div className="gl-sidebar__toggle">
              <Button
                type="text"
                icon={sidebarCollapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
                onClick={() => toggleSidebar()}
                className="gl-sidebar__toggle-btn"
                aria-label={sidebarCollapsed ? t("nav.expandSidebar") : t("nav.collapseSidebar")}
              />
            </div>
          </Sider>
        )}

        {isDrawerMode && (
          <Drawer
            placement="left"
            open={drawerOpen}
            onClose={() => setDrawerOpen(false)}
            styles={{
              root: { width: SIDEBAR_WIDTH },
              wrapper: { background: "transparent" },
              body: { padding: 0, background: "var(--gl-color-bg-elevated)" },
              header: { display: "none" },
            }}
            className="gl-sidebar-drawer"
            aria-label={t("nav.sidebar")}
          >
            <nav className="gl-sidebar" aria-label={t("nav.sidebarMenu")}>
              {sidebarContent}
            </nav>
          </Drawer>
        )}

        {mainContent}
      </Layout>
    </Layout>
  );
}
