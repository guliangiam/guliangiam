const FILE_SIZE_UNITS = ["B", "KB", "MB", "GB", "TB", "PB"] as const;

/**
 * Render a byte count as a compact human-readable size (binary units).
 * Unknown or negative values fall back to `-` so tables keep a stable shape.
 */
export function formatFileSize(bytes?: number | null): string {
  if (bytes === null || bytes === undefined || !Number.isFinite(bytes) || bytes < 0) {
    return "-";
  }
  if (bytes === 0) return "0 B";

  const exponent = Math.min(
    Math.floor(Math.log(bytes) / Math.log(1024)),
    FILE_SIZE_UNITS.length - 1,
  );
  if (exponent <= 0) return `${Math.round(bytes)} B`;

  const value = bytes / 1024 ** exponent;
  const rounded = value.toFixed(2).replace(/\.?0+$/, "");
  return `${rounded} ${FILE_SIZE_UNITS[exponent]}`;
}
