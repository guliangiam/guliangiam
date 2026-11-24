import { useTranslation } from "react-i18next";
import { Select, Skeleton } from "antd";
import { useState } from "react";
import { loadLanguageResources } from "@/shared/i18n";
import { toLanguageOptions } from "@/shared/constants/locale-options";

interface LanguageSwitcherProps {
  className?: string;
  style?: React.CSSProperties;
}

export function LanguageSwitcher({ className, style }: LanguageSwitcherProps) {
  const { i18n, t } = useTranslation();
  const [loading, setLoading] = useState(false);

  const handleChange = async (value: string) => {
    setLoading(true);
    try {
      await loadLanguageResources(value);
      i18n.changeLanguage(value);
    } finally {
      setLoading(false);
    }
  };

  const options = toLanguageOptions(t);

  return (
    <Select
      value={i18n.language}
      onChange={handleChange}
      options={options}
      className={className}
      style={{ width: 140, ...style }}
      size="small"
      suffixIcon={
        loading ? <Skeleton.Input active size="small" style={{ width: 16 }} /> : undefined
      }
      disabled={loading}
    />
  );
}
