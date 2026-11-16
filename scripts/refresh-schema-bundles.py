"""Maintainer-only: re-fetch pinned official schemas and rebuild offline bundles.

Never called by the CLI. Source hash mismatches abort before any files change.
"""
import concurrent.futures
import gzip
import hashlib
import json
from pathlib import Path
from urllib.request import urlopen

ROOT = Path(__file__).resolve().parents[1] / "internal/schema/bundles"


def reduce_schema(schema):
    if not isinstance(schema, dict):
        return schema
    result = {}
    for key, value in schema.items():
        if key in ("description", "externalDocs"):
            continue
        if key in ("properties", "definitions", "patternProperties", "$defs"):
            # Property names are data, even when named 'description' or 'externalDocs'.
            value = {name: reduce_schema(child) for name, child in value.items()}
        elif key in ("items", "additionalProperties", "not"):
            value = reduce_schema(value)
        elif key in ("allOf", "anyOf", "oneOf"):
            value = [reduce_schema(child) for child in value]
        result[key] = value
    return result


def fetch(entry):
    url = f"https://raw.githubusercontent.com/kubernetes/kubernetes/{entry['version']}/api/openapi-spec/swagger.json"
    if entry["source"] != url:
        raise ValueError("unexpected source URL")
    with urlopen(url, timeout=45) as response:
        raw = response.read(64 * 1024 * 1024 + 1)
    if len(raw) > 64 * 1024 * 1024 or hashlib.sha256(raw).hexdigest() != entry["source_sha256"]:
        raise ValueError(f"source hash or size mismatch: {entry['version']}")
    definitions = json.loads(raw)["definitions"]
    reduced = {name: reduce_schema(schema) for name, schema in definitions.items()}
    payload = json.dumps({"definitions": reduced}, separators=(",", ":"), sort_keys=True).encode()
    return entry, gzip.compress(payload, mtime=0)


if __name__ == "__main__":
    entries = json.loads((ROOT / "sources.json").read_text())
    with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
        results = list(pool.map(fetch, entries))
    for entry, bundle in results:
        (ROOT / (entry["version"] + ".json.gz")).write_bytes(bundle)
        entry["bundle_sha256"] = hashlib.sha256(bundle).hexdigest()
        entry["reviewed"] = "2026-11-16"
        print(entry["version"], "source verified; property names preserved")
    (ROOT / "sources.json").write_text(json.dumps(entries, indent=2) + "\n")
