/**
 * 语言 / 国家选项的唯一前端事实源。
 *
 * 语言列表与 `src/shared/i18n/index.ts` 的语言包一一对应；LanguageSwitcher、
 * 组织设置与系统设置的语言选择都从这里取值，避免各页面各自维护一份副本。
 */

export type LocaleOption = {
  /** BCP-47 语言码，同时是语言包目录名与 organization_settings.languages 的取值 */
  value: string;
  /** 母语名称（不翻译，便于用户识别） */
  nativeName: string;
};

export const LANGUAGE_OPTIONS: readonly LocaleOption[] = [
  { value: "zh-CN", nativeName: "简体中文" },
  { value: "zh-TW", nativeName: "繁體中文" },
  { value: "en", nativeName: "English" },
  { value: "ja", nativeName: "日本語" },
  { value: "ko", nativeName: "한국어" },
  { value: "de", nativeName: "Deutsch" },
  { value: "fr", nativeName: "Français" },
  { value: "es", nativeName: "Español" },
  { value: "pt", nativeName: "Português" },
  { value: "vi", nativeName: "Tiếng Việt" },
  { value: "pl", nativeName: "Polski" },
  { value: "tr", nativeName: "Türkçe" },
  { value: "uk", nativeName: "Українська" },
  { value: "ru", nativeName: "Русский" },
] as const;

export const LANGUAGE_CODES = LANGUAGE_OPTIONS.map((option) => option.value);

/** 新组织默认启用的语言（与种子数据一致）。 */
export const DEFAULT_LANGUAGE_CODES = ["zh-CN", "en"];

/** 新组织默认启用的国家/地区（ISO 3166-1 alpha-2）。 */
export const DEFAULT_COUNTRY_CODES = ["CN", "US", "GB", "SG"];

export type LanguageSelectOption = { value: string; label: string };

/**
 * 生成语言下拉选项。zh-CN / zh-TW / ja 的显示名走语言包（随当前界面语种变化），
 * 其余语言用母语名称（与 Casdoor 一致：语言名不翻译）。
 */
export function toLanguageOptions(translate: (key: string) => string): LanguageSelectOption[] {
  const i18nKeys: Record<string, string> = {
    "zh-CN": "common:lang.zhCN",
    "zh-TW": "common:lang.zhTW",
    ja: "common:lang.ja",
  };

  return LANGUAGE_OPTIONS.map((option) => {
    const key = i18nKeys[option.value];
    const translated = key ? translate(key) : "";
    return {
      value: option.value,
      label: translated && translated !== key ? translated : option.nativeName,
    };
  });
}
