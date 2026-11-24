import { memo, useCallback } from "react";
import { AsyncSearchSelect } from "@/shared/ui/AsyncSearchSelect";
import { getUserOptions } from "@/shared/api/iam/user";

type UserSelectProps = {
  value?: string;
  onChange?: (value: string) => void;
  placeholder?: string;
  organizationId?: string;
  allowClear?: boolean;
};

/**
 * UserSelect — AsyncSearchSelect 包装，查用户列表。
 *
 * 替代在各页面内联 AsyncSearchSelect + getUserOptions + 过滤 + map 的重复模式。
 * 字段显示：displayName 优先，fallback username（符合"字段显示以用户可读名称为准"原则）。
 */
export const UserSelect = memo(function UserSelect({
  value,
  onChange,
  placeholder = "Select user",
  organizationId,
  allowClear,
}: UserSelectProps) {
  const loader = useCallback(
    async (search: string) => {
      const users = await getUserOptions(organizationId);
      const filtered = search
        ? users.filter(
            (u) =>
              u.username.toLowerCase().includes(search.toLowerCase()) ||
              (u.displayName ?? "").toLowerCase().includes(search.toLowerCase()),
          )
        : users;
      // label 统一为「displayName (username)」形态；重名时退化为单值。
      return filtered.map((u) => ({
        label:
          u.displayName && u.displayName !== u.username
            ? `${u.displayName} (${u.username})`
            : u.username || u.displayName,
        value: u.id,
      }));
    },
    [organizationId],
  );

  return (
    <AsyncSearchSelect
      value={value}
      onChange={onChange as never}
      placeholder={placeholder}
      allowClear={allowClear}
      loader={loader}
    />
  );
});
