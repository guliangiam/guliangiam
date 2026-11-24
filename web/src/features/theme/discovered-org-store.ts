import { useEffect } from "react";
import { create } from "zustand";

type DiscoveredOrgState = {
  orgSlug: string | null;
  setOrgSlug: (slug: string | null) => void;
};

/**
 * 登录前页面（登录/注册）发现的组织 slug。OrganizationThemeProvider 挂在
 * Router 之外且无会话组织可读，由页面控制器把当前组织上下文发布到这里，
 * 主题 Provider 据此走匿名公开投影取组织主题（token + 自定义 CSS）。
 */
export const useDiscoveredOrgStore = create<DiscoveredOrgState>((set) => ({
  orgSlug: null,
  setOrgSlug: (orgSlug) => set({ orgSlug }),
}));

/** 发布当前页面的组织 slug；卸载时清空，避免残留组织主题影响其他匿名页面。 */
export function usePublishDiscoveredOrg(slug: string | null | undefined) {
  const setOrgSlug = useDiscoveredOrgStore((s) => s.setOrgSlug);
  useEffect(() => {
    setOrgSlug(slug?.trim() || null);
    return () => setOrgSlug(null);
  }, [slug, setOrgSlug]);
}
