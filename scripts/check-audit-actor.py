#!/usr/bin/env python3
"""Reject audit call sites that pass a display name where a UUID is required.

Why this exists
---------------
`guliangiam.records.actor_id` is a `UUID` column. Audit calls that pass a user's
*display name* (`user.Username`) instead of their UUID compile, pass review, and
pass every unit test — the value only fails at INSERT time, where PostgreSQL
answers `invalid input syntax for type uuid`. Because audit rows are written in
the *same transaction* as the business write, the failure rolls the business
write back too: updating a profile, rotating an MFA method or deleting a
credential all answer 500. That is exactly what shipped in 21 call sites
(`internal/app/users/*`, `internal/app/auth/*`) and was only found by reading
the call sites against the column type — no test, no gate, no runtime check
(the SQL runtime gate is skipped when no DSN is configured) could see it.

What it checks
-------------
Audit call sites are recognised by shape, not by function name: a line ending in
`<x>.OrganizationID, <actor>,` whose next line starts with a quoted action string
(the `orgID, actorID, action, …` parameter run shared by
`embedAuditOutboxPlain` / `embedAuditEvent`). For every such site the actor
expression must be UUID-shaped:

  * `""` — a deliberately anonymous/system event, or
  * an identifier/selector ending in `ID`/`Id` (`user.ID`, `session.UserID` …),
    or a field whose name is a known user reference (`p.User`), or
  * a UUID-shaped string literal.

Anything ending in a display-name field (`Username`, `Name`, `DisplayName`,
`Email`, `Slug`, `Login`, `Account`, `Title`) is rejected, as is a non-empty
string literal that is not a UUID. Usernames belong in `metadata`, not in
`actor_id` (《开发设计》§8.1).

Usage: python3 scripts/check-audit-actor.py [root]
Exit code 0 = every audit actor argument can be a UUID,
             1 = at least one call site passes a non-UUID value.

输入：internal/app/** 审计调用点
输出：PASS/FAIL + 违规文件/行
--check 行为：无 --check；失败即 exit 1
消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
"""

import os
import re
import sys

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

# `<x>.OrganizationID, <actor>,` — the org/actor parameter pair.
ORG_ACTOR = re.compile(r"^([A-Za-z_][A-Za-z0-9_.]*)\.OrganizationID,\s*(.+),$")

# Display-name fields: never a UUID. Kept as a denylist rather than "must end in
# ID" so that legitimate reference fields (`p.User`, `o.User`) stay allowed.
DISPLAY_FIELDS = (
    "Username",
    "Name",
    "DisplayName",
    "Email",
    "Slug",
    "Login",
    "Account",
    "Title",
)

UUID_LITERAL = re.compile(
    r'^"[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}"$'
)
STRING_LITERAL = re.compile(r'^".*"$')


def actor_violation(actor: str) -> str:
    """Return a human-readable reason when *actor* cannot be a UUID, else ''."""
    if actor == '""':
        return ""
    if STRING_LITERAL.match(actor):
        return "" if UUID_LITERAL.match(actor) else "string literal is not a UUID"
    field = actor.rsplit(".", 1)[-1]
    if field in DISPLAY_FIELDS:
        return f"`{field}` is a display name, not a UUID"
    return ""


def scan(root: str) -> tuple[int, list[tuple[str, int, str, str]]]:
    sites = 0
    violations: list[tuple[str, int, str, str]] = []
    for dirpath, _dirnames, filenames in os.walk(os.path.join(root, "internal")):
        for filename in sorted(filenames):
            if not filename.endswith(".go") or filename.endswith("_test.go"):
                continue
            path = os.path.join(dirpath, filename)
            try:
                lines = open(path, encoding="utf-8").read().splitlines()
            except (OSError, UnicodeDecodeError):
                continue
            for index, line in enumerate(lines[:-1]):
                match = ORG_ACTOR.match(line.strip())
                if not match:
                    continue
                if not lines[index + 1].strip().startswith('"'):
                    continue
                actor = match.group(2).strip()
                sites += 1
                reason = actor_violation(actor)
                if reason:
                    rel = os.path.relpath(path, root)
                    violations.append((rel, index + 1, actor, reason))
    return sites, violations


def main() -> int:
    root = sys.argv[1] if len(sys.argv) > 1 else REPO_ROOT
    sites, violations = scan(root)

    if violations:
        print("FAIL: 审计调用点把非 UUID 值写进 records.actor_id（uuid 列）：")
        for path, line, actor, reason in violations:
            print(f"  {path}:{line}: actor = {actor} — {reason}")
        print(
            "\n修复：传行为主体的 UUID（user.ID / session.UserID …），"
            "展示名放 metadata。见《开发设计》§8.1。"
        )
        return 1

    print(
        f"Audit actor check OK: {sites} audit call site(s) pass a UUID-shaped actor "
        "(or a deliberately anonymous one)"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
