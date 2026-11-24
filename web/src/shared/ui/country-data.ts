import {
  getCountries,
  getCountryCallingCode,
  isSupportedCountry,
  type CountryCode,
} from "libphonenumber-js";
import i18nIsoCountries from "i18n-iso-countries";
import enLocale from "i18n-iso-countries/langs/en.json";
import zhLocale from "i18n-iso-countries/langs/zh.json";

i18nIsoCountries.registerLocale(enLocale);
i18nIsoCountries.registerLocale(zhLocale);

export type CountryData = {
  code: string;
  name: string;
  phone: string;
};

const ISO_COUNTRY_PATTERN = /^[A-Z]{2}$/;
const E164_PATTERN = /^\+[1-9]\d{3,14}$/;
const LOCAL_NUMBER_PATTERN = /^\d{4,15}$/;
const DIAL_CODE_PATTERN = /^\+?[1-9]\d{0,2}$/;
const NUMBER_SEPARATORS = /[\s\-()]/g;

function nameLocale(language: string) {
  return language.toLowerCase().startsWith("zh") ? "zh" : "en";
}

/**
 * `countryCode` 在 API/DB 中只表示 ISO 3166-1 alpha-2（2 位大写字母）。
 * 非法值返回空串，由调用方给出可操作校验错误，不做静默映射。
 */
export function normalizeIsoCountryCode(value: string | null | undefined): string {
  const trimmed = (value ?? "").trim().toUpperCase();
  if (!ISO_COUNTRY_PATTERN.test(trimmed)) return "";
  return isSupportedCountry(trimmed as CountryCode) ? trimmed : "";
}

export function isIsoCountryCode(value: string | null | undefined): boolean {
  return normalizeIsoCountryCode(value) !== "";
}

/** 国家名称是数据，不是 i18n key；沿用现有 en/zh 数据源。 */
export function getCountryName(countryCode: string, language: string): string {
  const code = normalizeIsoCountryCode(countryCode);
  if (!code) return "";
  return i18nIsoCountries.getName(code, nameLocale(language)) ?? "";
}

/**
 * 拨号前缀（E.164 calling code，含 `+`）。ISO 码无法确定唯一前缀之外的
 * 任何号码语义，因此这里只接受国家选择器产出的 ISO 值。
 */
export function toDialCode(countryCode: string): string {
  const code = normalizeIsoCountryCode(countryCode);
  if (!code) return "";
  return `+${getCountryCallingCode(code as CountryCode)}`;
}

/**
 * 兼容旧数据的拨号前缀（`86` / `+86`）。仅用于迁移期把旧 profile 值拼成
 * E.164；新写入的 `countryCode` 必须是 ISO。
 */
function toLegacyDialCode(value: string): string {
  const trimmed = (value ?? "").trim();
  if (!DIAL_CODE_PATTERN.test(trimmed)) return "";
  return `+${trimmed.replace(/^\+/, "")}`;
}

/**
 * 把本地号码与 ISO 国家码（或旧版拨号前缀）合成为 E.164。
 *
 * - 号码已是 E.164：原样返回（即使同时选了国家）。
 * - 号码是本地号且给了可用前缀：补 `+` 与区号。
 * - 只有 ISO 国家码、号码却是本地号：返回空串（ISO 不是区号）。
 */
export function toE164(countryCode: string, phone: string): string {
  const trimmed = (phone ?? "").trim();
  if (!trimmed) return "";

  const normalized = trimmed.replace(NUMBER_SEPARATORS, "");
  if (normalized.startsWith("+")) {
    return E164_PATTERN.test(normalized) ? normalized : "";
  }

  const dialCode = toDialCode(countryCode) || toLegacyDialCode(countryCode);
  if (!dialCode || !LOCAL_NUMBER_PATTERN.test(normalized)) return "";

  return `${dialCode}${normalized}`;
}

export function getCountryCodeData(countryCodes: string[] | undefined, language: string) {
  const codes = countryCodes ?? getCountries();

  return codes
    .map((code) => normalizeIsoCountryCode(code))
    .filter((code): code is string => code !== "")
    .map((code) => ({
      code,
      name: getCountryName(code, language),
      phone: getCountryCallingCode(code as CountryCode),
    }))
    .filter((item) => item.name !== "")
    .sort((a, b) => Number(a.phone) - Number(b.phone));
}

export function getCountryFlag(countryCode: string) {
  if (!/^[A-Z]{2}$/.test(countryCode)) return "";
  return String.fromCodePoint(
    ...Array.from(countryCode, (letter) => 127397 + letter.charCodeAt(0)),
  );
}
