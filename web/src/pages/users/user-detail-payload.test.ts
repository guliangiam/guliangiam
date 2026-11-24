import { describe, expect, it } from "vitest";
import type { UserDetail } from "@/shared/api/iam/user";
import type { CustomPropertyRow, ProfileFormValues } from "./tabs/types";
import {
  MAX_CUSTOM_PROPERTIES,
  RESERVED_PROFILE_PROPERTY_KEYS,
  buildCredentialsPayload,
  buildCustomProperties,
  buildIdentities,
  buildProfileFormValues,
  buildUserFormValues,
  buildUserSettingsPayload,
  buildUserUpdatePayload,
  deriveAccountStatus,
  formatPropertyValue,
  validateCustomProperties,
} from "./user-detail-payload";

function buildUser(overrides: Record<string, unknown> = {}): UserDetail {
  return {
    id: "u-1",
    organizationId: "built-in",
    username: "alice",
    tag: "staff",
    userType: "normal",
    status: "active",
    isAdmin: false,
    isForbidden: false,
    isDeleted: false,
    score: 0,
    karma: 12,
    ranking: 5,
    balance: 0,
    balanceCredit: 0,
    balanceCurrency: "CNY",
    ipWhitelist: "",
    language: "en",
    externalId: "ext-1",
    signupApplicationId: "app-1",
    custom1: "",
    custom2: "",
    custom3: "",
    custom4: "",
    custom5: "",
    custom6: "",
    custom7: "",
    custom8: "",
    custom9: "",
    custom10: "",
    revision: 3,
    profile: {
      displayName: "Alice",
      email: "alice@example.com",
      phone: "+8613800000000",
      homepage: "https://alice.example.com",
      language: "en",
      properties: { department: "Engineering", karma: 12, ranking: 5 },
    },
    credentialSummary: { needUpdatePassword: false, lastChangePasswordTime: "2026-02-01" },
    identities: [],
    ...overrides,
  } as unknown as UserDetail;
}

describe("buildUserFormValues / buildProfileFormValues", () => {
  it("falls back to websiteUrl for homepage and to the account language", () => {
    const user = buildUser({
      language: "ja",
      profile: {
        displayName: "Alice",
        websiteUrl: "https://site.example.com",
        username: "alice",
      },
      credentialSummary: { needUpdatePassword: true },
    });

    const values = buildUserFormValues(user);
    expect(values).toMatchObject({
      owner: "built-in",
      name: "alice",
      displayName: "Alice",
      homepage: "https://site.example.com",
      language: "ja",
      karma: 12,
      ranking: 5,
      needUpdatePassword: true,
    });

    const profile = buildProfileFormValues(user);
    expect(profile).toMatchObject({
      displayName: "Alice",
      homepage: "https://site.example.com",
      language: "ja",
      tag: "staff",
    });
  });
});

describe("buildUserUpdatePayload", () => {
  it("returns only the revision when the form still matches the loaded user", () => {
    const user = buildUser();
    expect(buildUserUpdatePayload(buildUserFormValues(user), user)).toStrictEqual({ revision: 3 });
  });

  it("submits only the fields that changed", () => {
    const user = buildUser();
    const values = {
      ...buildUserFormValues(user),
      externalId: "ext-2",
      tag: "contractor",
      isAdmin: true,
      karma: 20,
      ranking: 2,
      custom1: "blue",
    };

    expect(buildUserUpdatePayload(values, user)).toStrictEqual({
      externalId: "ext-2",
      tag: "contractor",
      isAdmin: true,
      karma: 20,
      ranking: 2,
      custom1: "blue",
      revision: 3,
    });
  });

  it("omits numeric fields left undefined", () => {
    const user = buildUser();
    const values = buildUserFormValues(user);
    delete values.karma;
    delete values.ranking;

    const payload = buildUserUpdatePayload(values, user);
    expect(payload).not.toHaveProperty("karma");
    expect(payload).not.toHaveProperty("ranking");
  });

  it("clears optional strings by sending an explicit empty string", () => {
    const user = buildUser();
    const payload = buildUserUpdatePayload(
      { ...buildUserFormValues(user), externalId: undefined, tag: undefined },
      user,
    );

    expect(payload).toStrictEqual({ externalId: "", tag: "", revision: 3 });
  });

  it("omits optional string keys that were already empty and untouched", () => {
    const user = buildUser({ externalId: "", tag: "" });
    const payload = buildUserUpdatePayload(
      { ...buildUserFormValues(user), externalId: undefined, tag: undefined },
      user,
    );

    expect(Object.hasOwn(payload, "externalId")).toBe(false);
    expect(Object.hasOwn(payload, "tag")).toBe(false);
    expect(payload).toStrictEqual({ revision: 3 });
  });

  it("re-sends language when only the account-level language is set", () => {
    const user = buildUser({ language: "ja", profile: { displayName: "Alice" } });
    const payload = buildUserUpdatePayload(buildUserFormValues(user), user);

    expect(payload.language).toBe("ja");
  });
});

describe("account status model", () => {
  it("derives one state from legacy field combinations", () => {
    expect(deriveAccountStatus(buildUser())).toBe("active");
    expect(deriveAccountStatus(buildUser({ isForbidden: true }))).toBe("disabled");
    expect(deriveAccountStatus(buildUser({ status: "locked" }))).toBe("disabled");
    expect(
      deriveAccountStatus(buildUser({ status: "disabled", isForbidden: true, isDeleted: true })),
    ).toBe("deleted");
  });

  it("maps disabled/deleted to mutually consistent payload fields", () => {
    const user = buildUser();
    const base = buildUserFormValues(user);

    expect(buildUserUpdatePayload({ ...base, accountStatus: "disabled" }, user)).toStrictEqual({
      status: "disabled",
      isForbidden: true,
      revision: 3,
    });
    expect(buildUserUpdatePayload({ ...base, accountStatus: "deleted" }, user)).toStrictEqual({
      status: "disabled",
      isForbidden: true,
      isDeleted: true,
      revision: 3,
    });
  });

  it("reverses back to active with a consistent field set", () => {
    const user = buildUser({ status: "disabled", isForbidden: true, isDeleted: true });

    expect(
      buildUserUpdatePayload({ ...buildUserFormValues(user), accountStatus: "active" }, user),
    ).toStrictEqual({
      status: "active",
      isForbidden: false,
      isDeleted: false,
      revision: 3,
    });
  });

  it("does not rewrite legacy states when the status control is untouched", () => {
    const user = buildUser({ status: "locked" });

    expect(buildUserUpdatePayload(buildUserFormValues(user), user)).toStrictEqual({ revision: 3 });
  });
});

describe("buildUserSettingsPayload", () => {
  it("omits undefined profile keys and keeps empty strings that clear a field", () => {
    const payload = buildUserSettingsPayload(
      { displayName: "", email: undefined } as ProfileFormValues,
      [],
    );

    expect(payload).toStrictEqual({ displayName: "", properties: {} });
  });

  it("normalizes DatePicker values to YYYY-MM-DD", () => {
    const payload = buildUserSettingsPayload(
      {
        displayName: "Alice",
        birthday: { format: () => "2026-01-02" },
      } as unknown as ProfileFormValues,
      [],
    );

    expect(payload.birthday).toBe("2026-01-02");
  });

  it("merges settings with custom properties and backfills karma/ranking numbers", () => {
    const user = buildUser();
    const values = {
      ...buildProfileFormValues(user),
      karma: 9,
      ranking: 4,
    };
    const rows: CustomPropertyRow[] = [
      { key: "department", value: "Engineering", rawValue: "Engineering" },
      { key: "active", value: "true", rawValue: true },
      { key: "revoked", value: "false", rawValue: true },
      { key: "", value: "ignored" },
      { key: "blank", value: "" },
      { key: " meta ", value: '{"a":1}', rawValue: { a: 1 } },
    ];

    const payload = buildUserSettingsPayload(values, rows);

    expect(payload).toMatchObject({
      displayName: "Alice",
      homepage: "https://alice.example.com",
      properties: {
        department: "Engineering",
        active: true,
        revoked: "false",
        meta: { a: 1 },
        karma: 9,
        ranking: 4,
      },
    });
    expect(payload.properties).not.toHaveProperty("blank");
    expect(payload).not.toHaveProperty("tag");
  });
});

describe("buildCredentialsPayload", () => {
  it("defaults both flags to false", () => {
    expect(buildCredentialsPayload({})).toStrictEqual({
      rotateAccessSecret: false,
      needUpdatePassword: false,
    });
  });

  it("preserves the submitted flags", () => {
    expect(
      buildCredentialsPayload({ rotateAccessSecret: true, needUpdatePassword: true }),
    ).toStrictEqual({ rotateAccessSecret: true, needUpdatePassword: true });
  });
});

describe("buildCustomProperties", () => {
  it("drops system keys folded into profile.properties", () => {
    const rows = buildCustomProperties({
      properties: {
        department: "Engineering",
        karma: 12,
        ranking: 5,
        homepage: "https://alice.example.com",
        affiliation: "acme",
        active: false,
      },
    });

    expect(rows).toStrictEqual([
      { key: "department", value: "Engineering", rawValue: "Engineering" },
      { key: "active", value: "false", rawValue: false },
    ]);
  });

  it("returns an empty list when properties are missing", () => {
    expect(buildCustomProperties(undefined)).toStrictEqual([]);
    expect(buildCustomProperties({})).toStrictEqual([]);
  });
});

describe("formatPropertyValue", () => {
  it("renders scalars and JSON for structured values", () => {
    expect(formatPropertyValue(null)).toBe("");
    expect(formatPropertyValue(undefined)).toBe("");
    expect(formatPropertyValue("Engineering")).toBe("Engineering");
    expect(formatPropertyValue(12)).toBe("12");
    expect(formatPropertyValue(true)).toBe("true");
    expect(formatPropertyValue({ a: 1 })).toBe('{"a":1}');
  });
});

describe("validateCustomProperties", () => {
  it("accepts a valid list", () => {
    expect(validateCustomProperties([{ key: "department", value: "Engineering" }])).toBeNull();
  });

  it("rejects empty, reserved, duplicated and oversized keys", () => {
    expect(validateCustomProperties([{ key: " ", value: "x" }])).toBe("keyRequired");
    expect(validateCustomProperties([{ key: "homepage", value: "x" }])).toBe("keyReserved");
    expect(
      validateCustomProperties([
        { key: "department", value: "a" },
        { key: "department", value: "b" },
      ]),
    ).toBe("keyDuplicate");
    expect(validateCustomProperties([{ key: "a".repeat(65), value: "x" }])).toBe("keyTooLong");
  });

  it("rejects more than the supported number of rows", () => {
    const rows = Array.from({ length: MAX_CUSTOM_PROPERTIES + 1 }, (_, index) => ({
      key: `key-${index}`,
      value: "x",
    }));
    expect(validateCustomProperties(rows)).toBe("tooMany");
  });

  it("exposes the reserved keys used by the UI", () => {
    expect(RESERVED_PROFILE_PROPERTY_KEYS.has("homepage")).toBe(true);
    expect(RESERVED_PROFILE_PROPERTY_KEYS.has("karma")).toBe(true);
  });
});

describe("buildIdentities", () => {
  it("maps identity records with empty fallbacks", () => {
    const user = buildUser({
      identities: [
        {
          id: "identity-1",
          providerId: "github",
          identityType: "oauth2",
          subject: "alice-gh",
          providerAccount: "alice",
          linkedAt: "2026-01-01T00:00:00Z",
        },
        { id: "identity-2" },
      ],
    });

    expect(buildIdentities(user)).toStrictEqual([
      {
        id: "identity-1",
        providerId: "github",
        identityType: "oauth2",
        subject: "alice-gh",
        providerAccount: "alice",
        linkedAt: "2026-01-01T00:00:00Z",
      },
      {
        id: "identity-2",
        providerId: "",
        identityType: "",
        subject: "",
        providerAccount: "",
        linkedAt: null,
      },
    ]);
  });
});
