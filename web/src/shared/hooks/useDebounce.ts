import { useEffect, useState } from "react";

/**
 * useDebounce — debounces a rapidly-changing value.
 * Returns the debounced value after `delay` ms of no updates.
 *
 * Used by MenuSearch to avoid re-rendering results on every keystroke.
 * Debounce pattern matches legacy CommandPalette (300ms).
 */
export function useDebounce<T>(value: T, delay = 300): T {
  const [debounced, setDebounced] = useState(value);

  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(timer);
  }, [value, delay]);

  return debounced;
}
