import type {
  ServiceAccountCredentials,
  ServiceAccountItem,
} from "@/shared/api/iam/identity-pools";

export function formatWorkloadDate(value: string | null): string {
  if (!value) return "-";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

export function isValidHttpsUri(value: string): boolean {
  if (!value || /\s/.test(value)) return false;
  try {
    const parsed = new URL(value);
    return parsed.protocol === "https:" && Boolean(parsed.hostname);
  } catch {
    return false;
  }
}

export function isValidSubjectPattern(pattern: string): boolean {
  if (!pattern || /\s/.test(pattern)) return false;
  const wildcardCount = pattern.split("*").length - 1;
  if (wildcardCount === 0) return true;
  return wildcardCount === 1 && pattern.length > 2 && pattern.endsWith("/*");
}

export function getServiceAccountCredentials(
  account: ServiceAccountItem,
): ServiceAccountCredentials {
  if (!account.clientSecretPlain) {
    throw new Error("The server did not return the one-time client secret");
  }
  return {
    clientId: account.clientId,
    clientSecretPlain: account.clientSecretPlain,
  };
}
