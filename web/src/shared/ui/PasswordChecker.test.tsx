import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { TFunction } from "i18next";
import { DEFAULT_PASSWORD_POLICY, type PasswordPolicy } from "@/shared/api/iam/auth";
import {
  PasswordChecker,
  buildPasswordRules,
  checkPasswordComplexity,
  type PasswordRule,
} from "./PasswordChecker";

// Mock react-i18next
vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: "en" },
  }),
  Trans: ({ children }: { children: React.ReactNode }) => children,
}));

const fakeT = ((key: string) => key) as unknown as TFunction;

function policyWith(overrides: Partial<PasswordPolicy>): PasswordPolicy {
  return { ...DEFAULT_PASSWORD_POLICY, ...overrides };
}

describe("buildPasswordRules", () => {
  it("emits exactly one minimum-length rule", () => {
    const rules = buildPasswordRules(DEFAULT_PASSWORD_POLICY, fakeT);
    const lengthRules = rules.filter((rule) => rule.key === "minLength");
    expect(lengthRules).toHaveLength(1);
    expect(lengthRules[0].validate("a".repeat(DEFAULT_PASSWORD_POLICY.minLength))).toBe(true);
    expect(lengthRules[0].validate("a".repeat(DEFAULT_PASSWORD_POLICY.minLength - 1))).toBe(false);
    expect(DEFAULT_PASSWORD_POLICY.passwordHistoryCount).toBe(5);
    expect(DEFAULT_PASSWORD_POLICY.passwordMaxAgeDays).toBe(90);
  });

  it("counts Unicode code points for minimum and maximum length", () => {
    const rules = buildPasswordRules(policyWith({ minLength: 2, maxLength: 2 }), fakeT);
    const minLengthRule = rules.find((rule) => rule.key === "minLength");
    const maxLengthRule = rules.find((rule) => rule.key === "maxLength");

    expect(minLengthRule?.validate("🌍🌎")).toBe(true);
    expect(minLengthRule?.validate("🌍")).toBe(false);
    expect(maxLengthRule?.validate("🌍🌎")).toBe(true);

    const smallerMaxRule = buildPasswordRules(
      policyWith({ minLength: 1, maxLength: 1 }),
      fakeT,
    ).find((rule) => rule.key === "maxLength");
    expect(smallerMaxRule?.validate("🌍🌎")).toBe(false);
  });

  it("follows the organization policy instead of a hardcoded threshold", () => {
    const rules = buildPasswordRules(policyWith({ minLength: 12 }), fakeT);
    expect(checkPasswordComplexity("Str0ng!8", rules)).toBe("form.minLength");
    expect(checkPasswordComplexity("Str0ng!Pass1", rules)).toBeNull();
  });
});

describe("checkPasswordComplexity", () => {
  it("does not treat an empty password as a complexity error", () => {
    expect(
      checkPasswordComplexity("", buildPasswordRules(DEFAULT_PASSWORD_POLICY, fakeT)),
    ).toBeNull();
  });

  it("returns first failing rule label", () => {
    const rules = buildPasswordRules(DEFAULT_PASSWORD_POLICY, fakeT);
    expect(checkPasswordComplexity("Ab1!", rules)).toBe("form.minLength");
  });

  it("returns null for strong password", () => {
    const rules = buildPasswordRules(DEFAULT_PASSWORD_POLICY, fakeT);
    expect(checkPasswordComplexity("Str0ngP@ssw0rd!", rules)).toBeNull();
  });

  it("returns error for common password", () => {
    const rules = buildPasswordRules(
      policyWith({
        minLength: 8,
        requireUppercase: false,
        requireLowercase: false,
        requireDigit: false,
        requireSpecial: false,
      }),
      fakeT,
    );
    expect(checkPasswordComplexity("admin123", rules)).toBe("passwordRule.commonPassword");
  });

  it("returns error for password without uppercase", () => {
    const rules = buildPasswordRules(DEFAULT_PASSWORD_POLICY, fakeT);
    expect(checkPasswordComplexity("lowercase123!abc", rules)).toBe("passwordRule.uppercase");
  });

  it("returns error for password without lowercase", () => {
    const rules = buildPasswordRules(DEFAULT_PASSWORD_POLICY, fakeT);
    expect(checkPasswordComplexity("UPPERCASE123!ABC", rules)).toBe("passwordRule.lowercase");
  });

  it("returns error for password without digit", () => {
    const rules = buildPasswordRules(DEFAULT_PASSWORD_POLICY, fakeT);
    expect(checkPasswordComplexity("NoDigitsHere!abc", rules)).toBe("passwordRule.digit");
  });

  it("returns error for password without special character", () => {
    const rules = buildPasswordRules(DEFAULT_PASSWORD_POLICY, fakeT);
    expect(checkPasswordComplexity("NoSpecialChar123abc", rules)).toBe("passwordRule.specialChar");
  });

  it("uses custom rules when provided", () => {
    const customRules: PasswordRule[] = [
      { key: "minLength", label: "At least 6 characters", validate: (p) => p.length >= 6 },
    ];
    expect(checkPasswordComplexity("short", customRules)).toBe("At least 6 characters");
    expect(checkPasswordComplexity("longenough", customRules)).toBeNull();
  });
});

describe("PasswordChecker component", () => {
  it("renders nothing when visible is false", () => {
    const { container } = render(<PasswordChecker password="test" visible={false} />);
    expect(container.innerHTML).toBe("");
  });

  it("renders nothing when password is empty", () => {
    const { container } = render(<PasswordChecker password="" />);
    expect(container.innerHTML).toBe("");
  });

  it("renders password strength when showStrength is true", () => {
    render(<PasswordChecker password="Test123!" showStrength />);
    expect(screen.getByText("Password strength")).toBeInTheDocument();
  });

  it("does not render strength when showStrength is false", () => {
    render(<PasswordChecker password="Test123!" showStrength={false} />);
    expect(screen.queryByText("Password strength")).not.toBeInTheDocument();
  });

  it("renders a single minimum-length rule", () => {
    render(<PasswordChecker password="Test123!" />);
    const minLengthNodes = screen.getAllByText(/form\.minLength/);
    expect(minLengthNodes).toHaveLength(1);
  });

  it("shows Weak label for weak password", () => {
    const strictRules: PasswordRule[] = [
      { key: "minLength", label: "Minimum 12 chars", validate: (p) => p.length >= 12 },
    ];
    render(<PasswordChecker password="a" rules={strictRules} showStrength />);
    expect(screen.getByText("Weak")).toBeInTheDocument();
  });

  it("shows Strong label for strong password", () => {
    render(<PasswordChecker password="Str0ngP@ssw0rd!X" showStrength />);
    expect(screen.getByText("Strong")).toBeInTheDocument();
  });

  it("renders with custom rules", () => {
    const customRules: PasswordRule[] = [
      { key: "minLength", label: "Minimum 6 chars", validate: (p) => p.length >= 6 },
    ];
    render(<PasswordChecker password="short" rules={customRules} />);
    expect(screen.getByText("Minimum 6 chars")).toBeInTheDocument();
  });
});
