import { useSessionStore } from "@/features/session/session-store";
import { DEFAULT_PASSWORD_POLICY, type PasswordPolicy } from "@/shared/api/iam/auth";

/**
 * 当前会话的生效密码策略。
 *
 * 唯一权威来源是后端账户上下文（组织策略 + 默认值）；仅在会话尚未加载
 * 或未登录页面上使用同一个兜底默认值，前端不得再按场景写死长度。
 */
export function usePasswordPolicy(): PasswordPolicy {
  return useSessionStore((state) => state.passwordPolicy) ?? DEFAULT_PASSWORD_POLICY;
}
