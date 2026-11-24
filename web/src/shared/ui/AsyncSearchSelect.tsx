import type { CSSProperties } from "react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Button, Select, Skeleton, Space } from "antd";
import { useTranslation } from "react-i18next";

/**
 * AsyncSearchSelect — 异步搜索选择器（全量加载 + debounce 搜索）。
 *
 * Naming convention for wrappers: `<Entity>Select`
 *   • value type: string（单选），或 string[] + mode="multiple"（多选但不分页）
 *   • 适用场景：组织/应用/国家/地区等小到中等数据集
 *   • 实现：loader(search) 返回 options[]，无分页
 *   • 已有包装：OrganizationSelect, UserSelect, ApplicationSelect, CountryCodeSelect, RegionSelect
 *
 * 对照 PaginateSelect：
 *   AsyncSearchSelect → 全量加载、命名 `<Entity>Select`
 *   PaginateSelect    → 分页加载、命名 `<Entity>PaginateSelect`
 */

type SelectOption = {
  label: string;
  value: string;
};

type AsyncSearchSelectProps = {
  value?: string | string[] | null;
  onChange?: (value: string | string[]) => void;
  placeholder?: string;
  mode?: "multiple";
  allowClear?: boolean;
  /** antd Form.Item 注入的 id：必须透传到内部 input，label 的 htmlFor 才能命中。 */
  id?: string;
  ariaLabel?: string;
  style?: CSSProperties;
  className?: string;
  loader: (search: string) => Promise<SelectOption[]>;
  staticOptions?: SelectOption[];
};

function normalizeValueOptions(value?: string | string[] | null): SelectOption[] {
  if (!value) {
    return [];
  }

  const values = Array.isArray(value) ? value : [value];
  // Don't use raw value as label — that leaks UUIDs/IDs to the user when
  // the loader hasn't resolved the display name yet. Show a neutral placeholder
  // until the real options arrive.
  return values.filter(Boolean).map((item) => ({ label: "", value: item }));
}

export function AsyncSearchSelect({
  value,
  onChange,
  placeholder,
  mode,
  allowClear,
  id,
  ariaLabel,
  style,
  className,
  loader,
  staticOptions,
}: AsyncSearchSelectProps) {
  const { t } = useTranslation("common");
  const [request, setRequest] = useState({ search: "", retry: 0 });
  const [loading, setLoading] = useState(false);
  const [options, setOptions] = useState<SelectOption[]>([]);
  const [loadFailed, setLoadFailed] = useState(false);

  useEffect(() => {
    let cancelled = false;
    const timer = window.setTimeout(async () => {
      setLoading(true);
      setLoadFailed(false);
      try {
        const result = await loader(request.search.trim());
        if (!cancelled) {
          setOptions(result);
        }
      } catch {
        if (!cancelled) {
          setOptions([]);
          setLoadFailed(true);
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    }, 250);

    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [loader, request]);

  const mergedOptions = useMemo(() => {
    const merged = new Map<string, SelectOption>();
    normalizeValueOptions(value).forEach((item) => merged.set(item.value, item));
    (staticOptions ?? []).forEach((item) => merged.set(item.value, item));
    options.forEach((item) => merged.set(item.value, item));
    return Array.from(merged.values());
  }, [options, staticOptions, value]);

  const renderPopup = useCallback(
    (menu: React.ReactNode) => (
      <>
        {menu}
        {loadFailed && !loading ? (
          <Space style={{ padding: 8 }}>
            <span>{t("loadFailed")}</span>
            <Button
              size="small"
              onMouseDown={(event) => event.preventDefault()}
              onClick={() =>
                setRequest((current) => ({ search: current.search, retry: current.retry + 1 }))
              }
            >
              {t("retry")}
            </Button>
          </Space>
        ) : null}
      </>
    ),
    [loadFailed, loading, t],
  );

  return (
    <Select
      showSearch
      id={id}
      aria-label={ariaLabel ?? placeholder}
      mode={mode}
      allowClear={allowClear}
      filterOption={false}
      placeholder={placeholder}
      style={style}
      className={className}
      value={value ?? undefined}
      options={mergedOptions}
      onSearch={(search) =>
        setRequest((current) => (current.search === search ? current : { search, retry: 0 }))
      }
      onChange={onChange}
      popupRender={renderPopup}
      notFoundContent={
        loading ? <Skeleton.Input active size="small" style={{ width: 80 }} /> : null
      }
    />
  );
}
