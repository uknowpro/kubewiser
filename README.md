# kubewiser

Evidence-oriented Kubernetes inspection CLI. Initial implementation: offline manifest checks and a machine-readable report. No cluster requests or resource mutations.

## Build and use

Requires Go 1.23 or newer.

```sh
go build -o kubewiser ./cmd/kubewiser
./kubewiser scan files --target-version 1.32 --format json ./deploy
```

Flags must precede the input path. A path may be a YAML/JSON file or a directory. Directories are scanned recursively in sorted order, excluding hidden directories and symlink entries. Multi-document YAML is supported. Render Helm charts separately before scanning.

Exit codes: `0` no findings in the checks performed; `1` findings; `2` invalid arguments or operational failure. **Exit 0 does not mean Kubernetes compatibility is proven.** Reports always include `status: partial` and unchecked areas.

## Implemented checks and boundaries

- YAML syntax, duplicate mapping keys, mapping-shaped documents, required string apiVersion/kind.
- A deliberately small API removal table: Deployment legacy APIs (1.16), Ingress and CRD beta APIs (1.22), CronJob/PDB/PSP/HPA beta APIs (1.25/1.26).
- Targets 1.16–1.32 are accepted for this initial rule table. Other targets fail explicitly. This is not a claim of full coverage for those releases.
- Deterministic JSON, rule identifiers, source location, input SHA-256 hashes, remediation guidance and official evidence link.
- Raw manifest values and parser diagnostic excerpts are not returned. File paths are included and can be sensitive.

Not yet implemented: complete Kubernetes/CRD schemas, arbitrary unknown-field detection, metadata/spec validation, List item recursion, Helm rendering, live cluster collection, drift, application/network/cloud analysis, or automatic patches. List documents produce a review finding instead of silently skipping their items. Partial results are not emitted after file I/O errors.

## Development

```sh
go test -race ./...
go vet ./...
```

Next milestones: versioned schema bundles and full validation; read-only snapshots; Helm/drift normalization; upgrade action plans; additional MCP tools over the same engine. Local models connect through an MCP-capable client. LLM output must not override deterministic findings.

API rule source: https://kubernetes.io/docs/reference/using-api/deprecation-guide/

## Local MCP

```sh
./kubewiser mcp --input /absolute/path/to/manifests
```

Configure an MCP-capable client to launch this command. The `scan_files` tool accepts `target_version` and returns the same structured report as the CLI. The input is fixed by the operator at startup; the model cannot choose another path. The operator-selected files must be trusted local inputs. The pinned SDK handles stdio framing and negotiation; no HTTP listener or model runtime is started. Local DeepSeek/Ollama use requires a separate MCP-capable client.
