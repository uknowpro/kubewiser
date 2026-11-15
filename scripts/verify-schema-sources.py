"""Maintainer-only verification: explicitly downloads official pinned sources."""
import hashlib
import json
from pathlib import Path
from urllib.request import urlopen

root = Path(__file__).resolve().parents[1] / "internal/schema/bundles"
for entry in json.loads((root / "sources.json").read_text()):
    expected = f"https://raw.githubusercontent.com/kubernetes/kubernetes/{entry['version']}/api/openapi-spec/swagger.json"
    if entry["source"] != expected:
        raise SystemExit("Unexpected source URL")
    with urlopen(expected, timeout=90) as response:
        data = response.read(64 * 1024 * 1024 + 1)
    if hashlib.sha256(data).hexdigest() != entry["source_sha256"]:
        raise SystemExit(f"Source hash mismatch: {entry['version']}")
    bundle = (root / (entry["version"] + ".json.gz")).read_bytes()
    if hashlib.sha256(bundle).hexdigest() != entry["bundle_sha256"]:
        raise SystemExit(f"Bundle hash mismatch: {entry['version']}")
    print(entry["version"], "verified")
