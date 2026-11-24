import { create } from "zustand";
import type { PasswordPolicy, SessionPolicy } from "@/shared/api/iam/auth";
import type { SessionAccount, SessionOrganization } from "@/shared/types/domain";

type SessionState = {
  account: SessionAccount | null;
  organization: SessionOrganization | null;
  passwordPolicy: PasswordPolicy | null;
  sessionPolicy: SessionPolicy | null;
  needUpdatePassword: boolean | null;
  impersonatedUser: { id: string; username: string; displayName: string } | null;
  /** 冒充会话的 opaque imp token，仅存内存（不落 localStorage）；http 层注入 Authorization。 */
  impersonationToken: string | null;
  setSession: (payload: {
    account: SessionAccount | null;
    organization: SessionOrganization | null;
    passwordPolicy?: PasswordPolicy | null;
    sessionPolicy?: SessionPolicy | null;
    needUpdatePassword?: boolean | null;
  }) => void;
  setImpersonatedUser: (user: { id: string; username: string; displayName: string } | null) => void;
  setImpersonationToken: (token: string | null) => void;
};

export const useSessionStore = create<SessionState>((set) => ({
  account: null,
  organization: null,
  passwordPolicy: null,
  sessionPolicy: null,
  needUpdatePassword: null,
  impersonatedUser: null,
  impersonationToken: null,
  setSession: ({ account, organization, passwordPolicy, sessionPolicy, needUpdatePassword }) =>
    set((state) => ({
      account,
      organization,
      // 未显式传入时保留上一次的策略快照；切换组织后由 bootstrap 覆盖。
      passwordPolicy: passwordPolicy === undefined ? state.passwordPolicy : passwordPolicy,
      sessionPolicy: sessionPolicy === undefined ? state.sessionPolicy : sessionPolicy,
      // 同上：登录引导与登出会显式传入，避免提示状态在切换用户时串台。
      needUpdatePassword:
        needUpdatePassword === undefined ? state.needUpdatePassword : needUpdatePassword,
    })),
  // 清除冒充用户时同步清除 token：不允许出现「无冒充用户却仍注入 imp Authorization」的孤儿态。
  // needUpdatePassword 一并置空：冒充期间它来自目标用户的 /auth/account，若残留 true，
  // 退出后导航会被密码门禁劫持回 /account；恢复的管理员真实值由 bootstrap 重取覆盖。
  setImpersonatedUser: (user) =>
    set(
      user === null
        ? { impersonatedUser: null, impersonationToken: null, needUpdatePassword: null }
        : { impersonatedUser: user },
    ),
  setImpersonationToken: (token) => set({ impersonationToken: token }),
}));
