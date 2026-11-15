# kubewiser

Evidence-oriented Kubernetes inspection CLI. Initial implementation: offline manifest checks and a machine-readable report. No cluster requests or resource mutations.

## Binary distribution (no Go installation required)

Linux and macOS archives for amd64 and arm64, plus a Windows 10+ x64 and x86 ZIPs are prepared by `scripts/release.sh`. See [installation and release instructions](docs/releases.md). Tagged releases are staged as GitHub drafts after CI. Downloads: [GitHub Releases](https://github.com/uknowpro/kubewiser/releases). Windows users can extract the ZIP and run `.\kubewiser.exe --version` in PowerShell; Go and WSL are not required.

Use `kubewiser --version` to inspect the embedded version, commit and platform.

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

## Command names

`kubewiser` and `kubewise` are equivalent commands, including `scan files`, `mcp`, and `--version`. Both use the same CLI implementation and return the same output and exit codes. The product/version banner remains `kubewiser` for either name.

Linux/macOS archives include both commands. Windows ZIPs ship one EXE and `install.ps1`. Run `.\install.ps1` once from the extracted folder: it installs one physical executable with two hardlink names and registers the user PATH, so both commands work in PowerShell and CMD without shell aliases. Requires a hardlink-capable local filesystem (normally NTFS); see [installation details](docs/releases.md). Existing terminal applications may need restarting.

```sh
go install github.com/uknowpro/kubewiser/cmd/kubewiser@latest
go install github.com/uknowpro/kubewiser/cmd/kubewise@latest
```

For prerelease source installation, replace `@latest` with the exact release tag. Windows 10 32-bit users should download `windows_386.zip`; x64 users should prefer `windows_amd64.zip`.

## Next development stages

See [the implementation roadmap and primary references](docs/roadmap.md).
