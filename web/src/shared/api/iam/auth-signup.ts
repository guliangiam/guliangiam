import { requestDocumentJson } from "@/shared/api/http";
import { AUTH_PASSWORD_CHECK, AUTH_PASSWORD_RESET, AUTH_SIGNUP } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────

export type SignupPayload = {
  organizationSlug: string;
  username: string;
  password: string;
  email?: string;
  phone?: string;
  signupAppId?: string;
  verificationCode?: string;
  invitationCode?: string;
  captchaToken?: string;
  captchaId?: string;
};

export type SignupResult = {
  status: string;
  userId?: string;
  message?: string;
};

export type ResetPasswordPayload = {
  organizationSlug: string;
  username: string;
  verificationCode: string;
  newPassword: string;
  captchaToken?: string;
  captchaId?: string;
};

export type ResetPasswordResult = {
  status: string;
};

export type CheckPasswordPayload = {
  organizationSlug: string;
  username: string;
  password: string;
};

export type CheckPasswordResult = {
  valid: boolean;
};

// ─────────────────────────────────────────────
// API Functions
// ─────────────────────────────────────────────

export async function signup(payload: SignupPayload): Promise<SignupResult> {
  return requestDocumentJson(AUTH_SIGNUP, {
    method: "POST",
    body: JSON.stringify(payload),
    headers: { "Content-Type": "application/json" },
  }) as Promise<SignupResult>;
}

export async function resetPassword(payload: ResetPasswordPayload): Promise<ResetPasswordResult> {
  return requestDocumentJson(AUTH_PASSWORD_RESET, {
    method: "POST",
    body: JSON.stringify(payload),
    headers: { "Content-Type": "application/json" },
  }) as Promise<ResetPasswordResult>;
}

export async function checkPassword(payload: CheckPasswordPayload): Promise<CheckPasswordResult> {
  return requestDocumentJson(AUTH_PASSWORD_CHECK, {
    method: "POST",
    body: JSON.stringify(payload),
    headers: { "Content-Type": "application/json" },
  }) as Promise<CheckPasswordResult>;
}
