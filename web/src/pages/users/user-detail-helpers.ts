/**
 * UserDetailPage 的纯 helper（无 JSX、无闭包）。
 *
 * 从 UserDetailPage.tsx 拆分而来：把 role/group/identity 的
 * 「本地状态 → API item」映射从事件处理器里抽出来，消除重复。
 */
import type {
  ReplaceUserIdentityItem,
  UserGroupMembership,
  UserRoleBinding,
} from "@/shared/api/iam/user";
import type { IdentityFormItem } from "./tabs/types";

/** UserRoleBinding[] → replaceUserRoleBindings 的 item 形状。 */
export function toRoleBindingItems(
  bindings: UserRoleBinding[],
): Array<{ roleId: string; roleName: string }> {
  return bindings.map((rb) => ({
    roleId: rb.roleId ?? rb.id ?? "",
    roleName: rb.roleName ?? rb.name ?? "",
  }));
}

/** UserGroupMembership[] → replaceUserGroupMemberships 的 item 形状。 */
export function toGroupMembershipItems(
  memberships: UserGroupMembership[],
): Array<{ groupId: string; groupName: string }> {
  return memberships.map((gm) => ({
    groupId: gm.groupId ?? gm.id ?? "",
    groupName: gm.groupName ?? gm.name ?? "",
  }));
}

/** 可编辑 identity 状态 → replaceUserIdentities 的 item 形状。 */
export function toIdentityItems(items: IdentityFormItem[]): ReplaceUserIdentityItem[] {
  return items.map((i) => ({
    id: i.id,
    providerId: i.providerId,
    identityType: i.identityType,
    subject: i.subject,
    providerAccount: i.providerAccount,
  }));
}
