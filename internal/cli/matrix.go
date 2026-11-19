package cli

import (
	"fmt"
	"strings"
)

func matrixLabel(key, lang string) string {
	labels := map[string][2]string{
		"manifest": {"Manifest comparison", "Manifest 전후 비교"}, "api": {"API versions", "API 버전"}, "schema": {"Target schema", "목표 스키마"}, "defaults": {"Defaults / behavior", "기본값 / 동작 변경"}, "lifecycle": {"Support lifecycle", "유지보수 종료"}, "ingress": {"Ingress", "Ingress"}, "storage": {"Storage / CSI", "스토리지 / CSI"}, "cpu": {"CPU / image platform", "CPU / 이미지 플랫폼"}, "os": {"OS", "OS"}, "application": {"Application behavior", "애플리케이션 동작"}, "cgroup": {"Cgroup mode", "cgroup 모드"}, "kernel": {"Kernel", "커널"}, "runtime": {"Container runtime", "컨테이너 런타임"}, "drivers": {"Kubelet / runtime drivers", "kubelet / 런타임 드라이버"}, "manifest-field": {"Manifest field change", "Manifest 필드 변경"}, "target-validation": {"Proposed manifest validation", "변경 후 manifest 검증"},
		"input_missing": {"Input missing", "입력 부족"}, "analyzer_unsupported": {"Analyzer not implemented", "판정기 미지원"}, "execution_required": {"Execution validation required", "실행 검증 필요"}, "action_required": {"Action required", "조치 필요"}, "review_required": {"Review required", "검토 필요"}, "not_detected": {"No finding in supported checks", "지원 범위 내 미발견"}, "baseline_met": {"Target baseline met (supplied evidence)", "목표 최소조건 충족 (제공 근거)"},
	}
	if pair, ok := labels[key]; ok {
		return tr(lang, pair[0], pair[1])
	}
	return key
}
func matrixHTML(b *strings.Builder, root map[string]any, lang string) {
	rows := list(root["matrix"])
	if len(rows) == 0 {
		return
	}
	fmt.Fprintf(b, `<section><h2>%s</h2><p class="table-note">%s</p>`, tr(lang, "Manifest and environment review matrix", "Manifest · 환경 전체 검토 매트릭스"), tr(lang, "Coverage inventory, not a compatibility certificate. Missing evidence and unsupported checks remain visible. Target manifests, when supplied, are proposals; raw field values are omitted.", "호환 보증이 아닌 검토 범위표입니다. 입력 부족·판정 미지원 항목도 표시합니다. 변경 후 manifest는 제안 상태이며 필드 원문 값은 표시하지 않습니다."))
	tableHead(b, "review-matrix", []string{tr(lang, "Topic / resource", "항목 / 리소스"), "As-Is", "To-Be", tr(lang, "Verdict", "판정"), tr(lang, "Evidence / action", "근거 / 조치")})
	for _, raw := range rows {
		r := obj(raw)
		fmt.Fprintf(b, `<tr><td><strong>%s</strong><div class="resource">%s</div></td><td>%s</td><td>%s</td><td>%s</td><td><details><summary>%s</summary><p>%s</p><p>%s</p></details></td></tr>`, htmlCell(matrixLabel(str(r, "topic"), lang)), htmlCell(r["resource"]), htmlCell(r["current"]), htmlCell(r["target"]), htmlCell(matrixLabel(str(r, "status"), lang)), tr(lang, "Evidence / action", "근거 / 조치"), htmlCell(r["evidence"]), htmlCell(r["action"]))
	}
	b.WriteString(`</tbody></table></div></section>`)
}
func matrixMarkdown(b *strings.Builder, root map[string]any, lang string) {
	rows := list(root["matrix"])
	if len(rows) == 0 {
		return
	}
	fmt.Fprintf(b, "## %s\n\n| %s | As-Is | To-Be | %s | %s |\n| --- | --- | --- | --- | --- |\n", tr(lang, "Manifest and environment review matrix", "Manifest · 환경 전체 검토 매트릭스"), tr(lang, "Topic / resource", "항목 / 리소스"), tr(lang, "Verdict", "판정"), tr(lang, "Evidence / action", "근거 / 조치"))
	for _, raw := range rows {
		r := obj(raw)
		fmt.Fprintf(b, "| %s | %s | %s | %s | %s |\n", markdownCell(matrixLabel(str(r, "topic"), lang)+" "+str(r, "resource")), markdownCell(str(r, "current")), markdownCell(str(r, "target")), markdownCell(matrixLabel(str(r, "status"), lang)), markdownCell(str(r, "evidence")+"; "+str(r, "action")))
	}
	b.WriteString("\n")
}
