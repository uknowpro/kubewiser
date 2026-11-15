# kubewiser

Evidence-oriented Kubernetes inspection CLI and local MCP server. Both `kubewiser` and `kubewise` run the same engine. Analysis is offline; only an explicit `collect` command (or explicitly enabled MCP collection) contacts your selected Kubernetes API server. No telemetry, automatic update checks, document downloads, cloud API calls or resource mutations.

## Download and install

[GitHub Releases](https://github.com/uknowpro/kubewiser/releases) provides Linux/macOS amd64 and arm64, Windows 10+ amd64 and 386 builds. **End users do not need Go, Helm, kubectl or WSL.** Windows ZIPs contain one EXE; `install.ps1` registers both commands using hardlinks to one executable. See [installation instructions](docs/releases.md), including native test limitations and checksums.

## Implemented inspection paths

| Area | Implemented | Boundary |
| --- | --- | --- |
| Versioned validation | Embedded official Kubernetes 1.16–1.37 `.0` schemas; unknown fields, types and schema-required fields; local CRDs; deprecated/removed API rules | Schema validation is not admission validation or a guarantee of workload compatibility |
| Collection | Explicit context/namespace; sequential GETs, pagination, request/time/item limits; gaps and redacted paths | Fixed resource allowlist; no Secrets/custom resources; no exec/cloud auth plugins |
| Helm and drift | Local vendored chart rendering; target capabilities; source hashes; snapshot comparison with named-list normalization | No install, dependency downloads, live lookup, values.schema.json validation or inferred field ownership |
| Upgrade planning | Minor-by-minor steps, required/conditional actions, kubelet skew evidence and dependency recommendations | Vendor matrices, behavior changes and real application tests require separate validation |
| Network and migration | NetworkPolicy isolation intent, Ingress/Service exposure, existing Hubble NDJSON; EKS/AKS/GKE dependency actions | No active probes, firewall reachability proof, infrastructure generation or stateful data migration |

See [official sources and coverage](docs/official-sources.md), [operational details](docs/operations.md) and [remaining roadmap](docs/roadmap.md).

## Examples

Flags precede positional paths. Use `--help` for commands and `versions` for the bundled official URLs and SHA-256 hashes.

```sh
kubewiser scan files --target-version 1.37 --recommend ./deploy
kubewiser scan helm --target-version 1.37 --release app --namespace team --values ./values.yaml ./chart
kubewiser collect --kubeconfig ./config --context staging --namespace team --output ./snapshot.json
kubewiser scan snapshot --target-version 1.37 ./snapshot.json
kubewiser diff --source ./deploy --snapshot ./snapshot.json
kubewiser diff --chart ./chart --target-version 1.37 --release app --snapshot ./snapshot.json
kubewiser plan upgrade --snapshot ./snapshot.json --from 1.35 --to 1.37
kubewiser scan network --snapshot ./snapshot.json --flows ./hubble.jsonl
kubewiser plan migrate --source ./deploy --from eks --to aks
```

JSON is the default. `--format text` gives concise text for direct scan reports; other results remain JSON. Scan exit codes: `0` no findings in performed checks, `1` findings, `2` argument/operational error. Diff returns `1` for changes. Successful collection/planning/network commands return `0` even if they contain gaps or actions. **Always inspect `unchecked`, coverage and collection gaps; exit 0 does not prove compatibility.**

## Local MCP and local models

```sh
kubewiser mcp --input /absolute/path/deploy --snapshot /absolute/path/snapshot.json
```

An MCP-capable client can connect a local DeepSeek/Ollama model to this stdio server. Kubewiser does not launch or call an LLM. Tools: `scan_files`, `plan_upgrade`, `plan_migrate`, `scan_network`; `compare_drift` requires input and snapshot, `scan_helm` requires `--chart`. Paths and cluster scope are fixed by the operator at startup, not chosen by the model. Use `--namespace` when the desired manifests omit it.

Collection is absent by default. Enabling `--allow-cluster` also requires explicit `--kubeconfig`, `--context`, `--namespace`; collection returns a redacted snapshot to the MCP client. The client/model can transmit received data: use a local-only client configuration if external transmission is prohibited. Paths, hashes, resource names, IPs and remaining snapshot metadata can still be sensitive. Treat chosen charts and manifests as trusted local inputs.

## Build and development

Requires Go 1.26+; CI pins the toolchain in `.go-version`. Runtime binaries require no Go installation.

```sh
go build -o kubewiser ./cmd/kubewiser
go test -race ./...
go vet ./...
```

Third-party dependencies and embedded official schemas are covered by [THIRD_PARTY_NOTICES.txt](THIRD_PARTY_NOTICES.txt). Reports use format v2; consumers of earlier alpha reports must update their parsers.
