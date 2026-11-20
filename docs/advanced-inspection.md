# Advanced inspection / 추가 고도화

Available from **v0.2.0-alpha.5**. All examples are review examples. No cloud changes, deployment, image pulls or network probes occur.

## Explicit exec authentication

Static credentials remain supported. Exec plugins are blocked unless the operator supplies the exact executable path:

```sh
kubewiser collect --kubeconfig ~/.kube/config --context my-aks --namespace my-app \
  --allow-exec-auth /absolute/path/to/kubelogin --output snapshot.json
```

The selected kubeconfig command must match that path or its basename. No shell is used. The plugin gets no interactive stdin, has a 30-second deadline and a 1 MiB stdout limit. Plugin output/errors are never printed; only a validated token or certificate pair is used in memory. There is no automatic credential refresh. `Always` interactive mode, `provideClusterInfo`, and legacy auth-provider plugins remain rejected. Authenticate separately when your login workflow requires interaction.

**This option authorizes an external authentication program.** It can contact identity servers, access its credential cache or launch child processes (for example Azure CLI); those operations are not confined by kubewiser's API-origin guard. Default analysis remains offline. The API collection itself remains bounded GET-only with verified TLS, no redirects and no proxy. As of v0.2.0-alpha.11, MCP rejects exec authentication. Explicit CLI collection can still use it; pass the resulting saved snapshot to MCP. Static-credential MCP collection remains separately gated by `--allow-cluster`.

## Saved AKS upgrade evidence

Capture upgrade information yourself using your approved Azure workflow. kubewiser does not run Azure CLI. In a project profile set `cluster_id`, then add:

```json
{
  "upgrade_evidence": {
    "captured_at": "2026-11-16T12:00:00Z",
    "profile": {
      "id": "/subscriptions/EXAMPLE/resourceGroups/EXAMPLE/providers/Microsoft.ContainerService/managedClusters/EXAMPLE/upgradeProfiles/default",
      "controlPlaneProfile": {
        "kubernetesVersion": "1.34.2",
        "upgrades": [{"kubernetesVersion": "1.35.2", "isPreview": false}]
      }
    }
  }
}
```

Use the actual `az aks get-upgrades --output json` result as `profile` and its actual capture time. Both CLI-style top-level `controlPlaneProfile` and REST `properties.controlPlaneProfile` are accepted. The example above is synthetic and expires; never change its timestamp to pretend it is current evidence.

`aks_route` checks cluster ID, current minor/exact patch when supplied, candidate versions and a 24-hour freshness window. Preview candidates are excluded. Its results include `next_step_offered`, `not_offered`, `cluster_mismatch`, `version_mismatch`, `stale`, `unknown`. A matching local export is **not independently authenticated** and validates only the next candidate, not an entire 1.21 → 1.35 route. Plans remain `review_required`. The earliest offered candidate up to the requested target is displayed for review, not automatically selected for execution.

## Pinned patch schemas

The bundled exact patches now include **1.35.9, 1.36.5 and 1.37.1**, in addition to 1.16.0 through 1.37.0. These are pinned snapshots, not a promise to track the latest patch automatically.

```sh
kubewiser scan files --target-version 1.35.9 --format html ./deploy > patch-report.html
kubewiser versions --format markdown > schema-inventory.md
```

An exact bundled patch is preferred; otherwise the minor `.0` schema is used and the report states the actual version and uncovered patch differences. Nothing is downloaded at runtime. All bundles were rebuilt against verified official source hashes to preserve legitimate properties named `description` while removing descriptive prose.

## CRD CEL

```sh
kubewiser scan cel --source ./crds-and-resources --format html > cel.html
kubewiser scan cel --source ./crds-and-resources --snapshot ./old-snapshot.json --format markdown > changes.md
```

Uses the pinned Kubernetes CEL library **v0.37.2** for structural CRD `x-kubernetes-validations`. Nested and `oldSelf` rules use the official implementation. Old resources match by exact API/kind/namespace/name; missing old objects are reported. Rule count, expression size, schema size/depth and runtime cost are bounded. Failures omit actual values and custom messages. Findings return exit code 1; invalid input/limits return 2. MCP exposes `scan_cel` with operator-fixed paths.

This is a separate check from schema scanning. It does not reproduce target-server feature gates, defaulting/pruning, admission webhooks, ValidatingAdmissionPolicy or ratcheting. No API calls occur. Treat snapshot redaction and missing old fields as incomplete transition evidence.

## CRD drift normalization

```sh
kubewiser diff --source ./deploy --snapshot ./snapshot.json \
  --crd-defaults --crd-conversion --format html > drift.html
```

Both options are opt-in. Local structural defaults are normalized on copies of both objects. `None` conversion changes the local observed API version only when both supplied served CRD schemas declare/default to strategy None. Webhook conversion is never called or inferred; it remains `conversion_unverified`. Pruning, server admission and resource-specific semantic migration are not simulated. Raw inputs and snapshots are not mutated.

## Add-on and image evidence

Upgrade plans recognize exact official image repositories for the pinned catalog in [addons.json](../internal/analyze/addons.json): selected cert-manager releases, Cilium 1.20.4 and historical ingress-nginx versions. Ingress-nginx retirement is a separate required review; an old tested matrix is not ongoing support. Unknown versions/mirrors/digest-only references are not guessed. Every rule records its source and review date.

Set `target_platform` (`os` and `architecture`) to review Node status inventory and image platform evidence. Optional `image_indexes` entries contain `image` and `index` (the complete local OCI/Docker index JSON). Index platform descriptors are compared with the target; digest references are compared with the SHA-256 of the supplied exact index bytes. Tag-only evidence remains mutable. This does not inspect or execute binaries, validate kernel modules, or pull from registries.

## Service paths and observations

```sh
kubewiser scan path --source ./network-inputs --source-pod team/client \
  --destination-service team/api --port 443 --protocol TCP --flows ./hubble.jsonl --format html > path.html
```

Service selectors and numeric/named `targetPort` map to supplied Pod backends (maximum 200). Per-backend ingress/egress intent is evaluated. Matching Hubble evidence uses endpoint, protocol and destination port and retains timestamps. Missing endpoints and selectorless/ExternalName Services remain unchecked. CNI-specific policy resources make the combined result unknown. DNS, readiness, Service/NAT implementation, routing and cloud firewall enforcement are not verified by declarations or historical flows.

## Configured templates

Use [the synthetic configured profile](examples/advanced/project.json) as a field reference, replacing every example value with your approved requirements:

```sh
kubewiser generate --profile ./project.json --output ./new-drafts
```

`templates.node_pool` pins the provider version and writes explicit values to `nodepool.auto.tfvars.json`. `enable_canary_pool` stays false. `templates.canary` supplies image, namespace, requested replicas, CPU/memory requests and selectors. Helm stays disabled; the manifest keeps zero replicas. The manifest is checked against the target schema before creating the output directory. Label/quantity checks and local HCL syntax tests supplement schema validation. Provider/cloud validation still belongs in an approved Terraform plan workflow. These are canary/node-pool review templates, not an automatic conversion of an entire production system.

## 한국어

- **인증:** `--allow-exec-auth /절대/경로/kubelogin`으로 지정한 실행 파일만 허용합니다. 기본은 차단이며, 이 옵션을 선택하면 인증 서버 통신이나 인증 도구의 자식 프로세스 실행이 발생할 수 있습니다. 토큰·인증 오류 원문은 출력하지 않습니다.
- **AKS 경로:** 실제 조회 결과를 프로필의 `upgrade_evidence`에 넣으면 클러스터·현재 버전·조회 시각·다음 후보를 대조합니다. 24시간이 지난 결과는 stale입니다. 입력 파일의 진위를 독립적으로 보증하거나 전체 경로를 승인하지 않습니다.
- **스키마:** 1.35.9·1.36.5·1.37.1 정확한 패치 번들을 추가했습니다. 없는 패치는 `.0`을 사용했다고 표시하며 실행 중 내려받지 않습니다. 정상 `description` 필드가 누락되던 번들 생성 오류도 수정했습니다.
- **CEL:** `scan cel`로 로컬 CRD 규칙을 공식 Kubernetes 라이브러리로 검사합니다. 이전 스냅샷이 있으면 `oldSelf`도 검사합니다. 실제 admission webhook·서버 기본값·기능 게이트를 재현하는 것은 아닙니다.
- **차이 비교:** `--crd-defaults`, `--crd-conversion`은 로컬 기본값과 `None` 변환을 선택적으로 반영합니다. Webhook 변환은 실행하지 않습니다.
- **호환성:** 확인된 애드온 카탈로그와 로컬 OCI 플랫폼 근거를 대조합니다. 알 수 없는 버전이나 CPU/OS는 호환으로 추정하지 않습니다.
- **네트워크:** Service → Pod → targetPort 경로와 기존 Hubble 기록을 연결합니다. 실제 DNS·NAT·CNI·클라우드 방화벽의 동작 보증은 아닙니다.
- **템플릿:** 입력한 노드풀·canary 요구사항을 반영하고 생성 전에 검사합니다. Terraform 활성화는 false, Helm은 disabled, manifest는 replica 0입니다. 배포는 실행하지 않습니다.

## Primary sources reviewed 2026-11-16

- [Kubernetes credential plugins](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#client-go-credential-plugins)
- [AKS kubelogin](https://learn.microsoft.com/en-us/azure/aks/kubelogin-authentication)
- [AKS upgrade profile API](https://learn.microsoft.com/en-us/rest/api/aks/managed-clusters/get-upgrade-profile?view=rest-aks-2026-01-01)
- [Kubernetes CEL implementation](https://github.com/kubernetes/apiextensions-apiserver/tree/v0.37.2/pkg/apiserver/schema/cel)
- [cert-manager supported releases](https://cert-manager.io/docs/releases/)
- [Cilium Kubernetes compatibility](https://docs.cilium.io/en/stable/network/kubernetes/compatibility/)
- [ingress-nginx retirement and matrix](https://github.com/kubernetes/ingress-nginx)
- [OCI image index](https://github.com/opencontainers/image-spec/blob/main/image-index.md)

Configured node-pool templates currently accept Linux OS SKUs only (AzureLinux, AzureLinux3, Ubuntu, Ubuntu2204, Ubuntu2404). Actual SKU availability still requires provider validation. Configured canary namespaces are carried into both manifest and Helm output.
