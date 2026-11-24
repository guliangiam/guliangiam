import { Select, Space } from "antd";
import { type FC, useCallback, useMemo } from "react";
import { useTranslation } from "react-i18next";
import { getCountryCodeData, getCountryFlag } from "./country-data";

type RegionOption = {
  value?: string | number | null;
  label?: React.ReactNode;
};

function renderRegionOption({ value: code, label }: RegionOption) {
  return (
    <Space>
      <span aria-hidden="true">{getCountryFlag(String(code))}</span>
      {label}
    </Space>
  );
}

type RegionSelectProps = {
  value?: string;
  defaultValue?: string;
  onChange?: (value: string) => void;
  size?: "small" | "middle" | "large";
  style?: React.CSSProperties;
  className?: string;
  countryCodes?: string[];
  /** antd Form.Item 注入的 id，透传给内部 Select 以关联 label。 */
  id?: string;
  ariaLabel?: string;
  allowClear?: boolean;
  disabled?: boolean;
};

export const RegionSelect: FC<RegionSelectProps> = ({
  value,
  defaultValue,
  onChange,
  size,
  style,
  className,
  countryCodes,
  id,
  ariaLabel,
  allowClear,
  disabled,
}) => {
  const { t, i18n } = useTranslation();
  const language = i18n.resolvedLanguage ?? i18n.language;
  const countryData = useMemo(
    () => getCountryCodeData(countryCodes, language),
    [countryCodes, language],
  );

  const filterOption = useCallback(
    (input: string, option?: { label?: string }) =>
      (option?.label ?? "").toLowerCase().includes(input.toLowerCase()),
    [],
  );

  return (
    <Select
      showSearch
      id={id}
      aria-label={ariaLabel}
      disabled={disabled}
      size={size}
      className={className}
      style={{ width: "100%", ...style }}
      value={value}
      defaultValue={value === undefined ? defaultValue : undefined}
      allowClear={allowClear}
      placeholder={t("selectCountryOrRegion")}
      onChange={(nextValue?: string) => onChange?.(nextValue ?? "")}
      filterOption={filterOption}
      options={countryData.map((item) => ({
        value: item.code,
        label: `${item.name} (${item.code})`,
      }))}
      optionRender={renderRegionOption}
    />
  );
};
