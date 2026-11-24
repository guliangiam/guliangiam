import dayjs from "dayjs";
import { describe, expect, it } from "vitest";
import { userCreateSchema } from "@/shared/api/iam/schemas";
import { buildUserCreatePayload } from "./user-create-payload";

describe("buildUserCreatePayload", () => {
  it("normalizes and preserves the complete form data contract", () => {
    const payload = buildUserCreatePayload({
      organizationId: " org-1 ",
      username: " alice ",
      displayName: " Alice Smith ",
      externalId: " ext-1 ",
      userType: "normal",
      status: "active",
      isAdmin: true,
      isForbidden: true,
      isDeleted: true,
      needUpdatePassword: true,
      ipWhitelist: " 127.0.0.1 ",
      tag: " vip ",
      score: 12,
      karma: 42,
      ranking: 7,
      balance: 20,
      balanceCredit: 5,
      balanceCurrency: "EUR",
      signupApplicationId: " app-1 ",
      password: "a-secure-password",
      birthday: dayjs("2000-01-02"),
      firstName: " Alice ",
      lastName: " Smith ",
      avatarUrl: " https://example.com/avatar.png ",
      email: " alice@example.com ",
      phone: " +12025550123 ",
      countryCode: "US",
      region: "NY",
      location: " New York ",
      gender: "female",
      bio: " About Alice ",
      homepage: " https://example.com ",
      education: " University ",
      language: "en",
      realName: " Alice Smith ",
      idCardType: "passport",
      idCard: " X12345 ",
      affiliation: " Example Inc ",
      title: " Engineer ",
      roleBindingIds: ["role-1"],
      groupIds: ["group-1"],
    });

    expect(payload).toEqual({
      organizationId: "org-1",
      username: "alice",
      externalId: "ext-1",
      userType: "normal",
      status: "active",
      isAdmin: true,
      isForbidden: true,
      isDeleted: true,
      needUpdatePassword: true,
      ipWhitelist: "127.0.0.1",
      tag: "vip",
      score: 12,
      karma: 42,
      ranking: 7,
      balance: 20,
      balanceCredit: 5,
      balanceCurrency: "EUR",
      signupApplicationId: "app-1",
      password: "a-secure-password",
      profile: {
        displayName: "Alice Smith",
        firstName: "Alice",
        lastName: "Smith",
        avatarUrl: "https://example.com/avatar.png",
        email: "alice@example.com",
        phone: "+12025550123",
        countryCode: "US",
        region: "NY",
        location: "New York",
        gender: "female",
        birthday: "2000-01-02",
        bio: "About Alice",
        homepage: "https://example.com",
        education: "University",
        language: "en",
        realName: "Alice Smith",
        idCardType: "passport",
        idCard: "X12345",
        affiliation: "Example Inc",
        title: "Engineer",
      },
      roleBindingIds: ["role-1"],
      groupIds: ["group-1"],
    });
  });
});

describe("userCreateSchema", () => {
  const validValues = {
    organizationId: "org-1",
    username: "alice",
    displayName: "Alice",
    email: "",
    phone: "",
    password: "",
    userType: "normal",
    status: "active",
  };

  it("rejects values that become empty after trimming", () => {
    expect(
      userCreateSchema.safeParse({
        ...validValues,
        username: "   ",
      }).success,
    ).toBe(false);
    expect(
      userCreateSchema.safeParse({
        ...validValues,
        displayName: "   ",
      }).success,
    ).toBe(false);
  });

  it("rejects malformed homepage URLs", () => {
    expect(
      userCreateSchema.safeParse({
        ...validValues,
        homepage: "not a url",
      }).success,
    ).toBe(false);
  });

  it("keeps only the policy-independent maximum length in the schema", () => {
    // 最小长度由组织策略在表单/后端判定，schema 不应再固定 12。
    expect(
      userCreateSchema.safeParse({
        ...validValues,
        password: "Str0ng!Pass",
      }).success,
    ).toBe(true);
    expect(
      userCreateSchema.safeParse({
        ...validValues,
        password: "A".repeat(129),
      }).success,
    ).toBe(false);
  });
});
