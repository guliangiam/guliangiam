import { CheckCircleTwoTone, CloseCircleTwoTone } from "@ant-design/icons";
import { Progress, theme, Typography } from "antd";
import type { TFunction } from "i18next";
import { memo, useMemo } from "react";
import { useTranslation } from "react-i18next";
import type { PasswordPolicy } from "@/shared/api/iam/auth";
import { usePasswordPolicy } from "@/shared/hooks/usePasswordPolicy";

export type PasswordRule = {
  key: string;
  label: string;
  validate: (password: string) => boolean;
};

type PasswordCheckerProps = {
  password: string;
  rules?: PasswordRule[];
  visible?: boolean;
  showStrength?: boolean;
};

// Common weak passwords list (top 100 truncated)
const COMMON_PASSWORDS = new Set([
  "password",
  "123456",
  "12345678",
  "qwerty",
  "abc123",
  "monkey",
  "master",
  "dragon",
  "111111",
  "baseball",
  "iloveyou",
  "trustno1",
  "sunshine",
  "princess",
  "football",
  "shadow",
  "superman",
  "michael",
  "letmein",
  "welcome",
  "1234567",
  "123456789",
  "1234567890",
  "password1",
  "password123",
  "admin",
  "admin123",
  "root",
  "toor",
  "pass",
  "test",
  "guest",
  "qwerty123",
]);

/**
 * 按组织生效策略生成规则列表。长度只有一条（minLength），不再同时出现
 * 8 与 12 两套门槛；所有文案走 i18n。
 */
// oxlint-disable-next-line react/only-export-components -- 规则生成函数与 PasswordChecker 强绑定
export function buildPasswordRules(policy: PasswordPolicy, t: TFunction): PasswordRule[] {
  const rules: PasswordRule[] = [
    {
      key: "minLength",
      label: t("form.minLength", { min: policy.minLength }),
      validate: (p) => Array.from(p).length >= policy.minLength,
    },
  ];

  if (policy.maxLength > 0) {
    rules.push({
      key: "maxLength",
      label: t("form.maxLength", { max: policy.maxLength }),
      validate: (p) => Array.from(p).length <= policy.maxLength,
    });
  }
  if (policy.requireUppercase) {
    rules.push({
      key: "uppercase",
      label: t("passwordRule.uppercase"),
      validate: (p) => /[A-Z]/.test(p),
    });
  }
  if (policy.requireLowercase) {
    rules.push({
      key: "lowercase",
      label: t("passwordRule.lowercase"),
      validate: (p) => /[a-z]/.test(p),
    });
  }
  if (policy.requireDigit) {
    rules.push({
      key: "digit",
      label: t("passwordRule.digit"),
      validate: (p) => /[0-9]/.test(p),
    });
  }
  if (policy.requireSpecial) {
    rules.push({
      key: "specialChar",
      label: t("passwordRule.specialChar"),
      validate: (p) => /[!@#$%^&*()_+\-=[\]{};':"\\|,.<>/?]/.test(p),
    });
  }
  if (policy.rejectCommonPasswords) {
    rules.push({
      key: "noCommonPassword",
      label: t("passwordRule.commonPassword"),
      validate: (p) => !COMMON_PASSWORDS.has(p.toLowerCase()),
    });
  }

  return rules;
}

/**
 * 当前会话策略对应的规则列表。页面/控制器统一通过该 hook 取规则，
 * 避免在调用点再次硬编码长度。
 */
// oxlint-disable-next-line react/only-export-components -- 规则 hook 与 PasswordChecker 共用同一生成逻辑
export function usePasswordRules(): PasswordRule[] {
  const { t } = useTranslation("common");
  const policy = usePasswordPolicy();
  return useMemo(() => buildPasswordRules(policy, t), [policy, t]);
}

// oxlint-disable-next-line react/only-export-components -- 复杂度校验与 PasswordChecker 共用规则生成逻辑，拆分需同步调整调用方
export function checkPasswordComplexity(password: string, rules: PasswordRule[]): string | null {
  if (!password) return null;
  for (const rule of rules) {
    if (!rule.validate(password)) return rule.label;
  }
  return null;
}

function computeStrength(password: string, rules: PasswordRule[]): number {
  if (!password) return 0;
  const passed = rules.filter((r) => r.validate(password)).length;
  const ratio = passed / rules.length;
  // Scale: 0-33 = weak, 34-66 = fair, 67-100 = strong
  return Math.round(ratio * 100);
}

function getStrengthColor(strength: number): string {
  if (strength < 34) return "var(--gl-color-error)";
  if (strength < 67) return "var(--gl-color-warning)";
  return "var(--gl-color-success)";
}

function getStrengthLabel(strength: number): string {
  if (strength < 34) return "Weak";
  if (strength < 67) return "Fair";
  return "Strong";
}

export const PasswordChecker = memo(function PasswordChecker({
  password,
  rules,
  visible = true,
  showStrength = true,
}: PasswordCheckerProps) {
  const { token } = theme.useToken();
  const defaultRules = usePasswordRules();

  const activeRules = useMemo(() => rules ?? defaultRules, [rules, defaultRules]);

  const strength = useMemo(() => computeStrength(password, activeRules), [password, activeRules]);

  if (!visible || !password) return null;
  const strengthColor = getStrengthColor(strength);
  const strengthLabel = getStrengthLabel(strength);

  return (
    <output
      style={{ padding: "8px 0", display: "flex", flexDirection: "column", gap: 4 }}
      aria-live="polite"
      aria-label={`Password strength: ${strengthLabel}`}
    >
      {showStrength && (
        <div style={{ marginBottom: 4 }}>
          <div
            style={{
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
              marginBottom: 4,
            }}
          >
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              Password strength
            </Typography.Text>
            <Typography.Text style={{ fontSize: 12, color: strengthColor, fontWeight: 600 }}>
              {strengthLabel}
            </Typography.Text>
          </div>
          <Progress percent={strength} showInfo={false} strokeColor={strengthColor} size="small" />
        </div>
      )}
      {activeRules.map((rule) => {
        const passed = rule.validate(password);
        return (
          <div
            key={rule.key}
            style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 13 }}
          >
            {passed ? (
              <CheckCircleTwoTone twoToneColor={token.colorSuccess} />
            ) : (
              <CloseCircleTwoTone twoToneColor={token.colorError} />
            )}
            <span
              style={{
                color: passed
                  ? "var(--gl-color-success-text, #52c41a)"
                  : "var(--gl-color-error-text, #ff4d4f)",
              }}
            >
              {rule.label}
            </span>
          </div>
        );
      })}
    </output>
  );
});

PasswordChecker.displayName = "PasswordChecker";
