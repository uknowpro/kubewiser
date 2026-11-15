package cli

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"strings"
)

// A standalone report: no scripts, remote assets or input-controlled markup.
func writeHTML(w io.Writer, report any, langs ...string) error {
	data, err := json.Marshal(report)
	if err != nil {
		return err
	}
	var value any
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	if err = decoder.Decode(&value); err != nil {
		return err
	}
	var b strings.Builder
	lang := language(langs)
	b.WriteString(`<!doctype html>
<html lang="` + lang + `"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'">
<title>Kubewiser report</title><style>
:root{color-scheme:light dark;font-family:system-ui,sans-serif;line-height:1.5}body{max-width:1400px;margin:0 auto;padding:32px}h1{margin-bottom:8px}h2,h3{margin-top:32px}section{margin:16px 0}.table-wrap{overflow-x:auto}table{border-collapse:collapse;width:100%;font-size:14px}th,td{border:1px solid #8886;text-align:left;vertical-align:top;padding:10px;white-space:pre-wrap;overflow-wrap:anywhere}th{background:#8882}tbody tr:nth-child(even){background:#8881}.muted{opacity:.7}li{white-space:pre-wrap;overflow-wrap:anywhere;margin:6px 0}@media print{body{padding:0;font-size:11px}.table-wrap{overflow:visible}tr{break-inside:avoid}h2,h3{break-after:avoid}}
` + reviewCSS + `</style></head><body><main>
`)
	if !reviewHTML(&b, value, lang) {
		b.WriteString("<h1>Kubewiser report</h1>")
		htmlValue(&b, value, 2)
	}

	b.WriteString("</main></body></html>\n")
	_, err = io.WriteString(w, b.String())
	return err
}
func htmlCell(v any) string {
	if v == nil {
		return "—"
	}
	if s, ok := v.(string); ok {
		return html.EscapeString(s)
	}
	data, _ := json.Marshal(v)
	return html.EscapeString(string(data))
}
func htmlValue(b *strings.Builder, v any, depth int) {
	start := func() { b.WriteString(`<div class="table-wrap"><table>`) }
	end := func() { b.WriteString("</tbody></table></div>\n") }
	switch x := v.(type) {
	case map[string]any:
		scalars, nested := []string{}, []string{}
		for _, k := range markdownKeys(x) {
			switch x[k].(type) {
			case map[string]any, []any:
				nested = append(nested, k)
			default:
				scalars = append(scalars, k)
			}
		}
		if len(scalars) > 0 {
			start()
			b.WriteString("<thead><tr><th scope=\"col\">항목 / Field</th><th scope=\"col\">값 / Value</th></tr></thead><tbody>")
			for _, k := range scalars {
				fmt.Fprintf(b, "<tr><th scope=\"row\">%s</th><td>%s</td></tr>\n", htmlCell(k), htmlCell(x[k]))
			}
			end()
		}
		for _, k := range nested {
			level := depth
			if level > 6 {
				level = 6
			}
			fmt.Fprintf(b, "<section><h%d>%s</h%d>\n", level, htmlCell(k), level)
			htmlValue(b, x[k], depth+1)
			b.WriteString("</section>\n")
		}
	case []any:
		if len(x) == 0 {
			b.WriteString("<p class=\"muted\">없음 / None.</p>\n")
			return
		}
		columns := map[string]any{}
		objects := true
		for _, item := range x {
			m, ok := item.(map[string]any)
			if !ok {
				objects = false
				break
			}
			for k := range m {
				columns[k] = nil
			}
		}
		if objects && len(columns) > 0 {
			keys := markdownKeys(columns)
			if len(keys) > 6 {
				level := depth
				if level > 6 {
					level = 6
				}
				for i, item := range x {
					m := item.(map[string]any)
					title := m["rule_id"]
					if title == nil {
						title = m["id"]
					}
					if title == nil {
						title = m["resource"]
					}
					fmt.Fprintf(b, "<section><h%d>%d. %s</h%d>\n", level, i+1, htmlCell(title), level)
					htmlValue(b, m, depth+1)
					b.WriteString("</section>\n")
				}
				return
			}

			start()
			b.WriteString("<thead><tr>")
			for _, k := range keys {
				fmt.Fprintf(b, "<th scope=\"col\">%s</th>", htmlCell(k))
			}
			b.WriteString("</tr></thead><tbody>")
			for _, item := range x {
				m := item.(map[string]any)
				b.WriteString("<tr>")
				for _, k := range keys {
					fmt.Fprintf(b, "<td>%s</td>", htmlCell(m[k]))
				}
				b.WriteString("</tr>\n")
			}
			end()
		} else {
			b.WriteString("<ul>")
			for _, item := range x {
				fmt.Fprintf(b, "<li>%s</li>\n", htmlCell(item))
			}
			b.WriteString("</ul>\n")
		}
	default:
		fmt.Fprintf(b, "<p>%s</p>\n", htmlCell(v))
	}
}
