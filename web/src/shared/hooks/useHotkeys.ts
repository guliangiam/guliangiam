import { useEffect, useCallback } from "react";

type ModKey = "mod" | "ctrl" | "alt" | "shift";

/**
 * Register a global keyboard shortcut.
 * Use "mod" to match both Cmd (Mac) and Ctrl (Windows/Linux).
 */
export function useHotkeys(keys: string, callback: (e: KeyboardEvent) => void) {
  const handler = useCallback(
    (e: KeyboardEvent) => {
      const parts = keys.toLowerCase().split("+");
      const key = parts[parts.length - 1];
      const modifiers: Set<ModKey> = new Set(parts.slice(0, -1) as ModKey[]);

      const modPressed = modifiers.has("mod") ? e.metaKey || e.ctrlKey : true;
      const ctrlPressed = modifiers.has("ctrl") ? e.ctrlKey : true;
      const altPressed = modifiers.has("alt") ? e.altKey : true;
      const shiftPressed = modifiers.has("shift") ? e.shiftKey : true;

      if (e.key.toLowerCase() === key && modPressed && ctrlPressed && altPressed && shiftPressed) {
        callback(e);
      }
    },
    [keys, callback],
  );

  useEffect(() => {
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [handler]);
}
