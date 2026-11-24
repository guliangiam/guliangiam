import type { FormInstance } from "antd";
import type { TFunction } from "i18next";
import { getErrorDetails, isHttpError } from "@/shared/api/http";

/** HTTP 层错误的最小结构；不直接调用 http 模块，避免测试 mock 缺导出时抛错。 */
type HttpLikeError = {
  status: number;
  code?: string;
  retryAfterSeconds?: number;
  meta?: Record<string, unknown>;
};

function looksLikeHttpError(error: unknown): error is HttpLikeError {
  return (
    typeof error === "object" &&
    error !== null &&
    typeof (error as { status?: unknown }).status === "number"
  );
}

export type ServerFieldError = {
  field: string;
};

/** Form item name path, e.g. `email` or `["profile", "email"]`. */
export type FieldNamePath = string | (string | number)[];

type FieldTarget = {
  name: FieldNamePath;
  label?: string;
};

/** Server fields sometimes arrive as `profile.email`/`credential.password`. */
export function serverFieldLeaf(field: string): string {
  const leaf = field.split(".").pop();
  return leaf && leaf !== "" ? leaf : field;
}

/** Backend paths may use array indexes (`items[0].name`); form name paths use `items.0.name`. */
function normalizeServerFieldPath(field: string): string {
  return field
    .replace(/\[(\d+)\]/g, ".$1")
    .replace(/^\./, "")
    .trim();
}

export type ServerFieldResolverOptions = {
  /** Localized labels keyed by full path (`profile.email`) or by leaf (`email`). */
  labels?: Record<string, string>;
  /** Backend field name → form field path (`name` → `username`). */
  aliases?: Record<string, string>;
};

/**
 * Build a `resolve` callback for `applyServerFieldErrors`.
 *
 * Every mounted `Form.Item` path is compared with the full server path first,
 * so `profile.email` never overwrites a bare `email` item. The leaf is only
 * used when exactly one mounted item owns it; ambiguous leaves stay unmatched
 * instead of guessing a control. `aliases` maps backend names to form paths
 * before matching, and `labels` supplies the localized field name used by
 * `serverError.invalidField` (falls back to `serverError.validation`).
 */
export function buildServerFieldResolver<T>(
  form: Pick<FormInstance<T>, "getFieldsError">,
  options: ServerFieldResolverOptions = {},
): (field: string) => FieldTarget | undefined {
  const registered = form.getFieldsError().map(({ name }) => ({
    path: name.map(String),
    name: name as FieldNamePath,
  }));
  const byPath = new Map(registered.map((item) => [item.path.join("."), item.name]));
  const byLeaf = new Map<string, string[]>();
  for (const item of registered) {
    const leaf = item.path[item.path.length - 1] ?? "";
    byLeaf.set(leaf, [...(byLeaf.get(leaf) ?? []), item.path.join(".")]);
  }

  return (field) => {
    const normalized = normalizeServerFieldPath(field);
    const leaf = serverFieldLeaf(normalized);
    // Full path always wins: an aliased leaf must not hijack a nested field
    // such as `items.0.name`.
    const direct = byPath.get(normalized);
    const aliased =
      direct ??
      (options.aliases?.[normalized] ? byPath.get(options.aliases[normalized]) : undefined) ??
      (options.aliases?.[leaf] ? byPath.get(options.aliases[leaf]) : undefined);
    const candidates = byLeaf.get(leaf) ?? [];
    let name = aliased ?? (candidates.length === 1 ? byPath.get(candidates[0]) : undefined);
    if (!name) {
      // Array-item rejections (`providerItems[0].clientId`) mark the mounted
      // list field itself (`providerItems`) red: the per-row control is not a
      // registered form item, and dropping the error would hide it entirely.
      const parts = normalized.split(".");
      for (let i = parts.length - 1; i >= 1; i -= 1) {
        const parent = byPath.get(parts.slice(0, i).join("."));
        if (parent) {
          name = parent;
          break;
        }
      }
    }
    if (!name) return undefined;
    const targetPath = typeof name === "string" ? name : name.map(String).join(".");
    const label =
      options.labels?.[normalized] ??
      options.labels?.[targetPath] ??
      options.labels?.[leaf] ??
      options.labels?.[serverFieldLeaf(targetPath)];
    return label ? { name, label } : { name };
  };
}

/**
 * Candidate field names embedded in English validation prose.
 *
 * Most backend domains reject a form field without structured `details`
 * (e.g. `displayName must not be blank`, `invalid clientId`,
 * `"redirectUris" is required`, `providerItems[0].clientId must …`). These
 * messages are the only place the offending field appears. The extraction is
 * deliberately generous: every candidate is still matched against the
 * mounted form items by the resolver, so a word that is not a real field
 * name can never mark the wrong control red.
 */
export function extractFieldCandidatesFromMessage(message: string): string[] {
  const found: string[] = [];
  const push = (field: string | undefined | null) => {
    if (!field) return;
    const trimmed = field.trim();
    if (trimmed === "" || trimmed.length > 96) return;
    if (!found.includes(trimmed)) found.push(trimmed);
  };

  // Leading field path before the validation verb: `displayName must not …`
  // / `providerItems[0].clientId is required`.
  const leading =
    /^([A-Za-z][A-Za-z0-9_]*(?:(?:\.|\]|\[)+[A-Za-z0-9_]+)*)\s+(?:must|should|can\s*not|cannot|is|has|appears|looks|needs)\b/i.exec(
      message,
    );
  push(leading?.[1]);

  // Quoted names: `"redirectUris" is required`.
  const quoted = message.match(/["'`]([^"'`\n]{1,96})["'`]/g);
  if (quoted) for (const token of quoted) push(token.slice(1, -1));

  // `invalid clientId` / `missing organizationId` / `unknown providerType`.
  const afterAdjective = message.match(
    /\b(?:invalid|missing|unknown|unsupported)\s+([A-Za-z][A-Za-z0-9_.\][]*)/gi,
  );
  if (afterAdjective) for (const token of afterAdjective) push(token.replace(/^\S+\s+/, ""));

  // `<field> already exists` / `<field> is required` anywhere in the prose.
  const beforeVerb = message.match(
    /\b([A-Za-z][A-Za-z0-9_]*(?:(?:\.|\]|\[)+[A-Za-z0-9_]+)*)\s+(?:already\s+exists|is\s+required|must\s+be|must\s+not)\b/gi,
  );
  if (beforeVerb) for (const token of beforeVerb) push(token.split(/\s+/)[0]);

  return found;
}

/**
 * Read field-level detail from a 422/400 response.
 *
 * The backend uses `details.field` for single-field rejections (pagination,
 * sort) and `details.fields` for form-level validation. Both shapes are
 * accepted, and object/array forms of `fields` are normalized to the same list.
 * `getErrorDetails` owns the canonical `details` → deprecated `meta` fallback.
 * When the response carries no structured detail, field names embedded in the
 * English validation message are returned as candidates; the caller's resolver
 * still decides whether any of them matches a mounted form item.
 */
export function extractServerFieldErrors(error: unknown): ServerFieldError[] {
  const details = getErrorDetails(error);
  if (!isHttpError(error) || (error.status !== 422 && error.status !== 400)) return [];
  if (!details) {
    if (error.code === "already_exists") return [];
    return extractFieldCandidatesFromMessage(error.message ?? "").map((field) => ({ field }));
  }

  const found: ServerFieldError[] = [];
  const push = (field: unknown) => {
    if (typeof field !== "string" || field.trim() === "") return;
    if (found.some((item) => item.field === field)) return;
    found.push({ field });
  };

  const collect = (value: unknown) => {
    if (Array.isArray(value)) {
      for (const item of value) {
        if (typeof item === "string") {
          push(item);
        } else if (item && typeof item === "object") {
          const detail = item as Record<string, unknown>;
          push(detail.field);
          push(detail.name);
        }
      }
      return;
    }

    if (value && typeof value === "object") {
      for (const field of Object.keys(value)) push(field);
    }
  };

  for (const key of ["fields", "fieldErrors", "errors", "details"]) {
    collect(details[key]);
  }
  if (found.length === 0) push(details.field);
  // Structured detail present but carrying no usable field name: fall back to
  // the field embedded in the English validation message, same as the
  // detail-less path above.
  if (found.length === 0 && error.code !== "already_exists") {
    for (const candidate of extractFieldCandidatesFromMessage(error.message ?? "")) push(candidate);
  }
  return found;
}

/**
 * Write 422 field detail into the matching form items.
 *
 * `resolve` decides which form item a server field belongs to; use
 * `buildServerFieldResolver` when the form items are dynamic. Returning
 * `undefined` leaves the field unmatched, so the caller can fall back to the
 * localized toast/alert message instead of guessing a control. Several server
 * fields that resolve to the same item are collapsed into one update.
 */
export function applyServerFieldErrors<T>(
  form: FormInstance<T>,
  error: unknown,
  resolve: (field: string) => FieldTarget | undefined,
  t: TFunction,
): boolean {
  const issues = extractServerFieldErrors(error);
  if (issues.length === 0) return false;

  const updates: Array<{ name: FieldNamePath; errors: string[] }> = [];
  const seen = new Set<string>();
  for (const issue of issues) {
    const target = resolve(issue.field);
    if (!target) continue;
    const key = typeof target.name === "string" ? target.name : target.name.join(".");
    if (seen.has(key)) continue;
    seen.add(key);
    // Without a localized label the raw backend field name is still more
    // actionable than the generic "check the highlighted fields" sentence —
    // the inline error must never be that sentence itself (it would tell the
    // user to look at red text while being the only red text).
    updates.push({
      name: target.name,
      errors: [
        t("common:serverError.invalidField", {
          field:
            target.label ??
            serverFieldLeaf(typeof target.name === "string" ? target.name : target.name.join(".")),
        }),
      ],
    });
  }
  if (updates.length === 0) return false;

  form.setFields(updates as Parameters<typeof form.setFields>[0]);
  return true;
}

/**
 * 可展示的本地化错误文案。
 *
 * - HTTP 错误（后端响应）走状态/错误码映射，绝不回显后端英文原文；
 * - 前端本地抛出的 Error（如 WebAuthn/校验）保留其已本地化的 message；
 * - 其他未知值回退到调用方给出的 i18n key。
 */
export function localizedError(error: unknown, t: TFunction, fallbackKey: string): string {
  if (looksLikeHttpError(error)) return serverErrorMessage(error, t, fallbackKey);
  if (error instanceof Error && error.message) return error.message;
  return t(fallbackKey);
}

/**
 * Localized sentence for a failed request.
 *
 * The backend message is never returned: it is English prose written for
 * operators, and the console renders in fourteen languages. Status-only
 * mapping plus the caller's localized fallback keeps every failure readable.
 */
export function serverErrorMessage(error: unknown, t: TFunction, fallbackKey: string): string {
  if (!looksLikeHttpError(error)) return t(fallbackKey);
  // 具体错误码优先：可识别的语义不能被通用状态码兜底覆盖。
  if (error.code === "already_exists") return t("common:serverError.alreadyExists");
  // 登录锁定（423 account_locked）与普通 401/403 语义不同：提示等待而非重试凭据。
  if (error.code === "account_locked" || error.status === 423)
    return t("common:serverError.accountLocked");
  if (error.status === 401) return t("common:serverError.unauthorized");
  if (error.status === 403) return t("common:forbidden");
  if (error.status === 404) return t("common:resourceNotFound");
  if (error.status === 409) return t("common:serverError.conflict");
  // 规范 4.10：412 = 版本前提不符（刷新后重试）；428 = 写请求缺版本前提。
  if (error.status === 412) return t("common:serverError.preconditionFailed");
  if (error.status === 422) return t("common:serverError.validation");
  if (error.status === 428) return t("common:serverError.preconditionRequired");
  if (error.status === 429) {
    // Retry-After 已在 http.ts 解析为 retryAfterSeconds；有值就给出可操作的等待秒数。
    const seconds = error.retryAfterSeconds;
    return typeof seconds === "number" && seconds > 0
      ? t("common:tooManyRequestsRetry", { seconds })
      : t("common:tooManyRequests");
  }
  // 上游厂商真实错误码（如阿里云 OCR/CloudAuth）经错误 Meta 透出：公开机器码，
  // 回显不泄露厂商自然语言消息，也比泛化 5xx 兜底更能定位开通/授权问题。
  const providerCode = error.meta?.providerCode;
  if (typeof providerCode === "string" && providerCode !== "")
    return t("common:serverError.providerRejected", { code: providerCode });
  if (error.status >= 500) return t("common:serverError.unavailable");
  return t(fallbackKey);
}
