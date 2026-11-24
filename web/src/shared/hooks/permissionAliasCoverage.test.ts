import fs from "node:fs";
import path from "node:path";

const SRC_ROOT = path.resolve(__dirname, "../..");
const ALIAS_FILE = path.join(SRC_ROOT, "shared/hooks/usePermissions.ts");

// These files intentionally contain singular `user:*` tokens that are not
// permission guards: the alias table, the Zoom OAuth scope string, and this
// scanner (which spells the tokens it searches for).
const NON_GUARD_FILES = new Set([
  "shared/hooks/usePermissions.ts",
  "shared/lib/provider-auth.ts",
  "shared/hooks/permissionAliasCoverage.test.ts",
]);

function collectSourceFiles(dir: string): string[] {
  const files: string[] = [];
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      files.push(...collectSourceFiles(full));
      continue;
    }
    if (entry.name.endsWith(".ts") || entry.name.endsWith(".tsx")) {
      files.push(full);
    }
  }
  return files;
}

// Parse the declared alias groups from the implementation file so this test
// fails when the map is trimmed without migrating the remaining guards.
function readAliasGroups(): Array<readonly [string, string]> {
  const groups: Array<readonly [string, string]> = [];
  for (const match of fs
    .readFileSync(ALIAS_FILE, "utf8")
    .matchAll(/\["(users:[a-z]+)",\s*"(user:[a-z]+)"\]/g)) {
    groups.push([match[1], match[2]]);
  }
  return groups;
}

// DELETE /users/{id} is guarded by the canonical plural `users:delete`
// (routes.go + permissions.yaml users domain), which needs no alias. The
// alias table therefore has no singular/plural delete pair, and singular
// `user:delete` may only appear as a test fixture, never as a production
// guard; the assertion below enforces that.
const SINGULAR_TO_CANONICAL = new Map<string, string>([
  ["user:read", "users:read"],
  ["user:write", "users:write"],
]);

describe("permission alias coverage", () => {
  it("every singular user:* guard in web/src is covered by an alias group", () => {
    const aliasGroups = readAliasGroups();
    expect(aliasGroups.length).toBeGreaterThan(0);

    const hits: Array<{ location: string; file: string; singular: string }> = [];
    for (const file of collectSourceFiles(SRC_ROOT)) {
      const relative = path.relative(SRC_ROOT, file).split(path.sep).join("/");
      if (NON_GUARD_FILES.has(relative)) continue;

      fs.readFileSync(file, "utf8")
        .split("\n")
        .forEach((line, index) => {
          for (const match of line.matchAll(/user:(read|write|delete)/g)) {
            hits.push({ location: `${relative}:${index + 1}`, file: relative, singular: match[0] });
          }
        });
    }

    expect(hits.length).toBeGreaterThan(0);

    const uncovered: string[] = [];
    const misplacedDeletes: string[] = [];

    for (const hit of hits) {
      const canonical = SINGULAR_TO_CANONICAL.get(hit.singular);
      if (!canonical) {
        // user:delete — production guards use the plural `users:delete`, so
        // the singular spelling must not appear outside a test fixture.
        const isTestFixture = /\.test\.tsx?$/.test(hit.file);
        // eslint-disable-next-line no-console -- 输出逐条命中证据，供门禁/审计复核。
        console.log(
          `ALIAS_COVERAGE ${hit.location} ${hit.singular} -> no canonical key (production guards use plural users:delete) covered=n/a testFixture=${isTestFixture}`,
        );
        if (!isTestFixture) {
          misplacedDeletes.push(
            `${hit.location} uses ${hit.singular} outside a test fixture: production guards must use the canonical plural users:delete`,
          );
        }
        continue;
      }

      const covered = aliasGroups.some(
        ([canonicalKey, singular]) => canonicalKey === canonical && singular === hit.singular,
      );
      // eslint-disable-next-line no-console -- 输出逐条命中证据，供门禁/审计复核。
      console.log(
        `ALIAS_COVERAGE ${hit.location} ${hit.singular} -> ${canonical} covered=${covered}`,
      );
      if (!covered) {
        uncovered.push(
          `${hit.location} declares ${hit.singular} but no alias maps it to ${canonical}`,
        );
      }
    }

    expect(uncovered, `aliases missing for:\n${uncovered.join("\n")}`).toEqual([]);
    expect(
      misplacedDeletes,
      `singular deletes outside fixtures:\n${misplacedDeletes.join("\n")}`,
    ).toEqual([]);
  });
});
