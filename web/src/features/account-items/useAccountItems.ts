import { useMemo } from "react";
import type { AccountItem } from "@/shared/api/iam/organization";

type FieldPermission = {
  visible: boolean;
  disabled: boolean;
  regex?: string;
};

export type AccountItemsMap = Record<string, FieldPermission>;

type UseAccountItemsOptions = {
  accountItems?: AccountItem[];
  viewerRole: "self" | "admin";
};

export function useAccountItems({
  accountItems,
  viewerRole,
}: UseAccountItemsOptions): AccountItemsMap {
  return useMemo(() => {
    if (!accountItems || accountItems.length === 0) {
      return {};
    }

    const map: AccountItemsMap = {};

    for (const item of accountItems) {
      const visible = item.visible;

      // ViewRule determines who can see the field
      let canView = true;
      if (item.viewRule === "Admin") {
        canView = viewerRole === "admin";
      } else if (item.viewRule === "Self") {
        canView = true; // self always sees their own fields
      }

      // ModifyRule determines who can edit the field
      let disabled = false;
      if (item.modifyRule === "Immutable") {
        disabled = true;
      } else if (item.modifyRule === "Admin") {
        disabled = viewerRole !== "admin";
      }

      map[item.name] = {
        visible: visible && canView,
        disabled,
        regex: item.regex || undefined,
      };
    }

    return map;
  }, [accountItems, viewerRole]);
}

export function isFieldVisible(map: AccountItemsMap, fieldName: string): boolean {
  if (!map || Object.keys(map).length === 0) return true;
  const perm = map[fieldName];
  if (perm === undefined) return true;
  return perm.visible;
}

export function isFieldDisabled(map: AccountItemsMap, fieldName: string): boolean {
  if (!map || Object.keys(map).length === 0) return false;
  const perm = map[fieldName];
  if (perm === undefined) return false;
  return perm.disabled;
}

export function getFieldRegex(map: AccountItemsMap, fieldName: string): string | undefined {
  if (!map || Object.keys(map).length === 0) return undefined;
  return map[fieldName]?.regex;
}

// ─── Dynamic Tab helpers ────────────────────────────────────────

// Mapping from accountItem names to the page's business section keys.
const FIELD_TO_TAB_KEY: Record<string, string> = {
  Username: "basic",
  "Display name": "basic",
  "First name": "basic",
  "Last name": "basic",
  Email: "basic",
  Phone: "basic",
  "Country/Region": "basic",
  Location: "basic",
  Birthday: "basic",
  Gender: "basic",
  Bio: "basic",
  Homepage: "basic",
  Education: "basic",
  Language: "basic",
  Affiliation: "basic",
  Tag: "basic",
  Score: "account",
  Ranking: "account",
  Karma: "account",
  Balance: "assets",
  Currency: "assets",
  "Real name": "realname",
  "ID card type": "realname",
  "ID card": "realname",
  Password: "credentials",
  "Confirm password": "credentials",
  "Multi-factor authentication": "mfa",
  "Signup application": "basic",
  "Is admin": "account",
  "Is forbidden": "account",
  "Is deleted": "account",
  Type: "basic",
  Roles: "account",
  Groups: "account",
  Properties: "realname",
};

/** Extract unique tab values from accountItems, preserving insertion order */
export function getUniqueTabs(accountItems?: AccountItem[]): string[] {
  if (!accountItems || accountItems.length === 0) return [];
  const seen = new Set<string>();
  const tabs: string[] = [];
  for (const item of accountItems) {
    const tab = item.tab?.trim();
    if (tab && !seen.has(tab)) {
      seen.add(tab);
      tabs.push(tab);
    }
  }
  return tabs;
}

/** Get all accountItems that belong to a specific tab */
export function getAccountItemsByTab(accountItems: AccountItem[], tab: string): AccountItem[] {
  return accountItems.filter((item) => (item.tab?.trim() ?? "") === tab);
}

/** Map an accountItem tab name to one of the page's business section keys. */
export function mapTabNameToKey(tabName: string): string {
  const lower = tabName.toLowerCase();
  if (lower.includes("mfa") || lower.includes("authenticator")) return "mfa";
  if (lower.includes("credential") || lower.includes("password") || lower.includes("security"))
    return "credentials";
  if (lower.includes("session") || lower.includes("consent")) return "sessions";
  if (
    lower.includes("finance") ||
    lower.includes("payment") ||
    lower.includes("balance") ||
    lower.includes("managed")
  )
    return "assets";
  if (
    lower.includes("real") ||
    lower.includes("id") ||
    lower.includes("custom") ||
    lower.includes("propert") ||
    lower.includes("identity")
  )
    return "realname";
  if (lower.includes("account") || lower.includes("role") || lower.includes("group"))
    return "account";
  // basic / info / profile / personal / contact 都归入资料分区。
  return "basic";
}

/** Get the hardcoded tab key for a given accountItem name */
export function getFieldTabKey(fieldName: string): string {
  return FIELD_TO_TAB_KEY[fieldName] ?? "basic";
}
