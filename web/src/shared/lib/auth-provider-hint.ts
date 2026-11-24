import { resolveProviderHintFallbackUrl } from "@/shared/lib/auth-route-context";

type ProviderHintRedirectWindow = Window & {
  GuliangIAMProviderHintRedirect?: {
    run: () => void;
  };
};

export function runProviderHintRedirect(values: {
  currentHref: string;
  scriptId?: string;
  scriptSrc?: string;
  windowObject?: Window;
  documentObject?: Document;
}) {
  const windowObject = (values.windowObject ?? window) as ProviderHintRedirectWindow;
  const documentObject = values.documentObject ?? document;
  const scriptId = values.scriptId ?? "guliangiam-provider-hint-redirect";
  const scriptSrc = values.scriptSrc ?? "/ProviderHintRedirect.js";
  const fallbackUrl = resolveProviderHintFallbackUrl(values.currentHref);
  let cancelled = false;

  const fallback = () => {
    if (!cancelled) {
      windowObject.location.replace(fallbackUrl);
    }
  };

  const runRedirect = () => {
    const redirectApi = windowObject.GuliangIAMProviderHintRedirect;
    if (!redirectApi || typeof redirectApi.run !== "function") {
      fallback();
      return;
    }

    redirectApi.run();
  };

  const existingScript = documentObject.getElementById(scriptId) as HTMLScriptElement | null;
  if (existingScript) {
    if (existingScript.dataset.loaded === "true") {
      runRedirect();
      return () => {
        cancelled = true;
      };
    }

    existingScript.addEventListener("load", runRedirect, { once: true });
    existingScript.addEventListener("error", fallback, { once: true });
    return () => {
      cancelled = true;
      existingScript.removeEventListener("load", runRedirect);
      existingScript.removeEventListener("error", fallback);
    };
  }

  const script = documentObject.createElement("script");
  const handleLoad = () => {
    script.dataset.loaded = "true";
    runRedirect();
  };

  script.id = scriptId;
  script.src = scriptSrc;
  script.async = true;
  script.addEventListener("load", handleLoad, { once: true });
  script.addEventListener("error", fallback, { once: true });
  documentObject.body.appendChild(script);

  return () => {
    cancelled = true;
    script.removeEventListener("load", handleLoad);
    script.removeEventListener("error", fallback);
  };
}
