import { ANALYTICS_WEB_VITALS } from "@/shared/api/paths";
/**
 * Web Vitals monitoring — lazy loaded to avoid adding web-vitals
 * to the critical rendering path.
 *
 * The `web-vitals` library (~1KB gzipped) is dynamically imported
 * only when initWebVitals() is called, allowing the bundler to
 * place it in a separate chunk.
 */

interface WebVitalMetric {
  name: string;
  value: number;
  rating: string;
  delta: number;
  id: string;
}

function sendToAnalytics(metric: WebVitalMetric) {
  // Send to analytics endpoint or console in dev
  if (import.meta.env.DEV) {
    console.warn(`[WebVitals] ${metric.name}: ${metric.value} (${metric.rating})`);
  }
  // Production: send to analytics API
  const body = JSON.stringify({
    name: metric.name,
    value: Math.round(metric.value),
    rating: metric.rating,
    delta: Math.round(metric.delta),
    id: metric.id,
    path: window.location.pathname,
    timestamp: Date.now(),
  });
  if (navigator.sendBeacon) {
    navigator.sendBeacon(ANALYTICS_WEB_VITALS, body);
  }
}

export function initWebVitals() {
  // Dynamic import ensures web-vitals is code-split into its own chunk
  import("web-vitals").then(({ onLCP, onCLS, onINP, onTTFB, onFCP }) => {
    onLCP(sendToAnalytics);
    onCLS(sendToAnalytics);
    onINP(sendToAnalytics);
    onTTFB(sendToAnalytics);
    onFCP(sendToAnalytics);
    return undefined;
  });
}
