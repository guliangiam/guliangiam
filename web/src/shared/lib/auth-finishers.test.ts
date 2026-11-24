import { describe, it, expect, vi, beforeEach } from "vitest";
import { finishConsoleLogin, hasOauthAuthorizeParams, persistLastLogin } from "./auth-finishers";

describe("hasOauthAuthorizeParams", () => {
  it("accepts a search carrying client_id and redirect_uri", () => {
    expect(
      hasOauthAuthorizeParams(
        "?client_id=app-1&redirect_uri=https%3A%2F%2Frp.example.com%2Fcb&response_type=code",
      ),
    ).toBe(true);
  });

  it("rejects an empty search (manual visit without authorize params)", () => {
    expect(hasOauthAuthorizeParams("")).toBe(false);
  });

  it("rejects partial params", () => {
    expect(hasOauthAuthorizeParams("?client_id=app-1")).toBe(false);
    expect(hasOauthAuthorizeParams("?redirect_uri=https%3A%2F%2Frp.example.com%2Fcb")).toBe(false);
    expect(hasOauthAuthorizeParams("?client_id=&redirect_uri=")).toBe(false);
  });
});

describe("persistLastLogin", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("writes both last-login keys", () => {
    persistLastLogin({ organization: "org-1", application: "app-1" });
    expect(localStorage.getItem("lastLoginOrg")).toBe("org-1");
    expect(localStorage.getItem("lastLoginApplication")).toBe("app-1");
  });
});

describe("finishConsoleLogin", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("persists last login, invalidates session, then navigates with replace", async () => {
    const calls: string[] = [];
    const invalidateSession = vi.fn<() => Promise<void>>(async () => {
      calls.push("invalidate");
    });
    const navigate = vi.fn<(to: string, options?: { replace?: boolean }) => void>(() => {
      calls.push("navigate");
    });
    const showSuccess = vi.fn<(content: string) => void>(() => {
      calls.push("success");
    });

    await finishConsoleLogin(
      { organization: "org-1", application: "app-1" },
      { invalidateSession, navigate, returnTo: "/dashboard", showSuccess },
    );

    expect(localStorage.getItem("lastLoginOrg")).toBe("org-1");
    expect(localStorage.getItem("lastLoginApplication")).toBe("app-1");
    expect(invalidateSession).toHaveBeenCalledTimes(1);
    expect(navigate).toHaveBeenCalledWith("/dashboard", { replace: true });
    expect(calls.indexOf("invalidate")).toBeLessThan(calls.indexOf("navigate"));
  });
});
