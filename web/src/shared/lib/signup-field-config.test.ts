import { describe, it, expect, vi } from "vitest";
import type { TFunction } from "i18next";
import { getDefaultSignupItems, getSignupItems, getFieldConfig } from "./signup-field-config";
import enCommon from "@/shared/i18n/locales/en/common.json";
import enAuth from "@/shared/i18n/locales/en/auth.json";
import enResource from "@/shared/i18n/locales/en/resource.json";

describe("getSignupItems", () => {
  it("returns platform defaults when nothing is configured", () => {
    expect(getSignupItems({ signupItems: [] })).toEqual(getDefaultSignupItems());
    expect(getSignupItems({ signupItems: null })).toEqual(getDefaultSignupItems());
    expect(getSignupItems({})).toEqual(getDefaultSignupItems());
  });

  it("overrides defaults by name and keeps default order", () => {
    const merged = getSignupItems({
      signupItems: [{ name: "Email", visible: true, required: true, prompted: false }],
    });
    const email = merged.find((item) => item.name === "Email");
    expect(email?.required).toBe(true);
    expect(merged.map((item) => item.name)).toEqual(
      getDefaultSignupItems().map((item) => item.name),
    );
  });

  it("appends custom-only items after the defaults", () => {
    const merged = getSignupItems({
      signupItems: [{ name: "Bio", visible: true, required: false, prompted: false }],
    });
    expect(merged[merged.length - 1].name).toBe("Bio");
    expect(merged.length).toBe(getDefaultSignupItems().length + 1);
  });

  it("ignores unnamed rows instead of crashing", () => {
    const merged = getSignupItems({
      signupItems: [{ name: "", visible: true, required: false, prompted: false }],
    });
    expect(merged).toEqual(getDefaultSignupItems());
  });
});

const identityT = ((key: string) => key) as unknown as TFunction;
const bundles: Record<string, unknown> = {
  common: enCommon,
  auth: enAuth,
  resource: enResource,
};
function resolveKey(fullyQualified: string): unknown {
  const [ns, ...path] = fullyQualified.split(":");
  let node: unknown = bundles[ns];
  for (const seg of path.join(":").split(".")) {
    node = (node as Record<string, unknown>)?.[seg];
  }
  return node;
}

describe("getFieldConfig default label/placeholder localization", () => {
  const extendedNames = [
    "Display name",
    "First name",
    "Last name",
    "Country code",
    "Region",
    "Location",
    "Affiliation",
    "Title",
    "Homepage",
    "Address",
    "Bio",
    "Gender",
    "Birthday",
    "Education",
    "Real name",
    "ID card",
  ];

  it("maps every known field to an existing fully-qualified key", () => {
    for (const name of [
      ...extendedNames,
      "Username",
      "Email",
      "Phone",
      "Password",
      "Confirm password",
    ]) {
      const cfg = getFieldConfig(
        { name, visible: true, required: false, prompted: false },
        identityT,
      );
      expect(cfg.label, `${name} label`).toMatch(/^(common|auth|resource):/);
      expect(typeof resolveKey(cfg.label), `${name} label key ${cfg.label}`).toBe("string");
    }
  });

  it("maps every known field placeholder to an existing fully-qualified key", () => {
    const withKeyedPlaceholder = extendedNames.filter(
      (name) => name !== "Homepage" && name !== "Birthday",
    );
    for (const name of [
      ...withKeyedPlaceholder,
      "Username",
      "Email",
      "Phone",
      "Password",
      "Confirm password",
    ]) {
      const cfg = getFieldConfig(
        { name, visible: true, required: false, prompted: false },
        identityT,
      );
      expect(cfg.placeholder, `${name} placeholder`).toMatch(/^(common|auth|resource):/);
      expect(typeof resolveKey(cfg.placeholder), `${name} placeholder key ${cfg.placeholder}`).toBe(
        "string",
      );
    }
  });

  it("uses literal format hints for homepage and birthday placeholders", () => {
    const homepage = getFieldConfig(
      { name: "Homepage", visible: true, required: false, prompted: false },
      identityT,
    );
    const birthday = getFieldConfig(
      { name: "Birthday", visible: true, required: false, prompted: false },
      identityT,
    );
    expect(homepage.placeholder).toBe("https://example.com");
    expect(birthday.placeholder).toBe("YYYY-MM-DD");
  });

  it("never queries phantom auth.field/auth.placeholder keys for custom names", () => {
    const spy = vi.fn<(key: string) => string>((key) => key);
    const cfg = getFieldConfig(
      { name: "Favorite color", visible: true, required: false, prompted: false },
      spy as unknown as TFunction,
    );
    expect(cfg.label).toBe("Favorite color");
    expect(cfg.placeholder).toBe("Enter your favorite color");
    for (const call of spy.mock.calls) {
      expect(String(call[0])).not.toMatch(/^auth\.(field|placeholder)\./);
    }
  });
});
