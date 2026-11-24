"""gen_openapi.py 响应语义单元测试（标准库 only）。

覆盖：SuccessStatus 声明、缺省 200 回落、DELETE 不再固定写成 200、
ResponseHeaders 进入 OpenAPI。入口：`python3 -m unittest scripts.test_gen_openapi`
"""

from __future__ import annotations

import importlib.util
import os
import tempfile
import unittest
from pathlib import Path

_SPEC = importlib.util.spec_from_file_location(
    "gen_openapi", Path(__file__).resolve().with_name("gen_openapi.py")
)
gen_openapi = importlib.util.module_from_spec(_SPEC)
_SPEC.loader.exec_module(gen_openapi)

_TEMPLATE = """package testmod

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		%s
	}
}
"""


def generate(route_fields: str) -> str:
    with tempfile.NamedTemporaryFile("w", suffix=".go", delete=False) as handle:
        handle.write(_TEMPLATE % route_fields)
        path = handle.name
    try:
        defs = gen_openapi.parse_routes_file(path, "testmod")
    finally:
        os.unlink(path)
    return gen_openapi.generate_openapi(defs)


def route(method: str, **extra: str) -> str:
    fields = {
        "Method": f'"{method}"',
        "Pattern": '"/api/v1/widgets"',
        "OperationID": '"WidgetOp"',
        "Summary": '"Widget operation"',
        "Permission": '"widgets:write"',
        "Surface": "route.SurfaceManagement",
    }
    fields.update({key: value for key, value in extra.items()})
    return "{" + ", ".join(f"{key}: {value}" for key, value in fields.items()) + "},"


class SuccessStatusTests(unittest.TestCase):
    def test_declared_status_is_used(self) -> None:
        spec = generate(route("POST", SuccessStatus="201"))
        self.assertIn("        '201':", spec)
        self.assertIn("          description: Resource created.", spec)
        self.assertNotIn("        '200':", spec)

    def test_http_constant_and_default_200(self) -> None:
        spec = generate(route("GET", SuccessStatus="http.StatusCreated"))
        self.assertIn("        '201':", spec)
        default = generate(route("GET"))
        self.assertIn("        '200':", default)
        self.assertIn("          description: Widget operation", default)


class DeleteStatusTests(unittest.TestCase):
    def test_delete_is_not_forced_to_200(self) -> None:
        spec = generate(route("DELETE"))
        self.assertIn("        '200':", spec)
        self.assertNotIn("        '204':", spec)
        self.assertIn("        '404':", spec)

    def test_delete_204_has_no_content(self) -> None:
        spec = generate(route("DELETE", SuccessStatus="204"))
        self.assertIn("        '204':", spec)
        self.assertNotIn("        '200':", spec)
        response = spec.split("        '204':", 1)[1].split("        '404':", 1)[0]
        self.assertNotIn("content:", response)
        self.assertIn("          description: No Content.", response)


class ResponseHeaderTests(unittest.TestCase):
    def test_headers_are_documented_on_declared_status(self) -> None:
        spec = generate(
            route(
                "POST",
                SuccessStatus="202",
                ResponseHeaders='[]string{"Location", "Retry-After"}',
            )
        )
        self.assertIn("        '202':", spec)
        self.assertIn("            Location:", spec)
        self.assertIn("            Retry-After:", spec)

    def test_redirect_status_has_headers_and_no_body(self) -> None:
        spec = generate(
            route(
                "POST",
                SuccessStatus="303",
                ResponseHeaders='[]string{"Location"}',
            )
        )
        response = spec.split("        '303':", 1)[1].split("        '500':", 1)[0]
        self.assertNotIn("content:", response)
        self.assertIn("            Location:", response)


class ErrorResponseTests(unittest.TestCase):
    def test_declaration_is_optional_and_output_is_stable(self) -> None:
        spec = generate(route("POST", SuccessStatus="201"))
        self.assertEqual(spec, generate(route("POST", SuccessStatus="201")))
        self.assertNotIn("        '409':", spec)
        self.assertNotIn("        '422':", spec)
        self.assertEqual(spec.count("        '500':"), 1)

    def test_declared_statuses_are_sorted_and_deduplicated(self) -> None:
        spec = generate(
            route("POST", SuccessStatus="201", ErrorResponses="[]int{422, 409, 409}")
        )
        self.assertEqual(spec.count("        '409':"), 1)
        self.assertEqual(spec.count("        '422':"), 1)
        self.assertLess(spec.index("        '409':"), spec.index("        '422':"))
        self.assertLess(spec.index("        '422':"), spec.index("        '500':"))


class ApplicationDetailSchemaTests(unittest.TestCase):
    """The masked client-secret projection belongs only to the application detail read."""

    def test_get_application_references_the_dedicated_schema(self) -> None:
        spec = generate(
            route("GET", OperationID='"GetApplication"', Pattern='"/api/v1/applications/{id}"')
        )
        self.assertIn("$ref: '#/components/schemas/ApplicationDetail'", spec)

    def test_resource_detail_stays_opaque(self) -> None:
        spec = generate(route("GET"))
        self.assertIn(
            "    ResourceDetail:\n      type: object\n      additionalProperties: true\n",
            spec,
            "ResourceDetail must not gain application-only credential fields",
        )
        detail = spec.split("    ApplicationDetail:", 1)[1].split("    EntryTranscriptPage:", 1)[0]
        self.assertIn("clientSecretSet:", detail)
        self.assertIn("clientSecretMasked:", detail)
        self.assertNotIn("clientSecret:", detail, "plaintext key must never be declared")


if __name__ == "__main__":
    unittest.main()
