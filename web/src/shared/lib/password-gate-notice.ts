/**
 * Two distinct 403 password-gate codes with deliberately separate notice copy,
 * event names, and UI treatments. They must never be treated as interchangeable:
 * password_change_required means an initial/administrator-reset password must
 * be changed, while password_expired means the password aged out of the
 * organization's max-age policy.
 */
export type PasswordGateNotice = {
  code: "password_change_required" | "password_expired";
  eventName: "auth:password-change-required" | "auth:password-expired";
  messageKey: "passwordChangeRequiredNotice" | "passwordExpiredNotice";
  severity: "warning" | "error";
};

export function passwordGateNoticeForCode(code: string): PasswordGateNotice | undefined {
  switch (code) {
    case "password_change_required":
      return {
        code,
        eventName: "auth:password-change-required",
        messageKey: "passwordChangeRequiredNotice",
        severity: "warning",
      };
    case "password_expired":
      return {
        code,
        eventName: "auth:password-expired",
        messageKey: "passwordExpiredNotice",
        severity: "error",
      };
    default:
      return undefined;
  }
}
