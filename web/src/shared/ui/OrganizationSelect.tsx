import { memo, useCallback, useEffect, useRef } from "react";
import { AsyncSearchSelect } from "@/shared/ui/AsyncSearchSelect";
import { getOrganizationOptions } from "@/shared/api/iam/organization";
import { useSessionStore } from "@/features/session/session-store";

type OrganizationSelectProps = {
  value?: string;
  onChange?: (value: string) => void;
  id?: string;
  placeholder?: string;
  ariaLabel?: string;
  style?: React.CSSProperties;
  allowClear?: boolean;
  /**
   * autoFillSessionOrg — 默认 true。
   * 当 value 为空（Create 表单初始态）且 session 已加载时，
   * 自动将当前 session 的 organization.id 作为默认值触发 onChange。
   *
   * 为什么在组件层做而不是每个 Create 页面自己初始化？
   *   → OrganizationSelect 是唯一知道"session org 是什么 + 用户有没有选过"的地方。
   *   → Create 页面只负责把 form.organizationId 传给 value，不管"从哪来"。
   *   → 一个开关 opt-out：某些页面（组织列表筛选）明确不想默认值时传 false。
   */
  autoFillSessionOrg?: boolean;
  /** 直接排除的组织 id 列表（如当前组织自身）。 */
  excludeIds?: string[];
  /**
   * 排除该组织及其全部后代（父组织选择器防环）。后代关系按选项列表的
   * parentId 链计算；列表分页之外的组织不在候选内，天然不可选。
   */
  excludeDescendantsOf?: string;
};

export const OrganizationSelect = memo(function OrganizationSelect({
  value,
  onChange,
  id,
  placeholder = "Select organization",
  ariaLabel,
  style,
  allowClear,
  autoFillSessionOrg = true,
  excludeIds,
  excludeDescendantsOf,
}: OrganizationSelectProps) {
  // excludeIds 以内容而非引用参与依赖，避免父组件每次渲染传新数组导致重复加载。
  const excludeKey = (excludeIds ?? []).join(",");
  const loader = useCallback(
    async (search: string) => {
      const orgs = await getOrganizationOptions();
      const excluded = new Set(excludeKey ? excludeKey.split(",") : []);
      if (excludeDescendantsOf) {
        excluded.add(excludeDescendantsOf);
        const childrenByParent = new Map<string, string[]>();
        for (const org of orgs) {
          if (!org.parentId) continue;
          const siblings = childrenByParent.get(org.parentId) ?? [];
          siblings.push(org.id);
          childrenByParent.set(org.parentId, siblings);
        }
        const queue = [excludeDescendantsOf];
        while (queue.length > 0) {
          const current = queue.shift()!;
          for (const child of childrenByParent.get(current) ?? []) {
            if (excluded.has(child)) continue;
            excluded.add(child);
            queue.push(child);
          }
        }
      }
      const filtered = orgs.filter(
        (o) =>
          !excluded.has(o.id) &&
          (!search ||
            o.slug.toLowerCase().includes(search.toLowerCase()) ||
            (o.displayName ?? "").toLowerCase().includes(search.toLowerCase())),
      );
      // label 统一为「displayName (slug)」形态；无 slug 时退化为单值。
      return filtered.map((o) => ({
        label:
          o.displayName && o.displayName !== o.slug
            ? `${o.displayName} (${o.slug})`
            : o.slug || o.displayName,
        value: o.id,
      }));
    },
    [excludeKey, excludeDescendantsOf],
  );

  // ── 默认值自动注入 ────────────────────────────────────
  // 用 ref 确保只触发一次：首次 mount 时 value 为空 + session.org 已就绪 → 填 session org。
  // 之后用户主动清除或改选不再自动填。
  const sessionOrgId = useSessionStore((s) => s.organization?.id);
  const didAutoFill = useRef(false);

  useEffect(() => {
    if (!autoFillSessionOrg) return;
    if (didAutoFill.current) return;
    if (value) return; // 用户已经选过了
    if (!sessionOrgId) return; // session 还没 bootstrap 完成
    didAutoFill.current = true;
    onChange?.(sessionOrgId);
  }, [autoFillSessionOrg, value, sessionOrgId, onChange]);

  return (
    <AsyncSearchSelect
      value={value}
      onChange={onChange as never}
      id={id}
      placeholder={placeholder}
      ariaLabel={ariaLabel}
      style={style}
      allowClear={allowClear}
      loader={loader}
    />
  );
});
