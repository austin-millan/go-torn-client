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

# Search-filter array params: keep stable component schema names for codegen.
# Older Torn specs used a non-standard items.schema="TypeName" pointer; newer
# specs inline the oneOf on items and omit the named schema entirely.
# ApiFactionSearchFilter's upstream schema is awkwardly named "Parameters";
# rename_parameters_schema later rewrites that to FactionSearchParameter.
ITEM_SCHEMA_FIXES = {
    "ApiCompanySearchFilter": "CompanySearchParameter",
    "ApiFactionSearchFilter": "Parameters",
    "ApiUserSearchFilter": "UserSearchParameter",
}

# Post-rename stable names. An existing $ref to either the intermediate or the
# final name is already correct; do not rewrite FactionSearchParameter back to
# Parameters on an already-preprocessed vendored spec.
ITEM_SCHEMA_FINAL_NAMES = {
    "ApiFactionSearchFilter": "FactionSearchParameter",
}


def fix_items_schema_refs(spec: dict[str, Any]) -> None:
    parameters = spec["components"]["parameters"]
    schemas = spec["components"].setdefault("schemas", {})
    for param_name, expected in ITEM_SCHEMA_FIXES.items():
        schema = parameters[param_name]["schema"]
        items = schema.get("items")
        if not isinstance(items, dict):
            continue

        ref = f"#/components/schemas/{expected}"
        final_name = ITEM_SCHEMA_FINAL_NAMES.get(param_name, expected)
        acceptable_refs = {
            ref,
            f"#/components/schemas/{final_name}",
        }

        if items.get("$ref") in acceptable_refs:
            continue

        if items.get("schema") in (expected, final_name):
            # Legacy Torn form: items.schema = "TypeName" with a sibling component.
            pass
        elif "oneOf" in items:
            # Current Torn form: oneOf inlined on items; hoist if the name is gone.
            # Prefer the intermediate name so rename_parameters_schema can run.
            hoist_name = expected
            if hoist_name not in schemas and final_name not in schemas:
                schemas[hoist_name] = copy.deepcopy(items)
            elif hoist_name not in schemas and final_name in schemas:
                # Already renamed on a prior pass; point at the final name.
                ref = f"#/components/schemas/{final_name}"
        elif "$ref" in items:
            # Unexpected ref target; normalize to the stable name below.
            pass
        else:
            raise SystemExit(
                f"{param_name}: expected items.schema={expected!r}, "
                f"$ref, or inlined oneOf; got {items!r}"
            )

        # Preserve style/explode siblings on the parameter itself; only schema changes.
        parameters[param_name]["schema"] = {
            "type": "array",
            "items": {"$ref": ref},
        }


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
