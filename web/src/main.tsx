import React from "react";
import ReactDOM from "react-dom/client";
import { RouterProvider } from "react-router-dom";
import { AppProviders } from "@/app/providers/AppProviders";
import { GlobalErrorBoundary } from "@/shared/ui/GlobalErrorBoundary";
import { initSentry, SentryErrorBoundary } from "@/shared/lib/sentry";
import { router } from "@/app/router/router";
import { ensureInitialLanguage } from "@/shared/i18n";
import "@/styles.css";

// Initialize Sentry before React renders so that SDK captures errors early
initSentry();

// zh-CN is the only statically bundled locale; wait for the detected language
// (en, ja, …) before mounting so the first paint is already translated.
void (async () => {
  await ensureInitialLanguage();
  ReactDOM.createRoot(document.getElementById("root")!).render(
    <React.StrictMode>
      <SentryErrorBoundary
        fallback={
          <GlobalErrorBoundary>
            <span />
          </GlobalErrorBoundary>
        }
      >
        <AppProviders>
          <RouterProvider router={router} />
        </AppProviders>
      </SentryErrorBoundary>
    </React.StrictMode>,
  );
})();
