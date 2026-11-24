import { memo, useCallback } from "react";
import { PaginateSelect } from "@/shared/ui/PaginateSelect";
import { getRoleOptions } from "@/shared/api/iam/role";

type RolePaginateSelectProps = {
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
 * RolePaginateSelect — PaginateSelect 包装，查角色列表。
 *
 * 替代在各页面内联 PaginateSelect + getRoleOptions + 过滤 + 分页 + map 的重复模式。
 * value = id（与现有 Casbin role 口径一致），
 * label = displayName || name（符合"字段显示以用户可读名称为准"原则）。
 */
export const RolePaginateSelect = memo(function RolePaginateSelect({
  value,
  onChange,
  placeholder = "Search roles",
  organizationId,
  allowClear,
  id,
  ariaLabel,
  allOptionValue,
  allOptionLabel,
  disabled,
}: RolePaginateSelectProps) {
  const loader = useCallback(
    async (search: string, page: number) => {
      const roles = await getRoleOptions(organizationId);
      const filtered = search
        ? roles.filter(
            (r) =>
              r.name.toLowerCase().includes(search.toLowerCase()) ||
              (r.displayName ?? "").toLowerCase().includes(search.toLowerCase()),
          )
        : roles;
      const start = (page - 1) * PAGE_SIZE;
      const end = start + PAGE_SIZE;
      return {
        // label 统一为「displayName (name)」形态；重名时退化为单值。
        options: filtered.slice(start, end).map((r) => ({
          label:
            r.displayName && r.displayName !== r.name ? `${r.displayName} (${r.name})` : r.name,
          value: r.id,
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
