import { describe, expect, it } from "vitest";
import { buildLdapPatchPayload, type LdapPatchFormValues } from "./ldap-patch-payload";

const baseValues: LdapPatchFormValues = {
  displayName: "Directory",
  serverName: "ldap-1",
  type: "OpenLDAP",
  serverHost: "ldap.example.com",
  port: 389,
  enableSsl: false,
  allowSelfSignedCert: false,
  baseDn: "dc=example,dc=com",
  filter: "",
  filterFields: ["uid", "mail"],
  admin: "cn=admin,dc=example,dc=com",
  password: "",
  passwordType: "Plain",
  attributeName: "uid",
  defaultGroups: [],
  enableGroups: false,
  customAttributes: "",
  autoSync: false,
  syncInterval: 60,
  isEnabled: true,
};

describe("buildLdapPatchPayload", () => {
  it("omits bindPassword when the field is blank so the stored credential survives", () => {
    const payload = buildLdapPatchPayload({ ...baseValues, password: "" }, 7);
    expect(payload).not.toHaveProperty("bindPassword");
    expect(payload.revision).toBe(7);
  });

  it("treats a whitespace-only password as blank", () => {
    const payload = buildLdapPatchPayload({ ...baseValues, password: "   " }, 7);
    expect(payload).not.toHaveProperty("bindPassword");
  });

  it("sends a replacement bindPassword only when one is typed", () => {
    const payload = buildLdapPatchPayload({ ...baseValues, password: "new-secret" }, 7);
    expect(payload.bindPassword).toBe("new-secret");
  });

  it("parses customAttributes and drops invalid or non-object JSON", () => {
    const parsed = buildLdapPatchPayload(
      { ...baseValues, customAttributes: '{"ldapAttr":"guliangiamField"}' },
      7,
    );
    expect(parsed.customAttributes).toEqual({ ldapAttr: "guliangiamField" });

    const invalid = buildLdapPatchPayload({ ...baseValues, customAttributes: "{bad" }, 7);
    expect(invalid.customAttributes).toBeUndefined();

    const array = buildLdapPatchPayload({ ...baseValues, customAttributes: "[]" }, 7);
    expect(array.customAttributes).toBeUndefined();
  });
});
