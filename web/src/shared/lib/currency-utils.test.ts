import { describe, it, expect } from "vitest";

// Currency formatting utilities
const supportedCurrencies = new Set([
  "USD",
  "EUR",
  "GBP",
  "CNY",
  "JPY",
  "KRW",
  "INR",
  "AUD",
  "CAD",
  "CHF",
  "HKD",
  "SGD",
  "TWD",
  "THB",
  "RUB",
  "BRL",
  "MXN",
  "PLN",
  "TRY",
  "ZAR",
  "AED",
]);

function isValidCurrency(code: string): boolean {
  return supportedCurrencies.has(code.toUpperCase());
}

function formatPrice(price: number, currency: string): string {
  if (price < 0) return "N/A";
  if (!isValidCurrency(currency)) return `${price.toFixed(2)}`;
  // JPY, KRW have no decimal places
  const noDecimal = new Set(["JPY", "KRW"]);
  if (noDecimal.has(currency.toUpperCase())) {
    return `${Math.round(price)}`;
  }
  return `${price.toFixed(2)}`;
}

describe("isValidCurrency", () => {
  it("accepts supported currencies", () => {
    expect(isValidCurrency("USD")).toBe(true);
    expect(isValidCurrency("EUR")).toBe(true);
    expect(isValidCurrency("CNY")).toBe(true);
    expect(isValidCurrency("JPY")).toBe(true);
  });

  it("rejects unsupported currencies", () => {
    expect(isValidCurrency("XYZ")).toBe(false);
    expect(isValidCurrency("")).toBe(false);
  });

  it("is case insensitive", () => {
    expect(isValidCurrency("usd")).toBe(true);
    expect(isValidCurrency("Usd")).toBe(true);
  });
});

describe("formatPrice", () => {
  it("formats USD with 2 decimal places", () => {
    expect(formatPrice(99.99, "USD")).toBe("99.99");
    expect(formatPrice(100, "USD")).toBe("100.00");
  });

  it("formats JPY without decimal places", () => {
    expect(formatPrice(1000, "JPY")).toBe("1000");
    expect(formatPrice(999.5, "JPY")).toBe("1000");
  });

  it("formats KRW without decimal places", () => {
    expect(formatPrice(50000, "KRW")).toBe("50000");
  });

  it("returns N/A for negative prices", () => {
    expect(formatPrice(-1, "USD")).toBe("N/A");
  });

  it("formats with decimals for unknown currency", () => {
    expect(formatPrice(50, "XYZ")).toBe("50.00");
  });
});
