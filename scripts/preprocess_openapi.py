#!/usr/bin/env python3
"""Normalize Torn's openapi.json in place so oapi-codegen emits valid Go."""

from __future__ import annotations

import argparse
import copy
import json
from pathlib import Path
from typing import Any


# Parameter components whose enum consts collide with sibling type names
# (ApiStripTagsFalse / ApiStripTagsTrue). Plain strings stay API-compatible.
STRIP_TAGS_PARAMS = (
    "ApiStripTags",
    "ApiStripTagsFalse",
    "ApiStripTagsTrue",
)

# Torn marks array items with a non-standard "schema": "TypeName" field.
ITEM_SCHEMA_FIXES = {
    "ApiCompanySearchFilter": "CompanySearchParameter",
    "ApiFactionSearchFilter": "Parameters",
    "ApiUserSearchFilter": "UserSearchParameter",
}


def fix_items_schema_refs(spec: dict[str, Any]) -> None:
    parameters = spec["components"]["parameters"]
    for param_name, expected in ITEM_SCHEMA_FIXES.items():
        schema = parameters[param_name]["schema"]
        items = schema.get("items")
        if not isinstance(items, dict):
            continue
        if items.get("schema") != expected and "$ref" not in items:
            raise SystemExit(
                f"{param_name}: expected items.schema={expected!r}, got {items!r}"
            )
        parameters[param_name]["schema"] = {
            "type": "array",
            "items": {"$ref": f"#/components/schemas/{expected}"},
        }
        # Preserve style/explode siblings on the parameter itself; only schema changes.


def simplify_strip_tags_enums(spec: dict[str, Any]) -> None:
    parameters = spec["components"]["parameters"]
    for name in STRIP_TAGS_PARAMS:
        param = parameters[name]
        default = param["schema"].get("default")
        schema: dict[str, Any] = {"type": "string"}
        if default is not None:
            schema["default"] = default
        param["schema"] = schema


def simplify_timestamp_params(spec: dict[str, Any]) -> None:
    """Query params are strings on the wire; drop integer|string oneOf unions."""
    parameters = spec["components"]["parameters"]
    for name in ("ApiTimestamp", "ApiTimestampSpecificPointInTime"):
        param = parameters[name]
        param["schema"] = {
            "type": "string",
            "description": param["schema"].get(
                "description",
                param.get("description", "Timestamp value as a string or integer digits."),
            ),
        }


def rename_parameters_schema(spec: dict[str, Any]) -> None:
    """Rename the awkward schema name Parameters -> FactionSearchParameter."""
    schemas = spec["components"]["schemas"]
    if "Parameters" not in schemas:
        return
    if "FactionSearchParameter" in schemas:
        raise SystemExit("FactionSearchParameter already exists")
    schemas["FactionSearchParameter"] = schemas.pop("Parameters")

    def rewrite(obj: Any) -> Any:
        if isinstance(obj, dict):
            out = {}
            for key, value in obj.items():
                if key == "$ref" and value == "#/components/schemas/Parameters":
                    out[key] = "#/components/schemas/FactionSearchParameter"
                else:
                    out[key] = rewrite(value)
            return out
        if isinstance(obj, list):
            return [rewrite(item) for item in obj]
        return obj

    rewritten = rewrite(spec)
    spec.clear()
    spec.update(rewritten)


def preprocess(spec: dict[str, Any]) -> dict[str, Any]:
    out = copy.deepcopy(spec)
    fix_items_schema_refs(out)
    simplify_strip_tags_enums(out)
    simplify_timestamp_params(out)
    rename_parameters_schema(out)
    return out


def format_spec(path: Path) -> None:
    spec = json.loads(path.read_text())
    fixed = preprocess(spec)
    path.write_text(json.dumps(fixed, indent=2, ensure_ascii=False) + "\n")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "spec",
        type=Path,
        nargs="?",
        default=Path("openapi.json"),
        help="OpenAPI JSON file to normalize and format (default: openapi.json)",
    )
    args = parser.parse_args()
    format_spec(args.spec.resolve())
    print(f"formatted {args.spec}")


if __name__ == "__main__":
    main()
