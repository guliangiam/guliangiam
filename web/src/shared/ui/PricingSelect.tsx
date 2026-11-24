import { memo, useCallback } from "react";
import { AsyncSearchSelect } from "@/shared/ui/AsyncSearchSelect";
import { getPricingOptions } from "@/shared/api/iam/pricing";

type PricingSelectProps = {
  value?: string;
  onChange?: (value: string) => void;
  placeholder?: string;
  organizationId?: string;
  allowClear?: boolean;
};

/**
 * PricingSelect — AsyncSearchSelect 包装，查定价列表。
 * 替代在页面内联 AsyncSearchSelect + loader 的重复模式。
 */
export const PricingSelect = memo(function PricingSelect({
  value,
  onChange,
  placeholder = "Select pricing",
  organizationId,
  allowClear,
}: PricingSelectProps) {
  const loader = useCallback(
    async (search: string) => {
      const pricings = await getPricingOptions(organizationId);
      const filtered = search
        ? pricings.filter(
            (p) =>
              (p.name ?? "").toLowerCase().includes(search.toLowerCase()) ||
              (p.displayName ?? "").toLowerCase().includes(search.toLowerCase()),
          )
        : pricings;
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
