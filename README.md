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

## 명령어 안내 / Command reference

아래의 `kubewiser`는 모두 `kubewise`로 바꿔 실행할 수 있습니다. 옵션은 입력 경로 앞에 둡니다. 예제의 경로·컨텍스트·네임스페이스는 실제 환경에 맞게 변경하세요.

### 결과 형식과 저장

v0.2.0-alpha.2부터 `--format markdown`과 `--format html`을 지원합니다. 이전 배포판은 업데이트가 필요합니다.

| 형식 | 용도 | 저장 예시 |
| --- | --- | --- |
| `json` (기본) | 자동화·프로그램 연동 | `--format json ... > report.json` |
| `text` | 직접 scan 보고서의 간략한 터미널 출력 | `--format text ...` |
| `markdown` | GitHub·에디터에서 표와 목록으로 검토 | `--format markdown ... > report.md` |
| `html` | 브라우저에서 보고서 열기·인쇄 | `--format html ... > report.html` |

`text`의 간략 출력은 `scan files`와 `scan snapshot`에 적용되며 다른 결과는 JSON으로 표시됩니다. Markdown/HTML은 scan, diff, plan, collect 요약에 적용됩니다. `versions`는 json/markdown/html을 지원합니다. MCP는 통신 프로토콜의 구조화된 응답을 사용합니다.

```sh
# Linux/macOS: Markdown 또는 HTML 파일로 저장
kubewiser scan files --target-version 1.37 --recommend --format markdown ./deploy > report.md
kubewiser scan files --target-version 1.37 --recommend --format html ./deploy > report.html
```

```powershell
# Windows PowerShell: 한글을 보존하도록 UTF-8로 저장
kubewiser scan files --target-version 1.37 --recommend --format markdown .\deploy | Out-File -Encoding utf8 report.md
kubewiser scan files --target-version 1.37 --recommend --format html .\deploy | Out-File -Encoding utf8 report.html
Start-Process .\report.html
```

HTML은 CSS를 포함한 단일 파일이며 외부 폰트·스크립트·이미지를 불러오지 않습니다. 브라우저에서 파일을 열어 읽거나 인쇄할 수 있습니다. 두 형식은 JSON의 근거·미검사 항목을 유지하며 입력 문자열의 HTML/Markdown 문법을 이스케이프합니다. 파일에는 경로·리소스명 등 메타데이터가 포함될 수 있습니다.

실제 생성 예시: [Markdown 보고서](docs/examples/report.md), [HTML 파일](docs/examples/report.html), [예제 입력](docs/examples/manifest.yaml). GitHub에서 HTML은 소스 코드로 표시되므로 다운로드한 파일을 브라우저로 여세요.

### `--help`, `--version`, `versions` — 도움말과 지원 버전

```sh
kubewiser --help
kubewiser --version
kubewiser versions --format markdown > supported-versions.md
```

`--version`은 실행 파일의 제품 버전·커밋·운영체제를 표시합니다. `versions`는 내장 Kubernetes 스키마 버전, 공식 출처 URL, 원본·번들 SHA-256과 확인일을 보여줍니다. 예를 들어 1.37.x 요청은 내장된 1.37.0 스키마를 사용합니다.

### `scan files` — YAML/JSON manifest 검사

파일 하나 또는 디렉터리를 재귀 검사합니다. 스키마 오류와 API 폐기·제거 여부를 찾습니다. `--recommend`는 readiness probe, resource requests 등 조건부 권고를 추가합니다. CRD와 해당 사용자 정의 리소스를 함께 입력하면 로컬 CRD 스키마도 사용합니다.

```sh
kubewiser scan files --target-version 1.37 --format markdown ./deployment.yaml
kubewiser scan files --target-version 1.37 --recommend --format html ./deploy > manifest-report.html
```

결과에서 `findings`의 규칙 ID, 파일·줄·필드 위치, `requirement`, `severity`, `source`를 확인합니다. `schema_checked`는 스키마로 검사한 리소스 수입니다.

### `scan helm` — 로컬 Helm 차트 렌더링 후 검사

압축을 푼 로컬 차트와 이미 준비된 의존 차트가 필요합니다. `--release`, `--namespace`, `--values`로 렌더링 조건을 고정합니다. Helm 설치는 필요하지 않으며 의존 차트 다운로드, live lookup, 리소스 적용은 하지 않습니다.

```sh
kubewiser scan helm --target-version 1.37 --release app --namespace team --values ./values.yaml --format markdown ./chart > helm-report.md
```

`render`에는 차트 버전·values 해시·capabilities가, `report`에는 검사 결과가 나옵니다. `--api-versions v1,apps/v1,example.org/v1/Widget`은 기본 capabilities를 **교체**하므로 필요한 전체 목록을 명시해야 합니다. values.schema.json 검증은 수행하지 않습니다.

### `collect` — 읽기 전용 스냅샷 수집

선택한 API 서버에서 지정한 namespace의 허용된 리소스만 GET으로 수집합니다. 이 명령은 네트워크에 접근합니다. kubeconfig·context·namespace·새 출력 파일 경로를 명시해야 합니다.

```sh
kubewiser collect --kubeconfig ./config --context staging --namespace team --output ./snapshot.json --format markdown > collection-summary.md
# Node와 StorageClass도 포함하고, 초당 요청 수와 전체 제한을 지정
kubewiser collect --kubeconfig ./config --context staging --namespace team --include-cluster-resources --qps 1 --max-items 3000 --timeout 3m --output ./snapshot-with-nodes.json --format html > collection-summary.html
```

`--output` 파일은 후속 분석용 **JSON 스냅샷**이고, 표준 출력은 수집 개수·요청 수·누락(`gaps`)·제거한 필드 수의 **요약**입니다. `--format`은 요약 형식만 바꿉니다. 기존 스냅샷 파일은 덮어쓰지 않습니다. Secret은 수집하지 않으며 지정된 민감 필드를 제거합니다. 인증 exec 플러그인은 지원하지 않습니다. 자세한 허용 목록과 한도는 [운영 문서](docs/operations.md)를 확인하세요.

### `scan snapshot` — 저장된 스냅샷의 목표 버전 검사

클러스터에 다시 접속하지 않고 저장된 리소스를 검사합니다.

```sh
kubewiser scan snapshot --target-version 1.37 --recommend --format html ./snapshot.json > snapshot-report.html
```

수집 누락과 제거된 필드는 `unchecked`에 함께 표시됩니다. 스냅샷에 없는 종류의 리소스까지 검증된 것으로 해석하지 마세요.

### `diff` — 원하는 상태와 수집한 상태 비교

manifest 경로(`--source`) 또는 Helm 차트(`--chart`) 중 하나와 `--snapshot`을 지정합니다. namespace를 생략하면 스냅샷의 namespace를 사용합니다.

```sh
kubewiser diff --source ./deploy --snapshot ./snapshot.json --format markdown > drift.md
kubewiser diff --chart ./chart --target-version 1.37 --release app --values ./values.yaml --snapshot ./snapshot.json --format html > drift.html
```

| 결과 종류 | 해석 |
| --- | --- |
| `changed` | 선언값과 관측값이 다름; 실제 값은 출력하지 않음 |
| `desired_only_field` | 선언한 필드가 스냅샷에 없음 |
| `observed_only_field` | 스냅샷에만 있음; 기본값·컨트롤러 추가 여부 확인 필요 |
| `not_observed` | 스냅샷에서 발견되지 않음; 삭제 외에도 범위·권한 누락 가능 |
| `observed_only_resource` | 제공한 manifest 집합 밖의 리소스; 삭제 권고가 아님 |
| `conversion_unverified` | API 버전이 달라 의미 비교 전에 변환 검증 필요 |

### `plan upgrade` — 버전 업그레이드 조치 목록

현재 버전 `--from`과 목표 버전 `--to`, `--source` 또는 `--snapshot` 중 하나를 지정합니다. 목표 마이너 버전은 더 높아야 합니다.

```sh
kubewiser plan upgrade --source ./deploy --from 1.35 --to 1.37 --format markdown > upgrade-plan.md
kubewiser plan upgrade --snapshot ./snapshot-with-nodes.json --from 1.35 --to 1.37 --format html > upgrade-plan.html
```

`steps`는 마이너별 진행 순서, `actions`는 조치·필수 여부·검증 방법·근거입니다. Node가 있으면 kubelet 버전 차이도 확인합니다. 애드온·실제 앱 동작·커널 호환성이 자동 검증되는 것은 아닙니다.

### `scan network` — 설정과 기존 통신 기록 분석

manifest 또는 스냅샷을 읽어 Service/Ingress 노출과 NetworkPolicy 격리 의도를 확인합니다. `--flows`는 선택 사항이며 기존 Hubble JSON Lines 파일을 추가 분석합니다.

```sh
kubewiser scan network --namespace team --format markdown ./deploy > network-policy.md
kubewiser scan network --snapshot ./snapshot.json --flows ./hubble.jsonl --format html > network-report.html
```

`exposures`는 설정상 노출, `isolation`은 정책의 ingress/egress 격리 의도, `observed_flows`는 관측된 통신·판정·횟수·시간입니다. 능동 포트 스캔이나 에이전트 설치를 하지 않으며 실제 연결 가능 여부를 확정하지 않습니다.

### `plan migrate` — 클라우드 이전 의존성 확인

`eks`, `aks`, `gke` 중 서로 다른 출발·대상 플랫폼을 지정합니다. 스토리지, 로드밸런서, 워크로드 ID, 플랫폼 종속 설정 등을 조치 목록으로 정리합니다.

```sh
kubewiser plan migrate --source ./deploy --from eks --to aks --format markdown > migration-plan.md
kubewiser plan migrate --snapshot ./snapshot.json --from aks --to gke --format html > migration-plan.html
```

클라우드 API 호출·인프라 생성·데이터 이전은 수행하지 않습니다. 스냅샷에서는 annotation이 제거되므로 플랫폼별 annotation 검토에는 원본 manifest가 더 적합합니다.

### `mcp` — 로컬 모델용 검사 도구 서버

```sh
kubewiser mcp --input /absolute/path/deploy --snapshot /absolute/path/snapshot.json --namespace team
# Helm과 기존 네트워크 기록도 도구에 연결
kubewiser mcp --input /absolute/path/deploy --chart /absolute/path/chart --values /absolute/path/values.yaml --flows /absolute/path/hubble.jsonl --namespace team
```

MCP 클라이언트가 위 명령을 프로세스로 실행하도록 설정합니다. 일반 보고서 생성 명령과 달리 요청을 기다리는 stdio 서버이며 `> report.md`로 저장하는 명령이 아닙니다. 상세 도구와 로컬 모델 연동은 다음 절을 참고하세요.

### 결과 해석과 종료 코드

| 항목 | 의미 |
| --- | --- |
| `required` | 필수 조치 |
| `conditional` | 해당 조건에 맞는지 확인한 뒤 조치 |
| `optional` | 측정·운영 요구를 고려한 권고 |
| `severity` | 오류·경고 등 심각도; 필수 여부와 별개 |
| `source` / `evidence` | 근거 문서와 판정 근거 수준 |
| `unchecked` / `gaps` | 검사하지 못한 영역·수집 누락 |
| `status: partial` | 전체 호환성 보증이 아닌 범위가 제한된 검사 |

scan은 발견 사항이 없으면 `0`, 있으면 `1`, 인자·실행 오류는 `2`입니다. diff는 차이가 있으면 `1`입니다. collect/plan/network는 성공 시 조치·누락이 있어도 `0`입니다. **종료 코드 0은 전체 호환성 보증이 아닙니다.** 발견 사항이 있어 `1`로 종료해도 Markdown/HTML 보고서는 생성됩니다. 셸의 `>`는 기존 보고서를 덮어쓰므로 보관할 파일명은 구분하세요.

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
