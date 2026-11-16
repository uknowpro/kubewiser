"""Maintainer-only: collect license/notice files for modules linked by the CLI."""
import json
import subprocess
from pathlib import Path

root = Path(__file__).resolve().parents[1]
data = subprocess.check_output(["go", "list", "-deps", "-json", "./cmd/kubewiser"], cwd=root, text=True)
decoder = json.JSONDecoder()
modules = {}
while data.strip():
    data = data.lstrip()
    package, end = decoder.raw_decode(data)
    data = data[end:]
    module = package.get("Module", {})
    if module.get("Dir") and not module.get("Main"):
        modules[module["Path"]] = module

parts = ["kubewiser third-party notices\n\nThe project is MIT licensed. Embedded schemas are derived from the official Kubernetes repository (Apache-2.0); versions and source hashes are recorded in internal/schema/bundles/sources.json. Description prose is removed while property names and schema constraints are preserved.\n"]
for name, module in sorted(modules.items()):
    directory = Path(module["Dir"])
    files = sorted(p for p in directory.iterdir() if p.is_file() and p.name.upper().split(".")[0] in {"LICENSE", "COPYING", "NOTICE", "COPYRIGHT"})
    if not files:
        raise SystemExit(f"Missing license material for {name}")
    parts.append("\n" + "=" * 72 + "\n" + name + " " + module["Version"] + "\n")
    for file in files:
        parts.append("\n" + file.name + "\n" + file.read_text(errors="strict") + "\n")

goroot = Path(subprocess.check_output(["go", "env", "GOROOT"], text=True).strip())
parts.append("\n" + "=" * 72 + "\nGo standard library/runtime\n\n" + (goroot / "LICENSE").read_text())
(root / "THIRD_PARTY_NOTICES.txt").write_text("".join(parts))
print(f"Collected license material for {len(modules)} linked modules and the Go runtime")
