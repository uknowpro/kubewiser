# 검사 결과와 권장 조치

필수 조치부터 해결한 뒤, 조건부 권고가 실제 워크로드에 해당하는지 확인하세요.

목표 Kubernetes 1.37 · 리소스 1개 · 스키마 검사 0개

| 필수 조치 | 조건 확인 | 선택 권고 |
| --- | --- | --- |
| 1 | 0 | 0 |

> 검사 범위는 부분적입니다. 변경은 자동 적용되지 않으며 권장안은 실제 워크로드에서 검증해야 합니다.

## 1. 필수 조치 · Kubernetes API 이전

batch/v1beta1/CronJob//example-legacy-job

| 현재 상태 | 권장 상태 |
| --- | --- |
| apiVersion: batch/v1beta1 | batch/v1 |

- **이유:** 1.21에서 폐기 예정으로 지정됐고, 1.25부터 제공되지 않습니다.
- **다음 조치:** 공식 이전 가이드에 따라 필드와 동작 변경도 함께 반영하세요. apiVersion만 바꾸는 것으로 충분하지 않을 수 있습니다.
- **검증 방법:** 수정 후 다시 검사하고 검증 환경에서 워크로드 동작을 확인하세요.
- **위치 / 규칙:** docs/examples/manifest.yaml:1 /apiVersion · KW-API-REMOVED
- **근거:** https://kubernetes.io/docs/reference/using-api/deprecation-guide/

## 검사하지 못한 항목

- Admission, CEL, controller behavior, feature-gate availability and application runtime compatibility are not evaluated
- OpenAPI validation does not replace Kubernetes admission or server semantic validation
- Schemas are pinned to minor .0 releases; patch-specific changes are not inferred

<details>
<summary>원본 근거와 메타데이터</summary>

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
