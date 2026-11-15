# 고도화 로드맵 / Development roadmap

검토일: 2026-11-15. 1~5 구현 묶음의 첫 버전을 v0.2.0-alpha.1에 반영했습니다. 전체 목표가 모두 검증됐다는 의미는 아니며 현재 경계는 [README](../README.md), [운영 범위](operations.md), [공식 근거](official-sources.md)에 기록합니다.

| 묶음 | 이번 구현 | 남은 확장 |
| --- | --- | --- |
| 1. 스키마·규칙 | 22개 버전 공식 스키마, CRD, 위치·근거, deprecated/removed | CEL, 패치별 변경, admission/semantic 규칙 |
| 2. 스냅샷 | 범위 고정 GET 수집, 한도, 누락·민감값 처리 | 더 넓은 리소스·이전 API 지원, 검토된 인증 방식 |
| 3. Helm·drift | 로컬 렌더, target capabilities, snapshot diff | managedFields 소유권, CRD 리스트 의미, 버전 변환 |
| 4. 업그레이드 | 마이너별 단계, 필수·조건부 조치, kubelet skew | 버전별 기본 동작 변경, addon matrix, 앱·OS 실증 |
| 5. 네트워크·이전 | Hubble 입력, 정책 격리 의도, EKS/AKS/GKE 의존 계획 | 전체 정책 경로 평가, 클라우드별 구체 매핑, Terraform/Helm/manifest 초안 생성 |

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

1. 대표 운영 스냅샷을 비식별화해 검증하고 RBAC/제한 튜닝.
2. 버전별 default/behavior 변경 규칙과 애드온 호환성 데이터 확충.
3. CRD 리스트·필드 소유권을 반영한 drift 정확도 개선.
4. 이전 경로 하나를 선정해 클라우드 매핑과 검토용 템플릿 생성.
5. 실제 Windows 10 x86/x64 장비와 대규모 입력 메모리 검증.

각 단계의 결과를 한국어·영어 릴리스 노트로 제공합니다.
