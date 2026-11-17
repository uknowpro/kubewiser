package cli

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/uknowpro/kubewiser/internal/scan"
)

func referenceHTML(b *strings.Builder, source, lang string) {
	if source == "" {
		return
	}
	u, err := url.Parse(source)
	if err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" {
		fmt.Fprintf(b, `<a class="source-link" rel="noreferrer" href="%s">%s ↗</a>`, htmlCell(source), tr(lang, "Reference", "근거 문서"))
	} else {
		fmt.Fprintf(b, `<span class="source-link">%s</span>`, htmlCell(source))
	}
}
func tableHead(b *strings.Builder, id string, labels []string) {
	fmt.Fprintf(b, `<div class="table-wrap"><table class="compact-table" id="%s"><thead><tr>`, id)
	for _, label := range labels {
		fmt.Fprintf(b, `<th scope="col">%s</th>`, htmlCell(label))
	}
	b.WriteString(`</tr></thead><tbody>`)
}
func compactFindingsHTML(b *strings.Builder, r review, lang string) {
	fmt.Fprintf(b, `<section><h2>%s (%d)</h2>`, tr(lang, "Inspection results", "검사 결과 요약"), len(r.Items))
	tableHead(b, "findings-table", []string{"#", tr(lang, "Priority", "구분"), tr(lang, "Finding / resource", "검토 항목 / 리소스"), tr(lang, "Current state", "현재 상태"), tr(lang, "Recommended state", "권장 상태"), tr(lang, "Action / evidence", "조치 / 상세 근거")})
	for n, i := range r.Items {
		kind := i.Requirement
		if kind != "required" && kind != "optional" {
			kind = "conditional"
		}
		fmt.Fprintf(b, `<tr class="%s"><td>%d</td><td><span class="badge">%s</span></td><td><strong>%s</strong><div class="resource">%s</div><div class="muted">%s</div></td><td>%s</td><td>%s</td><td>%s`, kind, n+1, requirement(lang, i.Requirement), htmlCell(i.Title), htmlCell(i.Resource), htmlCell(i.Rule), htmlCell(i.Current), htmlCell(i.Recommended), htmlCell(i.Action))
		fmt.Fprintf(b, `<details><summary>%s</summary><dl>`, tr(lang, "Details / verification", "상세 / 검증 방법"))
		for _, row := range [][2]string{{tr(lang, "Why", "이유"), i.Why}, {tr(lang, "Verify", "검증 방법"), i.Verify}, {tr(lang, "Location / deadline", "위치 / 조치 시점"), i.Location}} {
			if row[1] != "" {
				fmt.Fprintf(b, `<dt>%s</dt><dd>%s</dd>`, row[0], htmlCell(row[1]))
			}
		}
		b.WriteString(`</dl>`)
		referenceHTML(b, i.Source, lang)
		b.WriteString(`</details></td></tr>`)
	}
	if len(r.Items) == 0 {
		fmt.Fprintf(b, `<tr><td colspan="6">%s</td></tr>`, htmlCell(r.Summary))
	}
	b.WriteString(`</tbody></table></div></section>`)
}

type versionRow struct {
	at                                               int
	topic, current, target, action, evidence, source string
}

// This is a bounded catalog comparison, not an exhaustive release-note diff or
// an assertion that a catalog entry is present in the supplied cluster.
func versionComparisonHTML(b *strings.Builder, root map[string]any, lang string) {
	from, to := str(root, "from"), str(root, "to")
	lo, e1 := scan.Target(from)
	hi, e2 := scan.Target(to)
	if root["actions"] == nil || e1 != nil || e2 != nil || hi <= lo {
		return
	}
	rows := []versionRow{}
	for _, rule := range scan.Rules() {
		at := rule.Removed
		if at <= lo || at > hi {
			continue
		}
		resources := map[string]bool{}
		for _, a := range list(root["actions"]) {
			m := obj(a)
			if (str(m, "id") == "KW-API-REMOVED" || str(m, "id") == "KW-API-DEPRECATED") && strings.HasPrefix(str(m, "resource"), rule.API+"/"+rule.Kind+"/") {
				resources[str(m, "resource")] = true
			}
		}
		evidence := tr(lang, "Not detected in supplied inputs; usage remains unverified", "입력에서 미발견 · 실제 사용 여부 미확인")
		if len(resources) > 0 {
			evidence = fmt.Sprintf(tr(lang, "Detected: %d resource(s)", "입력에서 발견: %d개 리소스"), len(resources))
		}
		rows = append(rows, versionRow{at, rule.Kind + " · " + rule.API, tr(lang, "Served API", "제공되는 API") + ": " + rule.API, tr(lang, "Removed; replacement: ", "제공 중단 · 대체: ") + rule.Replacement, tr(lang, "Required if used: migrate fields and API clients before removal", "사용 시 필수: 제거 전에 필드·API 클라이언트 이전"), evidence, rule.Source})
	}
	for _, raw := range list(root["actions"]) {
		m := obj(raw)
		id := str(m, "id")
		if !strings.HasPrefix(id, "KW-BEHAVIOR-") {
			continue
		}
		at, err := scan.Target(str(m, "before_version"))
		if err != nil || at <= lo || at > hi {
			continue
		}
		current, target := str(m, "current"), str(m, "recommended")
		switch id {
		case "KW-BEHAVIOR-DOCKERSHIM":
			current = tr(lang, "Built-in dockershim available", "내장 dockershim 제공")
			target = tr(lang, "Built-in dockershim removed; verify CRI runtime", "내장 dockershim 제거 · CRI 런타임 확인")
		case "KW-BEHAVIOR-SA-TOKEN":
			current = tr(lang, "Automatic ServiceAccount token Secret generation", "ServiceAccount 토큰 Secret 자동 생성")
			target = tr(lang, "Automatic generation disabled by default; review token consumers", "기본 자동 생성 중단 · 토큰 사용 주체 확인")
		case "KW-BEHAVIOR-PSP":
			current = "PodSecurityPolicy"
			target = tr(lang, "Removed; validate replacement admission controls", "제거됨 · 대체 admission 정책 검증")
		}
		if current == "" {
			current = tr(lang, "Version-dependent behavior", "버전별 동작")
		}
		if target == "" {
			target = str(m, "message")
		}
		rows = append(rows, versionRow{at, id, current, target, str(m, "verification"), tr(lang, "Version-range review; actual impact unverified", "버전 구간 검토 · 실제 영향 미확인"), str(m, "source")})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].at != rows[j].at {
			return rows[i].at < rows[j].at
		}
		return rows[i].topic < rows[j].topic
	})
	fmt.Fprintf(b, `<section><h2>%s · %s → %s</h2><p class="table-note">%s</p>`, tr(lang, "As-Is / To-Be version changes", "As-Is / To-Be 버전 변경 요약"), htmlCell(from), htmlCell(to), tr(lang, "Selected bundled API-removal rules and reported behavior changes, ordered by first affected version. Not all release changes; not a verified AKS upgrade route. Missing findings do not establish absence. Existing issues predating As-Is remain in the findings table.", "내장 API 제거 규칙과 보고된 동작 변경을 최초 영향 버전순으로 표시합니다. 전체 릴리스 변경 목록이나 검증된 AKS 업그레이드 경로는 아닙니다. 미발견은 미사용을 뜻하지 않습니다. As-Is 이전부터 존재한 문제는 아래 검사 결과 표에서 확인하세요."))
	tableHead(b, "version-comparison", []string{tr(lang, "First affected", "최초 영향"), tr(lang, "Change", "변경 항목"), "As-Is " + from, "To-Be " + to, tr(lang, "Action", "검토 / 조치"), tr(lang, "Input evidence", "입력 근거")})
	for _, r := range rows {
		fmt.Fprintf(b, `<tr><td>1.%d</td><td>%s`, r.at, htmlCell(r.topic))
		referenceHTML(b, r.source, lang)
		fmt.Fprintf(b, `</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`, htmlCell(r.current), htmlCell(r.target), htmlCell(r.action), htmlCell(r.evidence))
	}
	if len(rows) == 0 {
		fmt.Fprintf(b, `<tr><td colspan="6">%s</td></tr>`, tr(lang, "No entries in the selected catalog range; this does not establish compatibility.", "해당 구간의 카탈로그 항목이 없습니다. 호환성을 보증하지 않습니다."))
	}
	b.WriteString(`</tbody></table></div></section>`)
}
