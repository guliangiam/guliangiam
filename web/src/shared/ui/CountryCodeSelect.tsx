import { Select, Space, type SelectProps } from "antd";
import { type FC, useCallback, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { getCountryCodeData, getCountryFlag, type CountryData } from "./country-data";

type CountryCodeSelectValue = string | string[];

type CommonProps = {
  style?: React.CSSProperties;
  disabled?: boolean;
  countryCodes?: string[];
  hasDefault?: boolean;
  /** antd Form.Item 注入的 id，透传给内部 Select 以关联 label。 */
  id?: string;
  ariaLabel?: string;
  allowClear?: boolean;
  placeholder?: string;
};

type SingleSelectProps = CommonProps & {
  mode?: undefined;
  onChange?: (value: string) => void;
  initValue?: string;
  value?: string;
};

type MultiSelectProps = CommonProps & {
  mode: "multiple" | "tags";
  onChange?: (value: string[]) => void;
  initValue?: string[];
  value?: string[];
};

type CountryCodeSelectProps = SingleSelectProps | MultiSelectProps;

type CountryOption = {
  value: string;
  label: string;
  text: string;
  country?: CountryData;
};

type CountryOptionRender = NonNullable<
  SelectProps<CountryCodeSelectValue, CountryOption>["optionRender"]
>;

const renderCountryOption: CountryOptionRender = (option) => {
  const country = option.data.country;
  if (!country) return option.label;
  return (
    <Space style={{ justifyContent: "space-between", width: "100%" }}>
      <Space>
        <span aria-hidden="true">{getCountryFlag(country.code)}</span>
        <span>{country.name}</span>
      </Space>
      <span>+{country.phone}</span>
    </Space>
  );
};

export const CountryCodeSelect: FC<CountryCodeSelectProps> = ({
  onChange,
  style,
  disabled,
  initValue,
  mode,
  countryCodes,
  hasDefault,
  value: controlledValue,
  id,
  ariaLabel,
  allowClear,
  placeholder,
}) => {
  const { i18n } = useTranslation();
  const language = i18n.resolvedLanguage ?? i18n.language;
  const initialCountry = countryCodes?.[0];
  const initialValueKey = Array.isArray(initValue) ? initValue.join(",") : initValue;
  const isControlled = controlledValue !== undefined;

  const countryData = useMemo(
    () => getCountryCodeData(countryCodes, language),
    [countryCodes, language],
  );

  const defaultKey = `${mode ?? "single"}:${initialValueKey !== undefined ? `init:${initialValueKey}` : `country:${initialCountry ?? ""}`}`;
  const defaultValue: CountryCodeSelectValue =
    initialValueKey !== undefined
      ? mode
        ? initialValueKey.split(",").filter(Boolean)
        : initialValueKey
      : initialCountry
        ? mode
          ? [initialCountry]
          : initialCountry
        : mode
          ? []
          : "";
  const [uncontrolled, setUncontrolled] = useState(() => ({
    defaultKey,
    value: defaultValue,
  }));
  const internalValue = uncontrolled.defaultKey === defaultKey ? uncontrolled.value : defaultValue;

  const handleChange = useCallback(
    (nextValue: CountryCodeSelectValue | undefined) => {
      const normalizedValue = mode
        ? Array.isArray(nextValue)
          ? nextValue
          : []
        : typeof nextValue === "string"
          ? nextValue
          : "";
      if (!isControlled) setUncontrolled({ defaultKey, value: normalizedValue });
      const notifyChange = onChange as ((value: CountryCodeSelectValue) => void) | undefined;
      notifyChange?.(normalizedValue);
    },
    [defaultKey, isControlled, mode, onChange],
  );

  const filterOption = useCallback(
    (input: string, option?: { text?: string }) =>
      (option?.text ?? "").toLowerCase().includes(input.toLowerCase()),
    [],
  );

  return (
    <Select
      showSearch
      id={id}
      aria-label={ariaLabel}
      style={style}
      disabled={disabled}
      value={controlledValue !== undefined ? controlledValue : internalValue}
      mode={mode}
      allowClear={allowClear}
      placeholder={placeholder}
      onChange={handleChange}
      filterOption={filterOption}
      optionLabelProp="label"
      options={[
        ...(hasDefault ? [{ value: "All", label: "All", text: "All" }] : []),
        ...countryData.map((country) => ({
          value: country.code,
          // The selected label shows the country name next to its calling code;
          // `+1` alone cannot distinguish the 25 countries that share it.
          label: `${country.name} (+${country.phone})`,
          text: `${country.name}, ${country.code}, ${country.phone}`,
          country,
        })),
      ]}
      optionRender={renderCountryOption}
    />
  );
};
