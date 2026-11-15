# Change plan

Start with required actions, then check whether conditional recommendations apply to your workloads.

1.21 → 1.35

| Required | Conditional | Optional |
| --- | --- | --- |
| 8 | 5 | 1 |

> Partial coverage. No automatic changes are applied. Recommendations must be validated for your workload.

## 1. Required · Confirm the actual AKS upgrade route



| Current state | Recommended state |
| --- | --- |
| Requested version range; provider route has not been verified. | Only the next upgrade offered for this exact cluster, verified before every step. |

- **Why:** Confirm the next offered upgrade for this exact AKS cluster; do not execute the entire mathematical minor sequence
- **Next action:** Review a fresh az aks get-upgrades result and AKS support guidance before each step. If the required path is unavailable, stop and resolve with the provider or assess a replacement cluster
- **Verify:** Review a fresh az aks get-upgrades result and AKS support guidance before each step. If the required path is unavailable, stop and resolve with the provider or assess a replacement cluster
- **Location / rule:**  · Deadline: 1.21 · KW-AKS-ROUTE
- **Source:** https://learn.microsoft.com/en-us/azure/aks/upgrade-aks-control-plane

## 2. Required · Rehearse recovery before upgrading



| Current state | Recommended state |
| --- | --- |
| Recovery evidence is not established by these inputs. | A tested recovery procedure meeting agreed RPO/RTO. |

- **Why:** Prepare and rehearse recovery before the first control-plane change; do not assume Kubernetes downgrade is a rollback
- **Next action:** Record backup restore evidence, application rollback limits, recovery owner and measured RPO/RTO
- **Verify:** Record backup restore evidence, application rollback limits, recovery owner and measured RPO/RTO
- **Location / rule:**  · Deadline: 1.21 · KW-AKS-RECOVERY
- **Source:** https://learn.microsoft.com/en-us/azure/backup/azure-kubernetes-service-backup-overview

## 3. Required · Verify disruption and spare capacity



| Current state | Recommended state |
| --- | --- |
| A rule reported a condition requiring review; see the original evidence below. | Review the documented requirement for the target environment. |

- **Why:** Validate PDBs, replicas, topology, drain time, surge IP addresses and compute quota before rotating nodes
- **Next action:** Drain a canary node in staging and measure service errors, latency and storage attachment; stop if the agreed budget is exceeded
- **Verify:** Drain a canary node in staging and measure service errors, latency and storage attachment; stop if the agreed budget is exceeded
- **Location / rule:**  · KW-AKS-DRAIN
- **Source:** https://learn.microsoft.com/en-us/azure/aks/upgrade-options

## 4. Required · Migrate the Kubernetes API

batch/v1beta1/CronJob//example-legacy-job

| Current state | Recommended state |
| --- | --- |
| apiVersion: batch/v1beta1 | batch/v1 |

- **Why:** Deprecated in 1.21; no longer served from 1.25.
- **Next action:** Migrate the resource using the official migration guide, including field and behavior changes. An apiVersion edit alone may be insufficient.
- **Verify:** Re-run target validation and test workload behavior in staging
- **Location / rule:**  · Deadline: 1.25 · KW-API-REMOVED
- **Source:** https://kubernetes.io/docs/reference/using-api/deprecation-guide/

## 5. Required · Define project acceptance criteria

maintenance\_window

| Current state | Recommended state |
| --- | --- |
| Missing or unknown project requirement. | A measurable requirement agreed with the service owner. |

- **Why:** Project acceptance requirement is not defined
- **Next action:** Agree and record a measurable value with the service owner before execution
- **Verify:** Agree and record a measurable value with the service owner before execution
- **Location / rule:**  · KW-PROJECT-REQUIREMENT
- **Source:** https://learn.microsoft.com/en-us/azure/aks/upgrade-aks-control-plane

## 6. Required · Define project acceptance criteria

max\_downtime

| Current state | Recommended state |
| --- | --- |
| Missing or unknown project requirement. | A measurable requirement agreed with the service owner. |

- **Why:** Project acceptance requirement is not defined
- **Next action:** Agree and record a measurable value with the service owner before execution
- **Verify:** Agree and record a measurable value with the service owner before execution
- **Location / rule:**  · KW-PROJECT-REQUIREMENT
- **Source:** https://learn.microsoft.com/en-us/azure/aks/upgrade-aks-control-plane

## 7. Required · Define project acceptance criteria

rpo

| Current state | Recommended state |
| --- | --- |
| Missing or unknown project requirement. | A measurable requirement agreed with the service owner. |

- **Why:** Project acceptance requirement is not defined
- **Next action:** Agree and record a measurable value with the service owner before execution
- **Verify:** Agree and record a measurable value with the service owner before execution
- **Location / rule:**  · KW-PROJECT-REQUIREMENT
- **Source:** https://learn.microsoft.com/en-us/azure/aks/upgrade-aks-control-plane

## 8. Required · Define project acceptance criteria

rto

| Current state | Recommended state |
| --- | --- |
| Missing or unknown project requirement. | A measurable requirement agreed with the service owner. |

- **Why:** Project acceptance requirement is not defined
- **Next action:** Agree and record a measurable value with the service owner before execution
- **Verify:** Agree and record a measurable value with the service owner before execution
- **Location / rule:**  · KW-PROJECT-REQUIREMENT
- **Source:** https://learn.microsoft.com/en-us/azure/aks/upgrade-aks-control-plane

## 9. Conditional · Check readiness before routing traffic

batch/v1beta1/CronJob//example-legacy-job

| Current state | Recommended state |
| --- | --- |
| No readinessProbe declared. | For a service-serving container, an application-specific readiness check. A batch-only job may not need one. |

- **Why:** A running process does not necessarily mean it is ready to serve requests.
- **Next action:** Confirm this container serves traffic; then choose a real health endpoint or check. No endpoint or port is guessed.
- **Verify:** Re-run target validation and test workload behavior in staging
- **Location / rule:**  · Deadline: 1.35 · KW-READINESS
- **Source:** https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/

## 10. Conditional · Review version behavior change



| Current state | Recommended state |
| --- | --- |
| A rule reported a condition requiring review; see the original evidence below. | Review the documented requirement for the target environment. |

- **Why:** Dockershim is removed: review Docker socket mounts, runtime monitoring and image build dependencies
- **Next action:** Verify the configured CRI runtime and run runtime-dependent agents on a test node
- **Verify:** Verify the configured CRI runtime and run runtime-dependent agents on a test node
- **Location / rule:**  · Deadline: 1.24 · KW-BEHAVIOR-DOCKERSHIM
- **Source:** https://kubernetes.io/docs/tasks/administer-cluster/migrating-from-dockershim/

## 11. Conditional · Review version behavior change



| Current state | Recommended state |
| --- | --- |
| A rule reported a condition requiring review; see the original evidence below. | Review the documented requirement for the target environment. |

- **Why:** ServiceAccount token Secrets are no longer automatically generated; review clients expecting them
- **Next action:** Test projected token rotation and external clients without copying credentials into reports
- **Verify:** Test projected token rotation and external clients without copying credentials into reports
- **Location / rule:**  · Deadline: 1.24 · KW-BEHAVIOR-SA-TOKEN
- **Source:** https://kubernetes.io/docs/concepts/security/service-accounts/

## 12. Conditional · Review version behavior change



| Current state | Recommended state |
| --- | --- |
| A rule reported a condition requiring review; see the original evidence below. | Review the documented requirement for the target environment. |

- **Why:** PodSecurityPolicy is removed; validate replacement admission controls and namespace policy before upgrade
- **Next action:** Test allowed and rejected workloads in staging; do not assume equivalent policies
- **Verify:** Test allowed and rejected workloads in staging; do not assume equivalent policies
- **Location / rule:**  · Deadline: 1.25 · KW-BEHAVIOR-PSP
- **Source:** https://kubernetes.io/docs/concepts/security/pod-security-policy/

## 13. Conditional · Verify component compatibility



| Current state | Recommended state |
| --- | --- |
| Kubelet skew or add-on compatibility needs review; see the original finding. | Compatible component versions at every minor upgrade step. |

- **Why:** Verify CNI, CSI, ingress, admission webhooks and managed-provider upgrade support for each step
- **Next action:** Check Kubernetes skew policy and CNI/CSI/ingress/webhook vendor matrices for each step.
- **Verify:** Use vendor version matrices and staging tests; these dependencies are not inferred from Kubernetes schemas
- **Location / rule:**  · KW-ADDON-MATRIX
- **Source:** https://kubernetes.io/releases/version-skew-policy/

## 14. Optional · Set workload resource requests

batch/v1beta1/CronJob//example-legacy-job

| Current state | Recommended state |
| --- | --- |
| No resource requests declared for this container. | CPU and memory requests based on measured usage. Numeric sizing is not available from these inputs. |

- **Why:** Scheduling needs resource requests to reserve suitable capacity.
- **Next action:** Measure representative usage, choose requests with workload owners, and validate under load.
- **Verify:** Re-run target validation and test workload behavior in staging
- **Location / rule:**  · Deadline: 1.35 · KW-REQUESTS
- **Source:** https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/

## Upgrade / migration steps

1. Gate 1: inventory and approve requirements, upgrade route, add-on compatibility and recovery evidence
2. Gate 2: rehearse version and architecture changes separately in staging; capture baseline and acceptance metrics
3. 1.22: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads
4. 1.23: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads
5. 1.24: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads
6. 1.25: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads
7. 1.26: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads
8. 1.27: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads
9. 1.28: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads
10. 1.29: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads
11. 1.30: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads
12. 1.31: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads
13. 1.32: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads
14. 1.33: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads
15. 1.34: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads
16. 1.35: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads
17. Gate 3: perform only the next provider-supported control-plane step, then compatible canary nodes; validate before continuing
18. Gate 4: stop on PDB/drain failure, unhealthy workloads, data integrity failure or exceeded error/downtime budget; use the rehearsed recovery runbook
19. Gate 5: complete the observation window and owner sign-off before retiring old pools or backups

## Not verified

- Admission, CEL, controller behavior, feature-gate availability and application runtime compatibility are not evaluated
- OpenAPI validation does not replace Kubernetes admission or server semantic validation
- Schemas are pinned to minor .0 releases; patch-specific changes are not inferred
- Audit logs/API client usage, add-on release matrices, actual node kernel/cgroup and image runtime dependencies are not automatically established
- Profile values are operator declarations, not live observations. A minor-version sequence is NOT a verified AKS upgrade route.
- Provider availability, support policy, quota, regional capacity and architecture transitions require fresh cluster-specific evidence. No Azure API is contacted.
- No local add-on compatibility evidence supplied

<details>
<summary>Original evidence and metadata</summary>

```json
{
  "actions": [
    {
      "before_version": "1.21",
      "id": "KW-AKS-ROUTE",
      "message": "Confirm the next offered upgrade for this exact AKS cluster; do not execute the entire mathematical minor sequence",
      "requirement": "required",
      "resource": "",
      "source": "https://learn.microsoft.com/en-us/azure/aks/upgrade-aks-control-plane",
      "verification": "Review a fresh az aks get-upgrades result and AKS support guidance before each step. If the required path is unavailable, stop and resolve with the provider or assess a replacement cluster"
    },
    {
      "before_version": "1.21",
      "id": "KW-AKS-RECOVERY",
      "message": "Prepare and rehearse recovery before the first control-plane change; do not assume Kubernetes downgrade is a rollback",
      "requirement": "required",
      "resource": "",
      "source": "https://learn.microsoft.com/en-us/azure/backup/azure-kubernetes-service-backup-overview",
      "verification": "Record backup restore evidence, application rollback limits, recovery owner and measured RPO/RTO"
    },
    {
      "id": "KW-AKS-DRAIN",
      "message": "Validate PDBs, replicas, topology, drain time, surge IP addresses and compute quota before rotating nodes",
      "requirement": "required",
      "resource": "",
      "source": "https://learn.microsoft.com/en-us/azure/aks/upgrade-options",
      "verification": "Drain a canary node in staging and measure service errors, latency and storage attachment; stop if the agreed budget is exceeded"
    },
    {
      "before_version": "1.25",
      "id": "KW-API-REMOVED",
      "message": "Not served since 1.25; migrate to batch/v1",
      "requirement": "required",
      "resource": "batch/v1beta1/CronJob//example-legacy-job",
      "source": "https://kubernetes.io/docs/reference/using-api/deprecation-guide/",
      "verification": "Re-run target validation and test workload behavior in staging"
    },
    {
      "before_version": "1.35",
      "id": "KW-REQUESTS",
      "message": "Set workload-appropriate resource requests after measuring usage",
      "requirement": "optional",
      "resource": "batch/v1beta1/CronJob//example-legacy-job",
      "source": "https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/",
      "verification": "Re-run target validation and test workload behavior in staging"
    },
    {
      "before_version": "1.35",
      "id": "KW-READINESS",
      "message": "If serving traffic, define a readiness probe appropriate to the application",
      "requirement": "conditional",
      "resource": "batch/v1beta1/CronJob//example-legacy-job",
      "source": "https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/",
      "verification": "Re-run target validation and test workload behavior in staging"
    },
    {
      "before_version": "1.24",
      "id": "KW-BEHAVIOR-DOCKERSHIM",
      "message": "Dockershim is removed: review Docker socket mounts, runtime monitoring and image build dependencies",
      "requirement": "conditional",
      "resource": "",
      "source": "https://kubernetes.io/docs/tasks/administer-cluster/migrating-from-dockershim/",
      "verification": "Verify the configured CRI runtime and run runtime-dependent agents on a test node"
    },
    {
      "before_version": "1.24",
      "id": "KW-BEHAVIOR-SA-TOKEN",
      "message": "ServiceAccount token Secrets are no longer automatically generated; review clients expecting them",
      "requirement": "conditional",
      "resource": "",
      "source": "https://kubernetes.io/docs/concepts/security/service-accounts/",
      "verification": "Test projected token rotation and external clients without copying credentials into reports"
    },
    {
      "before_version": "1.25",
      "id": "KW-BEHAVIOR-PSP",
      "message": "PodSecurityPolicy is removed; validate replacement admission controls and namespace policy before upgrade",
      "requirement": "conditional",
      "resource": "",
      "source": "https://kubernetes.io/docs/concepts/security/pod-security-policy/",
      "verification": "Test allowed and rejected workloads in staging; do not assume equivalent policies"
    },
    {
      "id": "KW-ADDON-MATRIX",
      "message": "Verify CNI, CSI, ingress, admission webhooks and managed-provider upgrade support for each step",
      "requirement": "conditional",
      "resource": "",
      "source": "https://kubernetes.io/releases/version-skew-policy/",
      "verification": "Use vendor version matrices and staging tests; these dependencies are not inferred from Kubernetes schemas"
    },
    {
      "id": "KW-PROJECT-REQUIREMENT",
      "message": "Project acceptance requirement is not defined",
      "requirement": "required",
      "resource": "maintenance_window",
      "source": "https://learn.microsoft.com/en-us/azure/aks/upgrade-aks-control-plane",
      "verification": "Agree and record a measurable value with the service owner before execution"
    },
    {
      "id": "KW-PROJECT-REQUIREMENT",
      "message": "Project acceptance requirement is not defined",
      "requirement": "required",
      "resource": "max_downtime",
      "source": "https://learn.microsoft.com/en-us/azure/aks/upgrade-aks-control-plane",
      "verification": "Agree and record a measurable value with the service owner before execution"
    },
    {
      "id": "KW-PROJECT-REQUIREMENT",
      "message": "Project acceptance requirement is not defined",
      "requirement": "required",
      "resource": "rpo",
      "source": "https://learn.microsoft.com/en-us/azure/aks/upgrade-aks-control-plane",
      "verification": "Agree and record a measurable value with the service owner before execution"
    },
    {
      "id": "KW-PROJECT-REQUIREMENT",
      "message": "Project acceptance requirement is not defined",
      "requirement": "required",
      "resource": "rto",
      "source": "https://learn.microsoft.com/en-us/azure/aks/upgrade-aks-control-plane",
      "verification": "Agree and record a measurable value with the service owner before execution"
    }
  ],
  "from": "1.21",
  "kind": "aks-project",
  "project": {
    "from_version": "1.21",
    "maintenance_window": "unknown",
    "max_downtime": "unknown",
    "mode": "in-place",
    "provider": "aks",
    "rpo": "unknown",
    "rto": "unknown",
    "schema_version": 1,
    "to_version": "1.35"
  },
  "report": {
    "documents": 1,
    "findings": [
      {
        "document": 1,
        "evidence": "confirmed",
        "field": "/apiVersion",
        "file": "docs/examples/manifest.yaml",
        "line": 1,
        "message": "Not served since 1.25; migrate to batch/v1",
        "requirement": "required",
        "resource": "batch/v1beta1/CronJob//example-legacy-job",
        "rule_id": "KW-API-REMOVED",
        "severity": "error",
        "source": "https://kubernetes.io/docs/reference/using-api/deprecation-guide/"
      },
      {
        "document": 1,
        "evidence": "confirmed",
        "field": "/spec/jobTemplate/spec/template/spec/containers/0/resources/requests",
        "file": "docs/examples/manifest.yaml",
        "line": 1,
        "message": "Set workload-appropriate resource requests after measuring usage",
        "requirement": "optional",
        "resource": "batch/v1beta1/CronJob//example-legacy-job",
        "rule_id": "KW-REQUESTS",
        "severity": "warning",
        "source": "https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/"
      },
      {
        "document": 1,
        "evidence": "confirmed",
        "field": "/spec/jobTemplate/spec/template/spec/containers/0/readinessProbe",
        "file": "docs/examples/manifest.yaml",
        "line": 1,
        "message": "If serving traffic, define a readiness probe appropriate to the application",
        "requirement": "conditional",
        "resource": "batch/v1beta1/CronJob//example-legacy-job",
        "rule_id": "KW-READINESS",
        "severity": "warning",
        "source": "https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/"
      }
    ],
    "inputs": [
      {
        "file": "docs/examples/manifest.yaml",
        "sha256": "0f45796b38e101e3f9f2b89ecf6a971f4ae65dda89f82c14c0181b68d3220af9"
      }
    ],
    "rules_version": "2026-11-15",
    "schema": {
      "bundle_sha256": "6fefde82edecb67157a6848edab0dee2fc89cbd18681835cf4605237d793440c",
      "reviewed": "2026-11-15",
      "source": "https://raw.githubusercontent.com/kubernetes/kubernetes/v1.35.0/api/openapi-spec/swagger.json",
      "source_sha256": "483500149ee52ce5753d75f5639101d985bb4f5e902cc05b1ba7627465d62446",
      "version": "v1.35.0"
    },
    "schema_checked": 0,
    "schema_version": "2",
    "status": "partial",
    "target_version": "1.35",
    "unchecked": [
      "Admission, CEL, controller behavior, feature-gate availability and application runtime compatibility are not evaluated",
      "OpenAPI validation does not replace Kubernetes admission or server semantic validation",
      "Schemas are pinned to minor .0 releases; patch-specific changes are not inferred"
    ]
  },
  "status": "review_required",
  "steps": [
    "Gate 1: inventory and approve requirements, upgrade route, add-on compatibility and recovery evidence",
    "Gate 2: rehearse version and architecture changes separately in staging; capture baseline and acceptance metrics",
    "1.22: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads",
    "1.23: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads",
    "1.24: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads",
    "1.25: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads",
    "1.26: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads",
    "1.27: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads",
    "1.28: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads",
    "1.29: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads",
    "1.30: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads",
    "1.31: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads",
    "1.32: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads",
    "1.33: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads",
    "1.34: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads",
    "1.35: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads",
    "Gate 3: perform only the next provider-supported control-plane step, then compatible canary nodes; validate before continuing",
    "Gate 4: stop on PDB/drain failure, unhealthy workloads, data integrity failure or exceeded error/downtime budget; use the rehearsed recovery runbook",
    "Gate 5: complete the observation window and owner sign-off before retiring old pools or backups"
  ],
  "to": "1.35",
  "unchecked": [
    "Admission, CEL, controller behavior, feature-gate availability and application runtime compatibility are not evaluated",
    "OpenAPI validation does not replace Kubernetes admission or server semantic validation",
    "Schemas are pinned to minor .0 releases; patch-specific changes are not inferred",
    "Audit logs/API client usage, add-on release matrices, actual node kernel/cgroup and image runtime dependencies are not automatically established",
    "Profile values are operator declarations, not live observations. A minor-version sequence is NOT a verified AKS upgrade route.",
    "Provider availability, support policy, quota, regional capacity and architecture transitions require fresh cluster-specific evidence. No Azure API is contacted.",
    "No local add-on compatibility evidence supplied"
  ]
}
```

</details>
