# Inspection review

Start with required actions, then check whether conditional recommendations apply to your workloads.

Target Kubernetes 1.37 · Resources 1 · Schema checked 0

| Required | Conditional | Optional |
| --- | --- | --- |
| 1 | 0 | 0 |

> Partial coverage. No automatic changes are applied. Recommendations must be validated for your workload.

## 1. Required · Migrate the Kubernetes API

batch/v1beta1/CronJob//example-legacy-job

| Current state | Recommended state |
| --- | --- |
| apiVersion: batch/v1beta1 | batch/v1 |

- **Why:** Deprecated in 1.21; no longer served from 1.25.
- **Next action:** Migrate the resource using the official migration guide, including field and behavior changes. An apiVersion edit alone may be insufficient.
- **Verify:** Re-run this inspection and test the workload in a staging environment.
- **Location / rule:** docs/examples/manifest.yaml:1 /apiVersion · KW-API-REMOVED
- **Source:** https://kubernetes.io/docs/reference/using-api/deprecation-guide/

## Not verified

- Admission, CEL, controller behavior, feature-gate availability and application runtime compatibility are not evaluated
- OpenAPI validation does not replace Kubernetes admission or server semantic validation
- Schemas are pinned to minor .0 releases; patch-specific changes are not inferred

<details>
<summary>Original evidence and metadata</summary>

```json
{
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
    "bundle_sha256": "4f565e7c9fb10bb8d9a0cb58ca0d867b0e73c8eb259102f244ed2049d92cf1c0",
    "reviewed": "2026-11-15",
    "source": "https://raw.githubusercontent.com/kubernetes/kubernetes/v1.37.0/api/openapi-spec/swagger.json",
    "source_sha256": "465276aedf437726de5a6ed23a41b86d78bb0d347edf251a755ae26a93cdd8a6",
    "version": "v1.37.0"
  },
  "schema_checked": 0,
  "schema_version": "2",
  "status": "partial",
  "target_version": "1.37",
  "unchecked": [
    "Admission, CEL, controller behavior, feature-gate availability and application runtime compatibility are not evaluated",
    "OpenAPI validation does not replace Kubernetes admission or server semantic validation",
    "Schemas are pinned to minor .0 releases; patch-specific changes are not inferred"
  ]
}
```

</details>
