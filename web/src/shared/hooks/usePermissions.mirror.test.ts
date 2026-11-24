import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

import { ADMIN_FULL_PERMISSIONS } from "./usePermissions";

// ────────────────────────────────────────────────────────────────────
// ADMIN_FULL_PERMISSIONS is the frontend fallback copy of the backend
// adminPermissionSet(). Whenever a permission point is registered in
// contracts/registries/permissions.yaml, the mirror must carry the
// `<frontend_prefix>:<action>` key, otherwise PermissionGuard hides the
// control for admins while the permissions API is unavailable.
//
// The C2 batch added five resources; this test keeps that mirror honest
// by deriving the expected keys from the registry itself instead of
// repeating them in a second hand-maintained list.
// ────────────────────────────────────────────────────────────────────

const REPO_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../../..");
const REGISTRY_PATH = path.join(REPO_ROOT, "contracts/registries/permissions.yaml");

type RegistryRow = {
  domain: string;
  resource: string;
  actions: string[];
  frontendPrefix?: string;
};

/** Minimal reader for the flat domain → resource → actions shape used by the registry. */
function parsePermissionsRegistry(source: string): RegistryRow[] {
  const rows: RegistryRow[] = [];
  let domain = "";
  let current: RegistryRow | undefined;

  for (const rawLine of source.split(/\r?\n/)) {
    const line = rawLine.replace(/#.*$/, "").trimEnd();
    if (!line.trim()) continue;

    if (/^[A-Za-z_][\w-]*:\s*$/.test(line)) {
      domain = line.slice(0, -1).trim();
      current = undefined;
      continue;
    }
    if (/^\s{2}[A-Za-z_][\w-]*:\s*$/.test(line)) {
      const resource = line.trim().slice(0, -1);
      current = { domain, resource, actions: [] };
      rows.push(current);
      continue;
    }
    if (!current) continue;

    const actions = line.trim().match(/^actions:\s*\[(.*)\]\s*$/);
    if (actions) {
      current.actions = actions[1]
        .split(",")
        .map((action) => action.trim())
        .filter(Boolean);
      continue;
    }
    const prefix = line.trim().match(/^frontend_prefix:\s*"([^"]+)"/);
    if (prefix) current.frontendPrefix = prefix[1];
  }

  return rows;
}

const C2_RESOURCES = [
  "adapters",
  "pay-recon",
  "pay-recon-items",
  "pay-channel-configs",
  "translations",
];
const C2_PREFIXES = ["adapter", "pay-recon", "pay-recon-items", "pay-channel-configs", "i18n"];

describe("ADMIN_FULL_PERMISSIONS mirror", () => {
  it("matches the C2 registry action set (both directions)", () => {
    const rows = parsePermissionsRegistry(readFileSync(REGISTRY_PATH, "utf8")).filter((row) =>
      C2_RESOURCES.includes(row.resource),
    );

    expect(rows).toHaveLength(C2_RESOURCES.length);
    for (const row of rows) {
      expect(row.frontendPrefix).toBeTruthy();
    }

    const registryKeys = rows
      .flatMap((row) => row.actions.map((action) => `${row.frontendPrefix}:${action}`))
      .sort();

    // Guard against a parser that silently yields an empty set.
    expect(registryKeys).toEqual([
      "adapter:delete",
      "adapter:read",
      "adapter:write",
      "i18n:read",
      "i18n:write",
      "pay-channel-configs:read",
      "pay-channel-configs:write",
      "pay-recon-items:approve",
      "pay-recon-items:write",
      "pay-recon:write",
    ]);

    const mirrorKeys = ADMIN_FULL_PERMISSIONS.actions
      .filter((action) => C2_PREFIXES.includes(action.split(":")[0]))
      .sort();

    // Equality in both directions: no registered action missing from the
    // fallback, and no stale C2 key left behind in the fallback.
    expect(mirrorKeys).toEqual(registryKeys);
  });

  it("mirrors the C2 Casbin resources used by guard slots", () => {
    for (const resource of [
      "adapters",
      "pay-recon",
      "pay-recon-items",
      "pay-channel-configs",
      "i18n",
    ]) {
      expect(ADMIN_FULL_PERMISSIONS.resources).toContain(resource);
    }
  });
});
