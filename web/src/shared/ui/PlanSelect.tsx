import { memo, useCallback } from "react";
import { AsyncSearchSelect } from "@/shared/ui/AsyncSearchSelect";
import { getPlanOptions } from "@/shared/api/iam/plan";

type PlanSelectProps = {
  value?: string;
  onChange?: (value: string) => void;
  placeholder?: string;
  organizationId?: string;
  allowClear?: boolean;
};

/**
 * PlanSelect — AsyncSearchSelect 包装，查计划列表。
 * 替代在页面内联 AsyncSearchSelect + loader 的重复模式。
 */
export const PlanSelect = memo(function PlanSelect({
  value,
  onChange,
  placeholder = "Select plan",
  organizationId,
  allowClear,
}: PlanSelectProps) {
  const loader = useCallback(
    async (search: string) => {
      const plans = await getPlanOptions(organizationId);
      const filtered = search
        ? plans.filter(
            (p) =>
              (p.name ?? "").toLowerCase().includes(search.toLowerCase()) ||
              (p.displayName ?? "").toLowerCase().includes(search.toLowerCase()),
          )
        : plans;
      return filtered.map((p) => ({
        label: p.displayName ?? p.name ?? p.id,
        value: p.id,
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
