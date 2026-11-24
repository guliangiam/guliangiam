import { describe, expect, it } from "vitest";
import type { OrganizationDetail } from "@/shared/api/iam/organization";
import type { OrganizationFormValues } from "./tabs/types";
import {
  buildOrganizationFormValues,
  buildOrganizationSecurityPolicyPayload,
  buildOrganizationThemePayload,
  buildOrganizationUpdatePayload,
} from "./organization-payload";

/**
 * 凭据字段是 write-only：读接口只回 `*Set` / `*Masked`。前端必须满足三件事：
 *   - 表单不预填任何凭据（masked 值只用于展示）；
 *   - 未填写/空串的凭据不进入 PATCH（省略 key = 后端 nil = 保持不变）；
 *   - masked 值永远不会被当作明文回写。
 */
const CREDENTIAL_KEYS = [
  "passwordSalt",
  "passwordObfuscatorKey",
  "defaultPassword",
  "masterPassword",
  "masterVerificationCode",
  "kerberosKeytab",
] as const;

/** Detail 的必填字段很多且与断言无关，故只铺满被测分支所需的字段。 */
function detailWithMaskedCredentials(): OrganizationDetail {
  return {
    settings: {},
    theme: {},
    securityPolicySummary: {},
    passwordSaltSet: true,
    passwordSaltMasked: "sa***lt",
    passwordObfuscatorKeySet: true,
    passwordObfuscatorKeyMasked: "ob***ey",
    defaultPasswordSet: true,
    defaultPasswordMasked: "de***rd",
    masterPasswordSet: true,
    masterPasswordMasked: "ma***rd",
    masterVerificationCodeSet: true,
    masterVerificationCodeMasked: "ve***de",
    kerberosKeytabSet: true,
    kerberosKeytabMasked: "ke***bt",
  } as OrganizationDetail;
}

const baseValues: OrganizationFormValues = { slug: "acme" };

describe("buildOrganizationFormValues credential inputs", () => {
  it("starts every credential input empty instead of prefilling the masked value", () => {
    const values = buildOrganizationFormValues(detailWithMaskedCredentials());

    for (const key of CREDENTIAL_KEYS) {
      expect(values[key]).toBe("");
    }
    expect(JSON.stringify(values)).not.toContain("***");
  });
});

describe("password policy field mapping", () => {
  it("reads passwordMaxAgeDays into the form's passwordExpireDays", () => {
    const detail = {
      ...detailWithMaskedCredentials(),
      securityPolicySummary: { passwordPolicy: { passwordMaxAgeDays: 33 } },
    } as OrganizationDetail;

    const values = buildOrganizationFormValues(detail);

    expect(values.passwordExpireDays).toBe(33);
  });

  it("writes passwordMaxAgeDays and never emits the legacy expireDays key", () => {
    const payload = buildOrganizationSecurityPolicyPayload({
      ...baseValues,
      passwordExpireDays: 33,
    });

    expect(payload.passwordPolicy).toHaveProperty("passwordMaxAgeDays", 33);
    expect(payload.passwordPolicy).not.toHaveProperty("expireDays");
  });

  it("maps every standard policy key into the form and keeps them out of the extra JSON", () => {
    const detail = {
      ...detailWithMaskedCredentials(),
      securityPolicySummary: {
        passwordPolicy: {
          minLength: 10,
          maxLength: 64,
          requireUppercase: false,
          requireLowercase: true,
          requireDigit: false,
          requireSpecial: true,
          rejectCommonPasswords: false,
          rejectUsername: true,
          passwordHistoryCount: 7,
          passwordMaxAgeDays: 90,
          customRule: "keep-me",
        },
      },
    } as OrganizationDetail;

    const values = buildOrganizationFormValues(detail);

    expect(values).toMatchObject({
      passwordMinLength: 10,
      passwordMaxLength: 64,
      passwordRequireUppercase: false,
      passwordRequireLowercase: true,
      passwordRequireDigit: false,
      passwordRequireSpecial: true,
      passwordRejectCommonPasswords: false,
      passwordRejectUsername: true,
      passwordHistoryCount: 7,
      passwordExpireDays: 90,
    });
    // 扩展 JSON 只承载非标准键。
    expect(values.passwordPolicyExtraJson).toContain("customRule");
    expect(values.passwordPolicyExtraJson).not.toContain("minLength");
    expect(values.passwordPolicyExtraJson).not.toContain("requireUppercase");
  });

  it("falls back to the spec defaults when the organization has no policy", () => {
    const values = buildOrganizationFormValues(detailWithMaskedCredentials());

    expect(values).toMatchObject({
      passwordMinLength: 8,
      passwordMaxLength: 128,
      passwordRequireUppercase: true,
      passwordRequireLowercase: true,
      passwordRequireDigit: true,
      passwordRequireSpecial: true,
      passwordRejectCommonPasswords: true,
      passwordRejectUsername: true,
      passwordHistoryCount: 5,
      passwordExpireDays: 0,
    });
  });

  it("submits all ten standard keys and strips standard keys hand-written into the extra JSON", () => {
    const payload = buildOrganizationSecurityPolicyPayload({
      ...baseValues,
      passwordMinLength: 12,
      passwordMaxLength: 96,
      passwordRequireUppercase: false,
      passwordRequireLowercase: true,
      passwordRequireDigit: false,
      passwordRequireSpecial: true,
      passwordRejectCommonPasswords: false,
      passwordRejectUsername: true,
      passwordHistoryCount: 9,
      passwordExpireDays: 45,
      // 手工在扩展区重复配置标准键：表单值必须赢，且扩展区不残留。
      passwordPolicyExtraJson: '{"minLength":4,"customRule":"keep-me"}',
    });

    expect(payload.passwordPolicy).toMatchObject({
      minLength: 12,
      maxLength: 96,
      requireUppercase: false,
      requireLowercase: true,
      requireDigit: false,
      requireSpecial: true,
      rejectCommonPasswords: false,
      rejectUsername: true,
      passwordHistoryCount: 9,
      passwordMaxAgeDays: 45,
      customRule: "keep-me",
    });
  });
});

describe("theme name default for a newly created organization", () => {
  it("prefills the form theme name with the default when the detail value is blank", () => {
    const detail = {
      ...detailWithMaskedCredentials(),
      theme: { themeName: "" },
    } as OrganizationDetail;

    expect(buildOrganizationFormValues(detail).themeName).toBe("default");
  });

  it("submits the default theme name so saving the detail page is not blocked", () => {
    const detail = {
      ...detailWithMaskedCredentials(),
      theme: { themeName: "   " },
    } as OrganizationDetail;

    const values = buildOrganizationFormValues(detail);

    expect(buildOrganizationThemePayload(values).themeName).toBe("default");
  });

  it("keeps an existing theme name untouched", () => {
    const detail = {
      ...detailWithMaskedCredentials(),
      theme: { themeName: "enterprise" },
    } as OrganizationDetail;

    const values = buildOrganizationFormValues(detail);

    expect(buildOrganizationThemePayload(values).themeName).toBe("enterprise");
  });
});

describe("buildOrganizationUpdatePayload credential submission", () => {
  it("omits every credential key when the inputs are untouched", () => {
    const payload = buildOrganizationUpdatePayload(baseValues, 7);

    for (const key of CREDENTIAL_KEYS) {
      expect(payload).not.toHaveProperty(key);
    }
    // The rest of the form is still submitted in full, so an org save keeps working.
    expect(payload.revision).toBe(7);
    expect(payload.displayName).toBe("");
  });

  it("submits exactly the credentials the operator filled in", () => {
    const payload = buildOrganizationUpdatePayload(
      {
        ...baseValues,
        passwordSalt: "operator-salt",
        masterPassword: "operator-master-secret",
      },
      7,
    );

    expect(payload.passwordSalt).toBe("operator-salt");
    expect(payload.masterPassword).toBe("operator-master-secret");
    expect(payload).not.toHaveProperty("passwordObfuscatorKey");
    expect(payload).not.toHaveProperty("defaultPassword");
    expect(payload).not.toHaveProperty("masterVerificationCode");
    expect(payload).not.toHaveProperty("kerberosKeytab");
  });

  it("submits all six credential columns when each one is filled in", () => {
    const payload = buildOrganizationUpdatePayload(
      {
        ...baseValues,
        passwordSalt: "salt-value",
        passwordObfuscatorKey: "obfuscator-key-value",
        defaultPassword: "default-password-value",
        masterPassword: "master-password-value",
        masterVerificationCode: "verification-code-value",
        kerberosKeytab: "keytab-value",
      },
      7,
    );

    expect(payload.passwordSalt).toBe("salt-value");
    expect(payload.passwordObfuscatorKey).toBe("obfuscator-key-value");
    expect(payload.defaultPassword).toBe("default-password-value");
    expect(payload.masterPassword).toBe("master-password-value");
    expect(payload.masterVerificationCode).toBe("verification-code-value");
    expect(payload.kerberosKeytab).toBe("keytab-value");
  });

  it("sends neither empty strings nor masked read values", () => {
    // The pre-fix form was prefilled from the read response; if that ever comes
    // back, `writeOnlyCredential` must still keep the value out of the PATCH.
    const payload = buildOrganizationUpdatePayload(
      buildOrganizationFormValues(detailWithMaskedCredentials()),
      7,
    );
    const serialized = JSON.stringify(payload);

    for (const key of CREDENTIAL_KEYS) {
      expect(payload).not.toHaveProperty(key);
    }
    expect(serialized).not.toContain("***");
    expect(serialized).not.toContain("Masked");
  });
});
