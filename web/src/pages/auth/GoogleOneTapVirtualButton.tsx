import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import type { ApplicationItem } from "@/shared/types/domain";
import { type DirectAuthProvider, getDirectOAuthProviderAuthUrl } from "@/shared/lib/provider-auth";

type GoogleCredentialResponse = {
  clientId?: string;
  credential?: string;
  select_by?: string;
};

type GoogleIdentityApi = {
  accounts: {
    id: {
      cancel?: () => void;
      initialize: (options: {
        callback: (response: GoogleCredentialResponse) => void;
        client_id: string;
      }) => void;
      prompt: () => void;
    };
  };
};

declare global {
  interface Window {
    google?: GoogleIdentityApi;
  }
}

type GoogleOneTapVirtualButtonProps = {
  application: ApplicationItem;
  provider: DirectAuthProvider;
};

function loadGoogleIdentityApi(t: (key: string) => string) {
  if (window.google?.accounts?.id) {
    return Promise.resolve(window.google);
  }

  return new Promise<GoogleIdentityApi>((resolve, reject) => {
    const scriptId = "guliangiam-google-one-tap";
    const existingScript = document.getElementById(scriptId) as HTMLScriptElement | null;

    const handleLoad = () => {
      if (window.google) {
        resolve(window.google);
        return;
      }

      reject(new Error(t("googleOneTapLoadFailed")));
    };

    if (existingScript) {
      existingScript.addEventListener("load", handleLoad, { once: true });
      existingScript.addEventListener(
        "error",
        () => reject(new Error(t("googleOneTapScriptFailed"))),
        { once: true },
      );
      return;
    }

    const script = document.createElement("script");
    script.id = scriptId;
    script.src = "https://accounts.google.com/gsi/client";
    script.async = true;
    script.defer = true;
    script.addEventListener("load", handleLoad, { once: true });
    script.addEventListener("error", () => reject(new Error(t("googleOneTapScriptFailed"))), {
      once: true,
    });
    document.head.appendChild(script);
  });
}

export default function GoogleOneTapVirtualButton({
  application,
  provider,
}: GoogleOneTapVirtualButtonProps) {
  const { t } = useTranslation("auth");

  useEffect(() => {
    const clientId = provider.clientId;
    if (!clientId) {
      return;
    }

    let cancelled = false;

    void (async () => {
      const authUrl = await getDirectOAuthProviderAuthUrl(application, provider, "signup");
      if (cancelled || !authUrl) {
        return;
      }

      const authParams = new URL(authUrl, window.location.origin).searchParams;
      const state = authParams.get("state");
      const redirectUri = authParams.get("redirect_uri");
      if (!state || !redirectUri) {
        return;
      }

      const googleApi = await loadGoogleIdentityApi(t);
      if (cancelled) {
        return;
      }

      googleApi.accounts.id.initialize({
        client_id: clientId,
        callback: (response) => {
          const nextUrl = new URL(redirectUri, window.location.origin);
          nextUrl.searchParams.set("state", state);
          nextUrl.searchParams.set("code", `GoogleIdToken-${JSON.stringify(response)}`);
          window.location.assign(nextUrl.toString());
        },
      });
      googleApi.accounts.id.prompt();
    })().catch(() => {
      return;
    });

    return () => {
      cancelled = true;
      window.google?.accounts?.id?.cancel?.();
    };
  }, [application, provider, t]);

  return null;
}
