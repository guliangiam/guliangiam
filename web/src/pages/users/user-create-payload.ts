/**
 * UserCreatePage 的表单值 → API 请求体装配
 *
 * 从 UserCreatePage.tsx 拆分而来。字段映射与默认值逐字段保持原样
 * （trim 语义与 `?? ""` / `?? false` / `?? 0` 默认值均未改动）。
 */
import type { CreateUserPayload } from "@/shared/api/iam/user";
import type { UserCreateFormValues } from "./tabs/types";

/** Assemble the createUser request body from the user create form values. */
export function buildUserCreatePayload(values: UserCreateFormValues): CreateUserPayload {
  return {
    organizationId: values.organizationId?.trim() ?? "",
    username: values.username?.trim() ?? "",
    externalId: values.externalId?.trim() ?? "",
    userType: values.userType?.trim() ?? "",
    status: values.status?.trim() ?? "",
    isAdmin: values.isAdmin ?? false,
    isForbidden: values.isForbidden ?? false,
    isDeleted: values.isDeleted ?? false,
    needUpdatePassword: values.needUpdatePassword ?? false,
    ipWhitelist: values.ipWhitelist?.trim() ?? "",
    tag: values.tag?.trim() ?? "",
    score: values.score ?? 0,
    karma: values.karma ?? 0,
    ranking: values.ranking ?? 0,
    balance: values.balance ?? 0,
    balanceCredit: values.balanceCredit ?? 0,
    balanceCurrency: values.balanceCurrency ?? "USD",
    signupApplicationId: values.signupApplicationId?.trim() ?? "",
    password: values.password ?? "",
    profile: {
      displayName: values.displayName?.trim() ?? "",
      firstName: values.firstName?.trim() ?? "",
      lastName: values.lastName?.trim() ?? "",
      avatarUrl: values.avatarUrl?.trim() ?? "",
      email: values.email?.trim() ?? "",
      phone: values.phone?.trim() ?? "",
      countryCode: values.countryCode ?? "",
      region: values.region ?? "",
      location: values.location?.trim() ?? "",
      gender: values.gender ?? "",
      birthday: values.birthday ? values.birthday.format("YYYY-MM-DD") : "",
      bio: values.bio?.trim() ?? "",
      homepage: values.homepage?.trim() ?? "",
      education: values.education?.trim() ?? "",
      language: values.language ?? "",
      realName: values.realName?.trim() ?? "",
      idCardType: values.idCardType ?? "",
      idCard: values.idCard?.trim() ?? "",
      affiliation: values.affiliation?.trim() ?? "",
      title: values.title?.trim() ?? "",
    },
    roleBindingIds: values.roleBindingIds ?? [],
    groupIds: values.groupIds ?? [],
  };
}
