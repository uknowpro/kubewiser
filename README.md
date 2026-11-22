# kubewiser

**English** | [한국어](README.ko.md)

**Know what to change before changing Kubernetes.**

Inspect manifests, Helm charts and read-only snapshots with a CLI that compares **current state → recommended state → next action**. Use official target-version schemas, traceable evidence, offline analysis and local MCP tools.

[Download](https://github.com/uknowpro/kubewiser/releases) · [Quick start](#quick-start) · [Report preview](#report-preview) · [Command reference](#command-reference) · [Roadmap](docs/roadmap.md)

> Alpha software. Analysis is offline by default, with no telemetry. Explicit collection contacts your selected API server; opt-in external authentication may also contact identity services. Reports show partial coverage and never apply changes automatically.

## 📑 Table of contents

- [✨ Features](#features)
- [📊 Report preview](#report-preview)
- [🧮 Before/after matrix](#comparison-matrix)
- [🚀 Quick start](#quick-start)
- [📦 Installation](#installation)
- [🛠️ Commands at a glance](#commands-at-a-glance)
- [📖 Command reference](#command-reference)
- [🤖 Local MCP and local models](#local-mcp-and-local-models)
- [🔧 Build and development](#build-and-development)
- [🔎 Scope and evidence](#scope-and-evidence)
- [💬 Development and feedback](#development-and-feedback)
- [🗺️ Roadmap](docs/roadmap.md)
- [📝 Release notes](https://github.com/uknowpro/kubewiser/releases)
- [📄 License](LICENSE)

<a id="features"></a>

## ✨ Features

| Feature | What you can do |
| --- | --- |
| 🔍 Manifest & API inspection | Find schema errors and deprecated/removed APIs using official Kubernetes 1.16–1.37 `.0` schemas, selected exact patches and supplied local CRDs. |
| 🎯 Actionable recommendations | Review required, conditional and optional actions with current/recommended comparisons, reasons and verification steps. |
| 🧮 Before/after matrix | Compare current/proposed manifests and target validation alongside cgroup, kernel and runtime coverage. |
| ⚖️ Helm & manifest drift | Render local charts and compare desired resources with explicitly scoped, GET-only snapshots. |
| 🪜 Upgrade planning | Review minor-version steps and workload API changes before upgrading. Provider availability and runtime compatibility need separate verification. |
| ☁️ Cloud migration review | Inventory EKS/AKS/GKE storage, load balancer and identity dependencies. Cross-cloud provisioning and data migration are not performed; AKS review scaffolds are available separately. |
| 🌐 Network intent analysis | Inspect Service/Ingress exposure, NetworkPolicy isolation intent and existing Hubble flows without active probes. |
| 📊 Shareable reports | Read terminal summaries or export JSON, Markdown and standalone HTML with evidence and unchecked areas. |
| 🤖 Local MCP integration | Expose scoped inspection tools to MCP clients, including clients using local DeepSeek/Ollama models. No built-in LLM calls. |
| 🔒 Offline analysis | Analyze local inputs without telemetry or automatic downloads. Explicit collection contacts only the selected Kubernetes API server. |

<a id="report-preview"></a>

## 📊 Report preview

Current generated reports from synthetic inputs: the full review matrix appears first, followed by cgroup, lifecycle, version-change and finding tables. Evidence can be expanded.

<a id="comparison-matrix"></a>

### Manifest and environment comparison matrix (v0.2.0-alpha.12)

`plan upgrade` and `plan aks` include a coverage matrix. HTML merges group and resource/pool cells vertically; individual verdicts remain separate. Markdown/text use the same group order. Missing cgroup/kernel/runtime/driver evidence remains visible as **input missing**; unsupported analyzers are identified separately. Without `--after`, the target is a recommendation, not an actual proposed manifest.

```bash
kubewiser plan upgrade --source ./before --after ./after \
  --from 1.21 --to 1.36 --cgroup-evidence ./pools.json \
  --format html --lang en > comparison.html
```

Includes workload-to-pool impact, PDB/drain risk and **resolved / remaining / introduced / cannot compare** finding progress. [Inputs, scope and examples](docs/upgrade-readiness.md). No drain or deployment is performed.

[Usage and verdict scope](docs/matrix.md) · [HTML report](docs/examples/matrix.html) · [Current input](docs/examples/readiness/before.yaml) · [Proposed input](docs/examples/readiness/after.yaml)

![Manifest changes and cgroup, kernel and runtime verdicts in a single review matrix](docs/images/matrix.png)

### Cgroup As-Is / To-Be (v0.2.0-alpha.8)

Compare per-pool v1/v2/hybrid modes, conflicting or missing evidence, target prerequisites and workload v1 dependency hints. [Evidence format and coverage](docs/cgroup.md). No node commands are executed.

```sh
kubewiser plan upgrade --source ./deploy --from 1.21 --to 1.36 --cgroup-evidence ./pools.json --format html --lang en > upgrade-review.html
```

### Support lifecycle (v0.2.0-alpha.7)

Inspect selected project, managed add-on and AKS node OS support deadlines in a separate table. Upstream and managed support are distinct; uncertain identities remain review hints. [Scope and seven catalog entries](docs/lifecycle.md).

```sh
kubewiser scan lifecycle --source ./deploy --format html --lang en > lifecycle.html
```

### Compact migration review (v0.2.0-alpha.6)

HTML reports now list findings in a compact table with expandable evidence. Upgrade/AKS plans also show a separate As-Is/To-Be table of bundled API removals and reported behavior changes. This is a selected catalog, not every release change or proof of actual cluster usage.

```sh
kubewiser plan upgrade --source ./deploy --from 1.21 --to 1.36 --format html --lang en > upgrade-1.21-1.36.html
```

[1.21 → 1.36 HTML example](docs/examples/upgrade-1.21-1.36.ko.html) (synthetic example input; download to view). Available from v0.2.0-alpha.6.

![As-Is / To-Be compact version comparison](docs/images/upgrade-1.21-1.36.ko.png)

Download the HTML file and open it locally. It loads no external fonts or scripts.

<a id="quick-start"></a>

## 🚀 Quick start

1. Download the matching archive from [Releases](https://github.com/uknowpro/kubewiser/releases), verify its checksum and extract it.
2. Add the binary to PATH, or run `./kubewiser` from the extracted folder (`.\kubewiser.exe` on Windows).
3. Inspect your manifests and open the report. **No Go, Helm or kubectl installation is required.**

```sh
kubewiser --version
kubewiser scan files --target-version 1.37 --recommend --format text ./deploy
kubewiser scan files --target-version 1.37 --recommend --format html ./deploy > report.html
```

For a legacy CronJob, the terminal review looks like this (selected lines from actual output):

```console
$ kubewiser scan files --target-version 1.37 --format text docs/examples/manifest.yaml
Inspection review
Target Kubernetes 1.37 · Resources 1 · Schema checked 0
...
Required: 1 | Conditional: 0 | Optional: 0

1. [Required] Migrate the Kubernetes API (KW-API-REMOVED)
   batch/v1beta1/CronJob//example-legacy-job
   Current: apiVersion: batch/v1beta1
   Recommended: batch/v1
...
```

Findings return exit code `1` while still producing a report. Always review unchecked areas too. Add `--lang ko` for Korean review explanations; original evidence can remain English.

<a id="installation"></a>

## 📦 Installation

| Platform | Archive suffix |
| --- | --- |
| Linux Intel/AMD · ARM64 | `linux_amd64.tar.gz` · `linux_arm64.tar.gz` |
| macOS Intel · Apple Silicon | `darwin_amd64.tar.gz` · `darwin_arm64.tar.gz` |
| Windows 10+ 64-bit · 32-bit | `windows_amd64.zip` · `windows_386.zip` |

Filenames also include the release version. `kubewiser` and `kubewise` are equivalent commands. Windows ZIPs contain one EXE and `install.ps1`; running the installer registers both names and the user PATH.

```powershell
.\install.ps1
kubewiser scan files --target-version 1.37 --format html .\deploy | Out-File -Encoding utf8 report.html
Start-Process .\report.html
```

See [installation details](docs/releases.md) for checksums, OS validation boundaries and setup.

<a id="commands-at-a-glance"></a>

## 🛠️ Commands at a glance

| Command | Purpose |
| --- | --- |
| `scan files` | Inspect a manifest file or directory |
| `scan cgroup` | Compare current/target cgroup modes and migration dependencies |
| `scan lifecycle` | Review support retirement dates and identification gaps |
| `scan cel` | Evaluate local CRD validation and transition rules |
| `scan helm` | Render and inspect a local chart |
| `collect` | Snapshot an explicit cluster context and namespace |
| `scan snapshot` | Validate saved resources for a target version |
| `diff` | Compare manifests or a chart with a snapshot |
| `plan upgrade` | Review minor upgrade steps and actions |
| `plan aks` / `generate` | Review an AKS project profile / create local draft templates |
| `scan path` | Evaluate supplied ingress and egress policy intent for two Pods |
| `scan network` | Inspect configuration and existing flows |
| `plan migrate` | Review cloud migration dependencies |
| `mcp` | Serve scoped local inspection tools |
| `versions` / `--version` / `--help` | Schema inventory / binary information / help |

Expand the sections below for commands, actual terminal output and exit codes.

### Set current and target versions

`--from` is the current version and `--to` is the target. Choose either local manifests with `--source` or a saved snapshot with `--snapshot`, never both.

```sh
kubewiser plan upgrade --source ./deploy --from 1.21 --to 1.36 --format html --lang en > upgrade-review.html
kubewiser plan upgrade --snapshot ./snapshot.json --from 1.21 --to 1.36 --format html --lang en > snapshot-review.html
```

For AKS projects, edit `from_version` and `to_version` in the existing profile. This is a **field excerpt, not a complete profile**.

```json
{
  "from_version": "1.21",
  "to_version": "1.36"
}
```

```sh
kubewiser plan aks --snapshot ./snapshot.json --profile ./project.json --format html --lang en > aks-review.html
```

These commands generate local reports and do not execute upgrades. A minor-version sequence does not establish the upgrade path actually offered by AKS.

### AKS project planning (v0.2.0-alpha.4+)

```sh
kubewiser plan aks --source ./deploy --profile docs/examples/project/aks.json --format html > aks-plan.html
kubewiser generate --profile docs/examples/project/aks.json --output ./new-review-drafts
kubewiser scan path --source ./network-inputs --source-pod team/client --destination-pod team/api --port 8080
```

[Project profile, architecture requirements, templates and network path guide](docs/project-planning.md). The 1.21 → 1.35 example is an unverified requirement, not an AKS-supported route. Generated Terraform/Helm/manifest files are review scaffolds; no infrastructure operations run.

### Advanced checks (v0.2.0-alpha.5+)

```sh
kubewiser scan files --target-version 1.35.9 ./deploy
kubewiser scan cel --source ./crds-and-resources --format html > cel.html
kubewiser diff --source ./deploy --snapshot ./snapshot.json --crd-defaults --crd-conversion
kubewiser scan path --source ./network-inputs --source-pod team/client --destination-service team/api --port 443 --flows ./hubble.jsonl
```

[Authentication, AKS exported evidence, patch schemas, CEL, image platforms and configured templates](docs/advanced-inspection.md). Older terminal excerpts below were captured for alpha.3/alpha.4; diagnostic wording can differ in newer versions.

<a id="command-reference"></a>

## 📖 Command reference

Every `kubewiser` command also works as `kubewise`. Put flags before positional paths. Run the checked-in `docs/examples/...` examples from the repository root. Replace example paths, context and namespace when using your own inputs. Fixtures are synthetic documentation data; `snapshot.json` contains one Service and is not a production snapshot. Terminal excerpts below come from actual local CLI runs. `echo $?` is the POSIX shell exit-code check; in PowerShell use `$LASTEXITCODE`. MCP is a waiting server, not a report-producing command.


<details>
<summary>Output formats and saving reports</summary>

Markdown/HTML require v0.2.0-alpha.2+. Comparison reviews and `--lang en|ko` for scan/plan/diff require v0.2.0-alpha.3+. JSON remains unchanged; original evidence wording may remain English even in Korean reviews.

| Format | Purpose | Example |
| --- | --- | --- |
| `json` (default) | Automation and structured processing | `--format json ... > report.json` |
| `text` | Concise terminal summary for direct scan reports | `--format text ...` |
| `markdown` | Tables and lists for GitHub or editors | `--format markdown ... > report.md` |
| `html` | Standalone browser report, including printing | `--format html ... > report.html` |

Text reports for scan files/snapshot/helm, diff and plan show current/recommended states and actions. Network and collection text output remains JSON. Markdown/HTML work for scan, diff, plan and collection summaries. `versions` supports json/markdown/html. MCP always uses structured protocol responses.

```sh
kubewiser scan files --target-version 1.37 --format markdown docs/examples/manifest.yaml > report.md
kubewiser scan files --target-version 1.37 --format html docs/examples/manifest.yaml > report.html
```

```powershell
# Windows PowerShell: preserve Unicode as UTF-8
kubewiser scan files --target-version 1.37 --format html .\deploy | Out-File -Encoding utf8 report.html
Start-Process .\report.html
```

HTML includes its CSS and loads no external fonts, scripts or images. Both readable formats retain the JSON report's evidence and unchecked areas and escape input-controlled markup. Reports can still contain sensitive paths, resource names and metadata. Redirection is silent on success: open the saved file to view it. Findings can produce exit code 1 while still saving a complete report.

</details>

<details>
<summary>--help, --version, versions — inspect commands and supported versions</summary>

`--version` identifies the binary; `versions` lists embedded schema versions, official source URLs, hashes and review dates. Exact bundled patches (1.35.9, 1.36.5, 1.37.1) are preferred; other patches explicitly report the minor `.0` fallback.

**Terminal example** — Long JSON is excerpted to selected fields. Actual output includes additional evidence and unchecked areas.

```console
$ kubewiser --help
Commands: kubewiser or kubewise
  --version
  versions [--format json|markdown|html]
  scan files --target-version VERSION [--recommend] PATH
  scan helm --target-version VERSION --release NAME --namespace NS [--values FILE] CHART
...
$ echo $?
0
```

**Terminal example** — Actual output from a local development build. Release binaries display their own version, commit and platform.

```console
$ kubewiser --version
kubewiser dev (commit unknown; linux/amd64)
$ echo $?
0
```

**Terminal example** — Long JSON is excerpted to selected fields. Actual output includes additional evidence and unchecked areas.

```console
$ kubewiser versions
[
  {
    "version": "v1.16.0",
    "source": "https://raw.githubusercontent.com/kubernetes/kubernetes/v1.16.0/api/openapi-spec/swagger.json",
    "reviewed": "2026-11-15"
  }
]
$ echo $?
0
```

</details>

<details>
<summary>scan files — validate YAML/JSON manifests</summary>

Inspect a file or directory recursively for schema errors and API deprecation/removal. Add `--recommend` for conditional readiness/resource recommendations. Supply CRDs alongside custom resources to use local schemas. Review `findings`, source positions, `requirement`, `severity`, `source` and `schema_checked`.

**Terminal example** — Long JSON is excerpted to selected fields. Actual output includes additional evidence and unchecked areas.

```console
$ kubewiser scan files --target-version 1.37 --format text docs/examples/manifest.yaml
Inspection review
Target Kubernetes 1.37 · Resources 1 · Schema checked 0
Start with required actions, then check whether conditional recommendations apply to your workloads.
Required: 1 | Conditional: 0 | Optional: 0

1. [Required] Migrate the Kubernetes API (KW-API-REMOVED)
   batch/v1beta1/CronJob//example-legacy-job
   Current: apiVersion: batch/v1beta1
   Recommended: batch/v1
   Why: Deprecated in 1.21; no longer served from 1.25.
   Action: Migrate the resource using the official migration guide, including field and behavior changes. An apiVersion edit alone may be insufficient.
   Verify: Re-run this inspection and test the workload in a staging environment.
   Location: docs/examples/manifest.yaml:1 /apiVersion
   Source: https://kubernetes.io/docs/reference/using-api/deprecation-guide/

NOT VERIFIED (partial coverage):
- Admission, CEL, controller behavior, feature-gate availability and application runtime compatibility are not evaluated
- OpenAPI validation does not replace Kubernetes admission or server semantic validation
- Schemas are pinned to minor .0 releases; patch-specific changes are not inferred
$ echo $?
1
```

</details>

<details>
<summary>scan helm — render and inspect a local chart</summary>

Use an unpacked chart with dependencies already vendored locally. `--release`, `--namespace` and optional `--values` fix rendering inputs. No Helm installation, dependency downloads, live lookup or apply is performed. `render` records chart metadata/capabilities; `report` contains findings. `--api-versions` replaces the entire default capability list. Helm values.schema.json validation is skipped.

**Terminal example** — Long JSON is excerpted to selected fields. Actual output includes additional evidence and unchecked areas.

```console
$ kubewiser scan helm --target-version 1.37 --release app --namespace team docs/examples/chart
{
  "render": {
    "chart": "demo",
    "chart_version": "0.1.0",
    "release": "app",
    "namespace": "team",
    "target_version": "1.37"
  },
  "report": {
    "documents": 1,
    "schema_checked": 1,
    "findings": [],
    "status": "partial"
  }
}
$ echo $?
0
```

Additional usage with your own inputs:

```sh
kubewiser scan helm --target-version 1.37 --release app --namespace team --values ./values.yaml --format html ./chart > helm-report.html
```

</details>

<details>
<summary>collect — create a read-only snapshot</summary>

Requires explicit `--kubeconfig`, `--context`, `--namespace` and a new `--output` file. This command contacts the selected API server. The output file is always a JSON snapshot; stdout is a summary whose presentation can be Markdown/HTML. Existing snapshots are never overwritten. Secrets are excluded and selected sensitive fields are removed. Exec authentication is blocked by default; `--allow-exec-auth /absolute/path` explicitly allows a fixed external authentication executable that may contact identity services. See [authentication boundaries](docs/advanced-inspection.md#explicit-exec-authentication). Defaults and allowlists are in [operational details](docs/operations.md).

**Terminal example** — Captured against an empty loopback mock API server, not a real cluster. Only the temporary output path is shown as snapshot.json.

```console
$ kubewiser collect --kubeconfig ./config --context demo --namespace team --qps 10 --output snapshot.json
{
  "gaps": [
    {
      "resource": "coverage",
      "reason": "Secrets, custom resources, unlisted kinds and other namespaces are not collected"
    }
  ],
  "output": "snapshot.json",
  "redacted_fields": 0,
  "requests": 13,
  "resources": 0
}
$ echo $?
0
```

Additional usage with your own inputs:

```sh
kubewiser collect --kubeconfig ./config --context staging --namespace team --include-cluster-resources --qps 1 --max-items 3000 --timeout 3m --output ./snapshot-with-nodes.json --format markdown > collection-summary.md
```

</details>

<details>
<summary>scan snapshot — inspect saved resources</summary>

Validate a saved snapshot against a target version without reconnecting to the cluster. Collection gaps and redacted fields appear in `unchecked`. Resources absent from the snapshot have not been validated.

**Terminal example** — Long JSON is excerpted to selected fields. Actual output includes additional evidence and unchecked areas.

```console
$ kubewiser scan snapshot --target-version 1.37 --format text docs/examples/snapshot.json
Inspection review
Target Kubernetes 1.37 · Resources 1 · Schema checked 1
No findings in the checks performed. Review coverage before deciding that a change is safe.
Required: 0 | Conditional: 0 | Optional: 0

NOT VERIFIED (partial coverage):
- Admission, CEL, controller behavior, feature-gate availability and application runtime compatibility are not evaluated
- OpenAPI validation does not replace Kubernetes admission or server semantic validation
- Schemas are pinned to minor .0 releases; patch-specific changes are not inferred
- coverage: Synthetic documentation fixture: only one Service included
- Redacted fields may not be schema-validated
$ echo $?
0
```

</details>

<details>
<summary>diff — compare desired and observed state</summary>

Use exactly one of `--source` or `--chart`, plus `--snapshot`. Namespace defaults to the snapshot namespace. `changed` means differing values (values are omitted); `desired_only_field`/`observed_only_field` indicate fields present on one side. Observed-only fields can be defaults or controller additions. `not_observed` may reflect missing collection scope or permissions. `observed_only_resource` is not a deletion recommendation. `conversion_unverified` means API-version conversion is needed.

**Terminal example** — Long JSON is excerpted to selected fields. Actual output includes additional evidence and unchecked areas.

```console
$ kubewiser diff --source docs/examples/desired.yaml --snapshot docs/examples/snapshot.json
{
  "changes": [
    {
      "resource": "/Service/team/web",
      "path": "/spec/ports",
      "type": "changed",
      "message": "Declared and observed values differ; values omitted"
    }
  ]
}
$ echo $?
1
```

Additional usage with your own inputs:

```sh
kubewiser diff --chart ./chart --target-version 1.37 --release app --values ./values.yaml --snapshot ./snapshot.json --format html > drift.html
```

</details>

<details>
<summary>plan upgrade — list upgrade actions</summary>

Specify an increasing minor-version range with `--from`/`--to` and exactly one of `--source` or `--snapshot`. `steps` lists minor-by-minor progression; `actions` includes requirements, verification and evidence. Node inputs enable kubelet skew checks. Add-on, application and kernel compatibility still require separate validation.

**Terminal example** — Long JSON is excerpted to selected fields. Actual output includes additional evidence and unchecked areas.

```console
$ kubewiser plan upgrade --source docs/examples/manifest.yaml --from 1.24 --to 1.26
{
  "kind": "upgrade",
  "from": "1.24",
  "to": "1.26",
  "steps": [
    "1.25: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads",
    "1.26: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads"
  ],
  "actions": [
    {
      "id": "KW-API-REMOVED",
      "resource": "batch/v1beta1/CronJob//example-legacy-job",
      "requirement": "required",
      "message": "Not served since 1.25; migrate to batch/v1",
      "verification": "Re-run target validation and test workload behavior in staging",
      "source": "https://kubernetes.io/docs/reference/using-api/deprecation-guide/",
      "before_version": "1.26"
    }
  ]
}
$ echo $?
0
```

Additional usage with your own inputs:

```sh
kubewiser plan upgrade --snapshot ./snapshot-with-nodes.json --from 1.35 --to 1.37 --format html > upgrade-plan.html
```

</details>

<details>
<summary>scan network — inspect intent and existing flows</summary>

Read manifests or a snapshot; optionally supply existing Hubble JSON Lines with `--flows`. `exposures` shows configured exposure, `isolation` shows policy isolation intent and `observed_flows` aggregates observed endpoints, verdicts and times. No probes or agents are started. Configured or observed connectivity is not a reachability guarantee.

**Terminal example** — Long JSON is excerpted to selected fields. Actual output includes additional evidence and unchecked areas.

```console
$ kubewiser scan network --snapshot docs/examples/snapshot.json --flows docs/examples/hubble.jsonl
{
  "exposures": [
    {
      "resource": "v1/Service/team/web",
      "type": "LoadBalancer",
      "evidence": "configured exposure, not reachability tested"
    }
  ],
  "observed_flows": [
    {
      "source": "team/web",
      "destination": "team/db",
      "verdict": "FORWARDED",
      "count": 1,
      "first_observed": "2026-11-15T00:00:00Z",
      "last_observed": "2026-11-15T00:00:00Z"
    }
  ]
}
$ echo $?
0
```

Additional usage with your own inputs:

```sh
kubewiser scan network --namespace team --format markdown ./deploy > network-policy.md
```

</details>

<details>
<summary>plan migrate — inventory cloud dependencies</summary>

Choose distinct providers from `eks`, `aks`, `gke` using `--from`/`--to`. Review storage, load balancing, workload identity and platform-specific settings. No cloud API calls, provisioning, template generation or data movement occurs. Original manifests retain provider annotations that snapshots redact.

**Terminal example** — Long JSON is excerpted to selected fields. Actual output includes additional evidence and unchecked areas.

```console
$ kubewiser plan migrate --source docs/examples/desired.yaml --from eks --to aks
{
  "kind": "migration",
  "from": "eks",
  "to": "aks",
  "actions": [
    {
      "id": "KW-MIGRATE-LB",
      "resource": "v1/Service/team/web",
      "requirement": "required",
      "message": "Recreate provider load balancer configuration, annotations and health checks; verify client IP and exposure",
      "verification": "Validate target provider configuration and test in staging before cutover",
      "source": "https://learn.microsoft.com/en-us/azure/aks/what-is-aks"
    }
  ]
}
$ echo $?
0
```

Additional usage with your own inputs:

```sh
kubewiser plan migrate --snapshot ./snapshot.json --from aks --to gke --format html > migration-plan.html
```

</details>

<details>
<summary>mcp — serve tools to a local model client</summary>

```console
$ kubewiser mcp --input docs/examples/desired.yaml --snapshot docs/examples/snapshot.json --namespace team
```

No banner or report is printed while waiting for MCP requests. Configure an MCP-capable client to launch the command, or press Ctrl+C to stop a manual session. Do not redirect it to report.md. Add fixed `--chart`, `--values` and `--flows` paths to enable those inputs. See the local-model details below.

</details>

<details>
<summary>Interpreting results and exit codes</summary>

| Field | Meaning |
| --- | --- |
| `required` | Required action |
| `conditional` | Act when the stated condition applies |
| `optional` | Recommendation based on measured operational needs |
| `severity` | Error/warning severity, separate from requirement |
| `source` / `evidence` | Documentation source and evidence level |
| `unchecked` / `gaps` | Unvalidated areas and collection gaps |
| `status: partial` | Limited inspection coverage, not full compatibility assurance |

Scan exits with `0` for no findings, `1` for findings and `2` for argument/operational errors. Diff returns `1` for changes. Successful collect/plan/network commands return `0` even with actions or gaps. **Exit 0 does not prove compatibility.** Findings exit code 1 still produces reports. Shell `>` overwrites existing report files; choose filenames accordingly.

</details>
<a id="local-mcp-and-local-models"></a>

## 🤖 Local MCP and local models

```sh
kubewiser mcp --input /absolute/path/deploy --snapshot /absolute/path/snapshot.json
```

An MCP-capable client can connect a local DeepSeek/Ollama model to this stdio server. Kubewiser does not launch or call an LLM. Tools: `scan_files`, `scan_cgroup`, `scan_lifecycle`, `plan_upgrade`, `plan_migrate`, `scan_network`; `scan_cel` requires input, `plan_aks` requires a profile; `compare_drift` requires input and snapshot, `scan_helm` requires `--chart`. Paths and cluster scope are fixed by the operator at startup, not chosen by the model. Use `--namespace` when the desired manifests omit it.

All MCP tools now declare four explicit behavior hints and have named protocol-call tests. MCP collection rejects authentication subprocesses; use explicit CLI collection and a saved snapshot when exec authentication is needed. [Verification evidence and M8ven follow-up](docs/mcp-trust.md) · [Security policy](SECURITY.md).

Collection is absent by default. Enabling `--allow-cluster` also requires explicit `--kubeconfig`, `--context`, `--namespace`; collection returns a redacted snapshot to the MCP client. The client/model can transmit received data: use a local-only client configuration if external transmission is prohibited. Paths, hashes, resource names, IPs and remaining snapshot metadata can still be sensitive. Treat chosen charts and manifests as trusted local inputs.

<a id="build-and-development"></a>

## 🔧 Build and development

Requires Go 1.26+; CI pins the toolchain in `.go-version`. Runtime binaries require no Go installation.

```sh
go build -o kubewiser ./cmd/kubewiser
go test -race ./...
go vet ./...
```

Third-party dependencies and embedded official schemas are covered by [THIRD_PARTY_NOTICES.txt](THIRD_PARTY_NOTICES.txt). Reports use format v2; consumers of earlier alpha reports must update their parsers.

<a id="scope-and-evidence"></a>

## 🔎 Scope and evidence

Selected exact patch schemas and separate CRD CEL inspection are available; the actual schema version and remaining coverage are reported. Admission webhooks, target feature gates, application/kernel execution and full network reachability require environment evidence. No automatic infrastructure generation or data migration is performed. Numeric CPU/memory sizing needs measurements; this report does not invent values.

[Official sources](docs/official-sources.md) · [Operational boundaries](docs/operations.md) · [Report design references](docs/report-design.md)

<a id="development-and-feedback"></a>

## 💬 Development and feedback

Report reproducible problems through [GitHub Issues](https://github.com/uknowpro/kubewiser/issues). Include the command, target version and a small sanitized example. Do not attach kubeconfig credentials, Secrets or unreviewed production snapshots.

[Roadmap](docs/roadmap.md) · [Release notes](https://github.com/uknowpro/kubewiser/releases) · [License](LICENSE) · [Third-party notices](THIRD_PARTY_NOTICES.txt)
