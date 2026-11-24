import { useCallback, useEffect, useRef, useState } from "react";
import type { FormInstance } from "antd";

/**
 * Draft auto-save hook for edit forms.
 *
 * Extracts the core value prop from GlEditShell (草稿自动保存 + debounce 2s → localStorage)
 * into a reusable hook that can be consumed by ANY form — including ResourceFormPage,
 * standalone DetailPage Tabs, and future GlEditShell replacements.
 *
 * Design goals:
 *   - Tiny surface area: just (form, opts) → { hasDraft, dirty, lastSavedAt, clearDraft, bindValuesChange }
 *   - Plays nice with useUnsavedChangesGuard (route guard lives there, dirty flag drives it)
 *   - Version-aware: bumps on schema change → old drafts are discarded
 *   - Silent on errors: localStorage quota / parse failures never break the form
 */

export interface UseGlDraftOptions {
  /** localStorage key. Auto-prefixed `gl:draft:` if bare. */
  draftKey?: string;
  /** Route + entityId convenience — builds key as `gl:draft:{route}:{entityId}` */
  route?: string;
  entityId?: string;
  /** Enable/disable switch — disable in CreatePage or sensitive forms (password, MFA) */
  enable?: boolean;
  /** Debounce window in ms. Default 2000. */
  debounceMs?: number;
  /** Draft schema version. Bump to invalidate stale drafts after form-field changes. */
  version?: number;
  /** Auto-restore draft on mount. Default true. */
  autoRestore?: boolean;
  /**
   * 不写入/不恢复的字段（如 revision、凭据、密钥）。
   * 这些字段必须始终以服务端当前值为准，不能被旧草稿覆盖。
   */
  ignoreKeys?: string[];
}

export interface UseGlDraftResult {
  /** Has the form been modified since load? (drives useUnsavedChangesGuard) */
  dirty: boolean;
  /** True if we just restored a saved-from-localStorage draft on mount */
  hasDraft: boolean;
  /** Timestamp of the last successful localStorage setItem. null = never saved. */
  lastSavedAt: number | null;
  /** Clear localStorage draft + reset dirty state. Call after successful submit. */
  clearDraft: () => void;
  /**
   * Form onValuesChange handler — attach directly to <Form onValuesChange={bindValuesChange}>.
   * Debounced writes to localStorage + flips dirty flag instantly.
   */
  bindValuesChange: (changed: unknown, all: Record<string, unknown>) => void;
}

const DEFAULT_DEBOUNCE_MS = 2000;
const DEFAULT_VERSION = 1;
// 凭据字段默认不落盘（clientSecret/password/sshPassword/apiKey/privateKey/masterVerificationCode 等），
// 调用方无需逐个登记 ignoreKeys；显式 ignoreKeys 仍然叠加生效。
const SENSITIVE_KEY_PATTERN = /secret|password|key$|verificationCode/i;

function isPlainObject(value: unknown): value is Record<string, unknown> {
  if (value === null || typeof value !== "object") return false;
  const proto = Object.getPrototypeOf(value) as unknown;
  return proto === Object.prototype || proto === null;
}

/** Build the resolved localStorage key from opts. Returns null when draft disabled. */
function resolveKey(opts: UseGlDraftOptions): string | null {
  if (opts.enable === false) return null;
  if (opts.draftKey) return opts.draftKey;
  const route = opts.route ?? "unknown";
  const id = opts.entityId;
  if (id === undefined || id === null || id === "") return null;
  return `gl:draft:${route}:${id}`;
}

interface DraftEnvelope {
  v: number;
  values: Record<string, unknown>;
  savedAt: number;
}

export function useGlDraft(
  form: FormInstance | null | undefined,
  opts: UseGlDraftOptions,
): UseGlDraftResult {
  const { debounceMs = DEFAULT_DEBOUNCE_MS, version = DEFAULT_VERSION, autoRestore = true } = opts;

  const key = resolveKey(opts);
  // 依赖以稳定字符串参与 effect，避免调用方每次渲染传入新数组导致重复恢复。
  const ignoreKeysKey = (opts.ignoreKeys ?? []).join(",");
  const stripIgnored = useCallback(
    (values: Record<string, unknown>): Record<string, unknown> => {
      const ignored = ignoreKeysKey ? ignoreKeysKey.split(",") : [];
      const strip = (input: Record<string, unknown>): Record<string, unknown> => {
        const next: Record<string, unknown> = {};
        for (const [field, value] of Object.entries(input)) {
          if (ignored.includes(field) || SENSITIVE_KEY_PATTERN.test(field)) continue;
          next[field] = isPlainObject(value) ? strip(value) : value;
        }
        return next;
      };
      return strip(values);
    },
    [ignoreKeysKey],
  );

  const [dirty, setDirty] = useState(false);
  const [hasDraft, setHasDraft] = useState(false);
  const [lastSavedAt, setLastSavedAt] = useState<number | null>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // --- Mount: restore draft ------------------------------------------------
  useEffect(() => {
    if (!autoRestore || !key || !form) return;
    try {
      const raw = localStorage.getItem(key);
      if (!raw) return;
      const env = JSON.parse(raw) as DraftEnvelope;
      if (!env || typeof env !== "object") {
        localStorage.removeItem(key);
        return;
      }
      if (env.v !== version) {
        // schema drifted → drop stale draft
        localStorage.removeItem(key);
        return;
      }
      if (env.values && typeof env.values === "object") {
        const restored = stripIgnored(env.values);
        form.setFieldsValue(restored);
        // 旧版本草稿可能已把明文凭据落盘：恢复时顺手重写为脱敏后的内容。
        if (JSON.stringify(restored) !== JSON.stringify(env.values)) {
          try {
            localStorage.setItem(key, JSON.stringify({ ...env, values: restored }));
          } catch {
            /* noop */
          }
        }
        // One-time mount restore; no dependency can change without `key` changing.
        // eslint-disable-next-line react/set-state-in-effect
        setHasDraft(true);
        // eslint-disable-next-line react/set-state-in-effect
        setLastSavedAt(env.savedAt ?? null);
      }
    } catch {
      // corrupt → silently discard
      try {
        localStorage.removeItem(key);
      } catch {
        /* noop */
      }
    }
  }, [key, version, form, autoRestore, stripIgnored]);

  // --- Unmount: flush pending debounce timer ------------------------------
  useEffect(
    () => () => {
      if (debounceRef.current) {
        clearTimeout(debounceRef.current);
        debounceRef.current = null;
      }
    },
    [],
  );

  // --- onValuesChange handler ----------------------------------------------
  const bindValuesChange = useCallback(
    (_changed: unknown, all: Record<string, unknown>) => {
      setDirty(true);

      if (!key) return; // draft disabled → only track dirty

      // debounced write
      if (debounceRef.current) clearTimeout(debounceRef.current);
      debounceRef.current = setTimeout(() => {
        try {
          const now = Date.now();
          const env: DraftEnvelope = { v: version, values: stripIgnored(all), savedAt: now };
          localStorage.setItem(key, JSON.stringify(env));
          setHasDraft(true);
          setLastSavedAt(now);
        } catch {
          // quota exceeded, private mode, etc. — silent fail
        }
      }, debounceMs);
    },
    [key, version, debounceMs, stripIgnored],
  );

  // --- clearDraft ----------------------------------------------------------
  const clearDraft = useCallback(() => {
    if (key) {
      try {
        localStorage.removeItem(key);
      } catch {
        /* noop */
      }
    }
    if (debounceRef.current) {
      clearTimeout(debounceRef.current);
      debounceRef.current = null;
    }
    setHasDraft(false);
    setLastSavedAt(null);
    setDirty(false);
  }, [key]);

  return { dirty, hasDraft, lastSavedAt, clearDraft, bindValuesChange };
}
