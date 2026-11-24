import type { TFunction } from "i18next";
import { menuGroups } from "./menu";

/**
 * 组织「菜单权限」树的数据源。
 *
 * 直接由真实 `menuGroups` 派生，避免设置页维护一份与导航栏逐渐漂移的
 * 硬编码快照。节点 key 保持为路由路径，历史 `navPreferences` 仍然兼容。
 */
export type NavTreeDataNode = {
  title: string;
  key: string;
  children?: NavTreeDataNode[];
};

export const NAV_TREE_ALL_KEY = "all";

export function buildNavTree(t: TFunction): NavTreeDataNode[] {
  const groups: NavTreeDataNode[] = menuGroups.map((group) => ({
    title: t(group.label),
    key: `group:${group.key}`,
    children: group.children.map((child) => ({
      title: t(child.label),
      key: child.key,
    })),
  }));

  return [
    {
      title: t("nav.all", "All"),
      key: NAV_TREE_ALL_KEY,
      children: groups,
    },
  ];
}
