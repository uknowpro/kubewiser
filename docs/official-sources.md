# Official Kubernetes sources / 공식 근거

검토일: 2026-11-15. 버전별 공식 Kubernetes 태그의 OpenAPI 정의를 실행 파일에 내장합니다. 실행 중 문서/스키마를 내려받지 않습니다. `kubewiser versions` 및 `internal/schema/bundles/sources.json`에 원본 URL, 원본 SHA-256, 번들 SHA-256, 확인일을 기록했습니다.

각 마이너의 `.0` 스키마를 사용합니다. 예를 들어 1.32.4 요청도 1.32.0 정의로 검사하므로 패치별 변경까지 검증했다는 의미가 아닙니다. 과거 버전 스키마 제공은 해당 버전의 운영 지원/EOL 상태를 보증하지 않습니다.

| Kubernetes | 공식 버전 고정 스키마 |
| --- | --- |
| v1.16.0 | [OpenAPI](https://raw.githubusercontent.com/kubernetes/kubernetes/v1.16.0/api/openapi-spec/swagger.json) |
| v1.17.0 | [OpenAPI](https://raw.githubusercontent.com/kubernetes/kubernetes/v1.17.0/api/openapi-spec/swagger.json) |
| v1.18.0 | [OpenAPI](https://raw.githubusercontent.com/kubernetes/kubernetes/v1.18.0/api/openapi-spec/swagger.json) |
| v1.19.0 | [OpenAPI](https://raw.githubusercontent.com/kubernetes/kubernetes/v1.19.0/api/openapi-spec/swagger.json) |
| v1.20.0 | [OpenAPI](https://raw.githubusercontent.com/kubernetes/kubernetes/v1.20.0/api/openapi-spec/swagger.json) |
| v1.21.0 | [OpenAPI](https://raw.githubusercontent.com/kubernetes/kubernetes/v1.21.0/api/openapi-spec/swagger.json) |
| v1.22.0 | [OpenAPI](https://raw.githubusercontent.com/kubernetes/kubernetes/v1.22.0/api/openapi-spec/swagger.json) |
| v1.23.0 | [OpenAPI](https://raw.githubusercontent.com/kubernetes/kubernetes/v1.23.0/api/openapi-spec/swagger.json) |
| v1.24.0 | [OpenAPI](https://raw.githubusercontent.com/kubernetes/kubernetes/v1.24.0/api/openapi-spec/swagger.json) |
| v1.25.0 | [OpenAPI](https://raw.githubusercontent.com/kubernetes/kubernetes/v1.25.0/api/openapi-spec/swagger.json) |
| v1.26.0 | [OpenAPI](https://raw.githubusercontent.com/kubernetes/kubernetes/v1.26.0/api/openapi-spec/swagger.json) |
| v1.27.0 | [OpenAPI](https://raw.githubusercontent.com/kubernetes/kubernetes/v1.27.0/api/openapi-spec/swagger.json) |
| v1.28.0 | [OpenAPI](https://raw.githubusercontent.com/kubernetes/kubernetes/v1.28.0/api/openapi-spec/swagger.json) |
| v1.29.0 | [OpenAPI](https://raw.githubusercontent.com/kubernetes/kubernetes/v1.29.0/api/openapi-spec/swagger.json) |
| v1.30.0 | [OpenAPI](https://raw.githubusercontent.com/kubernetes/kubernetes/v1.30.0/api/openapi-spec/swagger.json) |
| v1.31.0 | [OpenAPI](https://raw.githubusercontent.com/kubernetes/kubernetes/v1.31.0/api/openapi-spec/swagger.json) |
| v1.32.0 | [OpenAPI](https://raw.githubusercontent.com/kubernetes/kubernetes/v1.32.0/api/openapi-spec/swagger.json) |
| v1.33.0 | [OpenAPI](https://raw.githubusercontent.com/kubernetes/kubernetes/v1.33.0/api/openapi-spec/swagger.json) |
| v1.34.0 | [OpenAPI](https://raw.githubusercontent.com/kubernetes/kubernetes/v1.34.0/api/openapi-spec/swagger.json) |
| v1.35.0 | [OpenAPI](https://raw.githubusercontent.com/kubernetes/kubernetes/v1.35.0/api/openapi-spec/swagger.json) |
| v1.36.0 | [OpenAPI](https://raw.githubusercontent.com/kubernetes/kubernetes/v1.36.0/api/openapi-spec/swagger.json) |
| v1.37.0 | [OpenAPI](https://raw.githubusercontent.com/kubernetes/kubernetes/v1.37.0/api/openapi-spec/swagger.json) |

## Rules and semantics

- [공식 Deprecated API Migration Guide](https://kubernetes.io/docs/reference/using-api/deprecation-guide/): deprecated/removed 버전과 이전 시 주의사항. [1.32 보관 문서](https://v1-32.docs.kubernetes.io/docs/reference/using-api/deprecation-guide/)도 과거 이력 확인에 사용합니다.
- [공식 Version Skew Policy](https://kubernetes.io/releases/version-skew-policy/): 구성요소 버전 차이와 업그레이드 순서. 배포 도구와 클라우드 제약은 추가 검증합니다.
- [공식 CRD validation](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/): 로컬 CRD의 OpenAPI 스키마. CEL, admission/webhook, 변환, pruning 전체 의미는 실행하지 않습니다.
- [공식 NetworkPolicy](https://kubernetes.io/docs/concepts/services-networking/network-policies/): 선택된 Pod의 ingress/egress 격리 의도. 실제 연결 허용을 확정하지 않습니다.

번들은 원본의 definitions를 유지하며 description/externalDocs만 제거하고 gzip으로 압축했습니다. 런타임에서는 strict unknown-field 검사, nullable/int-or-string 변환을 적용합니다. preserve-unknown-fields 하위의 검증은 제한적입니다. 원격/파일 `$ref` 로딩은 차단됩니다. OpenAPI에 표현되지 않은 필수 조건이나 API 서버 admission 검사는 커버하지 않습니다. 문법·스키마 오류와 API 제거를 탐지할 수 있지만 실행 중 사용되지 않는 데드코드 전체를 탐지하는 도구는 아닙니다.

`go test ./internal/schema` verifies all embedded bundle hashes and representative validation cases. `python3 scripts/verify-schema-sources.py` is an explicitly invoked maintainer-only network check of the pinned upstream source hashes; it never runs inside the product or normal inspection. Adding a new version requires a reviewed source, hashes, rules and tests.
