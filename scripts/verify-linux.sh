#!/usr/bin/env bash
# Run against a locally built standalone binary; no cluster or external network required.
set -euo pipefail
binary=$(realpath "${1:?pass a kubewiser binary}")
root=$(cd "$(dirname "$0")/.." && pwd)
verify_dir=$(mktemp -d)
trap 'rm -rf "$verify_dir"' EXIT
"$binary" --version
"$binary" scan cel --source "$root/docs/examples/advanced/cel.yaml" > "$verify_dir/cel.json"
"$binary" generate --profile "$root/docs/examples/advanced/project.json" --output "$verify_dir/configured" > "$verify_dir/configured.json"
"$binary" plan aks --source "$root/docs/examples/manifest.yaml" --profile "$root/docs/examples/project/aks.json" --format json > "$verify_dir/plan.json"
"$binary" plan aks --source "$root/docs/examples/manifest.yaml" --profile "$root/docs/examples/project/aks.json" --format html --lang ko > "$verify_dir/plan.html"
"$binary" generate --profile "$root/docs/examples/project/aks.json" --output "$verify_dir/drafts" > "$verify_dir/generated.json"
if "$binary" generate --profile "$root/docs/examples/project/aks.json" --output "$verify_dir/drafts" > /dev/null 2>&1; then
  echo 'ERROR: output directory was overwritten' >&2; exit 1
fi
"$binary" scan helm --target-version 1.35 --namespace review "$verify_dir/drafts/chart" > "$verify_dir/helm.json"
VERIFY_DIR="$verify_dir" python3 - <<'PY'
import json, os
from pathlib import Path
p=Path(os.environ['VERIFY_DIR']);plan=json.loads((p/'plan.json').read_text())
assert plan['status']=='review_required'
assert not json.loads((p/'cel.json').read_text())['actions']
assert json.loads((p/'configured/nodepool.auto.tfvars.json').read_text())['enable_canary_pool'] is False
assert any(a['id']=='KW-AKS-ROUTE' for a in plan['actions'])
assert any(a['id']=='KW-API-REMOVED' and a['before_version']=='1.25' for a in plan['actions'])
assert '<html' in (p/'plan.html').read_text()
assert json.loads((p/'helm.json').read_text())['report']['documents']==0
resources=[{'apiVersion':'v1','kind':'Service','metadata':{'name':f'service-{i}','namespace':'test'},'spec':{'ports':[{'port':80,'targetPort':8080}]}} for i in range(10000)]
(p/'large.json').write_text(json.dumps({'apiVersion':'v1','kind':'List','items':resources}))
PY
"$binary" scan files --target-version 1.35 "$verify_dir/large.json" > "$verify_dir/large-report.json"
VERIFY_DIR="$verify_dir" python3 - <<'PY'
import json, os
from pathlib import Path
r=json.loads((Path(os.environ['VERIFY_DIR'])/'large-report.json').read_text())
assert r['documents']==10000 and r['schema_checked']==10000 and not r['findings']
print('PASS: AKS plan, HTML, draft conflict, disabled Helm chart and 10,000-resource schema scan')
PY
if command -v strace > /dev/null; then
  strace -f -qq -e signal=none -e trace=network -o "$verify_dir/network.trace" "$binary" plan aks --source "$root/docs/examples/manifest.yaml" --profile "$root/docs/examples/project/aks.json" > /dev/null
  if test -s "$verify_dir/network.trace"; then
    echo 'ERROR: unexpected network system call during offline AKS plan' >&2; cat "$verify_dir/network.trace"; exit 1
  fi
  strace -f -qq -e signal=none -e trace=network -o "$verify_dir/cel-network.trace" "$binary" scan cel --source "$root/docs/examples/advanced/cel.yaml" > /dev/null
  if test -s "$verify_dir/cel-network.trace"; then
    echo 'ERROR: unexpected network call during CEL validation' >&2; exit 1
  fi
  echo 'PASS: no network system calls during offline AKS plan or CEL validation'
else
  echo 'SKIP: strace unavailable; network syscall check not performed'
fi
