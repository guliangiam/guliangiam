import { useEffect, useState, useCallback, useRef, memo, useMemo } from "react";
import Input from "antd/es/input";
import type { InputRef } from "antd/es/input";
import List from "antd/es/list";
import Modal from "antd/es/modal";
import Spin from "antd/es/spin";
import Typography from "antd/es/typography";
import {
  SearchOutlined,
  PlusOutlined,
  HistoryOutlined,
  ThunderboltOutlined,
  DatabaseOutlined,
} from "@ant-design/icons";
import { useLocation, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { searchMenuItems } from "@/features/navigation/menu";
import { useHotkeys } from "@/shared/hooks/useHotkeys";
import { useDebounce } from "@/shared/hooks/useDebounce";
import { usePermissions } from "@/shared/hooks/usePermissions";
import { globalSearch } from "@/shared/api/iam/search";
import type { SearchItem as DataSearchItem, SearchResult } from "@/shared/api/iam/search";
import type { SessionAccount, SessionOrganization } from "@/shared/types/domain";

// ─────────────────────────────────────────────────────────────────────
// CommandPalette — A3 全局搜索组件（Phase B 完整实现）
//
// 设计：Command-⌘K 的灵感来源（VS Code Linear Vercel）
//   · 全局常驻入口（Header 中间位置）
//   · 4 个搜索分组：Recent / Create / Current Page / Navigate
//   · 快捷键：⌘K (Mac) / Ctrl+K (Win) 打开, / 在任何页面输入立即打开
//   · 键盘：↑↓ 选择, Enter 确认, Esc 关闭（任何焦点状态下）
//   · 数据：Recent 用 localStorage, Navigate 来自 menu.tsx, Create 固定入口
//
// 架构：纯前端实现，零后端依赖
//   · Recent 是 localStorage 记录最近 8 次非 Modal 导航
//   · Current Page 根据当前路由前缀匹配一组快速操作
//   · Navigate 复用 searchMenuItems（权限感知）
//   · Create 固定 17 个入口
// ─────────────────────────────────────────────────────────────────────

type Section = "recent" | "current" | "create" | "navigate" | "data";

type SearchItem = {
  key: string;
  label: string;
  groupLabel: string;
  icon?: React.ReactNode;
  section: Section;
  /** 分组标题渲染键；缺省用 section。数据结果按实体分组时逐组不同。 */
  sectionKey?: string;
  /** 自定义分组标题；缺省走 sectionLabel(section)。 */
  sectionTitle?: string;
};

const RECENT_KEY = "giam:command-palette:recent";
const MAX_RECENT = 8;

// 数据结果 → 详情路由映射（与列表页 Link 的构造口径一致：
// roles/permissions/groups/invitations 的 :organizationName 段实际传 organizationId，
// users 传 organizationSlug；无详情路由的实体（sessions/records）跳列表页）。
const DATA_ROUTES: Record<string, (item: DataSearchItem) => string> = {
  organizations: (i) => `/organizations/${encodeURIComponent(i.name ?? i.id)}`,
  users: (i) =>
    i.organizationSlug && i.username
      ? `/users/${encodeURIComponent(i.organizationSlug)}/${encodeURIComponent(i.username)}`
      : "/users",
  applications: (i) => `/applications/${encodeURIComponent(i.id)}`,
  roles: (i) =>
    i.organizationId && i.name
      ? `/roles/${encodeURIComponent(i.organizationId)}/${encodeURIComponent(i.name)}`
      : "/roles",
  permissions: (i) =>
    i.organizationId && i.name
      ? `/permissions/${encodeURIComponent(i.organizationId)}/${encodeURIComponent(i.name)}`
      : "/permissions",
  groups: (i) =>
    i.organizationId && i.name
      ? `/groups/${encodeURIComponent(i.organizationId)}/${encodeURIComponent(i.name)}`
      : "/groups",
  certs: (i) => `/certs/${encodeURIComponent(i.id)}`,
  keys: (i) => `/keys/${encodeURIComponent(i.id)}`,
  tokens: (i) => `/tokens/${encodeURIComponent(i.id)}`,
  sessions: () => "/sessions",
  tickets: (i) => `/tickets/${encodeURIComponent(i.id)}`,
  invitations: (i) =>
    i.organizationId && i.name
      ? `/invitations/${encodeURIComponent(i.organizationId)}/${encodeURIComponent(i.name)}`
      : "/invitations",
};

// Quick-create items — 固定入口，不依赖后端 API
const CREATE_ITEMS: Array<{ key: string; labelKey: string }> = [
  { key: "/users/create", labelKey: "menuSearch.create.user" },
  { key: "/organizations/create", labelKey: "menuSearch.create.org" },
  { key: "/roles/create", labelKey: "menuSearch.create.role" },
  { key: "/permissions/create", labelKey: "menuSearch.create.permission" },
  { key: "/groups/create", labelKey: "menuSearch.create.group" },
  { key: "/invitations/create", labelKey: "menuSearch.create.invitation" },
  { key: "/gateway/servers/create", labelKey: "menuSearch.create.server" },
  { key: "/gateway/sites/create", labelKey: "menuSearch.create.site" },
  { key: "/certs/create", labelKey: "menuSearch.create.cert" },
  { key: "/resources/create", labelKey: "menuSearch.create.resource" },
  { key: "/models/create", labelKey: "menuSearch.create.model" },
  { key: "/adapters/create", labelKey: "menuSearch.create.adapter" },
  { key: "/enforcers/create", labelKey: "menuSearch.create.enforcer" },
  { key: "/syncers/create", labelKey: "menuSearch.create.syncer" },
  { key: "/webhooks/create", labelKey: "menuSearch.create.webhook" },
  { key: "/payments/create", labelKey: "menuSearch.create.payment" },
  { key: "/products/create", labelKey: "menuSearch.create.product" },
];

// Current Page quick actions — 根据当前路由前缀匹配
// 返回 null 表示当前页无特殊快捷操作
const PAGE_ACTIONS: Array<{ prefix: string; actions: Array<{ key: string; labelKey: string }> }> = [
  {
    prefix: "/users",
    actions: [{ key: "/users/create", labelKey: "menuSearch.create.user" }],
  },
  {
    prefix: "/organizations",
    actions: [{ key: "/organizations/create", labelKey: "menuSearch.create.org" }],
  },
  {
    prefix: "/roles",
    actions: [{ key: "/roles/create", labelKey: "menuSearch.create.role" }],
  },
  {
    prefix: "/permissions",
    actions: [{ key: "/permissions/create", labelKey: "menuSearch.create.permission" }],
  },
  {
    prefix: "/groups",
    actions: [{ key: "/groups/create", labelKey: "menuSearch.create.group" }],
  },
  {
    prefix: "/gateway/servers",
    actions: [{ key: "/gateway/servers/create", labelKey: "menuSearch.create.server" }],
  },
  {
    prefix: "/gateway/sites",
    actions: [{ key: "/gateway/sites/create", labelKey: "menuSearch.create.site" }],
  },
  {
    prefix: "/certs",
    actions: [{ key: "/certs/create", labelKey: "menuSearch.create.cert" }],
  },
  {
    prefix: "/webhooks",
    actions: [{ key: "/webhooks/create", labelKey: "menuSearch.create.webhook" }],
  },
  {
    prefix: "/payments",
    actions: [{ key: "/payments/create", labelKey: "menuSearch.create.payment" }],
  },
];

type MenuSearchProps = {
  account: SessionAccount | null;
  organization: SessionOrganization | null;
};

export const MenuSearch = memo(function MenuSearch({ account, organization }: MenuSearchProps) {
  const { t } = useTranslation("common");
  const navigate = useNavigate();
  const location = useLocation();
  // 批次9a：菜单搜索与侧栏同源，叠加服务端授权菜单集合。
  const { permissions, isLoading: permissionsLoading } = usePermissions();
  const allowedMenus = permissionsLoading ? null : permissions.menus;
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const inputRef = useRef<InputRef | null>(null);
  const [activeIndex, setActiveIndex] = useState(-1);

  const debouncedQuery = useDebounce(query, 150);
  const debouncedApiQuery = useDebounce(query, 300);

  // ── 数据结果：输入防抖 300ms 调 GET /api/v1/search 聚合搜索 ──
  const [dataResult, setDataResult] = useState<SearchResult | null>(null);
  const [dataLoading, setDataLoading] = useState(false);
  const searchSeq = useRef(0);

  useEffect(() => {
    const keyword = debouncedApiQuery.trim();
    if (!open || keyword === "") {
      searchSeq.current += 1;
      setDataResult(null);
      setDataLoading(false);
      return;
    }
    const seq = ++searchSeq.current;
    setDataLoading(true);
    void (async () => {
      try {
        const result = await globalSearch(keyword);
        if (searchSeq.current === seq) setDataResult(result);
      } catch {
        // 403/网络失败等一律静默降级：不显示数据分组，不打断菜单搜索。
        if (searchSeq.current === seq) setDataResult(null);
      } finally {
        if (searchSeq.current === seq) setDataLoading(false);
      }
    })();
  }, [open, debouncedApiQuery]);

  // ── Recent: localStorage 记录最近访问 ──

  const [recentPaths, setRecentPaths] = useState<string[]>(() => {
    try {
      const raw = localStorage.getItem(RECENT_KEY);
      return raw ? (JSON.parse(raw) as string[]) : [];
    } catch {
      return [];
    }
  });

  // 每次导航后更新 recent（但不在 palette 打开时记录，避免 ⌘K 自身污染）
  useEffect(() => {
    if (open) return;
    const path = location.pathname;
    // 排除登录/回调等跳转页
    if (path === "/login" || path === "/" || path.startsWith("/auth/")) return;
    // oxlint-disable-next-line react/set-state-in-effect — 我们需要在每次 location 变化时同步 recent，这是 effect 的合法用途
    setRecentPaths((prev) => {
      const next = [path, ...prev.filter((p) => p !== path)].slice(0, MAX_RECENT);
      try {
        localStorage.setItem(RECENT_KEY, JSON.stringify(next));
      } catch {
        /* storage full — best-effort */
      }
      return next;
    });
  }, [location.pathname, open]);

  // ── 打开/关闭逻辑 ──

  const handleOpen = useCallback(() => {
    setQuery("");
    setActiveIndex(-1);
    setOpen(true);
    setTimeout(() => inputRef.current?.focus(), 50);
  }, []);

  const handleClose = useCallback(() => {
    setOpen(false);
    setQuery("");
    setActiveIndex(-1);
  }, []);

  // ⌘K / Ctrl+K 打开
  useHotkeys("mod+k", (e) => {
    e.preventDefault();
    if (open) handleClose();
    else handleOpen();
  });

  // Esc 在任何焦点下都关闭（Modal 自己也会处理）
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        handleClose();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, handleClose]);

  // 在页面按下 "/" 且无输入框聚焦时打开并聚焦搜索
  useEffect(() => {
    const onSlash = (e: KeyboardEvent) => {
      if (open) return;
      if (e.key !== "/") return;
      const target = e.target as HTMLElement | null;
      if (!target) return;
      const tag = target.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA" || target.isContentEditable) return;
      e.preventDefault();
      handleOpen();
    };
    window.addEventListener("keydown", onSlash);
    return () => window.removeEventListener("keydown", onSlash);
  }, [open, handleOpen]);

  // ── 构建搜索结果 ──

  // Recent section — 空 query 时显示
  const recentItems = useMemo<SearchItem[]>(() => {
    if (debouncedQuery !== "" || recentPaths.length === 0) return [];
    return recentPaths
      .map((p) => {
        // 通过 searchMenuItems 反查 label + icon
        const match = searchMenuItems(account, organization, "", t).find((m) => m.key === p);
        return {
          key: p,
          label: match?.label ?? p,
          groupLabel: t("menuSearch.section.recent"),
          icon: <HistoryOutlined />,
          section: "recent" as Section,
        };
      })
      .filter((item) => item.label !== item.key); // 只保留能反查到 label 的
  }, [debouncedQuery, recentPaths, account, organization, t]);

  // Create section — 搜索过滤
  const createItems = useMemo<SearchItem[]>(() => {
    const q = debouncedQuery.toLowerCase();
    return CREATE_ITEMS.map((c) => ({
      key: c.key,
      label: t(c.labelKey),
      groupLabel: t("menuSearch.section.create"),
      icon: <PlusOutlined />,
      section: "create" as Section,
    })).filter(
      (item) =>
        q === "" || item.label.toLowerCase().includes(q) || item.key.toLowerCase().includes(q),
    );
  }, [debouncedQuery, t]);

  // Current Page section — 根据当前路由匹配快捷操作
  const currentPageItems = useMemo<SearchItem[]>(() => {
    if (debouncedQuery !== "") return []; // 有搜索词时隐藏
    if (open === false) return [];
    const match = PAGE_ACTIONS.find(
      ({ prefix }) =>
        location.pathname.startsWith(prefix) && !location.pathname.includes("/create"),
    );
    if (!match) return [];
    return match.actions.map((a) => ({
      key: a.key,
      label: t(a.labelKey),
      groupLabel: t("menuSearch.section.current"),
      icon: <ThunderboltOutlined />,
      section: "current" as Section,
    }));
  }, [open, debouncedQuery, location.pathname, t]);

  // Navigate section — 搜索匹配
  const navigateItems = useMemo<SearchItem[]>(() => {
    if (open === false) return [];
    return searchMenuItems(account, organization, debouncedQuery, t, allowedMenus).map((item) => ({
      key: item.key,
      label: item.label,
      groupLabel: item.groupLabel,
      icon: item.icon,
      section: "navigate" as Section,
    }));
  }, [open, account, organization, debouncedQuery, t, allowedMenus]);

  // Data section — 后端聚合命中，按实体分组渲染（标题/副标题，点击跳详情）
  const dataItems = useMemo<SearchItem[]>(() => {
    if (!dataResult || debouncedQuery === "") return [];
    const items: SearchItem[] = [];
    for (const group of dataResult.groups) {
      const routeFor = DATA_ROUTES[group.entity];
      if (!routeFor) continue;
      for (const entry of group.items) {
        items.push({
          key: routeFor(entry),
          label: entry.title,
          groupLabel: entry.subtitle,
          icon: <DatabaseOutlined />,
          section: "data",
          sectionKey: `data:${group.entity}`,
          // 实体组标题复用各语言已有的 nav.* 实体名键。
          sectionTitle: t(`nav.${group.entity}`),
        });
      }
    }
    return items;
  }, [dataResult, debouncedQuery, t]);

  // ── 合并显示（按 section 排序） ──
  const mergedItems = useMemo<SearchItem[]>(() => {
    if (debouncedQuery === "") {
      // 空 query：Recent → Current → Create（前 8 个）
      return [...recentItems, ...currentPageItems, ...createItems.slice(0, 8)];
    }
    // 有 query：Create + Navigate + 数据结果（搜索优先）
    return [...createItems, ...navigateItems, ...dataItems];
  }, [debouncedQuery, recentItems, currentPageItems, createItems, navigateItems, dataItems]);

  // ── 键盘导航 ──

  const handleSelect = useCallback(
    (key: string) => {
      handleClose();
      navigate(key);
    },
    [navigate, handleClose],
  );

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === "Enter" && mergedItems.length > 0) {
        const idx = activeIndex >= 0 ? activeIndex : 0;
        handleSelect(mergedItems[idx].key);
      } else if (e.key === "ArrowDown") {
        e.preventDefault();
        setActiveIndex((prev) => (prev < mergedItems.length - 1 ? prev + 1 : 0));
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        setActiveIndex((prev) => (prev > 0 ? prev - 1 : mergedItems.length - 1));
      } else if (e.key === "Escape") {
        handleClose();
      }
    },
    [mergedItems, activeIndex, handleSelect, handleClose],
  );

  const sectionLabel = (s: Section): string => {
    switch (s) {
      case "recent":
        return t("menuSearch.section.recent");
      case "current":
        return t("menuSearch.section.current");
      case "create":
        return t("menuSearch.section.create");
      case "navigate":
        return t("menuSearch.section.navigate");
      case "data":
        return t("menuSearch.dataResults");
    }
  };

  return (
    <>
      {/* 常驻入口按钮 — 放在 Header 中间 */}
      <button
        type="button"
        className="gl-command-palette-trigger"
        onClick={handleOpen}
        aria-label={`${t("menuSearch.placeholder")} ⌘K`}
      >
        <SearchOutlined className="gl-command-palette-trigger__icon" />
        <span className="gl-command-palette-trigger__text">{t("menuSearch.placeholder")}</span>
        {/* 快捷键提示参与可见文本，因此必须原样包含在可访问名称里
            （label-content-name-mismatch：可见文本必须是名称的子串）。 */}
        <kbd className="gl-command-palette-trigger__kbd">⌘K</kbd>
      </button>

      {/* Modal — Command Palette 质感 */}
      <Modal
        open={open}
        onCancel={handleClose}
        footer={null}
        closable={false}
        width={640}
        centered
        styles={{
          body: { padding: 0 },
          mask: { background: "rgba(10, 20, 22, 0.45)", backdropFilter: "blur(2px)" },
        }}
        className="gl-command-palette"
        aria-label={t("menuSearch.placeholder")}
        destroyOnHidden
      >
        {/* 搜索输入 */}
        <div className="gl-command-palette__input-wrap">
          <Input
            ref={inputRef}
            prefix={<SearchOutlined className="gl-command-palette__search-icon" />}
            placeholder={t("menuSearch.placeholder")}
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setActiveIndex(-1);
            }}
            onKeyDown={handleKeyDown}
            size="large"
            variant="borderless"
            allowClear
          />
        </div>

        {/* 结果列表 */}
        <div role="listbox" aria-label={t("menuSearch.placeholder")}>
          {mergedItems.length === 0 && !dataLoading ? (
            <div className="gl-command-palette__empty">
              <SearchOutlined style={{ fontSize: 32, opacity: 0.4, marginBottom: 8 }} />
              <Typography.Text type="secondary">{t("menuSearch.noResults")}</Typography.Text>
            </div>
          ) : (
            <>
              {mergedItems.length > 0 && (
                <List
                  dataSource={mergedItems}
                  style={{ maxHeight: 420, overflow: "auto" }}
                  renderItem={(item, idx) => {
                    const prev = idx === 0 ? null : mergedItems[idx - 1];
                    const isNewGroup =
                      !prev ||
                      (prev.sectionKey ?? prev.section) !== (item.sectionKey ?? item.section);
                    const isDataStart =
                      item.section === "data" && (!prev || prev.section !== "data");
                    const isActive = idx === activeIndex;

                    return (
                      <div key={item.key + item.section}>
                        {isDataStart && (
                          <div className="gl-command-palette__section-label">
                            {t("menuSearch.dataResults")}
                          </div>
                        )}
                        {isNewGroup && (
                          <div className="gl-command-palette__section-label">
                            {item.sectionTitle ?? sectionLabel(item.section)}
                          </div>
                        )}
                        <div
                          className={`gl-command-palette__item${isActive ? " is-active" : ""}`}
                          onClick={() => handleSelect(item.key)}
                          onKeyDown={(e) => {
                            if (e.key === "Enter" || e.key === " ") {
                              e.preventDefault();
                              handleSelect(item.key);
                            }
                          }}
                          role="option"
                          aria-selected={isActive}
                          aria-label={`${item.label} - ${item.groupLabel}`}
                          tabIndex={isActive ? 0 : -1}
                        >
                          <span className="gl-command-palette__item-icon" aria-hidden="true">
                            {item.icon}
                          </span>
                          <div className="gl-command-palette__item-body">
                            <Typography.Text strong className="gl-command-palette__item-label">
                              {item.label}
                            </Typography.Text>
                            <Typography.Text
                              type="secondary"
                              className="gl-command-palette__item-sub"
                            >
                              {item.groupLabel}
                            </Typography.Text>
                          </div>
                        </div>
                      </div>
                    );
                  }}
                />
              )}
              {dataLoading && (
                <div className="gl-command-palette__section-label">
                  <Spin size="small" /> {t("menuSearch.dataLoading")}
                </div>
              )}
              {!dataLoading &&
                debouncedApiQuery.trim() !== "" &&
                dataItems.length === 0 &&
                mergedItems.length > 0 && (
                  <div className="gl-command-palette__section-label">
                    {t("menuSearch.dataEmpty")}
                  </div>
                )}
            </>
          )}
        </div>

        {/* 快捷键提示 footer */}
        <div className="gl-command-palette__footer">
          <span>
            <kbd>↑↓</kbd> {t("searchHintSelect")}
          </span>
          <span>
            <kbd>Enter</kbd> {t("searchHintNavigate")}
          </span>
          <span>
            <kbd>Esc</kbd> {t("searchHintClose")}
          </span>
          <span>
            <kbd>/</kbd> {t("searchHintOpen")}
          </span>
        </div>
      </Modal>
    </>
  );
});
