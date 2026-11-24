import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Button, Select, Skeleton, Space } from "antd";
import { useTranslation } from "react-i18next";

/**
 * PaginateSelect — 分页异步选择器（server-side paginated, multi-select 为主）。
 *
 * Naming convention for wrappers: `<Entity>PaginateSelect`
 *   • value type: string[] (多选)
 *   • 适用场景：用户/角色/权限等大数据集，需要分页加载
 *   • 实现：内部 loader(search, page) 返回 { options, hasMore }
 *   • 已有包装：UserPaginateSelect, GroupPaginateSelect, RolePaginateSelect
 *
 * 对照 AsyncSearchSelect：
 *   AsyncSearchSelect → 全量加载（无分页）、value=string（单选），命名 `<Entity>Select`
 *   PaginateSelect    → 分页加载、value=string[]（多选），命名 `<Entity>PaginateSelect`
 */

type SelectOption = {
  label: string;
  value: string;
};

type PaginateSelectProps = {
  value?: string[] | null;
  onChange?: (value: string[]) => void;
  placeholder?: string;
  allowClear?: boolean;
  /** antd Form.Item 注入的 id：透传到内部 input 后 label 的 htmlFor 才能命中。 */
  id?: string;
  ariaLabel?: string;
  /** Async loader: receives search text and page number, returns options + hasMore */
  loader: (search: string, page: number) => Promise<{ options: SelectOption[]; hasMore: boolean }>;
  /** Prepend a special "All" option with this value (e.g. "*") */
  allOptionValue?: string;
  allOptionLabel?: string;
  disabled?: boolean;
  style?: React.CSSProperties;
};

export function PaginateSelect({
  value,
  onChange,
  placeholder,
  allowClear,
  id,
  ariaLabel,
  loader,
  allOptionValue,
  allOptionLabel = "All",
  disabled,
  style,
}: PaginateSelectProps) {
  const { t } = useTranslation("common");
  const [request, setRequest] = useState({ search: "", retry: 0 });
  const [loading, setLoading] = useState(false);
  const [options, setOptions] = useState<SelectOption[]>([]);
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(false);
  const [loadFailed, setLoadFailed] = useState(false);
  const requestVersionRef = useRef(0);
  const loadingRef = useRef(false);

  // Reset and fetch on search change
  useEffect(() => {
    const requestVersion = ++requestVersionRef.current;
    loadingRef.current = true;
    const timer = window.setTimeout(async () => {
      if (requestVersion !== requestVersionRef.current) return;
      setLoading(true);
      setLoadFailed(false);
      try {
        const result = await loader(request.search.trim(), 1);
        if (requestVersion !== requestVersionRef.current) return;
        setOptions(result.options);
        setHasMore(result.hasMore);
        setPage(1);
      } catch {
        if (requestVersion === requestVersionRef.current) {
          setOptions([]);
          setHasMore(false);
          setPage(1);
          setLoadFailed(true);
        }
      } finally {
        if (requestVersion === requestVersionRef.current) {
          loadingRef.current = false;
          setLoading(false);
        }
      }
    }, 300);

    return () => {
      window.clearTimeout(timer);
      requestVersionRef.current += 1;
    };
  }, [loader, request]);

  // Load more on scroll to bottom
  const handlePopupScroll = useCallback(
    async (e: React.UIEvent<HTMLDivElement>) => {
      const { scrollTop, scrollHeight, clientHeight } = e.currentTarget;
      if (scrollHeight - scrollTop - clientHeight >= 10 || !hasMore || loadingRef.current) {
        return;
      }

      const requestVersion = requestVersionRef.current;
      const nextPage = page + 1;
      loadingRef.current = true;
      setLoading(true);
      setLoadFailed(false);
      try {
        const result = await loader(request.search.trim(), nextPage);
        if (requestVersion !== requestVersionRef.current) return;
        setOptions((prev) => {
          const merged = new Map<string, SelectOption>();
          prev.forEach((item) => merged.set(item.value, item));
          result.options.forEach((item) => merged.set(item.value, item));
          return Array.from(merged.values());
        });
        setHasMore(result.hasMore);
        setPage(nextPage);
      } catch {
        if (requestVersion === requestVersionRef.current) {
          setLoadFailed(true);
        }
      } finally {
        if (requestVersion === requestVersionRef.current) {
          loadingRef.current = false;
          setLoading(false);
        }
      }
    },
    [loader, request, page, hasMore],
  );

  // Merge current value options so selected items always display their label
  const mergedOptions = useMemo(() => {
    const merged = new Map<string, SelectOption>();

    // Keep selected values present without exposing raw IDs before their labels load.
    if (value) {
      for (const v of value) {
        if (v) merged.set(v, { label: "", value: v });
      }
    }

    // Loaded options override with proper labels
    for (const item of options) {
      merged.set(item.value, item);
    }

    // Prepend "All" option if configured
    if (allOptionValue) {
      merged.set(allOptionValue, { label: allOptionLabel, value: allOptionValue });
    }

    return Array.from(merged.values());
  }, [options, value, allOptionValue, allOptionLabel]);

  const handleChange = useCallback(
    (newValue: string[]) => {
      // If "All" is selected, replace with just the all value
      if (allOptionValue && newValue.includes(allOptionValue)) {
        onChange?.([allOptionValue]);
        return;
      }
      // Remove "All" if other items are added
      const filtered = newValue.filter((v) => v !== allOptionValue);
      onChange?.(filtered);
    },
    [onChange, allOptionValue],
  );

  const renderPopup = useCallback(
    (menu: React.ReactNode) => (
      <>
        {menu}
        {loadFailed && !loading && options.length === 0 ? (
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
    [loadFailed, loading, options.length, t],
  );

  return (
    <Select
      mode="multiple"
      showSearch
      id={id}
      aria-label={ariaLabel ?? placeholder}
      allowClear={allowClear}
      filterOption={false}
      placeholder={placeholder}
      value={value ?? undefined}
      options={mergedOptions}
      onSearch={(search) =>
        setRequest((current) => (current.search === search ? current : { search, retry: 0 }))
      }
      onChange={handleChange}
      onPopupScroll={handlePopupScroll}
      popupRender={renderPopup}
      notFoundContent={
        loading ? <Skeleton.Input active size="small" style={{ width: 80 }} /> : null
      }
      disabled={disabled}
      style={style}
    />
  );
}
