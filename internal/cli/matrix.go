package cli

import (
	"fmt"
	"sort"
	"strings"
)

func matrixLabel(key, lang string) string {
	labels := map[string][2]string{
		"dockerfile": {"Dockerfile / application runtime", "Dockerfile · 앱 런타임"},
		"manifest":   {"Manifest comparison", "Manifest 전후 비교"}, "api": {"API versions", "API 버전"}, "schema": {"Target schema", "목표 스키마"}, "defaults": {"Defaults / behavior", "기본값 / 동작 변경"}, "lifecycle": {"Support lifecycle", "유지보수 종료"}, "ingress": {"Ingress", "Ingress"}, "storage": {"Storage / CSI", "스토리지 / CSI"}, "cpu": {"CPU / image platform", "CPU / 이미지 플랫폼"}, "os": {"OS", "OS"}, "application": {"Application behavior", "애플리케이션 동작"}, "cgroup": {"Cgroup mode", "cgroup 모드"}, "kernel": {"Kernel", "커널"}, "runtime": {"Container runtime", "컨테이너 런타임"}, "drivers": {"Kubelet / runtime drivers", "kubelet / 런타임 드라이버"}, "manifest-field": {"Manifest field change", "Manifest 필드 변경"}, "target-validation": {"Proposed manifest validation", "변경 후 manifest 검증"},
		"workload-pool": {"Workload / pool impact", "워크로드 · 노드풀 영향"}, "drain": {"Drain / PDB risk", "Drain · PDB 위험"}, "finding-transition": {"Finding progress", "문제 해결 상태"}, "resolved": {"Resolved in supported check", "해결됨 (검사 범위 내)"}, "remaining": {"Remaining", "남아 있음"}, "introduced": {"New finding", "새로 발생"}, "cannot_compare": {"Cannot compare", "비교 불가"},
		"input_missing": {"Input missing", "입력 부족"}, "analyzer_unsupported": {"Analyzer not implemented", "판정기 미지원"}, "execution_required": {"Execution validation required", "실행 검증 필요"}, "action_required": {"Action required", "조치 필요"}, "review_required": {"Review required", "검토 필요"}, "not_detected": {"No finding in supported checks", "지원 범위 내 미발견"}, "baseline_met": {"Target baseline met (supplied evidence)", "목표 최소조건 충족 (제공 근거)"},
	}
	if pair, ok := labels[key]; ok {
		return tr(lang, pair[0], pair[1])
	}
	return key
}

type matrixDisplayRow struct {
	value           map[string]any
	group           int
	resource, field string
}

var matrixGroups = [][2]string{{"Manifest / API", "Manifest · API"}, {"Platform / nodes", "플랫폼 · 노드"}, {"Components / operations", "구성요소 · 운영"}, {"Proposed changes", "변경 후 검증"}}

func groupedMatrix(root map[string]any) []matrixDisplayRow {
	rows := []matrixDisplayRow{}
	for _, raw := range list(root["matrix"]) {
		r := obj(raw)
		group := 2
		switch str(r, "topic") {
		case "manifest", "api", "schema", "defaults":
			group = 0
		case "cpu", "os", "cgroup", "kernel", "runtime", "drivers", "workload-pool":
			group = 1
		case "manifest-field", "target-validation", "finding-transition":
			group = 3
		}
		resource, field, _ := strings.Cut(str(r, "resource"), "#")
		rows = append(rows, matrixDisplayRow{r, group, resource, field})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].group != rows[j].group {
			return rows[i].group < rows[j].group
		}
		return rows[i].resource < rows[j].resource
	})
	return rows
}
func matrixValue(value, lang string) string {
	switch value {
	case "unknown":
		return tr(lang, "Unknown", "미확인")
	case "Not established from supplied inputs":
		return tr(lang, "Not established", "입력 근거 없음")
	case "Target requirements require evidence":
		return tr(lang, "Evidence needed", "목표 근거 필요")
	case "present":
		return tr(lang, "Present", "있음")
	case "changed":
		return tr(lang, "Changed", "변경됨")
	case "Current findings remain in inspection table":
		return tr(lang, "See current findings", "현재 검사 결과 참조")
	}
	return value
}
func matrixHTML(b *strings.Builder, root map[string]any, lang string) {
	rows := groupedMatrix(root)
	if len(rows) == 0 {
		return
	}
	fmt.Fprintf(b, `<section><h2>%s</h2><p class="table-note">%s</p>`, tr(lang, "Manifest and environment review matrix", "Manifest · 환경 전체 검토 매트릭스"), tr(lang, "Grouped coverage review; merged cells share a topic group or resource, never a verdict. Missing evidence stays visible. Proposed manifests are not applied state; raw field values are omitted.", "그룹·리소스만 셀 병합하며 판정은 항목별로 유지합니다. 입력 부족도 표시합니다. 변경 후 manifest는 제안 상태이며 필드 원문 값은 생략합니다."))
	b.WriteString(`<style>#review-matrix{table-layout:fixed;font-size:12px}#review-matrix th,#review-matrix td{padding:7px 8px;overflow-wrap:anywhere}#review-matrix thead th:nth-child(1){width:9%}#review-matrix thead th:nth-child(2){width:14%}#review-matrix thead th:nth-child(3){width:16%}#review-matrix thead th:nth-child(4){width:14%}#review-matrix thead th:nth-child(5){width:19%}#review-matrix thead th:nth-child(6){width:15%}#review-matrix thead th:nth-child(7){width:13%}#review-matrix .matrix-group{vertical-align:middle;background:#e6edf5;color:#23415c}#review-matrix .matrix-resource{vertical-align:middle;background:#f3f6fa;font-weight:400}#review-matrix .group-start>*{border-top:2px solid #a9b9ca}#review-matrix .resource{font-size:11px}#review-matrix .matrix-topic{font-weight:600}</style>`)
	tableHead(b, "review-matrix", []string{tr(lang, "Group", "그룹"), tr(lang, "Resource / pool", "리소스 / 노드풀"), tr(lang, "Topic / field", "항목 / 필드"), "As-Is", "To-Be", tr(lang, "Verdict", "판정"), tr(lang, "Details", "상세")})
	for i, row := range rows {
		r := row.value
		newGroup := i == 0 || rows[i-1].group != row.group
		if newGroup {
			b.WriteString(`<tr class="group-start">`)
		} else {
			b.WriteString(`<tr>`)
		}
		if newGroup {
			n := 1
			for i+n < len(rows) && rows[i+n].group == row.group {
				n++
			}
			label := matrixGroups[row.group]
			fmt.Fprintf(b, `<td class="matrix-group" rowspan="%d"><strong>%s</strong></td>`, n, htmlCell(tr(lang, label[0], label[1])))
		}
		if newGroup || rows[i-1].resource != row.resource {
			n := 1
			for i+n < len(rows) && rows[i+n].group == row.group && rows[i+n].resource == row.resource {
				n++
			}
			label := row.resource
			if label == "" {
				label = tr(lang, "Overall", "전체")
			}
			fmt.Fprintf(b, `<td class="matrix-resource" rowspan="%d"><span class="resource">%s</span></td>`, n, htmlCell(label))
		}
		fmt.Fprintf(b, `<td><span class="matrix-topic">%s</span>`, htmlCell(matrixLabel(str(r, "topic"), lang)))
		if row.field != "" {
			fmt.Fprintf(b, `<div class="resource">%s</div>`, htmlCell(row.field))
		}
		fmt.Fprintf(b, `</td><td>%s</td><td>%s</td><td>%s</td><td><details><summary>%s</summary><p>%s</p><p>%s</p></details></td></tr>`, htmlCell(matrixValue(str(r, "current"), lang)), htmlCell(matrixValue(str(r, "target"), lang)), htmlCell(matrixLabel(str(r, "status"), lang)), tr(lang, "Evidence", "근거"), htmlCell(r["evidence"]), htmlCell(r["action"]))
	}
	b.WriteString(`</tbody></table></div></section>`)
}
func matrixMarkdown(b *strings.Builder, root map[string]any, lang string) {
	rows := groupedMatrix(root)
	if len(rows) == 0 {
		return
	}
	fmt.Fprintf(b, "## %s\n\n", tr(lang, "Manifest and environment review matrix", "Manifest · 환경 전체 검토 매트릭스"))
	for i, row := range rows {
		r := row.value
		if i == 0 || rows[i-1].group != row.group {
			label := matrixGroups[row.group]
			fmt.Fprintf(b, "\n### %s\n\n| %s | As-Is | To-Be | %s | %s |\n| --- | --- | --- | --- | --- |\n", tr(lang, label[0], label[1]), tr(lang, "Topic / resource / field", "항목 / 리소스 / 필드"), tr(lang, "Verdict", "판정"), tr(lang, "Evidence / action", "근거 / 조치"))
		}
		fmt.Fprintf(b, "| %s | %s | %s | %s | %s |\n", markdownCell(matrixLabel(str(r, "topic"), lang)+" "+str(r, "resource")), markdownCell(matrixValue(str(r, "current"), lang)), markdownCell(matrixValue(str(r, "target"), lang)), markdownCell(matrixLabel(str(r, "status"), lang)), markdownCell(str(r, "evidence")+"; "+str(r, "action")))
	}
	b.WriteString("\n")
}
