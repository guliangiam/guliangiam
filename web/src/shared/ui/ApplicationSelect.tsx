import { memo, useCallback } from "react";
import { AsyncSearchSelect } from "@/shared/ui/AsyncSearchSelect";
import { getApplicationOptions } from "@/shared/api/iam/application";

type ApplicationSelectProps = {
  value?: string;
  onChange?: (value: string) => void;
  placeholder?: string;
  organizationId?: string;
  allowClear?: boolean;
};

/**
 * ApplicationSelect — AsyncSearchSelect 包装，查应用列表。
 *
 * 替代在各页面内联 AsyncSearchSelect + getApplicationOptions + 过滤 + map 的重复模式。
 * 字段显示：client_id（应用标识）作为 label，因为它是应用间唯一且可读的区分符号。
 */
export const ApplicationSelect = memo(function ApplicationSelect({
  value,
  onChange,
  placeholder = "Select application",
  organizationId,
  allowClear,
}: ApplicationSelectProps) {
  const loader = useCallback(
    async (search: string) => {
      const apps = await getApplicationOptions(organizationId);
      const filtered = search
        ? apps.filter(
            (a) =>
              (a.slug ?? "").toLowerCase().includes(search.toLowerCase()) ||
              (a.displayName ?? "").toLowerCase().includes(search.toLowerCase()),
          )
        : apps;
      // label 统一为「displayName (slug)」形态；无 displayName 时退化为 slug。
      return filtered.map((a) => ({
        label:
          a.displayName && a.displayName !== a.slug
            ? `${a.displayName} (${a.slug})`
            : a.slug || a.displayName || a.id,
        value: a.id,
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
