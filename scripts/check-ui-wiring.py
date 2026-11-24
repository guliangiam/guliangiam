#!/usr/bin/env python3
"""Check that interactive controls on list pages can actually reach the server.

Why this exists
---------------
Two ways a console control ends up doing nothing, both of which shipped:

  1. **A filter dropdown whose value never reaches the request.** Eleven list
     pages held `filterValues` state, passed it to `ResourceListCard`, and never
     referenced it in the query — while twelve sibling pages passed the same prop
     and *did* forward it. Same component, same prop, half the pages wired: a
     connection that depends on somebody remembering is a connection that breaks.

  2. **Sort wiring with no sortable column.** `ResourceListCard` calls
     `onSortChange` only from the Ant Design table's `onChange`, which fires for a
     column declared `sorter`. Seven pages passed `onSortChange` while declaring no
     `sorter` at all, so the callback could never run.

  3. **Search state that never enters the request.** `NotificationListPage` kept
     `keyword` in the react-query key — so every keystroke refetched — while the
     request body stayed byte-identical, and the handler read no such parameter
     either. The search box was a silent no-op at both ends.

None of the three shows up in a type check, a lint run, or a unit test that mocks
HTTP.

The three rules here are deliberately narrow — each is a property that is either
true or false for a file, with no judgement about intent:

  1. **A page that passes `onFilterChange` must read `filterValues` in its
     request.** The state can be forwarded directly (`filters: filterValues`) or
     through a memo, so the check is "`filterValues` appears again after the
     `useState` declaration".

  2. **A page that passes `onSortChange` must declare a sortable column.** Either
     it has `sorter:` on a column (so the callback can fire) or it does not pass
     `onSortChange` at all.

  3. **A page that holds search/filter state must let that state reach a request
     expression**, directly or through a local alias, a memo object, a payload
     mutation (`params.search = keyword`), a derived value, or the list
     adapter's `buildParams` (when the API function is handed over by reference
     as `paginatedFn`). Otherwise the control is removed or the exception is
     recorded in `CLIENT_SIDE_FILTER`.

Usage: python3 scripts/check-ui-wiring.py
Exit code 0 = every control can reach the server, 1 = drift found.

输入：web/src/pages/*ListPage.tsx、web/src/shared/api/http.ts
输出：PASS/FAIL + 控件看着能用、实际无效的页面/控件
--check 行为：无 --check；失败即 exit 1
消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
"""

import os
import re
import sys

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PAGES_DIR = os.path.join(REPO_ROOT, "web/src/pages")

FILTER_VALUES_DECL = re.compile(r"const \[filterValues, setFilterValues\]")
# The forwarding form is exact and uniform across the console: the list params
# carry `filters: filterValues`, which `buildQueryString` turns into query
# parameters. An earlier version of this rule accepted several looser forms as
# fallbacks and a negative test caught it staying green while the forwarding line
# was deleted — the looser a rule, the more it hides.
FILTER_FORWARD = re.compile(r"filters:\s*filterValues\b")
SORT_CALLBACK = re.compile(r"\bonSortChange=\{")
SORTER_COLUMN = re.compile(r"\bsorter:")

# Pages that hold filter state the request deliberately does not carry. Each
# entry must state why, because "the server ignores it" is exactly the defect
# this gate exists to catch.
FILTER_EXEMPT: dict[str, str] = {}


def page_files():
    for dirpath, _dirs, files in os.walk(PAGES_DIR):
        for name in sorted(files):
            if not name.endswith(".tsx") or ".test." in name:
                continue
            yield os.path.join(dirpath, name)


def strip_ts_comments(source):
    """Blank comments, keep everything else.

    A comment explaining the eleven unwired filters quotes `filterValues` and
    `onSortChange`; matching raw source would flag the explanation itself.
    """
    out = []
    i, n = 0, len(source)
    while i < n:
        c = source[i]
        if c in "\"'`":
            quote = c
            j = i + 1
            while j < n:
                if source[j] == "\\":
                    j += 2
                    continue
                if source[j] == quote:
                    j += 1
                    break
                j += 1
            out.append(source[i:j])
            i = j
            continue
        if source.startswith("//", i):
            j = source.find("\n", i)
            i = n if j < 0 else j
            continue
        if source.startswith("/*", i):
            j = source.find("*/", i + 2)
            j = n if j < 0 else j + 2
            out.append("".join("\n" if ch == "\n" else " " for ch in source[i:j]))
            i = j
            continue
        out.append(c)
        i += 1
    return "".join(out)


def _call_arguments(text: str, callee: str) -> list[str]:
    """Return the argument text of every `callee(...)` call (balanced parens)."""
    found: list[str] = []
    for match in re.finditer(rf"\b{re.escape(callee)}\s*\(", text):
        depth = 0
        for index in range(match.end() - 1, len(text)):
            char = text[index]
            if char in "([{":
                depth += 1
            elif char in ")]}":
                depth -= 1
                if depth == 0:
                    found.append(text[match.end():index])
                    break
    return found


FILTER_STATE = re.compile(r"(?i)(keyword|search[A-Za-z]*|filter[A-Za-z]*|Filters)$")

# Pages whose "filter" input never has to reach the server because the filtered
# collection is loaded whole into the browser. Keep this list minimal and
# justified -- every entry is a claim that the filter is genuinely local.
CLIENT_SIDE_FILTER: dict[str, set[str]] = {
    # 支付渠道配置：/pay-channel-configs 一次性返回全部渠道（数量为个位数），
    # 页面在 useMemo 里按 channel/environment/status 本地过滤，服务端无对应查询参数。
    "web/src/pages/payments/PaymentsChannelConfigsPage.tsx": {
        "filterChannel", "filterEnv", "filterStatus",
    },
    # 对账批次：/pay-recon/batches 一次性返回批次列表（按日期窗口），
    # channel/status 由页面本地过滤，服务端查询只接受 batchId/organizationId 等参数。
    "web/src/pages/payments/PaymentsReconPage.tsx": {
        "filterChannel", "filterStatus",
    },
    # The i18n bundle is fetched in full, then searched in memory.
    "web/src/pages/i18n/I18nTranslationsPage.tsx": {"searchText"},
    # 组织树页：/groups 一次性返回当前组织全部组（服务端无搜索参数），搜索框在
    # useMemo 里对已建好的树做本地过滤（filterTree），不改变请求。
    "web/src/pages/groups/GroupTreePage.tsx": {"filterText"},
}


def _local_bindings(text: str) -> dict[str, str]:
    """Map every `const NAME = <expr>` to its initializer.

    The initializer is taken with depth-aware scanning to the matching `)`,
    `]`, `}` or the first top-level `;`, because a naive "up to the first
    semicolon" split swallows the *next* declaration (`const query = useQuery({
    ... const isEnabled = ...; })` would otherwise hide `isEnabled`).
    """
    bindings: dict[str, str] = {}
    for match in re.finditer(r"\bconst\s+([A-Za-z_$][\w$]*)\s*(?::[^=]+?)?=\s*", text):
        start = match.end()
        depth = 0
        index = start
        while index < len(text):
            char = text[index]
            if char in "([{":
                depth += 1
            elif char in ")]}":
                if depth == 0:
                    break
                depth -= 1
            elif char == ";" and depth == 0:
                break
            index += 1
        bindings.setdefault(match.group(1), text[start:index])
    return bindings


def _property_expressions(text: str, key: str) -> list[str]:
    """Return the initializer of every `<key>: <expr>` object property.

    Depth-aware, so an arrow function value is taken whole:
    `buildParams: (p) => ({ a: p.a, b: x || undefined })` is one expression
    ending at the `,` that closes the property, not at the `)` of the
    parameter list.
    """
    found: list[str] = []
    for match in re.finditer(rf"\b{re.escape(key)}\s*:\s*", text):
        start = match.end()
        depth = 0
        index = start
        while index < len(text):
            char = text[index]
            if char in "([{":
                depth += 1
            elif char in ")]}":
                if depth == 0:
                    break
                depth -= 1
            elif char in ",;" and depth == 0:
                break
            index += 1
        found.append(text[start:index])
    return found


def _request_expressions(text: str, callees: set[str]) -> str:
    """Text that can reach the network, following local aliases to a fixpoint.

    Pages rarely pass filter state straight into the API function. Four
    indirections are followed so the rule reports missing wiring rather than
    missing syntax sugar:

      * `const p = useMemo(...)` handed to the API function (`RecordsPage`),
      * `const p = {...}` then `p.search = keyword` (`NotificationListPage`),
      * a local derived from the filter state (`RolesPage` builds `isEnabled`),
      * the list adapter form, where the API function is handed over *by
        reference* (`paginatedFn: getApplicationsPaginated`) and the request is
        composed by the sibling `buildParams` (`CrudListPage` pages). Nothing
        here is `callee(...)`, so the call-argument scan alone finds no request
        expression and every filter on such a page reads as unwired.
    """
    reached = "\n".join(arg for callee in callees for arg in _call_arguments(text, callee))
    if any(
        re.search(rf"\b{re.escape(callee)}\b", text) and not _call_arguments(text, callee)
        for callee in callees
    ):
        reached += "\n" + "\n".join(_property_expressions(text, "buildParams"))
    bindings = _local_bindings(text)
    for _ in range(4):
        grown = reached
        for name, value in bindings.items():
            if re.search(rf"\b{re.escape(name)}\b", reached):
                grown += "\n" + value
        for match in re.finditer(r"([A-Za-z_$][\w$]*)\.([A-Za-z_$][\w$]*)\s*=\s*([^;\n]+)", text):
            if re.search(rf"\b{re.escape(match.group(1))}\b", reached):
                grown += "\n" + match.group(3)
        if grown == reached:
            break
        reached = grown
    return reached


def _derived_locals(text: str, seeds: set[str]) -> set[str]:
    """Local `const x = ...` names whose initializer mentions one of `seeds`."""
    bindings = _local_bindings(text)
    closure = set(seeds)
    for _ in range(4):
        grown = {
            name
            for name, value in bindings.items()
            if name not in closure and any(re.search(rf"\b{re.escape(s)}\b", value) for s in closure)
        }
        if not grown:
            break
        closure |= grown
    return closure


def search_state_never_sent(relative: str, source: str) -> list[str]:
    """Filter-ish state a page keeps but never lets influence its request."""
    offenders: list[str] = []
    for name in sorted(_search_state_names(source) - CLIENT_SIDE_FILTER.get(relative, set())):
        reached = _request_expressions(source, _api_callees(source))
        identifiers = set(re.findall(r"[A-Za-z_$][\w$]*", reached))
        if not (identifiers & _derived_locals(source, {name})):
            offenders.append(name)
    return offenders


def _search_state_names(source: str) -> set[str]:
    if "useQuery" not in source:
        return set()
    return {
        name
        for name in re.findall(r"const\s*\[\s*([A-Za-z_$][\w$]*)\s*,", source)
        if FILTER_STATE.search(name) and not name.endswith("Params")
    }


def _api_callees(source: str) -> set[str]:
    callees: set[str] = set()
    for block in re.finditer(r"import\s*(?:type\s*)?\{([^}]*)\}\s*from\s*\"(@/shared/api/[^\"]+)\"", source):
        for specifier in block.group(1).split(","):
            alias = specifier.strip().split(" as ")[-1].strip()
            if alias and not alias.endswith("Params") and not alias.endswith("Query"):
                callees.add(alias)
    return callees


def main():
    failed = False
    checked = 0
    filter_pages = 0
    sort_pages = 0
    unforwarded = []
    dead_sort = []
    unwired_search = []

    for path in page_files():
        with open(path, encoding="utf-8") as handle:
            source = strip_ts_comments(handle.read())
        relpath = os.path.relpath(path, REPO_ROOT)
        checked += 1

        if FILTER_VALUES_DECL.search(source):
            filter_pages += 1
            if not FILTER_FORWARD.search(source) and relpath not in FILTER_EXEMPT:
                unforwarded.append(relpath)

        if SORT_CALLBACK.search(source):
            sort_pages += 1
            if not SORTER_COLUMN.search(source):
                dead_sort.append(relpath)

        for name in search_state_never_sent(relpath, source):
            unwired_search.append(f"{relpath} :: {name}")

    if unforwarded:
        failed = True
        print(
            f"UI wiring check FAILED ({len(unforwarded)} page(s) hold filter state "
            "the request never carries):"
        )
        for hit in unforwarded:
            print(f"  - {hit}")
        print(
            "  A filter dropdown whose value is never forwarded reads as working "
            "and filters nothing. Forward it (`filters: filterValues`) or remove "
            "the control.",
            file=sys.stderr,
        )

    if dead_sort:
        failed = True
        print(
            f"UI wiring check FAILED ({len(dead_sort)} page(s) pass `onSortChange` "
            "with no sortable column):"
        )
        for hit in dead_sort:
            print(f"  - {hit}")
        print(
            "  `ResourceListCard` raises `onSortChange` from the table's `onChange`, "
            "which only fires for a column declared `sorter`. Without one the "
            "callback can never run, so the sort wiring is dead code promising a "
            "capability the page does not offer.",
            file=sys.stderr,
        )

    if unwired_search:
        failed = True
        print(
            f"UI wiring check FAILED ({len(unwired_search)} filter state(s) the "
            "request never carries):"
        )
        for hit in unwired_search:
            print(f"  - {hit}")
        print(
            "  A search box or filter whose value never influences the request is "
            "a silent no-op: the control renders, the query re-runs (the value is "
            "in the react-query key), and the response is byte-identical. Send it, "
            "record the reason in CLIENT_SIDE_FILTER, or delete the control.",
            file=sys.stderr,
        )

    if failed:
        return 1

    print(
        f"UI wiring check OK: {checked} page module(s) scanned; "
        f"{filter_pages} filter-holding page(s) all forward their values to the request; "
        f"{sort_pages} page(s) wire `onSortChange` and every one declares a sortable column; "
        f"no page keeps search state its request cannot see"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
