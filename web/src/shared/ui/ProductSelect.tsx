import { memo, useCallback } from "react";
import { AsyncSearchSelect } from "@/shared/ui/AsyncSearchSelect";
import { getProductOptions } from "@/shared/api/iam/product";

type ProductSelectProps = {
  value?: string;
  onChange?: (value: string) => void;
  placeholder?: string;
  organizationId?: string;
  allowClear?: boolean;
  /** Exclude the current product from options (for parentProductId self-reference) */
  excludeId?: string;
};

/**
 * ProductSelect — AsyncSearchSelect 包装，查产品列表。
 * 支持自引用排除（如 parentProductId 场景不能选自己）。
 */
export const ProductSelect = memo(function ProductSelect({
  value,
  onChange,
  placeholder = "Select product",
  organizationId,
  allowClear,
  excludeId,
}: ProductSelectProps) {
  const loader = useCallback(
    async (search: string) => {
      const products = await getProductOptions(organizationId);
      const filtered = products.filter((p) => p.id !== excludeId);
      const match = search
        ? filtered.filter(
            (p) =>
              (p.name ?? "").toLowerCase().includes(search.toLowerCase()) ||
              (p.displayName ?? "").toLowerCase().includes(search.toLowerCase()),
          )
        : filtered;
      return match.map((p) => ({
        label: p.displayName ?? p.name ?? p.id,
        value: p.id,
      }));
    },
    [organizationId, excludeId],
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
