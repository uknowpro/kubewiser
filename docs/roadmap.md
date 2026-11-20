# 고도화 현황 / Development roadmap

검토일: 2026-11-20. 현재 구현 기준: **v0.2.0-alpha.11**. 과거 백로그의 완료 항목을 중복해서 미완료로 표시하지 않도록 정리했습니다.

## 구현된 기능과 검증 경계

| 영역 | 구현 | 남는 검증 경계 |
| --- | --- | --- |
| 전후 매트릭스 | manifest 필드 차이·목표 재검증·cgroup/커널/런타임/드라이버 전체 검토표 | 제안 상태 비교이며 admission·실행 호환 보증 아님 |
| 유지보수·cgroup | 선택된 지원 종료 카탈로그, 제공 근거 기반 현재·목표 모드와 최소 조건 | 모든 제품·실제 노드 상태·앱 실행 호환 보증 아님 |
| 인증·수집 | 고정 범위 GET 수집, 제한/민감값 처리, 구버전 API, 명시적 실행 파일 exec 인증 | 인증 도구 자체의 외부 통신·자식 프로세스, 실제 AKS RBAC/인증은 환경에서 확인 |
| AKS 경로 | 로컬 조회 결과의 클러스터·버전·시각·다음 후보 대조 | 로컬 근거의 진위, 이후 모든 단계의 실제 제공 여부는 새 근거 필요 |
| 스키마·정책 | 22개 마이너 `.0`, 3개 정확한 패치, CRD, 별도 공식 라이브러리 CEL 검사 | 미번들 패치, 목표 기능 게이트, admission webhook/ValidatingAdmissionPolicy는 보증하지 않음 |
| 차이 분석 | CRD map/set, 관리자 근거, namespace 정규화, 선택적 로컬 기본값/None 변환 | Webhook 변환·pruning·실제 서버 기본값은 호출/재현하지 않음 |
| 업그레이드·애드온 | 첫 영향 버전 기한, 동작 변경 규칙, 내장된 공식 근거 카탈로그와 사용자 행렬 | 모든 공급자/버전의 무제한 행렬이 아님; 미등록 버전은 미확인 |
| 앱·OS | 노드 런타임 현황, 선언 의존성, 로컬 OCI 플랫폼·digest 대조 | 앱 실행·네이티브 라이브러리·커널 모듈·cgroup 실제 동작은 canary 증거 필요 |
| 네트워크 | Pod 및 Service→Pod/targetPort 정책 의도, 포트별 Hubble 근거 | DNS·NAT·CNI·클라우드 방화벽·라우팅 실동작은 능동 탐침 없이 확정 불가 |
| 템플릿 | 입력된 노드풀·canary 값, provider 버전 고정, Terraform/Helm/manifest와 검수 문서 | 전체 운영 시스템 자동 변환은 아님; 실제 provider plan·데이터 이전·배포는 실행하지 않음 |
| 검증·배포 | Linux 우선 단위/통합/race/vet, 1만 리소스, 오프라인 시스템 호출 검사, 6플랫폼 CI | 운영 AKS 리허설은 별도; Windows 10 실장비는 사용자 요청에 따라 필수 조건에서 제외 |

[추가 기능 사용법](advanced-inspection.md) · [AKS 계획](project-planning.md) · [Linux 검증](linux-validation.md)

## 실제 프로젝트에 필요한 입력과 실행 증거

다음은 자동 완료 처리할 개발 항목이 아니라 실제 환경에서 제공하거나 실행해야 하는 검증입니다.

1. 사용자가 승인한 인증 방식과 최소 읽기 권한으로 얻은 운영 스냅샷.
2. 클러스터에 대한 최신 AKS 업그레이드 조회 결과와 실제 작업 시점의 지원 상태.
3. 사용하는 이미지·애드온 버전 및 vendor 근거, 필요한 CPU/OS와 리소스 요구량.
4. 중단 예산, RPO/RTO, 백업 복원 리허설과 데이터 무결성 결과.
5. canary 노드/앱에서 측정한 성능·인증·네트워크·스토리지 검증과 담당자 승인.

이 증거 없이 업그레이드 가능, 무중단, 복구 가능, 운영 적용 완료라고 판정하지 않습니다. 운영 요구사항을 모른 채 전체 인프라나 리소스 크기를 자동 설계하지 않습니다.

## 유지보수 및 후속 확장

새 Kubernetes 패치와 애드온 버전의 공식 근거 추가, 더 많은 플랫폼/리소스 매핑, 공급자별 정책 평가기는 계속 확장 가능한 영역입니다. 알려지지 않은 기능을 지원한다고 추정하지 않고 결과에 미검사로 남깁니다. 외부 연동을 늘릴 때는 기본 오프라인 동작과 별도 명시적 옵션 원칙을 유지합니다.

## 참고 자료와 적용 방법

| 자료 | 확인한 기능/주제 | kubewiser에서 참고할 부분 |
| --- | --- | --- |
| [Kubeconform](https://github.com/yannh/kubeconform) | Kubernetes/CRD JSON Schema 검증, strict, schema-location | 검증 사례와 스키마 선택. 기본 다운로드 동작은 그대로 도입하지 않음 |
| [Pluto](https://github.com/FairwindsOps/pluto) | deprecated/removed API, 파일·Helm 릴리스 검사 | 버전 규칙 데이터 구조, 단계 구분, 테스트 사례 |
| [Kubernetes API migration guide](https://kubernetes.io/docs/reference/using-api/deprecation-guide/) | API 제거와 대체 API의 의미 변경 | 규칙의 1차 근거; 단순 apiVersion 치환을 넘는 조치 |
| [Version skew policy](https://kubernetes.io/releases/version-skew-policy/) | 구성요소 간 허용 버전 차이 | 업그레이드 순서·선행 조건 |
| [client-go](https://github.com/kubernetes/client-go) | discovery/dynamic 클라이언트 | 리소스 탐색과 제한된 읽기 수집. 일반 쓰기 메서드를 MCP에 노출하지 않음 |
| [Helm SDK](https://docs.helm.sh/docs/sdk/examples/) | Helm Go SDK 사용 예제 | 외부 helm 실행 의존을 줄이는 렌더링. 기존 SDK 버전 호환성부터 결정 |
| [Server-Side Apply](https://kubernetes.io/docs/reference/using-api/server-side-apply/) | managedFields와 필드 소유권 | 기본값/컨트롤러/사용자 변경을 분류하는 보조 근거 |
| [Polaris](https://github.com/FairwindsOps/polaris) | Kubernetes 모범 설정 검사 | 조건부 권고 규칙과 정상/비정상 fixture 설계 |
| [Hubble](https://docs.cilium.io/en/stable/observability/hubble/index.html) | Cilium 네트워크 관측 | 기존 flow 기록 연동. 에이전트 설치나 능동 스캔을 기본으로 하지 않음 |
| [EKS insights](https://docs.aws.amazon.com/eks/latest/userguide/view-cluster-insights.html) | 클러스터/업그레이드 인사이트 | 플랫폼별 근거를 분리하고 관측 기간을 기록 |
| [AKS component upgrades](https://learn.microsoft.com/en-us/azure/aks/upgrade-cluster-components) | 구성요소별 업데이트 | Kubernetes/노드 이미지/애드온 변경의 분리 |
| [GKE deprecations](https://docs.cloud.google.com/kubernetes-engine/docs/deprecations) | 기능·API 폐기 | 플랫폼 고유 제약 |
| [Terraform import](https://developer.hashicorp.com/terraform/language/import) | 기존 리소스 import와 설정 생성 | 생성된 구성과 상태 연결의 차이. 다른 클라우드로 자동 변환된다고 가정하지 않음 |

Helm SDK와 client-go는 고정된 Go 의존성으로 사용하며 나머지 프로젝트는 설계·테스트의 참고 자료입니다. 코드를 재사용할 때는 해당 버전의 라이선스·NOTICE와 의존성을 먼저 확인한다. 공식 문서도 바뀌므로 규칙에 문서 URL뿐 아니라 확인일·적용 범위·규칙 버전·회귀 사례를 기록한다.

## 외부 통신 원칙

- 검사·계획·템플릿 생성 중 스키마/문서/이미지/업데이트 다운로드, LLM 호출, 텔레메트리와 자동 오류 리포팅을 하지 않습니다.
- 명시적인 collect만 선택한 API 서버에 GET 요청합니다. TLS 검증, 요청/용량/시간 제한, proxy/redirect 차단을 유지합니다.
- `--allow-exec-auth`는 운영자가 지정한 인증 실행 파일을 별도로 허용합니다. 인증 서버 통신·캐시 접근·자식 프로세스는 인증 도구의 동작이며 API origin 제한으로 격리되지 않습니다.
- MCP 도구의 경로·클러스터·인증 실행 파일은 운영자가 고정합니다. 클라이언트/모델의 외부 전송 설정은 별도로 관리해야 합니다.
- 공식 자료 조회는 개발 단계의 명시적 조사입니다. 실행 파일의 자동 접속을 뜻하지 않습니다.

## MCP 검증 보강

11개 도구의 명시적 동작 속성·실제 프로토콜 호출·권한 제한·로컬 TLS 수집 테스트를 추가했습니다. MCP 인증 subprocess는 거부하며 CLI의 명시적 인증 수집은 유지합니다. [검증 근거 및 외부 평가 후속 절차](mcp-trust.md). 외부 Trusted 등급 획득을 의미하지 않습니다.
