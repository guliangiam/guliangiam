import { memo, useCallback } from "react";
import { AsyncSearchSelect } from "@/shared/ui/AsyncSearchSelect";
import { getOrderOptions } from "@/shared/api/iam/order";

type OrderSelectProps = {
  value?: string;
  onChange?: (value: string) => void;
  placeholder?: string;
  organizationId?: string;
  allowClear?: boolean;
};

/**
 * OrderSelect — AsyncSearchSelect 包装，查订单列表。
 *
 * 替代 TransactionCreatePage 等操作表单中 orderId 的自由输入框
 * （交互简化 §6：关联选择字段不得使用自由输入）。
 * 字段显示：displayName 优先，fallback name（符合"字段显示以用户可读名称为准"原则）。
 */
export const OrderSelect = memo(function OrderSelect({
  value,
  onChange,
  placeholder = "Select order",
  organizationId,
  allowClear,
}: OrderSelectProps) {
  const loader = useCallback(
    async (search: string) => {
      const orders = await getOrderOptions(organizationId);
      const filtered = search
        ? orders.filter(
            (o) =>
              (o.displayName ?? "").toLowerCase().includes(search.toLowerCase()) ||
              (o.name ?? "").toLowerCase().includes(search.toLowerCase()),
          )
        : orders;
      return filtered.map((o) => ({
        label: o.displayName ?? o.name ?? o.id,
        value: o.id,
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
