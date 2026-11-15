# Kubewiser report

| 항목 / Field | 값 / Value |
| --- | --- |
| documents | 1 |
| rules\_version | 2026-11-15 |
| schema\_checked | 0 |
| schema\_version | 2 |
| status | partial |
| target\_version | 1.37 |

## findings

### 1. KW-API-REMOVED

| 항목 / Field | 값 / Value |
| --- | --- |
| document | 1 |
| evidence | confirmed |
| field | /apiVersion |
| file | docs/examples/manifest.yaml |
| line | 1 |
| message | Not served since 1.25; migrate to batch/v1 |
| requirement | required |
| resource | batch/v1beta1/CronJob//example-legacy-job |
| rule\_id | KW-API-REMOVED |
| severity | error |
| source | https://kubernetes.io/docs/reference/using-api/deprecation-guide/ |

## inputs

| file | sha256 |
| --- | --- |
| docs/examples/manifest.yaml | 0f45796b38e101e3f9f2b89ecf6a971f4ae65dda89f82c14c0181b68d3220af9 |

## schema

| 항목 / Field | 값 / Value |
| --- | --- |
| bundle\_sha256 | 4f565e7c9fb10bb8d9a0cb58ca0d867b0e73c8eb259102f244ed2049d92cf1c0 |
| reviewed | 2026-11-15 |
| source | https://raw.githubusercontent.com/kubernetes/kubernetes/v1.37.0/api/openapi-spec/swagger.json |
| source\_sha256 | 465276aedf437726de5a6ed23a41b86d78bb0d347edf251a755ae26a93cdd8a6 |
| version | v1.37.0 |

## unchecked

- Admission, CEL, controller behavior, feature-gate availability and application runtime compatibility are not evaluated
- OpenAPI validation does not replace Kubernetes admission or server semantic validation
- Schemas are pinned to minor .0 releases; patch-specific changes are not inferred

