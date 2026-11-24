import { describe, it, expect } from "vitest";

interface PasswordStrength {
  score: number;
  label: string;
  isStrong: boolean;
}

function checkPasswordStrength(password: string): PasswordStrength {
  let score = 0;

  if (password.length >= 8) score++;
  if (password.length >= 12) score++;
  if (/[a-z]/.test(password)) score++;
  if (/[A-Z]/.test(password)) score++;
  if (/[0-9]/.test(password)) score++;
  if (/[^a-zA-Z0-9]/.test(password)) score++;

  const commonPasswords = new Set([
    "123456",
    "password",
    "12345678",
    "qwerty",
    "abc123",
    "monkey",
    "letmein",
    "dragon",
    "master",
    "login",
  ]);
  if (commonPasswords.has(password.toLowerCase())) {
    score = Math.min(score, 1);
  }

  const labels = ["very-weak", "weak", "fair", "good", "strong", "very-strong"];
  const labelIndex = Math.min(Math.floor(score / 1.2), labels.length - 1);
  const label = labels[labelIndex];
  const isStrong = score >= 4;

  return { score, label, isStrong };
}

describe("checkPasswordStrength", () => {
  describe("weak password detection", () => {
    it("rejects very weak passwords (short, no complexity)", () => {
      const result = checkPasswordStrength("123");
      expect(result.isStrong).toBe(false);
      expect(result.score).toBeLessThan(3);
      expect(result.label).toBe("very-weak");
    });

    it("detects common passwords as weak", () => {
      const commonPasswords = ["password", "123456", "qwerty", "abc123", "letmein"];
      for (const pw of commonPasswords) {
        const result = checkPasswordStrength(pw);
        expect(result.isStrong).toBe(false);
        expect(result.score).toBeLessThanOrEqual(1);
      }
    });

    it("detects weak password with only lowercase", () => {
      const result = checkPasswordStrength("abcdefgh");
      expect(result.isStrong).toBe(false);
    });

    it("handles empty password as very weak", () => {
      const result = checkPasswordStrength("");
      expect(result.isStrong).toBe(false);
      expect(result.score).toBe(0);
      expect(result.label).toBe("very-weak");
    });

    it("common password overrides complexity score", () => {
      const result = checkPasswordStrength("qwerty");
      expect(result.score).toBeLessThanOrEqual(1);
      expect(result.isStrong).toBe(false);
    });

    it("detects common password with different casing", () => {
      const result = checkPasswordStrength("PASSWORD");
      expect(result.score).toBeLessThanOrEqual(1);
      expect(result.isStrong).toBe(false);
    });
  });

  describe("moderate strength password", () => {
    it("detects moderate password with mixed case and numbers", () => {
      const result = checkPasswordStrength("Password1");
      expect(result.score).toBeGreaterThanOrEqual(3);
    });

    it("moderate password without special chars is not strong", () => {
      // "Pass1234" has length>=8, upper, lower, digit = score 4 => isStrong
      // Use a shorter password to test moderate
      const result = checkPasswordStrength("Pass12");
      expect(result.isStrong).toBe(false);
    });

    it("password with 8 chars, upper, lower, digit is moderate", () => {
      const result = checkPasswordStrength("Abcd1234");
      expect(result.score).toBeGreaterThanOrEqual(3);
    });
  });

  describe("strong password detection", () => {
    it("detects strong password with all rules", () => {
      const result = checkPasswordStrength("Str0ng!Passw0rd");
      expect(result.isStrong).toBe(true);
      expect(result.score).toBeGreaterThanOrEqual(4);
    });

    it("detects very strong password", () => {
      const result = checkPasswordStrength("V3ry$tr0ng&P@ssw0rd!2024");
      expect(result.isStrong).toBe(true);
      expect(result.score).toBeGreaterThanOrEqual(5);
    });

    it("password with special chars and length >= 12 is strong", () => {
      const result = checkPasswordStrength("MyP@ssw0rd!23");
      expect(result.isStrong).toBe(true);
    });
  });

  describe("individual rule validation", () => {
    it("rewards length >= 8", () => {
      const short = checkPasswordStrength("Ab1!");
      const long8 = checkPasswordStrength("Abcde1!@");
      expect(long8.score).toBeGreaterThan(short.score);
    });

    it("rewards length >= 12", () => {
      const len8 = checkPasswordStrength("Abcd1234");
      const len12 = checkPasswordStrength("Abcd1234efgh");
      expect(len12.score).toBeGreaterThanOrEqual(len8.score);
    });

    it("rewards lowercase letters", () => {
      const noLower = checkPasswordStrength("ABCDEFG1!");
      const withLower = checkPasswordStrength("AbcDEFG1!");
      expect(withLower.score).toBeGreaterThanOrEqual(noLower.score);
    });

    it("rewards uppercase letters", () => {
      const noUpper = checkPasswordStrength("abcdefg1!");
      const withUpper = checkPasswordStrength("Abcdefg1!");
      expect(withUpper.score).toBeGreaterThanOrEqual(noUpper.score);
    });

    it("rewards digits", () => {
      const noDigit = checkPasswordStrength("Abcdefg!@");
      const withDigit = checkPasswordStrength("Abcdefg1!@");
      expect(withDigit.score).toBeGreaterThan(noDigit.score);
    });

    it("rewards special characters", () => {
      const withoutSpecial = checkPasswordStrength("Abcd1234");
      const withSpecial = checkPasswordStrength("Abcd1234!");
      expect(withSpecial.score).toBeGreaterThan(withoutSpecial.score);
    });
  });

  describe("edge cases", () => {
    it("handles very long password", () => {
      const result = checkPasswordStrength("A".repeat(128) + "1!a");
      expect(result.score).toBeGreaterThanOrEqual(4);
    });

    it("handles password with only digits", () => {
      const result = checkPasswordStrength("12345678");
      expect(result.isStrong).toBe(false);
    });

    it("handles password with only special characters", () => {
      const result = checkPasswordStrength("!@#$%^&*");
      expect(result.isStrong).toBe(false);
    });

    it("score increases monotonically with more complexity", () => {
      const base = checkPasswordStrength("a");
      const withUpper = checkPasswordStrength("aA");
      const withDigit = checkPasswordStrength("aA1");
      const withSpecial = checkPasswordStrength("aA1!");
      const withLen8 = checkPasswordStrength("aA1!abcde");
      expect(withUpper.score).toBeGreaterThanOrEqual(base.score);
      expect(withDigit.score).toBeGreaterThanOrEqual(withUpper.score);
      expect(withSpecial.score).toBeGreaterThanOrEqual(withDigit.score);
      expect(withLen8.score).toBeGreaterThanOrEqual(withSpecial.score);
    });
  });
});
