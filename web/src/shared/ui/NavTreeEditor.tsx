import { Tree, Button } from "antd";
import type { TreeProps } from "antd";
import { ArrowDownOutlined, ArrowUpOutlined } from "@ant-design/icons";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { buildNavTree, NAV_TREE_ALL_KEY } from "@/features/navigation/menu-tree";
import type { NavTreeDataNode } from "@/features/navigation/menu-tree";

interface NavTreeEditorProps {
  disabled?: boolean;
  checkedKeys?: string[];
  defaultExpandedKeys?: string[];
  onCheck?: (checkedKeys: string[]) => void;
  /** 分组顺序（组 key 数组）；数组顺序即展示顺序，未列出的分组按静态顺序排后。 */
  groupOrder?: string[];
  /** 分组顺序变更回调；上移/下移按钮输出完整的有序组 key 数组。 */
  onGroupOrderChange?: (groupKeys: string[]) => void;
}

const GROUP_KEY_PREFIX = "group:";

function isLeafKey(key: string): boolean {
  return key !== NAV_TREE_ALL_KEY && !key.startsWith(GROUP_KEY_PREFIX);
}

function stripNonLeafKeys(keys: string[]): string[] {
  return keys.filter(isLeafKey);
}

/**
 * 组织菜单权限树。
 *
 * 数据来自真实导航配置（`menuGroups`），不再维护硬编码快照；
 * 节点 key 为路由路径，与既有的 `navPreferences` 存储完全兼容。
 *
 * 排序模型（零 DDL，全部落在 navItems 数组顺序 + navGroupOrder 键）：
 * - 叶子顺序 = `checkedKeys` 数组顺序（拖拽同组叶子后按新顺序回写）；
 * - 分组顺序 = `groupOrder`（每个分组标题行提供上移/下移按钮）。
 */
export function NavTreeEditor({
  disabled = false,
  checkedKeys = [NAV_TREE_ALL_KEY],
  defaultExpandedKeys = [NAV_TREE_ALL_KEY],
  onCheck,
  groupOrder,
  onGroupOrderChange,
}: NavTreeEditorProps) {
  const { t } = useTranslation();

  const staticTree = useMemo(() => buildNavTree(t), [t]);

  const { treeData, groupKeys, leavesByGroup, parentByLeaf } = useMemo(() => {
    const root = staticTree[0];
    const staticGroups = (root.children ?? []) as NavTreeDataNode[];

    // 分组顺序：groupOrder 优先，未列出的保持静态顺序排后。
    let orderedGroups = staticGroups;
    if (groupOrder && groupOrder.length > 0) {
      const rank = new Map(groupOrder.map((key, index) => [key, index]));
      orderedGroups = [...staticGroups].sort(
        (a, b) =>
          (rank.get(a.key.slice(GROUP_KEY_PREFIX.length)) ?? Number.MAX_SAFE_INTEGER) -
          (rank.get(b.key.slice(GROUP_KEY_PREFIX.length)) ?? Number.MAX_SAFE_INTEGER),
      );
    }

    // 叶子顺序：checkedKeys 数组顺序优先，未列出的保持静态顺序排后。
    const leafRank = new Map(stripNonLeafKeys(checkedKeys).map((key, index) => [key, index]));
    const orderedGroupNodes = orderedGroups.map((group) => ({
      ...group,
      children: [...(group.children ?? [])].sort(
        (a, b) =>
          (leafRank.get(a.key) ?? Number.MAX_SAFE_INTEGER) -
          (leafRank.get(b.key) ?? Number.MAX_SAFE_INTEGER),
      ),
    }));

    const groupLeaves: Record<string, string[]> = {};
    const leafParent: Record<string, string> = {};
    for (const group of orderedGroupNodes) {
      groupLeaves[group.key] = (group.children ?? []).map((child) => child.key);
      for (const child of group.children ?? []) {
        leafParent[child.key] = group.key;
      }
    }

    return {
      groupKeys: orderedGroupNodes.map((group) => group.key),
      leavesByGroup: groupLeaves,
      parentByLeaf: leafParent,
      treeData: [{ ...root, children: orderedGroupNodes }] as TreeProps["treeData"],
    };
  }, [staticTree, groupOrder, checkedKeys]);

  // 自动展开含勾选叶子的分组（数据异步加载后重算），否则叶子不可见、勾选态不可核对。
  const autoExpandKeys = useMemo(() => {
    const checked = new Set(checkedKeys);
    // 根节点必须展开，否则分组不可见。
    if (checked.has(NAV_TREE_ALL_KEY)) return [NAV_TREE_ALL_KEY, ...groupKeys];
    return [
      NAV_TREE_ALL_KEY,
      ...groupKeys.filter((groupKey) =>
        (leavesByGroup[groupKey] ?? []).some((leaf) => checked.has(leaf)),
      ),
    ];
  }, [checkedKeys, groupKeys, leavesByGroup]);
  const [expandedKeys, setExpandedKeys] = useState<string[]>(autoExpandKeys);
  // 依赖变化时在渲染期合并自动展开键（React 官方「props 变化调整 state」模式），
  // 避免 effect 内同步 setState 造成的级联渲染。
  // 必须按内容比较：上游 props/t 引用不稳定会让 autoExpandKeys 每轮都是新数组，
  // 引用比较会触发无限渲染；内容一致时不 setState，循环在下一轮自然终止。
  const [prevAutoExpandKeys, setPrevAutoExpandKeys] = useState(autoExpandKeys);
  if (prevAutoExpandKeys.join("|") !== autoExpandKeys.join("|")) {
    setPrevAutoExpandKeys(autoExpandKeys);
    setExpandedKeys((prev) => {
      const merged = Array.from(new Set([...prev, ...autoExpandKeys]));
      return merged.length === prev.length ? prev : merged;
    });
  }

  const moveGroup = (groupKey: string, delta: number) => {
    if (!onGroupOrderChange) return;
    const index = groupKeys.indexOf(groupKey);
    const target = index + delta;
    if (index < 0 || target < 0 || target >= groupKeys.length) return;
    const next = [...groupKeys];
    const [moved] = next.splice(index, 1);
    next.splice(target, 0, moved);
    onGroupOrderChange(next.map((key) => key.slice(GROUP_KEY_PREFIX.length)));
  };

  const withGroupActions = (data: TreeProps["treeData"]): TreeProps["treeData"] => {
    if (!onGroupOrderChange || disabled) return data;
    return (data ?? []).map((node) => {
      if (!node || !("children" in node) || !node.children) return node;
      const groups = node.children as NonNullable<TreeProps["treeData"]>;
      return {
        ...node,
        children: groups.map((group, groupIndex) => {
          const key = String((group as { key: string }).key);
          return {
            ...group,
            title: (
              <span style={{ display: "inline-flex", alignItems: "center", gap: 4 }}>
                <span>{(group as { title?: string }).title}</span>
                <Button
                  size="small"
                  type="text"
                  aria-label={`move-up-${key}`}
                  icon={<ArrowUpOutlined />}
                  disabled={groupIndex === 0}
                  onClick={(event) => {
                    event.stopPropagation();
                    moveGroup(key, -1);
                  }}
                />
                <Button
                  size="small"
                  type="text"
                  aria-label={`move-down-${key}`}
                  icon={<ArrowDownOutlined />}
                  disabled={groupIndex === groups.length - 1}
                  onClick={(event) => {
                    event.stopPropagation();
                    moveGroup(key, 1);
                  }}
                />
              </span>
            ),
          };
        }),
      };
    });
  };

  const handleDrop: NonNullable<TreeProps["onDrop"]> = (info) => {
    const dragKey = String(info.dragNode.key);
    const dropKey = String(info.node.key);
    // 仅允许同组内叶子之间拖拽排序。
    if (!isLeafKey(dragKey) || !isLeafKey(dropKey)) return;
    if (parentByLeaf[dragKey] !== parentByLeaf[dropKey]) return;

    const groupKey = parentByLeaf[dragKey];
    const leaves = leavesByGroup[groupKey] ?? [];
    const dragIndex = leaves.indexOf(dragKey);
    const dropIndex = leaves.indexOf(dropKey);
    if (dragIndex < 0 || dropIndex < 0) return;

    const next = [...leaves];
    next.splice(dragIndex, 1);
    // dropPosition 大于原始 dropIndex 表示落在目标节点之后。
    const insertAt = next.indexOf(dropKey) + (info.dropPosition > dropIndex ? 1 : 0);
    next.splice(insertAt, 0, dragKey);

    const checkedSet = new Set(stripNonLeafKeys(checkedKeys));
    const orderedChecked: string[] = [];
    for (const group of groupKeys) {
      const groupLeaves = group === groupKey ? next : (leavesByGroup[group] ?? []);
      for (const leaf of groupLeaves) {
        if (checkedSet.has(leaf)) orderedChecked.push(leaf);
      }
    }
    // 保留不在当前树中的历史勾选键（如旧版不带斜杠路径），追加在末尾。
    for (const key of checkedKeys) {
      if (isLeafKey(key) && !orderedChecked.includes(key)) orderedChecked.push(key);
    }
    onCheck?.(orderedChecked);
  };

  return (
    <Tree
      checkable
      disabled={disabled}
      draggable={!disabled}
      allowDrop={({ dragNode, dropNode }) => {
        const dragKey = String(dragNode.key);
        const dropKey = String(dropNode.key);
        return (
          isLeafKey(dragKey) &&
          isLeafKey(dropKey) &&
          parentByLeaf[dragKey] === parentByLeaf[dropKey]
        );
      }}
      onDrop={handleDrop}
      checkedKeys={checkedKeys}
      expandedKeys={expandedKeys}
      onExpand={(keys) => setExpandedKeys(keys as string[])}
      defaultExpandedKeys={defaultExpandedKeys}
      onCheck={(checked) => {
        onCheck?.(stripNonLeafKeys(checked as string[]));
      }}
      treeData={withGroupActions(treeData)}
    />
  );
}
