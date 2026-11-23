# Dockerfile cgroup v2 review / Dockerfile 자동 검사

Available in v0.2.0-alpha.13. The CLI automatically inspects application build declarations alongside `scan cgroup`, `plan upgrade` and `plan aks`. No Docker installation, cluster access, network call, build, pull or script execution is needed.

```sh
kubewiser scan cgroup --source ./project --format markdown
kubewiser plan upgrade --source ./project --from 1.21 --to 1.36 --format html > upgrade.html
kubewiser plan aks --source ./project --profile ./project.json --format html > aks.html
```

No new option is required. `--source` remains the manifest input: use a project scope containing Kubernetes YAML/JSON plus the relevant Dockerfiles. This does not turn the manifest loader into a general source-code/package-file parser. If manifests are in `deploy/` but the Dockerfile is above it, the parent is **not** searched: choose an appropriate input scope or explicitly add `--dockerfile ../Dockerfile`. The explicit option replaces automatic discovery and supports Dockerfile-only `scan cgroup --dockerfile ./Dockerfile`.

## Automatic scope

- Directory input: recursively find `Dockerfile`, `Containerfile`, `Dockerfile.*`, `Containerfile.*`, `*.Dockerfile` and `*.Containerfile` (case insensitive).
- Manifest file input: inspect matching files in the same directory only, without traversing descendants or parent repositories.
- Skip symbolic links, hidden directories, `node_modules`, `vendor`, `dist`, `build`, `target`, and `__pycache__`. Docker ignore files are not interpreted; the inspected scope is not an exact build context.
- Bounds: 10,000 visited entries, 32 files, 1 MiB/file, 8 MiB total, 1,000 logical instructions/file. Partial discovery, malformed automatic inputs and limits produce visible input-missing rows. Explicit invalid files fail with exit 2.
- Snapshot-only inputs have no source search. MCP operator-fixed tool inputs do not gain automatic filesystem discovery or a model-selectable file option in this release.

## Findings

| Evidence | Result | Follow-up |
| --- | --- | --- |
| Recognized `openjdk`/`eclipse-temurin` tag below selected HotSpot baseline | Review required | Check vendor backports and the actual runtime; upgrade a maintained build if needed |
| Recognized tag meets baseline | Execution validation required | Check effective JVM flags, image digest, memory/OOM and CPU behavior |
| Java `8`/`11` without update, private image, ARG, stage alias, digest-only or another runtime | Input missing | Supply/review actual runtime version and vendor support |
| v1 file names in RUN/CMD/ENTRYPOINT/ENV/ARG/HEALTHCHECK/ONBUILD | Review required | Inspect readers and fallbacks; v2 interfaces have different semantics |
| `-XX:-UseContainerSupport` | Review required | Review effective JVM resource sizing |
| COPY/ADD/RUN/ONBUILD | Input missing | Copied/installed content and inherited configuration are not resolved |
| Heredoc, non-default escape, custom frontend | Analyzer unsupported | This bounded instruction reader does not execute or implement a full Docker build frontend |

Rows identify file, numeric stage and starting line, plus the file SHA-256. All declared stages are reviewed, including unused builders; stage presence is not evidence of a deployed application dependency. A tag is mutable and may be overridden by later instructions. No green compatibility verdict is issued, and no stage is automatically bound to a Kubernetes image or node pool. Raw commands, ENV/ARG values and unrecognized image references are omitted; filenames and matched rule tokens remain visible.

Only selected Java/HotSpot declaration baselines are classified in this release. Node.js, .NET, Go, Python, copied startup scripts, application libraries and SBOMs remain unverified. `--after` compares manifests, not Dockerfile findings. A static token may be an unused string, inline comment, build-only operation or a safe v1/v2 fallback.

The grouped HTML matrix includes a **Dockerfile / application runtime** topic; existing inspection and cgroup tables remain below it. JSON adds `application_sources`; Markdown/text include the same evidence. No Dockerfile found is explicitly input missing, not a pass. `scan cgroup` returns 1 when actions exist; plans retain exit 0 on a successfully generated plan.

## 한국어 / Windows 10 PowerShell

기존 명령의 `--source` 안에서 자동으로 검사하므로 별도 옵션은 필요 없습니다. 다만 `--source ./deploy` 밖의 상위 Dockerfile은 자동으로 읽지 않습니다. 소스에 Dockerfile이 없거나 태그만으로 판단할 수 없으면 미확인으로 표시합니다. **Dockerfile 선언의 위험 단서 검사이며 실제 바이너리 실행 호환성 보증은 아닙니다.**

Windows PowerShell 5.1에서는 표준 출력 해석과 저장 인코딩을 모두 UTF-8로 지정합니다.

```powershell
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new()
.\kubewiser.exe plan upgrade --source .\project --from 1.21 --to 1.36 --format html --lang ko |
    Out-File -FilePath .\upgrade.html -Encoding utf8
```

[합성 Dockerfile](examples/dockerfile/Dockerfile) · [manifest](examples/dockerfile/workload.yaml) · [한국어 HTML](examples/dockerfile/report.ko.html) · [English HTML](examples/dockerfile/report.html)

## Official references reviewed 2026-11-23

- [Kubernetes cgroup v2 application migration guidance](https://kubernetes.io/docs/concepts/architecture/cgroups/): selected HotSpot feature baselines are 8u372, 11.0.16 and 15. These are feature thresholds, not current maintenance recommendations.
- [Dockerfile reference](https://docs.docker.com/reference/dockerfile/): stage, argument, parser directive and heredoc semantics.
- [Microsoft PowerShell character encoding](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_Character_Encoding): Windows PowerShell redirection and explicit UTF-8 output.
