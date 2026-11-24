import { useCallback, useState } from "react";
import { storeSigninUrlFromLocation } from "@/shared/lib/auth-signin-url";

export function useCasLoginPageActions() {
  const [preferredLanguage, setPreferredLanguage] = useState(() => {
    if (typeof window !== "undefined") {
      return window.localStorage.getItem("preferredLanguage") || navigator.language || "en";
    }

    return "en";
  });

  const changeLanguage = useCallback((nextLanguage: string) => {
    setPreferredLanguage(nextLanguage);
    if (typeof window !== "undefined") {
      window.localStorage.setItem("preferredLanguage", nextLanguage);
      window.location.reload();
    }
  }, []);

  const storeSigninUrl = useCallback(() => {
    if (typeof window === "undefined") {
      return;
    }

    storeSigninUrlFromLocation({
      pathname: window.location.pathname,
      search: window.location.search,
    });
  }, []);

  return {
    changeLanguage,
    preferredLanguage,
    storeSigninUrl,
  };
}
