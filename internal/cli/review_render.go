package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func language(a []string) string {
	if len(a) > 0 && a[0] == "ko" {
		return "ko"
	}
	return "en"
}
func reviewMarkdown(b *strings.Builder, value any, lang string) bool {
	r, ok := buildReview(value, lang)
	if !ok {
		return false
	}
	fmt.Fprintf(b, "# %s\n\n%s\n\n%s\n\n", r.Title, r.Summary, markdownCell(r.Scope))
	fmt.Fprintf(b, "| %s | %s | %s |\n| --- | --- | --- |\n| %d | %d | %d |\n\n", requirement(lang, "required"), requirement(lang, "conditional"), requirement(lang, "optional"), r.Required, r.Conditional, r.Optional)
	fmt.Fprintf(b, "> %s\n\n", tr(lang, "Partial coverage. No automatic changes are applied. Recommendations must be validated for your workload.", "검사 범위는 부분적입니다. 변경은 자동 적용되지 않으며 권장안은 실제 워크로드에서 검증해야 합니다."))
	for n, i := range r.Items {
		fmt.Fprintf(b, "## %d. %s · %s\n\n%s\n\n", n+1, requirement(lang, i.Requirement), markdownCell(i.Title), markdownCell(i.Resource))
		fmt.Fprintf(b, "| %s | %s |\n| --- | --- |\n| %s | %s |\n\n", tr(lang, "Current state", "현재 상태"), tr(lang, "Recommended state", "권장 상태"), markdownCell(i.Current), markdownCell(i.Recommended))
		for _, row := range [][2]string{{tr(lang, "Why", "이유"), i.Why}, {tr(lang, "Next action", "다음 조치"), i.Action}, {tr(lang, "Verify", "검증 방법"), i.Verify}, {tr(lang, "Location / rule", "위치 / 규칙"), i.Location + " · " + i.Rule}, {tr(lang, "Source", "근거"), i.Source}} {
			if row[1] != "" {
				fmt.Fprintf(b, "- **%s:** %s\n", row[0], markdownCell(row[1]))
			}
		}
		b.WriteString("\n")
	}
	if len(r.Steps) > 0 {
		fmt.Fprintf(b, "## %s\n\n", tr(lang, "Upgrade / migration steps", "업그레이드 / 이전 단계"))
		for n, s := range r.Steps {
			fmt.Fprintf(b, "%d. %s\n", n+1, markdownCell(s))
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(b, "## %s\n\n", tr(lang, "Not verified", "검사하지 못한 항목"))
	for _, s := range r.Unchecked {
		fmt.Fprintf(b, "- %s\n", markdownCell(s))
	}
	data, _ := json.MarshalIndent(value, "", "  ")
	fmt.Fprintf(b, "\n<details>\n<summary>%s</summary>\n\n```json\n%s\n```\n\n</details>\n", tr(lang, "Original evidence and metadata", "원본 근거와 메타데이터"), strings.ReplaceAll(string(data), "`", `\u0060`))
	return true
}
func reviewHTML(b *strings.Builder, value any, lang string) bool {
	r, ok := buildReview(value, lang)
	if !ok {
		return false
	}
	fmt.Fprintf(b, "<header><p class=\"eyebrow\">KUBEWISER · %s</p><h1>%s</h1><p>%s</p><p class=\"muted\">%s</p></header>", tr(lang, "EVIDENCE-BASED REVIEW", "근거 기반 검사"), htmlCell(r.Title), htmlCell(r.Summary), htmlCell(r.Scope))
	b.WriteString(`<div class="metrics">`)
	for _, m := range []struct {
		n           int
		label, kind string
	}{{r.Required, requirement(lang, "required"), "required"}, {r.Conditional, requirement(lang, "conditional"), "conditional"}, {r.Optional, requirement(lang, "optional"), "optional"}} {
		fmt.Fprintf(b, "<div class=\"metric %s\"><strong>%d</strong><span>%s</span></div>", m.kind, m.n, m.label)
	}
	b.WriteString("</div>")
	fmt.Fprintf(b, "<p class=\"notice\">%s</p>", tr(lang, "Partial coverage · Recommendations require workload validation · No automatic changes", "부분 검사 · 권장안은 워크로드별 검증 필요 · 변경 자동 적용 없음"))
	cgroupHTML(b, obj(value), lang)
	lifecycleHTML(b, obj(value), lang)
	versionComparisonHTML(b, obj(value), lang)
	compactFindingsHTML(b, r, lang)

	if len(r.Steps) > 0 {
		fmt.Fprintf(b, "<section><h2>%s</h2><ol>", tr(lang, "Upgrade / migration steps", "업그레이드 / 이전 단계"))
		for _, s := range r.Steps {
			fmt.Fprintf(b, "<li>%s</li>", htmlCell(s))
		}
		b.WriteString("</ol></section>")
	}
	fmt.Fprintf(b, "<section class=\"coverage\"><h2>%s</h2><p>%s</p><ul>", tr(lang, "Not verified", "검사하지 못한 항목"), tr(lang, "These limits still apply even when no findings are reported.", "발견 사항이 없어도 아래 검증 한계는 남아 있습니다."))
	for _, s := range r.Unchecked {
		fmt.Fprintf(b, "<li>%s</li>", htmlCell(s))
	}
	b.WriteString("</ul></section>")
	data, _ := json.MarshalIndent(value, "", "  ")
	fmt.Fprintf(b, "<details><summary>%s</summary><pre>%s</pre></details>", tr(lang, "Original evidence, source hashes and metadata", "원본 근거·출처 해시·메타데이터"), htmlCell(string(data)))
	return true
}
func writeTextReview(w io.Writer, value any, lang string) (bool, error) {
	r, ok := buildReview(value, lang)
	if !ok {
		return false, nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n%s\n%s\n%s: %d | %s: %d | %s: %d\n\n", r.Title, r.Scope, r.Summary, requirement(lang, "required"), r.Required, requirement(lang, "conditional"), r.Conditional, requirement(lang, "optional"), r.Optional)
	for n, i := range r.Items {
		fmt.Fprintf(&b, "%d. [%s] %s (%s)\n   %s\n", n+1, requirement(lang, i.Requirement), i.Title, i.Rule, i.Resource)
		for _, row := range [][2]string{{tr(lang, "Current", "현재"), i.Current}, {tr(lang, "Recommended", "권장"), i.Recommended}, {tr(lang, "Why", "이유"), i.Why}, {tr(lang, "Action", "조치"), i.Action}, {tr(lang, "Verify", "검증"), i.Verify}, {tr(lang, "Location", "위치"), i.Location}, {tr(lang, "Source", "근거"), i.Source}} {
			if row[1] != "" {
				fmt.Fprintf(&b, "   %s: %s\n", row[0], row[1])
			}
		}
		b.WriteString("\n")
	}
	for _, s := range r.Steps {
		fmt.Fprintf(&b, "%s: %s\n", tr(lang, "Step", "단계"), s)
	}
	fmt.Fprintf(&b, "%s\n", tr(lang, "NOT VERIFIED (partial coverage):", "미검사 항목 (부분 검사):"))
	for _, s := range r.Unchecked {
		fmt.Fprintf(&b, "- %s\n", s)
	}
	_, err := io.WriteString(w, b.String())
	return true, err
}

const reviewCSS = `
:root{color-scheme:light;font-family:system-ui,sans-serif;color:#172b3a;background:#f3f6fa}body{max-width:1080px;padding:40px 24px}header{margin-bottom:24px}h1{font-size:32px;letter-spacing:-.03em}h2{font-size:22px;margin:12px 0}h3{font-size:13px;text-transform:uppercase;letter-spacing:.04em;margin:0 0 12px}.eyebrow{font-size:12px;font-weight:700;color:#486079;letter-spacing:.12em}.metrics{display:grid;grid-template-columns:repeat(3,1fr);gap:16px}.metric{background:white;border:1px solid #d8e1ea;border-radius:12px;padding:18px 24px;border-top:4px solid #75869b}.metric.required{border-top-color:#b42318}.metric.conditional{border-top-color:#a66300}.metric.optional{border-top-color:#245eb0}.metric strong{font-size:34px;display:block}.metric span{font-size:14px}.notice{font-size:13px;color:#526277;background:#e8eef5;padding:12px 16px;border-radius:8px}.finding{background:white;border:1px solid #d8e1ea;border-radius:14px;padding:24px;margin:24px 0}.finding-head{display:flex;justify-content:space-between;gap:12px;align-items:center}.badge{font-size:12px;font-weight:700;border-radius:6px;padding:4px 10px;background:#fff0d1;color:#795000}.required .badge{background:#fee6e3;color:#a42118}.optional .badge{background:#e6efff;color:#245eb0}.resource{font-family:ui-monospace,monospace;font-size:13px;overflow-wrap:anywhere;color:#486079}.comparison{display:grid;grid-template-columns:1fr 1fr;gap:16px;margin:20px 0}.comparison section{margin:0;padding:18px;background:#f5f7fa;border-radius:8px;border:1px solid #dfe5ec}.comparison .recommended{background:#edf7f4;border-color:#b8dcd1}.comparison p{margin:0;white-space:pre-wrap;overflow-wrap:anywhere}dl{display:grid;grid-template-columns:130px 1fr;gap:12px;font-size:14px}dt{font-weight:650}dd{margin:0;overflow-wrap:anywhere}a{color:#245eb0}.coverage{border-top:1px solid #d8e1ea;padding-top:20px;font-size:14px}details{margin:24px 0;background:white;border:1px solid #d8e1ea;border-radius:8px;padding:16px}summary{cursor:pointer;font-weight:600}pre{white-space:pre-wrap;overflow-wrap:anywhere;font-size:12px}.muted{color:#58697c;opacity:1}@media(max-width:640px){body{padding:20px 12px}.comparison{grid-template-columns:1fr}.metrics{gap:8px}.metric{padding:12px}.metric strong{font-size:28px}dl{grid-template-columns:1fr;gap:6px}dd{margin-bottom:10px}}@media print{body{background:white}.finding{break-inside:avoid}details{display:block}details pre{display:block}}
`

const compactCSS = `
body{max-width:1680px;padding:24px}h1{font-size:26px}.metrics{display:flex;gap:12px}.metric{padding:8px 16px;display:flex;align-items:center;gap:12px}.metric strong{font-size:24px}.notice{padding:8px 12px}.compact-table{font-size:12px;line-height:1.45;table-layout:fixed;background:white}.compact-table th,.compact-table td{padding:8px;white-space:normal}.compact-table thead th{background:#e8eef5}.compact-table .resource{font-size:11px;margin-top:4px}.compact-table details{margin:4px 0 0;padding:0;border:0;background:transparent}.compact-table summary{font-size:11px;color:#245eb0}.compact-table dl{display:block;font-size:12px}.compact-table dt{margin-top:8px}.compact-table dd{margin-top:3px}.compact-table .badge{display:inline-block;padding:2px 5px;white-space:nowrap}.compact-table p{margin:4px 0}.compact-table .row-number{width:4%}.compact-table .priority{width:8%}.compact-table .subject{width:22%}.compact-table .state{width:22%}.compact-table .next{width:22%}.table-note{font-size:12px;color:#58697c}.compact-table .source-link{display:block;font-size:11px;margin-top:4px}@media(max-width:900px){.compact-table{min-width:1000px}body{padding:16px 10px}}@media print{body{max-width:none;padding:0}.compact-table{min-width:0;font-size:9px}.compact-table td,.compact-table th{padding:4px}.compact-table details:not([open]) dl{display:none}.metrics{gap:8px}thead{display:table-header-group}}
`

const compactColumnCSS = `#findings-table th:nth-child(1){width:3%}#findings-table th:nth-child(2){width:7%}#findings-table th:nth-child(3){width:20%}#findings-table th:nth-child(4){width:21%}#findings-table th:nth-child(5){width:23%}#findings-table th:nth-child(6){width:26%}#version-comparison th:nth-child(1){width:7%}#version-comparison th:nth-child(2){width:21%}#version-comparison th:nth-child(3){width:16%}#version-comparison th:nth-child(4){width:22%}#version-comparison th:nth-child(5){width:22%}#version-comparison th:nth-child(6){width:12%}`
