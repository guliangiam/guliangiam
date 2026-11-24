/**
 * Centralized currency options and utilities.
 *
 * All currency option definitions across the app should reference
 * CURRENCY_OPTIONS or useCurrencyOptions() instead of inline arrays.
 */

import { useTranslation } from "react-i18next";

/** Supported currency codes — single source of truth */
export const CURRENCY_CODES = [
  "CNY",
  "USD",
  "EUR",
  "GBP",
  "JPY",
  "KRW",
  "HKD",
  "TWD",
  "SGD",
  "AUD",
  "CAD",
  "CHF",
] as const;

export type CurrencyCode = (typeof CURRENCY_CODES)[number];

/** Static currency options with symbol labels (for non-i18n contexts) */
export const CURRENCY_OPTIONS = CURRENCY_CODES.map((code) => {
  const symbolMap: Record<CurrencyCode, string> = {
    CNY: "¥",
    USD: "$",
    EUR: "€",
    GBP: "£",
    JPY: "¥",
    KRW: "₩",
    HKD: "HK$",
    TWD: "NT$",
    SGD: "S$",
    AUD: "A$",
    CAD: "C$",
    CHF: "Fr",
  };
  return { label: `${code} (${symbolMap[code]})`, value: code };
}) as { label: string; value: CurrencyCode }[];

/** Payment provider currency subset (common payment currencies) */
export const PAYMENT_CURRENCY_CODES: readonly CurrencyCode[] = [
  "USD",
  "CNY",
  "EUR",
  "GBP",
  "JPY",
] as const;

/** Payment provider currency options (for PaymentProviderFields) */
export const PAYMENT_CURRENCY_OPTIONS = PAYMENT_CURRENCY_CODES.map((code) => ({
  label: code,
  value: code,
})) as { label: string; value: CurrencyCode }[];

/** Organization finance currency subset */
export const ORG_CURRENCY_CODES: readonly CurrencyCode[] = [
  "USD",
  "CNY",
  "EUR",
  "GBP",
  "JPY",
] as const;

/** Organization default currency subset (limited) */
export const ORG_DEFAULT_CURRENCY_CODES: readonly CurrencyCode[] = ["USD", "CNY", "EUR"] as const;

/** Balance currency codes (used in user account) */
export const BALANCE_CURRENCY_CODES: readonly CurrencyCode[] = [
  "USD",
  "CNY",
  "EUR",
  "GBP",
  "JPY",
  "KRW",
  "HKD",
  "AUD",
  "CAD",
  "SGD",
] as const;

/**
 * i18n-aware currency options hook.
 * Uses `currency.<CODE>` translation keys from the "common" namespace.
 */
export function useCurrencyOptions(codes: readonly CurrencyCode[] = CURRENCY_CODES) {
  const { t } = useTranslation("common");
  return codes.map((code) => ({
    label: t(`currency.${code}`),
    value: code,
  }));
}

/**
 * Factory for i18n-aware currency options (for use outside React hooks).
 * Accepts a translation function directly.
 */
export function getCurrencyOptions(
  t: (key: string) => string,
  codes: readonly CurrencyCode[] = CURRENCY_CODES,
) {
  return codes.map((code) => ({
    // 调用方可能传入绑定其他命名空间的 t（如 resource），显式 common: 前缀
    // 保证币种文案始终命中 common 命名空间，而不是回显原始 key。
    label: t(`common:currency.${code}`),
    value: code,
  }));
}
