# AKS project planning / AKS 프로젝트 계획

Available from **v0.2.0-alpha.4**. Analysis and generation remain local. No Azure API calls, infrastructure commands, image pulls, model calls or telemetry occur.

## English

### Plan an existing AKS upgrade

Copy [the profile](examples/project/aks.json), set current/target versions, maintenance window, maximum downtime and recovery objectives. The 1.21 → 1.35 example represents a requirement, **not a supported AKS upgrade route**. Unknown requirements produce required actions. A project plan always has `status: review_required`; completing text fields does not certify readiness.

```sh
kubewiser plan aks --source ./deploy --profile ./project.json --format html > aks-plan.html
kubewiser plan aks --snapshot ./snapshot.json --profile ./project.json --format markdown > aks-plan.md
kubewiser generate --profile ./project.json --output ./new-review-drafts
kubewiser mcp --input ./deploy --profile ./project.json
```

MCP adds `plan_aks` only when the operator configures a profile. The model cannot change its path, and generation is not exposed as an MCP write tool.

Optional `architecture_changes` entries require `area`, `current`, `target`, `reason`. Supported areas: `network`, `nodepool`, `cpu`, `os`, `ingress`, `storage`, `identity`. These are declared requirements, not observed configuration. Example:

```json
{"area":"cpu","current":"amd64","target":"arm64","reason":"Approved application platform requirement"}
```

Optional `addons` entries hold locally supplied vendor-matrix evidence. No sources are downloaded. Each entry needs `name`, `version`, `min_kubernetes`, `max_kubernetes`, `source` (HTTP/S URL), `reviewed`. Ranges are checked against the entire current-to-target minor range. They are **operator claims**, not a built-in verified compatibility database; patch support and vendor exceptions still require review. Use real vendor sources and dates, never guessed ranges.

### What the plan establishes

- API removal deadlines use the first affected version, including problems already present before starting.
- Crossed 1.24/1.25 milestones add dockershim, ServiceAccount token and PodSecurityPolicy behavior reviews.
- RuntimeClass, host networking, sysctls, privileged containers and hostPath indicate OS/runtime dependency review.
- AKS route, restore rehearsal, PDB/drain/surge/quota checks precede execution. Each architecture change has its own canary and acceptance work.
- Fresh `az aks get-upgrades` evidence is required before each actual step. The command is **not executed** by kubewiser. A list of regionally available versions alone does not establish a cluster-specific route. Unsupported old clusters may need provider assistance or a replacement strategy.
- Maintenance/RPO/RTO inputs and architecture requirements remain in the report's original evidence. No downgrade or zero-downtime guarantee is made.

### Generated artifacts

The destination directory must not exist. Files are private on POSIX (directory 0700, files 0600); existing directories and files are never overwritten. A failed write can leave a partial directory for inspection; choose a fresh directory on retry.

| File | Scope |
| --- | --- |
| `project.json` | Declared requirements, copied without workload content |
| `acceptance.md` | Owners, evidence, acceptance and recovery checklist |
| `nodepool.tf` | Opt-in additional AKS user pool; no control-plane import/upgrade; required sizing/version/cluster inputs have no guessed defaults |
| `chart/` | Disabled-by-default canary Deployment; requires an approved image when enabled |
| `manifest.yaml` | Canary ReplicaSet with zero replicas and placeholder image |
| `README.md` | Review and execution boundaries |

These are **review scaffolds**, not generated production architecture, workload conversion, data transfer or approved Terraform plans. Pin and validate the provider in your own Terraform workflow. Helm is rendered/tested locally through kubewiser; Terraform cloud/provider execution is not performed.

### Targeted network intent

```sh
kubewiser scan path --source ./network-inputs --source-pod team/client --destination-pod team/api --port 8080 --protocol TCP --format html > path.html
```

Supply explicit Pod endpoints and all relevant NetworkPolicies; include Namespace objects for namespace-label selectors. Evaluation combines egress and ingress, additive policies, selectors, numeric/named ports, port ranges and IPBlock exceptions. Missing namespace/IP evidence and host networking can return `unknown`. Result names are `allowed_by_policy`, `denied_by_policy`, `unknown`: these describe **supplied policy intent**, not observed reachability. Service/NAT translation, CNI enforcement, DNS and cloud routing/firewalls remain outside this evaluator.

### Drift and collection

Locally supplied CRD schemas enable map/set-list comparison; atomic/unspecified lists retain order. Hashed map keys avoid directly printing arbitrary key values (hashes are not anonymization). `field_managers` records hints only: it neither proves the cause of drift nor authorizes overwrite. API-version conversion remains unverified.

Collection now also includes Jobs, ReplicaSets and HPAs. Known older server versions select served beta endpoints for CronJob, Ingress, PDB and HPA within the supported inspection range. Unknown server versions keep stable endpoints and report failures as gaps. RBAC and authentication restrictions remain unchanged; no exec plugin is enabled.

## 한국어

`plan aks`는 기존 AKS의 버전 업그레이드와 아키텍처 변경 요구사항을 함께 검토합니다. [예제 프로필](examples/project/aks.json)의 버전·작업 시간·허용 중단·RPO·RTO를 프로젝트에 맞게 수정하세요. **1.21 → 1.35는 요구사항 예제이며 실제 가능한 업그레이드 경로라는 뜻이 아닙니다.** 미정 항목은 필수 확인 사항으로 남고, 결과는 항상 `review_required`입니다.

```sh
kubewiser plan aks --source ./deploy --profile ./project.json --format html --lang ko > aks-plan.html
kubewiser generate --profile ./project.json --output ./new-review-drafts
```

- `architecture_changes`: network/nodepool/cpu/os/ingress/storage/identity별 `current`, `target`, `reason`을 입력합니다. 설정을 관측한 결과가 아니라 사용자가 선언한 요구사항입니다.
- `addons`: 이름·버전·지원 Kubernetes 범위·출처·확인일을 입력하면 전체 버전 경로를 포함하는지 검사합니다. 공식 호환성 데이터베이스를 자동 조회하거나 입력 근거를 사실로 보증하지 않습니다.
- API 변경 시점, 1.24/1.25 동작 변화, OS·런타임 의존성, 실제 AKS 경로 확인, 복구 리허설, PDB·노드 배출·여유 IP·할당량을 조치로 제시합니다.
- 단계별 진행 여부는 실제 AKS 조회와 검증 환경 결과로 결정해야 합니다. Kubernetes 버전 다운그레이드를 복구 수단으로 가정하지 않습니다.
- 생성물은 추가 노드풀 Terraform, 기본 비활성 Helm 차트, replica 0 manifest, 검수 체크리스트입니다. 운영 구성 자동 변환·데이터 이전·배포는 수행하지 않습니다. 기존 출력 디렉터리는 덮어쓰지 않습니다.
- `scan path`는 입력한 두 Pod 사이의 ingress/egress 정책 의도를 분석합니다. 실통신을 확인하지 않으며 정보가 부족하면 `unknown`을 표시합니다.
- CRD map/set 목록 비교와 필드 관리자 근거를 추가했습니다. 관리자 정보만으로 변경 원인이나 덮어쓰기 권한을 확정하지 않습니다.

## Primary references (reviewed 2026-11-16)

- [AKS control-plane upgrades and offered routes](https://learn.microsoft.com/en-us/azure/aks/upgrade-aks-control-plane)
- [AKS upgrade options](https://learn.microsoft.com/en-us/azure/aks/upgrade-options)
- [AKS backup overview](https://learn.microsoft.com/en-us/azure/backup/azure-kubernetes-service-backup-overview)
- [Kubernetes API migration](https://kubernetes.io/docs/reference/using-api/deprecation-guide/)
- [Dockershim migration](https://kubernetes.io/docs/tasks/administer-cluster/migrating-from-dockershim/)
- [ServiceAccounts](https://kubernetes.io/docs/concepts/security/service-accounts/)
- [PodSecurityPolicy removal](https://kubernetes.io/docs/concepts/security/pod-security-policy/)
- [NetworkPolicy semantics](https://kubernetes.io/docs/concepts/services-networking/network-policies/)
- [Field management](https://kubernetes.io/docs/reference/using-api/server-side-apply/)
- [AzureRM node pool resource](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/kubernetes_cluster_node_pool)
