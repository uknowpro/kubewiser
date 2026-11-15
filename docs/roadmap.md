# 고도화 로드맵 / Development roadmap

검토일: 2026-11-15. 아래는 구현 계획이며 현재 기능으로 표시하지 않는다. 현재 구현은 오프라인 YAML 검사, 일부 API 제거 규칙, CLI/MCP 및 배포 기반이다.

## 우선순위와 완료 기준

| 순서 | 범위 | 완료 기준 |
| --- | --- | --- |
| P0 | 입력·보고서 정확성 | List 재귀 검사, 파일별 부분 실패, 최대 파일/문서/깊이 제한, 취소 지원, 규칙 ID/심각도/필수 여부/근거/미검사 범위 분리 |
| P1 | 버전별 스키마·규칙 | 알 수 없는 필드·타입·필수값, 로컬 CRD 스키마, deprecated/removed 구분, 목표 버전 지원표, 입력 위치와 근거, 오프라인 회귀 테스트 |
| P2 | 읽기 전용 스냅샷 | 명시한 context/namespace만 수집, GET/LIST 허용 목록, 요청 속도·동시성·페이지·타임아웃 제한, Secret 본문 제외, 누락·RBAC 실패·수집 시간 기록 |
| P3 | Helm·drift | Chart/values/release/namespace/capabilities 고정, 원본과 렌더링 결과 추적, 필드 소유권·기본값·리스트 의미를 고려한 diff |
| P4 | 업그레이드·권고 | 버전 단계별 선행 조치와 영향받는 리소스 연결, 기본값/동작 변경·애드온 호환성, 적용 조건과 검증 방법이 있는 권고 |
| P5 | 네트워크·OS·앱 영향 | 기존 관측 데이터 입력부터 시작, 허용 경로/관측/연결 시험 구분, hostPath/capabilities/sysctl/runtime/이미지 의존 근거, 미확인 사항 명시 |
| P6 | 클라우드 이전·생성 | 실제 사용하는 이전 경로 한 가지부터 지원, IAM/LB/스토리지/DNS 매핑, 미해결 변수·데이터 이전 절차, Terraform/Helm/manifest 초안과 검증 |

다음 구현 묶음은 P0+P1을 추천한다. 배포 환경 확대보다 판정의 정확도와 범위를 먼저 확립한다. 기존 1.16~1.32 제한을 숫자만 늘려 확장하지 않고, 지원표·검증된 스키마·규칙 데이터와 함께 확장한다. 일정은 대표 입력·목표 버전·실제 클러스터 규모가 정해진 뒤 산정한다.

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

오픈소스는 설계와 테스트의 참고 자료이며 새 런타임 의존성으로 추가한 것이 아니다. 코드를 재사용할 때는 해당 버전의 라이선스·NOTICE와 의존성을 먼저 확인한다. 공식 문서도 바뀌므로 규칙에 문서 URL뿐 아니라 확인일·적용 범위·규칙 버전·회귀 사례를 기록한다.

## 외부 전송 없는 기본 동작

- scan/explain의 기본 경로에서 스키마·문서·업데이트 자동 다운로드를 금지한다. 이번 참고 자료 조사는 개발 시 조사이며 실행 파일이 이 URL에 접속한다는 뜻이 아니다.
- 스키마와 규칙은 버전·해시가 고정된 로컬 번들로 읽는다. 별도의 명시적 업데이트 기능은 추후 설계하며 검사 실행과 분리한다.
- 스키마의 원격 $ref도 기본 차단한다. 캐시가 없거나 스키마가 없으면 미검사로 표시한다.
- 클러스터 모드는 사용자가 선택한 API 서버만 접근한다. kubeconfig exec 인증 플러그인·클라우드 인증·프록시·리다이렉트가 만드는 추가 통신도 문서화한다.
- 클라우드 API/관측 데이터 연동은 각 대상과 권한을 명시한다. 텔레메트리·자동 오류 리포팅은 추가하지 않는다.
- MCP 응답에는 경로·해시도 포함되므로 원본뿐 아니라 메타데이터 노출도 관리한다. 모델·클라이언트가 외부로 보내는 것은 서버의 로컬 통신과 별도다.

## 판정 원칙

스키마 통과는 실제 배포 성공과 같지 않다. CRD의 preserve-unknown-fields, CEL, admission, webhook, 컨트롤러 동작은 각각 검증 범위를 표시한다. API 서버가 반환한 apiVersion만으로 실제 구형 API 호출을 확정하지 않는다. Helm lookup과 서버 dry-run은 읽기 전용·오프라인 검증과 별도 모드다. 네트워크에서 관측되지 않은 연결을 미사용으로 확정하지 않는다.

## 다음 작업 백로그

1. Report 스키마 v2 및 coverage 결과 정의; severity/action/evidence를 분리.
2. YAML 위치를 유지한 리소스 모델, List 재귀 처리, 입력 제한·취소·부분 실패.
3. 로컬 스키마 번들 인터페이스와 네트워크 차단 테스트.
4. Deployment/Service/Ingress 등 대표 리소스에 대한 유효·무효·미검사 fixture와 버전 매트릭스.
5. deprecated/removed/default-change 규칙 외부 데이터화 및 근거 ID.
6. CLI/MCP 결과 일치, Windows x86 메모리 한계, 민감 데이터 출력 검증.

각 단계의 결과를 다음 릴리스 노트에 한국어·영어로 기록한다.
