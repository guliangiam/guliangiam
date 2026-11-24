/**
 * Viewport screenshot capture for the feedback flow.
 *
 * The launcher rasterises the page the user is looking at *before* navigating to
 * the ticket form, so the attachment shows the reported screen rather than the
 * form itself. The captured File is handed across the SPA navigation through a
 * module-level singleton (a data URL would risk blowing the sessionStorage quota
 * on a full-viewport PNG).
 *
 * html2canvas-pro is imported lazily: it is only pulled into the bundle when a
 * capture actually runs, keeping the initial load free of its weight. Capture is
 * best-effort — every caller degrades to "open the form without a screenshot"
 * when it throws, so a rendering quirk never blocks feedback submission.
 */

let pendingCapture: File | null = null;

/** Stash the just-captured screenshot for the ticket form to consume on mount. */
export function setPendingCapture(file: File): void {
  pendingCapture = file;
}

/** Take the stashed screenshot once; subsequent reads return null. */
export function consumePendingCapture(): File | null {
  const file = pendingCapture;
  pendingCapture = null;
  return file;
}

const FEEDBACK_UI_SELECTOR = "[data-feedback-ui='true']";

/**
 * Rasterise the current viewport into a PNG File. Feedback chrome (the launcher
 * itself) is excluded so it never appears in its own screenshot.
 */
export async function captureViewport(): Promise<File> {
  const { default: html2canvas } = await import("html2canvas-pro");
  const canvas = await html2canvas(document.body, {
    backgroundColor: null,
    // Cap the scale so a 3x phone does not produce a huge, slow canvas.
    scale: Math.min(window.devicePixelRatio || 1, 2),
    logging: false,
    useCORS: true,
    ignoreElements: (element) => element.closest(FEEDBACK_UI_SELECTOR) !== null,
    width: window.innerWidth,
    height: window.innerHeight,
    windowWidth: window.innerWidth,
    windowHeight: window.innerHeight,
    x: window.scrollX,
    y: window.scrollY,
    scrollX: window.scrollX,
    scrollY: window.scrollY,
  });

  const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, "image/png"));
  if (!blob) throw new Error("screenshot capture produced no image data");

  const stamp = new Date().toISOString().replace(/[:.]/g, "-");
  return new File([blob], `screenshot-${stamp}.png`, { type: "image/png" });
}
