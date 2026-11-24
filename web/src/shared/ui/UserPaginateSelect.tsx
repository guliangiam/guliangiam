import { memo, useCallback } from "react";
import { PaginateSelect } from "@/shared/ui/PaginateSelect";
import { getUserOptions } from "@/shared/api/iam/user";

type UserPaginateSelectProps = {
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
 * UserPaginateSelect — PaginateSelect 包装，查用户列表。
 *
 * 替代在各页面内联 PaginateSelect + getUserOptions + 过滤 + 分页 + map 的重复模式。
 * value = username（与现有 Casbin sub/obj 口径一致），
 * label = `${displayName} (${username})`（符合"字段显示以用户可读名称为准"原则）。
 */
export const UserPaginateSelect = memo(function UserPaginateSelect({
  value,
  onChange,
  placeholder = "Search users",
  organizationId,
  allowClear,
  id,
  ariaLabel,
  allOptionValue,
  allOptionLabel,
  disabled,
}: UserPaginateSelectProps) {
  const loader = useCallback(
    async (search: string, page: number) => {
      const users = await getUserOptions(organizationId);
      const filtered = search
        ? users.filter(
            (u) =>
              u.username.toLowerCase().includes(search.toLowerCase()) ||
              (u.displayName ?? "").toLowerCase().includes(search.toLowerCase()),
          )
        : users;
      const start = (page - 1) * PAGE_SIZE;
      const end = start + PAGE_SIZE;
      return {
        options: filtered.slice(start, end).map((u) => ({
          label: `${u.displayName} (${u.username})`,
          value: u.username,
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
