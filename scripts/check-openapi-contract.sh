#!/usr/bin/env bash
# Structural gate for the generated OpenAPI contract.
#
# Catches the class of defects that YAML validity alone cannot: dangling $ref
# targets, duplicate/missing operationId, undeclared path parameters, missing
# error responses, and OpenAPI 3.0 keywords that are invalid in 3.1.
#
# Run: bash scripts/check-openapi-contract.sh [path-to-spec]


# 输入：openapi/openapi.yaml
# 输出：PASS/FAIL + 缺陷列表
# --check 行为：无 --check；失败即 exit 1
# 消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
set -euo pipefail

SPEC_FILE="${1:-openapi/openapi.yaml}"

if [[ ! -f "$SPEC_FILE" ]]; then
  echo "ERROR: spec not found: $SPEC_FILE" >&2
  exit 1
fi

# 重复映射键：用运行期同一解析器（openapi/embed.go 的 gopkg.in/yaml.v3）判定。
# yaml.v3 解码到 map 时重复键直接失败（否则 /openapi/openapi.json 恒 500），
# 而 PyYAML 默认静默保留后者；且本判定不依赖可选的 PyYAML，避免其缺失时整体跳过。
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SPEC_ABS="$SPEC_FILE"
if [[ "$SPEC_ABS" != /* ]]; then
  SPEC_ABS="$PWD/$SPEC_ABS"
fi
(cd "$REPO_ROOT" && go run ./scripts/openapi-dupkeys "$SPEC_ABS")

python3 - "$SPEC_FILE" <<'PY'
import sys

try:
    import yaml
except ImportError:
    # 缺依赖是环境失败，不是契约通过。这里曾以 0 退出并只打一行 WARNING：
    # CI/新机器上整道门禁显示绿灯，而下面的 $ref / operationId / 参数 / 响应
    # 检查一条都没跑过。改为 fail-closed，并给出可执行的修复指引。
    print(
        "ERROR: PyYAML is not installed — the OpenAPI contract check DID NOT RUN.",
        file=sys.stderr,
    )
    print(
        "       This is NOT a pass: no $ref, operationId, path-parameter or "
        "error-response check was performed.",
        file=sys.stderr,
    )
    print(
        "       Fix: pip install pyyaml  "
        "(CI installs the same package in .github/workflows/nightly-db.yml)",
        file=sys.stderr,
    )
    raise SystemExit(1)

path = sys.argv[1]


def ordered_unique_mapping(loader, node, deep=False):
    mapping = {}
    for key_node, value_node in node.value:
        key = loader.construct_object(key_node, deep=deep)
        if key in mapping:
            duplicate_keys.append(key)
        mapping[key] = loader.construct_object(value_node, deep=deep)
    return mapping


class StrictLoader(yaml.SafeLoader):
    pass


duplicate_keys = []
StrictLoader.add_constructor(
    yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, ordered_unique_mapping
)

with open(path, encoding="utf-8") as fh:
    spec = yaml.load(fh, Loader=StrictLoader)

violations = []
for key in duplicate_keys:
    violations.append(f"duplicate mapping key: {key!r}")
HTTP_METHODS = ("get", "put", "post", "delete", "options", "head", "patch", "trace")

if spec.get("openapi") != "3.1.0":
    violations.append(f"openapi must be '3.1.0', got {spec.get('openapi')!r}")

info = spec.get("info") or {}
for key in ("title", "version"):
    if not info.get(key):
        violations.append(f"info.{key} is required")

paths = spec.get("paths") or {}
if not paths:
    violations.append("paths must not be empty")


def walk(node, pointer=""):
    if isinstance(node, dict):
        for key, value in node.items():
            if key == "$ref" and isinstance(value, str):
                yield value
            elif key == "nullable":
                violations.append(f"{pointer}: 'nullable' is not valid in OpenAPI 3.1")
            else:
                yield from walk(value, f"{pointer}/{key}")
    elif isinstance(node, list):
        for index, value in enumerate(node):
            yield from walk(value, f"{pointer}/{index}")


def resolve(ref):
    if not ref.startswith("#/"):
        return True  # external refs are out of scope for this gate
    node = spec
    for token in ref[2:].split("/"):
        token = token.replace("~1", "/").replace("~0", "~")
        if not isinstance(node, dict) or token not in node:
            return False
        node = node[token]
    return True


refs = list(walk(spec))
dangling = sorted({ref for ref in refs if not resolve(ref)})
for ref in dangling:
    violations.append(f"dangling $ref: {ref}")

operation_ids = {}
for pattern, item in paths.items():
    if not isinstance(item, dict):
        violations.append(f"paths[{pattern}] must be an object")
        continue

    declared = {
        p.get("name")
        for p in item.get("parameters", [])
        if isinstance(p, dict) and p.get("in") == "path"
    }

    for param in item.get("parameters", []):
        if isinstance(param, dict) and param.get("in") == "path":
            if param.get("required") is not True:
                violations.append(
                    f"{pattern}: path parameter {param.get('name')!r} must set required: true"
                )

    placeholders = [
        segment.split("}")[0].lstrip("{")
        for segment in pattern.split("/")
        if segment.startswith("{")
    ]

    operations = [(m, item[m]) for m in HTTP_METHODS if m in item]
    if not operations:
        violations.append(f"{pattern}: no HTTP operation defined")

    for method, op in operations:
        # Path parameters may be declared on the path item or on the operation.
        declared_path_params = declared | {
            p.get("name")
            for p in op.get("parameters", [])
            if isinstance(p, dict) and p.get("in") == "path"
        }
        for placeholder in placeholders:
            if placeholder not in declared_path_params:
                violations.append(
                    f"{method.upper()} {pattern}: path placeholder {{{placeholder}}} "
                    "is not declared as a parameter"
                )

        op_id = op.get("operationId")
        if not op_id:
            violations.append(f"{method.upper()} {pattern}: operationId is required")
        elif op_id in operation_ids:
            violations.append(
                f"{method.upper()} {pattern}: duplicate operationId {op_id!r} "
                f"(also used by {operation_ids[op_id]})"
            )
        else:
            operation_ids[op_id] = f"{method.upper()} {pattern}"

        responses = op.get("responses")
        if not responses:
            violations.append(f"{method.upper()} {pattern}: responses is required")
            continue

        for status in responses:
            if status != "default" and not (isinstance(status, str) and status.isdigit()):
                violations.append(
                    f"{method.upper()} {pattern}: response key {status!r} must be a status code or 'default'"
                )

        if not any(str(s).startswith(("2", "3")) for s in responses):
            violations.append(
                f"{method.upper()} {pattern}: no 2xx/3xx success response defined"
            )
        if not {"500", "default"} & set(responses):
            violations.append(
                f"{method.upper()} {pattern}: missing 500/default error response"
            )

components = spec.get("components") or {}
if "securitySchemes" not in components:
    violations.append("components.securitySchemes is required")

# List responses must carry a nested "page" object, matching presenter.ListResponse[T]
# (items + PageMeta{page,pageSize,total}) and AC-API-002. A scalar "page" (or top-level
# pageSize/total) is the historical generator defect.
schemas = components.get("schemas") or {}
paginated = schemas.get("PaginatedResponse")
if not isinstance(paginated, dict):
    violations.append("components.schemas.PaginatedResponse is required")
else:
    props = paginated.get("properties") or {}
    page_prop = props.get("page")
    if not (
        isinstance(page_prop, dict)
        and page_prop.get("$ref") == "#/components/schemas/PageMeta"
    ):
        violations.append(
            "components.schemas.PaginatedResponse.properties.page must be "
            "$ref: '#/components/schemas/PageMeta' (see presenter.ListResponse[T])"
        )
    stray = sorted({"pageSize", "total", "nextCursor"} & set(props))
    if stray:
        violations.append(
            "components.schemas.PaginatedResponse must not declare top-level "
            f"{', '.join(stray)}; they belong inside PageMeta"
        )

if violations:
    print(f"OpenAPI contract check FAILED ({len(violations)} violation(s)):")
    for item in violations:
        print(f"  - {item}")
    raise SystemExit(1)

print(
    f"OpenAPI contract OK: {len(paths)} paths, {len(operation_ids)} operations, "
    f"{len(set(refs))} distinct $ref, 0 dangling"
)
PY
