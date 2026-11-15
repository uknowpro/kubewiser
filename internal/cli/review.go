package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/uknowpro/kubewiser/internal/scan"
)

type reviewItem struct{ Title, Rule, Resource, Location, Requirement, Current, Recommended, Why, Action, Verify, Source string }
type review struct {
	Title, Summary, Scope           string
	Items                           []reviewItem
	Unchecked, Steps                []string
	Required, Conditional, Optional int
}

func tr(lang, en, ko string) string {
	if lang == "ko" {
		return ko
	}
	return en
}
func str(m map[string]any, k string) string { s, _ := m[k].(string); return s }
func obj(v any) map[string]any              { m, _ := v.(map[string]any); return m }
func list(v any) []any                      { s, _ := v.([]any); return s }
func texts(v any) []string {
	var a []string
	for _, x := range list(v) {
		if s, ok := x.(string); ok {
			a = append(a, s)
		}
	}
	return a
}
func requirement(lang, s string) string {
	switch s {
	case "required":
		return tr(lang, "Required", "필수 조치")
	case "conditional":
		return tr(lang, "Conditional", "조건 확인")
	case "optional":
		return tr(lang, "Optional", "선택 권고")
	}
	return tr(lang, "Review", "검토 필요")
}

// Human explanations are derived only from existing findings. No raw values,
// guessed CPU sizing, automatic patches, or extra scan results are introduced.
func buildReview(value any, lang string) (review, bool) {
	root := obj(value)
	if root == nil {
		return review{}, false
	}
	r := review{Title: tr(lang, "Inspection review", "검사 결과와 권장 조치")}
	m := root
	if obj(root["report"]) != nil && root["kind"] == nil {
		m = obj(root["report"])
	}
	var rows []any
	switch {
	case m["findings"] != nil:
		rows = list(m["findings"])
		r.Scope = fmt.Sprintf(tr(lang, "Target Kubernetes %v · Resources %v · Schema checked %v", "목표 Kubernetes %v · 리소스 %v개 · 스키마 검사 %v개"), m["target_version"], m["documents"], m["schema_checked"])
	case root["actions"] != nil:
		rows = list(root["actions"])
		r.Title = tr(lang, "Change plan", "변경 계획과 권장 조치")
		r.Scope = fmt.Sprintf("%v → %v", root["from"], root["to"])
		r.Steps = texts(root["steps"])
	case root["changes"] != nil:
		rows = list(root["changes"])
		r.Title = tr(lang, "Desired and observed state", "선언 상태와 관측 상태 비교")
		r.Scope = tr(lang, "Drift is a difference to review, not an automatic correction order.", "차이가 있다는 뜻이며, 선언 상태로 무조건 되돌리라는 권고는 아닙니다.")
	default:
		return r, false
	}
	r.Unchecked = texts(m["unchecked"])
	if root["actions"] != nil || root["changes"] != nil {
		r.Unchecked = texts(root["unchecked"])
	}
	for _, raw := range rows {
		f := obj(raw)
		i := explain(f, lang)
		r.Items = append(r.Items, i)
		switch i.Requirement {
		case "required":
			r.Required++
		case "optional":
			r.Optional++
		default:
			r.Conditional++
		}
	}
	rank := func(s string) int {
		switch s {
		case "required":
			return 0
		case "conditional":
			return 1
		case "optional":
			return 2
		}
		return 1
	}
	sort.SliceStable(r.Items, func(i, j int) bool { return rank(r.Items[i].Requirement) < rank(r.Items[j].Requirement) })
	if len(r.Items) == 0 {
		r.Summary = tr(lang, "No findings in the checks performed. Review coverage before deciding that a change is safe.", "실시한 검사에서 발견 사항이 없습니다. 변경 안전성을 판단하기 전에 미검사 범위를 확인하세요.")
	} else {
		r.Summary = tr(lang, "Start with required actions, then check whether conditional recommendations apply to your workloads.", "필수 조치부터 해결한 뒤, 조건부 권고가 실제 워크로드에 해당하는지 확인하세요.")
	}
	return r, true
}
func explain(f map[string]any, lang string) reviewItem {
	id := str(f, "rule_id")
	if id == "" {
		id = str(f, "id")
	}
	if id == "" {
		id = str(f, "type")
	}
	i := reviewItem{Rule: id, Resource: str(f, "resource"), Requirement: str(f, "requirement"), Source: str(f, "source")}
	i.Location = str(f, "file")
	if f["line"] != nil {
		i.Location += fmt.Sprint(":", f["line"])
	}
	field := str(f, "field")
	if field == "" {
		field = str(f, "path")
	}
	if field != "" {
		i.Location += " " + field
	}
	i.Title = id
	i.Current = tr(lang, "A rule reported a condition requiring review; see the original evidence below.", "검토가 필요한 조건이 감지됐습니다. 아래 원본 근거를 확인하세요.")
	i.Recommended = tr(lang, "Review the documented requirement for the target environment.", "목표 환경에 적용되는 공식 요구사항을 확인하세요.")
	i.Why = str(f, "message")
	i.Action = tr(lang, "Review the affected resource and documentation before editing the configuration.", "해당 리소스와 근거 문서를 확인한 뒤 설정을 수정하세요.")
	i.Verify = tr(lang, "Re-run this inspection and test the workload in a staging environment.", "수정 후 다시 검사하고 검증 환경에서 워크로드 동작을 확인하세요.")
	if x := str(f, "verification"); x != "" {
		i.Verify = x
	}
	switch id {
	case "KW-AKS-ROUTE":
		i.Title = tr(lang, "Confirm the actual AKS upgrade route", "실제 AKS 업그레이드 경로 확인")
		i.Current = tr(lang, "Requested version range; provider route has not been verified.", "요청한 버전 범위이며 공급자의 실제 경로는 확인되지 않았습니다.")
		i.Recommended = tr(lang, "Only the next upgrade offered for this exact cluster, verified before every step.", "단계마다 해당 클러스터에 실제 제공되는 다음 업그레이드만 확인해 진행합니다.")
		i.Action = i.Verify
	case "KW-AKS-RECOVERY":
		i.Title = tr(lang, "Rehearse recovery before upgrading", "업그레이드 전 복구 리허설")
		i.Current = tr(lang, "Recovery evidence is not established by these inputs.", "현재 입력으로 복구 가능성이 검증되지 않았습니다.")
		i.Recommended = tr(lang, "A tested recovery procedure meeting agreed RPO/RTO.", "합의한 RPO/RTO를 충족하는 검증된 복구 절차.")
		i.Action = i.Verify
	case "KW-AKS-DRAIN":
		i.Title = tr(lang, "Verify disruption and spare capacity", "중단 영향과 여유 용량 검증")
		i.Action = i.Verify
	case "KW-PROJECT-REQUIREMENT":
		i.Title = tr(lang, "Define project acceptance criteria", "프로젝트 완료 기준 확정")
		i.Current = tr(lang, "Missing or unknown project requirement.", "프로젝트 요구사항이 없거나 미정입니다.")
		i.Recommended = tr(lang, "A measurable requirement agreed with the service owner.", "서비스 담당자와 합의한 측정 가능한 기준.")
		i.Action = i.Verify
	case "KW-ADDON-EVIDENCE":
		i.Title = tr(lang, "Review supplied add-on compatibility evidence", "입력한 애드온 호환성 근거 검토")
		i.Action = i.Verify
	case "KW-API-REMOVED", "KW-API-DEPRECATED":
		i.Title = tr(lang, "Migrate the Kubernetes API", "Kubernetes API 이전")
		for _, rule := range scan.Rules() {
			if strings.HasPrefix(i.Resource, rule.API+"/"+rule.Kind+"/") {
				i.Current = "apiVersion: " + rule.API
				i.Recommended = rule.Replacement
				i.Why = fmt.Sprintf(tr(lang, "Deprecated in 1.%d; no longer served from 1.%d.", "1.%d에서 폐기 예정으로 지정됐고, 1.%d부터 제공되지 않습니다."), rule.Deprecated, rule.Removed)
				break
			}
		}
		i.Action = tr(lang, "Migrate the resource using the official migration guide, including field and behavior changes. An apiVersion edit alone may be insufficient.", "공식 이전 가이드에 따라 필드와 동작 변경도 함께 반영하세요. apiVersion만 바꾸는 것으로 충분하지 않을 수 있습니다.")
	case "KW-REQUESTS":
		i.Title = tr(lang, "Set workload resource requests", "워크로드 자원 요청량 설정")
		i.Current = tr(lang, "No resource requests declared for this container.", "이 컨테이너에 자원 요청량이 선언되지 않았습니다.")
		i.Recommended = tr(lang, "CPU and memory requests based on measured usage. Numeric sizing is not available from these inputs.", "실측 사용량에 근거한 CPU·메모리 requests. 현재 입력만으로 적정 수치를 계산할 수 없습니다.")
		i.Why = tr(lang, "Scheduling needs resource requests to reserve suitable capacity.", "스케줄러가 적절한 용량을 확보하려면 자원 요청량이 필요합니다.")
		i.Action = tr(lang, "Measure representative usage, choose requests with workload owners, and validate under load.", "대표 부하에서 사용량을 측정하고 담당자와 요청량을 정한 뒤 부하 테스트로 확인하세요.")
	case "KW-READINESS":
		i.Title = tr(lang, "Check readiness before routing traffic", "트래픽을 받기 전 준비 상태 확인")
		i.Current = tr(lang, "No readinessProbe declared.", "readinessProbe가 선언되지 않았습니다.")
		i.Recommended = tr(lang, "For a service-serving container, an application-specific readiness check. A batch-only job may not need one.", "서비스 트래픽을 받는 컨테이너라면 앱에 맞는 readiness 검사. 배치 전용 작업에는 필요하지 않을 수 있습니다.")
		i.Why = tr(lang, "A running process does not necessarily mean it is ready to serve requests.", "프로세스가 실행 중이어도 요청을 처리할 준비가 된 것은 아닙니다.")
		i.Action = tr(lang, "Confirm this container serves traffic; then choose a real health endpoint or check. No endpoint or port is guessed.", "트래픽 처리 여부부터 확인하고 실제 상태 확인 경로나 검사를 정하세요. 경로나 포트는 임의로 제시하지 않습니다.")
	case "KW-PRIVILEGED":
		i.Title = tr(lang, "Review privileged access", "특권 컨테이너 권한 검토")
		i.Current = "privileged: true"
		i.Recommended = tr(lang, "privileged: false where workload requirements permit; explicitly document exceptions.", "업무에 필요한 권한을 검증한 뒤 가능하면 privileged: false. 예외는 명시적으로 기록하세요.")
		i.Why = tr(lang, "Privileged mode exposes broad host capabilities.", "특권 모드는 호스트의 광범위한 권한을 노출합니다.")
		i.Action = tr(lang, "Identify required privileges and test a least-privilege configuration before changing production.", "필요 권한을 식별하고 최소 권한 설정을 검증한 뒤 운영에 반영하세요.")
	case "KW-HOSTPATH", "KW-MIGRATE-HOSTPATH":
		i.Title = tr(lang, "Review node-local storage dependency", "노드 로컬 저장소 의존성 검토")
		i.Current = tr(lang, "A hostPath volume is configured; its path is not reproduced here.", "hostPath 볼륨이 설정돼 있습니다. 실제 경로는 보고서에 표시하지 않습니다.")
		i.Recommended = tr(lang, "Portable storage when appropriate, or an explicitly validated host dependency.", "적합한 경우 이식 가능한 저장소 사용, 또는 명시적으로 검증한 호스트 의존성.")
		i.Action = tr(lang, "Check persistence, permissions and target OS paths; plan data movement before replacing storage.", "영속성·권한·대상 OS 경로를 확인하고 저장소 교체 전 데이터 이전을 계획하세요.")
	case "KW-HOST-NAMESPACE", "KW-NODE-DEPENDENCY":
		i.Title = tr(lang, "Validate node and runtime assumptions", "노드·런타임 의존 조건 검증")
		i.Current = tr(lang, "A host namespace, runtimeClass or sysctl dependency was detected.", "호스트 네임스페이스, runtimeClass 또는 sysctl 의존성이 감지됐습니다.")
		i.Recommended = tr(lang, "Use only necessary host features and verify them on target nodes.", "필요한 호스트 기능만 사용하고 대상 노드에서 지원 여부를 검증하세요.")
	case "KW-SCHEMA", "KW-REQUIRED", "KW-INPUT", "KW-API-UNAVAILABLE":
		i.Title = tr(lang, "Correct the input for the target API", "목표 API에 맞게 입력 수정")
		i.Current = tr(lang, "The input fails a syntax, required-field, API availability or schema check. Raw values are omitted.", "입력이 문법·필수 필드·API 제공 여부·스키마 검사 중 하나를 통과하지 못했습니다. 실제 값은 생략합니다.")
		i.Recommended = tr(lang, "A valid document using fields and types supported by the target version.", "목표 버전에서 지원하는 필드와 타입을 사용한 유효한 문서.")
		i.Action = tr(lang, "Use the file/field location and original constraint below to correct the input; unknown fields are not automatically deleted.", "파일·필드 위치와 아래 원본 제약 조건을 확인해 수정하세요. 알 수 없는 필드를 자동 삭제하지 않습니다.")
	case "KW-KUBELET-SKEW", "KW-ADDON-MATRIX":
		i.Title = tr(lang, "Verify component compatibility", "구성요소 호환성 확인")
		i.Current = tr(lang, "Kubelet skew or add-on compatibility needs review; see the original finding.", "kubelet 버전 차이 또는 애드온 호환성 검토가 필요합니다. 원본 판정을 확인하세요.")
		i.Recommended = tr(lang, "Compatible component versions at every minor upgrade step.", "각 마이너 업그레이드 단계에서 호환되는 구성요소 버전.")
		i.Action = tr(lang, "Check Kubernetes skew policy and CNI/CSI/ingress/webhook vendor matrices for each step.", "각 단계에서 Kubernetes 버전 차이 정책과 CNI·CSI·Ingress·Webhook 공급자 지원표를 확인하세요.")
	case "changed", "desired_only_field", "observed_only_field", "not_observed", "observed_only_resource", "conversion_unverified":
		i.Requirement = "conditional"
		i.Title = tr(lang, "Review a declared/observed difference", "선언·관측 상태 차이 검토")
		i.Current = tr(lang, "The snapshot and desired input differ at this resource/field. Values are intentionally omitted.", "스냅샷과 선언 입력이 이 리소스·필드에서 다릅니다. 실제 값은 표시하지 않습니다.")
		i.Recommended = tr(lang, "An agreed desired state after checking defaults, ownership, conversion and collection gaps.", "기본값·필드 소유권·버전 변환·수집 누락을 확인한 뒤 합의한 목표 상태.")
		i.Action = tr(lang, "Confirm which state is intended before editing. Missing observations do not prove deletion; extra resources are not deletion recommendations.", "어느 상태가 의도한 것인지 먼저 확인하세요. 관측 누락은 삭제의 증거가 아니며, 추가 리소스도 삭제 권고가 아닙니다.")
	default:
		if strings.HasPrefix(id, "KW-ARCH-") {
			i.Title = tr(lang, "Review architecture change", "아키텍처 변경 검토")
			i.Action = i.Verify
		}
		if strings.HasPrefix(id, "KW-BEHAVIOR-") {
			i.Title = tr(lang, "Review version behavior change", "버전별 동작 변경 검토")
			i.Action = i.Verify
		}
		if strings.HasPrefix(id, "KW-MIGRATE-") {
			i.Title = tr(lang, "Review cloud migration dependency", "클라우드 이전 의존성 검토")
			i.Current = tr(lang, "A resource or setting matched a platform dependency check.", "리소스 또는 설정이 플랫폼 의존성 검사에 해당합니다.")
			i.Recommended = str(f, "message")
			i.Action = tr(lang, "Map the source dependency to the target provider, validate in staging, then plan cutover. No infrastructure is created.", "출발 환경 의존성을 대상 플랫폼에 매핑하고 검증 환경에서 확인한 뒤 전환을 계획하세요. 인프라는 생성하지 않습니다.")
		}
	}
	if x := str(f, "current"); x != "" {
		i.Current = x
	}
	if x := str(f, "recommended"); x != "" {
		i.Recommended = x
	}
	if x := str(f, "before_version"); x != "" {
		i.Location += tr(lang, " · Deadline: ", " · 조치 시점: ") + x
	}
	if managers := list(f["field_managers"]); len(managers) > 0 {
		names := []string{}
		for _, m := range managers {
			names = append(names, str(obj(m), "manager"))
		}
		i.Why += tr(lang, " Recorded managers (hints only): ", " 기록된 필드 관리자(보조 근거): ") + strings.Join(names, ", ")
	}
	return i
}
