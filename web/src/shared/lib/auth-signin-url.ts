const SIGNIN_URL_STORAGE_KEY = "signinUrl";

function resolveSigninUrlStorage(storage?: Storage) {
  if (storage) {
    return storage;
  }

  if (typeof window === "undefined") {
    return null;
  }

  return window.sessionStorage;
}

export function loadStoredSigninUrl(storage?: Storage) {
  return resolveSigninUrlStorage(storage)?.getItem(SIGNIN_URL_STORAGE_KEY) ?? "";
}

export function storeSigninUrlFromLocation(values: {
  pathname: string;
  search: string;
  storage?: Storage;
}) {
  const targetStorage = resolveSigninUrlStorage(values.storage);
  if (!targetStorage) {
    return;
  }

  targetStorage.setItem(SIGNIN_URL_STORAGE_KEY, `${values.pathname}${values.search}`);
}

export function resolvePromptRedirectUrl(values: { search: string; storedSigninUrl?: string }) {
  const params = new URLSearchParams(values.search);
  const redirectUri = params.get("redirectUri");
  const code = params.get("code");
  const state = params.get("state");
  const oauth = params.get("oauth");

  if (!redirectUri || !code || !state) {
    return oauth === "true" ? values.storedSigninUrl || "" : "";
  }

  const target = new URL(redirectUri);
  target.searchParams.set("code", code);
  target.searchParams.set("state", state);
  return target.toString();
}
