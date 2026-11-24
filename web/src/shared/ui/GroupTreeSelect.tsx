import { TreeSelect } from "antd";
import { type FC, useMemo } from "react";

type GroupItem = {
  id: string;
  name: string;
  displayName: string;
  parentId?: string;
};

// antd TreeSelect 的选项以 value 标识；TreeDataNode 类型不含 value，
// 这里定义局部节点类型，避免「TreeNode value is invalidate: undefined」告警
// 与「父组无法选择」。
type GroupTreeNode = {
  title: string;
  key: string;
  value: string;
  children?: GroupTreeNode[];
};

type GroupTreeSelectProps = {
  value?: string | string[];
  onChange?: (value: string | string[]) => void;
  groups?: GroupItem[];
  /**
   * 需要从选项中剔除的组 id（当前组及其全部后代）。
   * 用于编辑页阻止「把父组选成自己/自己的后代」形成环。
   */
  excludeIds?: string[];
  /**
   * 选中值取组 id（默认）还是组 name。
   * name 口径用于存储组名引用的字段（如 application.defaultGroup，
   * 与 Casbin sub / GroupPaginateSelect 的 name 口径一致）。
   */
  valueKey?: "id" | "name";
  multiple?: boolean;
  placeholder?: string;
  disabled?: boolean;
  /** antd Form.Item 注入的 id，透传给内部控件以关联 label。 */
  id?: string;
  ariaLabel?: string;
  style?: React.CSSProperties;
};

function buildTree(
  groups: GroupItem[],
  excludeIds?: string[],
  valueKey: "id" | "name" = "id",
): GroupTreeNode[] {
  const excluded = new Set(excludeIds ?? []);
  const visible = groups.filter((group) => !excluded.has(group.id));
  const map = new Map<string, GroupTreeNode>();
  const roots: GroupTreeNode[] = [];

  for (const group of visible) {
    // title 统一为「displayName (name)」形态；重名时退化为单值。
    // key 必须与 value 一致（rc-tree-select 对 key≠value 告警）；
    // valueKey="name" 时组名即业务键，组织内唯一。
    map.set(group.id, {
      title:
        group.displayName && group.displayName !== group.name
          ? `${group.displayName} (${group.name})`
          : group.name,
      key: group[valueKey],
      value: group[valueKey],
      children: [],
    });
  }

  // 只挂载可见组：excludeIds 中的组（当前组及其后代）不在 map 里，必须跳过，
  // 否则会把 undefined 推进 children / roots，clean() 读 children 即崩溃。
  for (const group of visible) {
    const node = map.get(group.id);
    if (!node) continue;
    const parent = group.parentId ? map.get(group.parentId) : undefined;
    if (parent) {
      parent.children!.push(node);
    } else {
      roots.push(node);
    }
  }

  // Remove empty children arrays to avoid expand arrows on leaf nodes
  function clean(nodes: GroupTreeNode[]) {
    for (const node of nodes) {
      if (!node) continue;
      if (!node.children || node.children.length === 0) {
        delete node.children;
      } else {
        clean(node.children);
      }
    }
  }
  clean(roots);

  return roots;
}

export const GroupTreeSelect: FC<GroupTreeSelectProps> = ({
  value,
  onChange,
  groups = [],
  excludeIds,
  valueKey = "id",
  multiple = false,
  placeholder = "Select group",
  disabled,
  id,
  ariaLabel,
  style,
}) => {
  const treeData = useMemo(
    () => buildTree(groups, excludeIds, valueKey),
    [groups, excludeIds, valueKey],
  );

  return (
    <TreeSelect
      value={value}
      onChange={onChange}
      disabled={disabled}
      id={id}
      aria-label={ariaLabel ?? placeholder}
      treeData={treeData}
      multiple={multiple}
      placeholder={placeholder}
      style={{ width: "100%", ...style }}
      showSearch
      treeNodeFilterProp="title"
      allowClear
    />
  );
};
