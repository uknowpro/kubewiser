# Linux validation / Linux 검증

## Reproduce

```sh
go test -race ./...
go vet ./...
CGO_ENABLED=0 go build -o /tmp/kubewiser ./cmd/kubewiser
bash scripts/verify-linux.sh /tmp/kubewiser
go test ./internal/analyze -run '^$' -bench BenchmarkDriftTenThousand -benchtime=1x -benchmem
```

The integration script needs Bash and Python 3 for verification only; the distributed binary needs neither Go nor Python. When `strace` is present, it checks network system calls during offline AKS planning and CEL inspection. Without strace it reports that check as skipped. It never calls a real cluster or cloud provider. Release CI runs this script on Linux amd64 and arm64.

## Local evidence, 2026-11-16

Ubuntu 26.04.1 LTS, linux/amd64, Intel i9-12900K:

- Unit/integration tests cover CRD map/set ordering, hashed keys, field managers, strict profiles, API deadlines, add-on ranges, CLI JSON/text/Markdown/HTML, HTML escaping, MCP tool scope, output collisions and local Helm rendering.
- Alpha.5 adds tests for bounded explicit exec authentication, local AKS route evidence, CEL create/update rules, CRD defaults/None conversion, Service backend/flow matching, OCI platform evidence, configured templates and generated HCL syntax.
- Targeted policy tests cover ingress/egress intersection, additive allows, named/numeric/range ports, IPBlock exceptions, missing namespace evidence and host networking.
- 10,000 unchanged Service resources in the drift benchmark: approximately **28 ms**, **8.12 MB allocated per operation**, 160,163 allocations. One local benchmark iteration, not a production SLA or peak-memory measurement.
- The standalone integration script validates **10,000 Services** against the 1.35 schema and exercises AKS planning, HTML output, draft generation, configured templates, CRD CEL inspection and the disabled Helm chart.

## Boundaries

Synthetic inputs and loopback mock APIs do not establish live AKS readiness. No real upgrade, backup restore, Azure provisioning, Terraform provider execution, CNI dataplane enforcement or production workload test was performed. The generated Terraform scaffold follows the linked provider resource documentation; validate it with a deliberately pinned provider and your actual platform inputs before use.

한국어: Ubuntu/Linux의 단위·통합 테스트와 실제 실행 파일로 검증합니다. 1만 리소스 결과는 합성 입력에 대한 로컬 측정치이며 운영 성능 보증이 아닙니다. 실제 AKS 변경·복구·앱 실행 검증은 프로젝트 환경에서 수행해야 합니다. Windows 10 실장비 검증은 이번 필수 완료 조건에서 제외했습니다.
