# MCP verification evidence / MCP 검증 근거

Reviewed: 2026-11-20. Implementation: v0.2.0-alpha.11.

Kubewiser does not claim a Trusted grade or third-party certification. The public [M8ven listing](https://m8ven.ai/mcp/uknowpro/kubewiser) inspected on this date evaluated commit `35eb313` (2026-11-15), reported partial Go analysis coverage, missing tool annotations and limited named MCP-call test coverage. This release addresses the code-level findings; the external grade must be reassessed by M8ven.

## Tool behavior

Every registered tool advertises all four boolean [MCP tool annotations](https://modelcontextprotocol.io/specification/2025-06-18/schema#toolannotations). These are descriptive hints, not enforcement or a security certificate.

| Tools | readOnlyHint | destructiveHint | idempotentHint | openWorldHint |
| --- | --- | --- | --- | --- |
| `scan_files`, `scan_cgroup`, `scan_lifecycle`, `plan_upgrade`, `plan_migrate`, `scan_network`, `scan_cel`, `plan_aks`, `compare_drift`, `scan_helm` | true | false | true | false |
| `collect_cluster` (explicitly enabled only) | true | false | true | true |

Idempotence describes effects on the environment, not identical results: files, time-sensitive lifecycle results and cluster state can change between calls. Collection can produce remote access logs and consumes API requests. It returns a snapshot to the MCP client; it does not save or apply resources. Offline tools read operator-fixed local inputs. Chart rendering uses the existing restricted local renderer, not a sandbox for hostile charts.

**Authentication boundary:** MCP collection rejects `--allow-exec-auth` and never executes an authentication plugin. If interactive/external authentication is necessary, explicitly run CLI `collect --allow-exec-auth ...` first, then supply the saved snapshot to MCP. That CLI option retains its documented identity-provider, cache and subprocess effects. Static-credential MCP collection still requires explicit kubeconfig, context, namespace and `--allow-cluster`.

## Reproduce

```sh
go test -race ./internal/mcpserver
go test -race ./...
go vet ./...
```

`internal/mcpserver/trust_test.go` verifies:

- Named MCP calls for all ten offline tools, structured successful results and invalid local inputs.
- Exact tool inventory so adding a tool without a corresponding call case fails.
- All four boolean hints in `tools/list`, including explicit false values.
- Unconfigured tools cannot be called; model-supplied file, kubeconfig, cluster and namespace overrides are rejected.
- `collect_cluster` calls only a local TLS test API, uses GET within the fixed namespace, excludes Secrets and strips the tested sensitive fields.
- Missing collection configuration and authentication subprocess requests fail closed.

These tests do not contact AKS, Azure, an external cluster or M8ven. TLS test traffic is loopback only. This is not an exhaustive vulnerability assessment; privacy redaction is field-specific and remaining metadata may be sensitive.

## Publisher follow-up

1. Claim the M8ven listing using the repository owner's account and inspect the detailed findings.
2. Ask for reassessment of this release/commit; provide the tests and CI results.
3. If a code-reading limitation remains, request clarification of Go verification coverage.
4. Consider continuous verification only after reviewing the requested GitHub permissions. No account linking or verification submission is performed by kubewiser.

Publisher verification, independent code/runtime verification and the Trusted grade are separate. Adoption/reputation and M8ven's analysis coverage are outside this repository's control. No telemetry, reporting client or remote trust badge fetch is added to the binary.

## 한국어

모든 도구의 네 가지 동작 속성을 명시하고 실제 MCP 호출 테스트를 추가했습니다. `collect_cluster`는 외부 API 접근 여부를 true로 표시하며 기본 등록하지 않습니다. 모델이 경로·namespace·인증 설정을 변경할 수 없고, 실제 클러스터 대신 로컬 TLS 모의 서버로 수집을 검증합니다.

MCP에서는 인증 실행 파일 옵션을 거부합니다. 필요한 경우 사용자가 CLI 수집을 명시적으로 수행한 후 저장된 스냅샷을 MCP에 제공하세요. 기존 CLI 인증 기능은 유지합니다. 이 제한은 외부 인증 프로그램의 임의 부작용을 읽기 전용 MCP 도구의 동작으로 숨기지 않기 위한 것입니다.

M8ven의 Trusted 등급을 보장하거나 자체 부여하지 않습니다. 게시자 인증·최신 커밋 재평가·Go 분석 범위 확인은 서비스 측 후속 절차입니다. 이번 보강으로 외부 리포팅이나 자동 계정 연결을 추가하지 않았습니다.
