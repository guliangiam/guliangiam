import { useState, useRef, useCallback, useEffect } from "react";
import { Button } from "antd";
import { useTranslation } from "react-i18next";

type ConfirmButtonProps = {
  /** Async confirm handler — shows loading state during execution. */
  onConfirm: () => void | Promise<void>;
  /** Button label in its primary state (e.g. "Delete"). */
  children?: React.ReactNode;
  /** Danger variant (default: true). */
  danger?: boolean;
  /** Size. */
  size?: "small" | "middle" | "large";
  /** Disabled in the primary state. */
  disabled?: boolean;
  /** Time before auto-reset, in ms (default: 5000). */
  timeout?: number;
  /** Custom confirm-phase label override. */
  confirmLabel?: string;
};

/**
 * ConfirmButton – inline two-state confirmation.
 *
 * State machine:
 *   [PRIMARY] click → [READY] (shows confirmLabel, countdown) → confirm → async handler → reset
 *   [PRIMARY] click → [READY] → second click = confirm handler
 *   [PRIMARY] click → [READY] → Esc / outside click → reset
 *   [PRIMARY] click → [READY] → timeout → reset
 *
 * Compared to AntD Popconfirm:
 *   ✅ No dropdown → no mouse travel → faster for list rows
 *   ✅ Works with async handlers (shows loading)
 *   ✅ Keyboard-friendly (Esc cancels, Enter confirms)
 *   ✅ Lightweight — zero extra popups on the DOM
 *
 * Use case: every-row delete buttons in a bulk action table. Prefer
 * PopconfirmModal for one-shot confirmations (more explicit description
 * + warning Alert).
 */
export function ConfirmButton({
  onConfirm,
  children,
  danger = true,
  size = "small",
  disabled,
  timeout = 5000,
  confirmLabel,
}: ConfirmButtonProps) {
  const { t } = useTranslation();
  const [confirming, setConfirming] = useState(false);
  const [running, setRunning] = useState(false);
  const rootRef = useRef<HTMLSpanElement>(null);
  const runningRef = useRef(false);
  const onConfirmRef = useRef(onConfirm);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    onConfirmRef.current = onConfirm;
  }, [onConfirm]);

  const reset = useCallback(() => {
    setConfirming(false);
    if (timerRef.current !== null) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  }, []);

  // Auto-reset on timer
  useEffect(() => {
    if (!confirming || running) return;
    timerRef.current = setTimeout(reset, timeout);
    return () => {
      if (timerRef.current !== null) clearTimeout(timerRef.current);
    };
  }, [confirming, running, timeout, reset]);

  // Escape cancels; the focused button handles Enter/Space natively.
  useEffect(() => {
    if (!confirming) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !runningRef.current) reset();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [confirming, reset]);

  // Outside click resets back to primary state
  useEffect(() => {
    if (!confirming) return;
    const onClick = (e: MouseEvent) => {
      if (!runningRef.current && !rootRef.current?.contains(e.target as Node)) reset();
    };
    // Delay a tick so the primary click doesn't immediately reset
    const id = setTimeout(() => document.addEventListener("mousedown", onClick), 0);
    return () => {
      clearTimeout(id);
      document.removeEventListener("mousedown", onClick);
    };
  }, [confirming, reset]);

  async function handleConfirm() {
    if (runningRef.current) return;
    runningRef.current = true;
    setRunning(true);
    try {
      await onConfirmRef.current();
    } finally {
      runningRef.current = false;
      setRunning(false);
      reset();
    }
  }

  const label = confirming
    ? (confirmLabel ?? t("common:confirm", "Confirm"))
    : (children ?? t("common:delete", "Delete"));

  return (
    <span ref={rootRef}>
      <Button
        type={confirming ? "primary" : "text"}
        danger={!confirming && danger}
        size={size}
        disabled={running || (!confirming && disabled)}
        loading={running}
        onClick={() => (confirming ? void handleConfirm() : setConfirming(true))}
      >
        {label}
      </Button>
    </span>
  );
}

export default ConfirmButton;
