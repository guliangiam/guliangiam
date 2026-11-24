import { memo, useCallback } from "react";
import { PaginateSelect } from "@/shared/ui/PaginateSelect";
import { getGroups } from "@/shared/api/iam";

type GroupPaginateSelectProps = {
  value?: string[] | null;
  onChange?: (value: string[]) => void;
  placeholder?: string;
  organizationId?: string;
  allowClear?: boolean;
  /** antd Form.Item 注入的 id，透传给内部 Select 以关联 label。 */
  id?: string;
  ariaLabel?: string;
  /** 添加 "All" 通配选项的 value（如 "*"） */
  allOptionValue?: string;
  allOptionLabel?: string;
  disabled?: boolean;
};

const PAGE_SIZE = 20;

/**
 * GroupPaginateSelect — PaginateSelect 包装，查用户组列表。
 *
 * 替代在各页面内联 PaginateSelect + getGroups + 过滤 + 分页 + map 的重复模式。
 * value = name（与现有 Casbin sub 口径一致），
 * label = displayName || name（符合"字段显示以用户可读名称为准"原则）。
 */
export const GroupPaginateSelect = memo(function GroupPaginateSelect({
  value,
  onChange,
  placeholder = "Search groups",
  organizationId,
  allowClear,
  id,
  ariaLabel,
  allOptionValue,
  allOptionLabel,
  disabled,
}: GroupPaginateSelectProps) {
  const loader = useCallback(
    async (search: string, page: number) => {
      const groups = await getGroups(organizationId);
      const filtered = search
        ? groups.filter(
            (g) =>
              g.name.toLowerCase().includes(search.toLowerCase()) ||
              (g.displayName ?? "").toLowerCase().includes(search.toLowerCase()),
          )
        : groups;
      const start = (page - 1) * PAGE_SIZE;
      const end = start + PAGE_SIZE;
      return {
        // label 统一为「displayName (name)」形态；重名时退化为单值。
        options: filtered.slice(start, end).map((g) => ({
          label:
            g.displayName && g.displayName !== g.name ? `${g.displayName} (${g.name})` : g.name,
          value: g.name,
        })),
        hasMore: end < filtered.length,
      };
    },
    [organizationId],
  );

  return (
    <PaginateSelect
      value={value}
      onChange={onChange}
      placeholder={placeholder}
      id={id}
      ariaLabel={ariaLabel}
      allowClear={allowClear}
      loader={loader}
      allOptionValue={allOptionValue}
      allOptionLabel={allOptionLabel}
      disabled={disabled}
    />
  );
});
