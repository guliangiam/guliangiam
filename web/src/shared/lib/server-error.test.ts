import { describe, expect, it, vi } from "vitest";
import type { FormInstance } from "antd";
import type { TFunction } from "i18next";
import { HttpError } from "@/shared/api/http";
import {
  applyServerFieldErrors,
  buildServerFieldResolver,
  extractFieldCandidatesFromMessage,
  extractServerFieldErrors,
  serverErrorMessage,
  serverFieldLeaf,
} from "./server-error";

function validationError(status: number, message: string, code = "unprocessable_entity") {
  const error = new HttpError(status, undefined, message);
  error.code = code;
  return error;
}

function unprocessable(details: Record<string, unknown>) {
  const error = new HttpError(422, undefined, "server prose must not be rendered");
  error.code = "unprocessable_entity";
  error.details = details;
  return error;
}

const t = ((key: string, options?: { field?: string; seconds?: number; code?: string }) =>
  options?.field
    ? `${key}:${options.field}`
    : options?.seconds
      ? `${key}:${options.seconds}`
      : options?.code
        ? `${key}:${options.code}`
        : key) as TFunction;

describe("serverFieldLeaf", () => {
  it("strips the profile/credential prefix", () => {
    expect(serverFieldLeaf("profile.email")).toBe("email");
    expect(serverFieldLeaf("credential.password")).toBe("password");
    expect(serverFieldLeaf("username")).toBe("username");
  });
});

describe("extractServerFieldErrors", () => {
  it("reads the array form of details.fields", () => {
    expect(
      extractServerFieldErrors(unprocessable({ fields: [{ field: "email" }, "phone"] })),
    ).toEqual([{ field: "email" }, { field: "phone" }]);
  });

  it("reads the map form of details.fields", () => {
    expect(extractServerFieldErrors(unprocessable({ fields: { username: "taken" } }))).toEqual([
      { field: "username" },
    ]);
  });

  it("falls back to the single details.field shape", () => {
    expect(extractServerFieldErrors(unprocessable({ field: "pageSize" }))).toEqual([
      { field: "pageSize" },
    ]);
  });

  it("ignores non-422 responses", () => {
    const conflict = new HttpError(409, undefined, "conflict");
    conflict.details = { field: "revision" };
    expect(extractServerFieldErrors(conflict)).toEqual([]);
  });

  it("falls back to field names embedded in the English message", () => {
    expect(extractServerFieldErrors(validationError(422, "displayName must not be blank"))).toEqual(
      [{ field: "displayName" }],
    );
    expect(
      extractServerFieldErrors(validationError(400, "invalid clientId for this grant type")),
    ).toEqual([{ field: "clientId" }]);
  });

  it("does not mine the message when structured detail names a field", () => {
    const error = unprocessable({ field: "pageSize" });
    error.message = "displayName must not be blank";
    expect(extractServerFieldErrors(error)).toEqual([{ field: "pageSize" }]);
  });

  it("skips message mining for already_exists conflicts", () => {
    expect(
      extractServerFieldErrors(validationError(400, "slug already exists", "already_exists")),
    ).toEqual([]);
  });
});

describe("extractFieldCandidatesFromMessage", () => {
  it("reads leading, quoted, adjective and verb-adjacent field names", () => {
    expect(extractFieldCandidatesFromMessage("redirectUris must be an array of objects")).toEqual([
      "redirectUris",
    ]);
    expect(extractFieldCandidatesFromMessage('"homepageUrl" is required')).toContain("homepageUrl");
    expect(extractFieldCandidatesFromMessage("missing organizationId")).toContain("organizationId");
    expect(
      extractFieldCandidatesFromMessage("providerItems[0].clientId must not be blank"),
    ).toEqual(["providerItems[0].clientId"]);
  });

  it("returns nothing for prose without field identifiers", () => {
    expect(extractFieldCandidatesFromMessage("the request could not be processed")).toEqual([]);
  });
});

describe("buildServerFieldResolver", () => {
  const formWith = (names: (string | number)[][]) =>
    ({
      getFieldsError: () => names.map((name) => ({ name, errors: [], warnings: [] })),
    }) as unknown as FormInstance<Record<string, unknown>>;

  it("matches the full path before the leaf so profile.email and email stay apart", () => {
    const resolve = buildServerFieldResolver(formWith([["email"], ["profile", "email"]]));

    expect(resolve("profile.email")).toEqual({ name: ["profile", "email"], label: undefined });
    expect(resolve("email")).toEqual({ name: ["email"], label: undefined });
  });

  it("does not guess between two items sharing a leaf", () => {
    const resolve = buildServerFieldResolver(formWith([["email"], ["profile", "email"]]));

    expect(resolve("contact.email")).toBeUndefined();
  });

  it("normalizes array indexes and applies aliases before matching", () => {
    const resolve = buildServerFieldResolver(formWith([["items", 0, "name"], ["username"]]), {
      aliases: { name: "username", "items[0].name": "items.0.name" },
    });

    expect(resolve("items[0].name")).toEqual({ name: ["items", 0, "name"], label: undefined });
    expect(resolve("name")).toEqual({ name: ["username"], label: undefined });
  });

  it("maps an array-item rejection to the mounted list field", () => {
    const resolve = buildServerFieldResolver(formWith([["providerItems"], ["clientId"]]), {
      labels: { providerItems: "Identity providers" },
    });

    // `providerItems.0.providerId` is not a registered item; the parent list
    // must take the error so the rejection is visible.
    expect(resolve("providerItems[0].providerId")).toEqual({
      name: ["providerItems"],
      label: "Identity providers",
    });
  });

  it("prefers labels by full path over the shared leaf", () => {
    const resolve = buildServerFieldResolver(formWith([["email"], ["profile", "email"]]), {
      labels: { email: "Email", "profile.email": "Profile email" },
    });

    expect(resolve("profile.email")).toEqual({
      name: ["profile", "email"],
      label: "Profile email",
    });
    expect(resolve("email")).toEqual({ name: ["email"], label: "Email" });
  });
});

describe("applyServerFieldErrors", () => {
  it("maps a nested server field to the leaf form item", () => {
    const setFields = vi.fn<(...args: unknown[]) => void>();
    const form = { setFields } as unknown as FormInstance<Record<string, unknown>>;
    const applied = applyServerFieldErrors(
      form,
      unprocessable({ fields: [{ field: "profile.email" }] }),
      (field) => {
        const leaf = serverFieldLeaf(field);
        return leaf === "email" ? { name: "email", label: "Email" } : undefined;
      },
      t,
    );

    expect(applied).toBe(true);
    expect(setFields).toHaveBeenCalledWith([
      {
        name: "email",
        errors: ["common:serverError.invalidField:Email"],
      },
    ]);
  });

  it("returns false when no server field matches a form item", () => {
    const setFields = vi.fn<(...args: unknown[]) => void>();
    const form = { setFields } as unknown as FormInstance<Record<string, unknown>>;
    expect(
      applyServerFieldErrors(
        form,
        unprocessable({ fields: [{ field: "unknown" }] }),
        () => undefined,
        t,
      ),
    ).toBe(false);
    expect(setFields).not.toHaveBeenCalled();
  });

  it("uses the field name itself as the inline label when none is localized", () => {
    const setFields = vi.fn<(...args: unknown[]) => void>();
    const form = { setFields } as unknown as FormInstance<Record<string, unknown>>;
    const applied = applyServerFieldErrors(
      form,
      validationError(422, "displayName must not be blank"),
      (field) => (field === "displayName" ? { name: "displayName" } : undefined),
      t,
    );

    expect(applied).toBe(true);
    // The inline sentence must name the field; the generic "check the
    // highlighted fields" text would be self-referential on the red item.
    expect(setFields).toHaveBeenCalledWith([
      { name: "displayName", errors: ["common:serverError.invalidField:displayName"] },
    ]);
  });

  it("collapses several server fields resolving to the same item", () => {
    const setFields = vi.fn<(...args: unknown[]) => void>();
    const form = { setFields } as unknown as FormInstance<Record<string, unknown>>;
    const applied = applyServerFieldErrors(
      form,
      unprocessable({ fields: [{ field: "profile.email" }, { field: "email" }] }),
      () => ({ name: "email", label: "Email" }),
      t,
    );

    expect(applied).toBe(true);
    expect(setFields).toHaveBeenCalledWith([
      {
        name: "email",
        errors: ["common:serverError.invalidField:Email"],
      },
    ]);
  });
});

describe("serverErrorMessage", () => {
  it("never renders backend prose", () => {
    expect(serverErrorMessage(unprocessable({ field: "email" }), t, "fallback")).toBe(
      "common:serverError.validation",
    );
    expect(serverErrorMessage(new HttpError(409, undefined, "server prose"), t, "fallback")).toBe(
      "common:serverError.conflict",
    );
    // `code` 来自响应体（http.ts 在构造后回填），直接构造 HttpError 时需手动设置。
    const alreadyExists = new HttpError(409, undefined, "server prose");
    alreadyExists.code = "already_exists";
    expect(serverErrorMessage(alreadyExists, t, "fallback")).toBe(
      "common:serverError.alreadyExists",
    );
    expect(serverErrorMessage(new HttpError(503, undefined, "server prose"), t, "fallback")).toBe(
      "common:serverError.unavailable",
    );
  });

  // 厂商真实错误码经错误 Meta 透出后，运维能直接定位开通/授权问题；
  // 无 Meta 时必须保持泛化 5xx 兜底，不能误判成厂商拒绝。
  it("prefers the provider code over the generic 5xx copy when Meta carries it", () => {
    const rejected = new HttpError(503, undefined, "server prose");
    rejected.meta = { provider: "aliyun_ocr", providerCode: "InvalidProduct.NotActivated" };
    expect(serverErrorMessage(rejected, t, "fallback")).toBe(
      "common:serverError.providerRejected:InvalidProduct.NotActivated",
    );
    expect(serverErrorMessage(new HttpError(503, undefined, "prose"), t, "fallback")).toBe(
      "common:serverError.unavailable",
    );
    expect(serverErrorMessage(new HttpError(400, undefined, "prose"), t, "fallback")).toBe(
      "fallback",
    );
  });

  it("uses the localized fallback for unmapped statuses", () => {
    expect(serverErrorMessage(new HttpError(400, undefined, "server prose"), t, "fallback")).toBe(
      "fallback",
    );
  });

  it("maps 423/account_locked to the lockout copy", () => {
    const locked = new HttpError(423, undefined, "server prose");
    locked.code = "account_locked";
    expect(serverErrorMessage(locked, t, "fallback")).toBe("common:serverError.accountLocked");
    // 状态码单独出现也必须命中（code 缺失时不留给 generic fallback）。
    expect(serverErrorMessage(new HttpError(423, undefined, "prose"), t, "fallback")).toBe(
      "common:serverError.accountLocked",
    );
  });

  it("consumes retryAfterSeconds on 429 and falls back when absent", () => {
    const limited = new HttpError(429, undefined, "server prose");
    limited.retryAfterSeconds = 42;
    expect(serverErrorMessage(limited, t, "fallback")).toBe("common:tooManyRequestsRetry:42");
    expect(serverErrorMessage(new HttpError(429, undefined, "prose"), t, "fallback")).toBe(
      "common:tooManyRequests",
    );
  });

  it("maps revision precondition statuses (412/428)", () => {
    expect(serverErrorMessage(new HttpError(412, undefined, "prose"), t, "fallback")).toBe(
      "common:serverError.preconditionFailed",
    );
    expect(serverErrorMessage(new HttpError(428, undefined, "prose"), t, "fallback")).toBe(
      "common:serverError.preconditionRequired",
    );
  });
});
