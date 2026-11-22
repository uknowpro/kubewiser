# Upgrade readiness / 업그레이드 영향과 해결 추적

Available in v0.2.0-alpha.12. All analysis uses supplied local files or saved snapshots. No live cluster access, eviction, drain, apply or workload execution is performed.

```sh
kubewiser plan upgrade --source docs/examples/readiness/before.yaml \
  --after docs/examples/readiness/after.yaml --from 1.21 --to 1.36 \
  --cgroup-evidence docs/examples/readiness/pools.json \
  --format html --lang ko > readiness.html
```

AKS plans use the same analyses with profile `cgroup_pools`. Source/snapshot and `--namespace` keep their existing meaning. To associate workloads, provide Node objects alongside workload manifests in the source directory, or use a saved snapshot containing Nodes. No Node objects means **input missing**, never inferred placement from OS/Kubernetes version.

## Workload → pool impact

- A Pod's `spec.nodeName` is a supplied binding (observed only when taken from a snapshot), even if that Node is now cordoned. It is not a fresh live observation.
- Templates and unbound Pods produce candidates using nodeSelector, required node affinity (OR terms, AND requirements), node-name fields, cordon state and untolerated NoSchedule/NoExecute taints. Basic Equal/Exists tolerations are supported; other operators are unknown. Preferred affinity is not a hard filter.
- Pool names come from an optional per-pool `node_selector` (a map of node labels), or recognized AKS/EKS/GKE node-pool label keys. Conflicting selectors/labels and duplicate Node identities stay unknown. Explicit selectors are not overridden by a conflicting provider-label inference.
- Matching pools are connected to supplied current/target cgroup evidence. v1 file references in that workload are highlighted for a v2 target. Merely changing the hierarchy is not proof of application failure or success.
- Candidates describe the **current supplied inventory**, not target-node scheduling. Capacity, inter-Pod affinity, volume topology, scheduler profiles, readiness, finite toleration durations and future node labels require separate validation. Node constraints inherited only from a missing controller are not reconstructed.

Example evidence fragment (all fields remain optional except pool name):

```json
{"name":"apppool","node_selector":{"agentpool":"apppool"},"current":{"mode":"v1","evidence":"reviewed current inventory"},"target":{"mode":"v2","evidence":"reviewed target specification"}}
```

## PDB / drain risks

The review reports single desired replicas, unmanaged Pod hints, node-local emptyDir/hostPath data dependencies and absence of an alternative hard-constraint candidate among supplied Nodes. It does not promise full rescheduling capacity or predict drain duration. DaemonSet-specific drain flags, mirror Pods and controllers' recovery behavior are not simulated.

PDB matching is namespace-scoped and follows LabelSelector expressions. Nil selectors select none; empty `policy/v1beta1` selectors select none; empty `policy/v1` selectors select all in the namespace. Exactly one supported minAvailable/maxUnavailable is required. Percentages round up. For one selected controller, a theoretical budget is shown assuming it is the only selected controller and all desired replicas are healthy; it is not live allowance. A zero maxUnavailable or 100% minAvailable flags healthy-Pod eviction risk at desired scale. Generation-matched supplied status can identify no allowance; its freshness remains unverified. Unhealthy-Pod eviction exceptions and rollouts require a separate review.

Each placement/drain pass stops after 100,000 inventory comparisons and reports truncation. Narrow the input scope if that limit is reached; unreviewed resources are not considered safe.

## Finding progress

With `--after`, both inputs are evaluated against the **same target version and rules**. Matched resource identity includes API group, kind, namespace and name. The supported scope includes target schema/API/recommendation findings, static cgroup v1 references/host mounts single-replica/local-volume risks and declared zero/100% PDB budget risks.

| Status | Meaning |
| --- | --- |
| resolved | The previous finding disappeared and the counterpart resource passed the bundled schema check. Only that check is resolved. |
| remaining | The same finding remains. |
| introduced | A finding appeared against a schema-valid current counterpart. |
| cannot_compare | Missing/renamed/group-changed resource, unknown/invalid counterpart schema, ambiguous list position or incomplete snapshot evidence. |

Named container/env/volume indices are normalized to avoid false progress from reordering. Other ambiguous positional lists are not treated as resolved. Removing a resource from the proposal does not mark its findings resolved. API group changes require manual mapping. PDB selector changes remain incomparable; live-status and full scheduler-risk resolution are not tracked automatically. CLI comparisons involving snapshots with redacted fields or collection gaps conservatively mark progress incomparable. Raw field values are omitted.

## 한국어

매트릭스에 **워크로드·노드풀 영향**, **Drain·PDB 위험**, **문제 해결 상태**를 추가합니다. 저장된 실제 배치와 정적 배치 후보를 구분하고, cgroup 전환 근거와 연결합니다. Pod 배치 성공이나 무중단을 보장하지 않습니다.

`--after` 사용 시 같은 목표 버전으로 양쪽을 검사해 해결됨·남아 있음·새로 발생·비교 불가를 표시합니다. 리소스 삭제·이름 변경은 자동 해결로 처리하지 않습니다. 일부 cgroup 정적 의존성과 복제본/로컬 볼륨 위험도 추적하며 PDB 실행 상태·전체 스케줄링 위험 해결은 보증하지 않습니다. 원문 값을 출력하지 않지만 리소스명·노드명 등 메타데이터는 남습니다.

## Official references

- [PDB configuration and rounding](https://kubernetes.io/docs/tasks/run-application/configure-pdb/)
- [Assigning Pods to Nodes](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/)
- [Taints and tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/)
- [cgroup v2](https://kubernetes.io/docs/concepts/architecture/cgroups/)

Rules reviewed 2026-11-22. Unimplemented version/feature-gate behavior remains unknown; these references are not fetched at runtime.
