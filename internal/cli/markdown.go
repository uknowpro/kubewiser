package cli

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"sort"
	"strings"
)

// Render the same public report as JSON, retaining evidence and unchecked fields.
// Escape input-controlled Markdown/HTML so resource names cannot inject markup.
func writeMarkdown(w io.Writer, report any) error {
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
	b.WriteString("# Kubewiser report\n\n")
	markdownValue(&b, value, 2)
	_, err = io.WriteString(w, b.String())
	return err
}

func markdownCell(v any) string {
	var s string
	switch x := v.(type) {
	case nil:
		return "—"
	case string:
		s = x
	default:
		data, _ := json.Marshal(x)
		s = string(data)
	}
	s = html.EscapeString(s)
	s = strings.NewReplacer("\\", "\\\\", "|", "\\|", "`", "\\`", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "\r\n", "<br>", "\n", "<br>", "\r", "<br>").Replace(s)
	return s
}
func markdownKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
func markdownValue(b *strings.Builder, v any, depth int) {
	switch x := v.(type) {
	case map[string]any:
		keys := markdownKeys(x)
		scalars, nested := []string{}, []string{}
		for _, k := range keys {
			switch x[k].(type) {
			case map[string]any, []any:
				nested = append(nested, k)
			default:
				scalars = append(scalars, k)
			}
		}
		if len(scalars) > 0 {
			b.WriteString("| 항목 / Field | 값 / Value |\n| --- | --- |\n")
			for _, k := range scalars {
				fmt.Fprintf(b, "| %s | %s |\n", markdownCell(k), markdownCell(x[k]))
			}
			b.WriteString("\n")
		}
		for _, k := range nested {
			level := depth
			if level > 6 {
				level = 6
			}
			fmt.Fprintf(b, "%s %s\n\n", strings.Repeat("#", level), markdownCell(k))
			markdownValue(b, x[k], depth+1)
		}
	case []any:
		if len(x) == 0 {
			b.WriteString("없음 / None.\n\n")
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
					fmt.Fprintf(b, "%s %d. %s\n\n", strings.Repeat("#", level), i+1, markdownCell(title))
					markdownValue(b, m, depth+1)
				}
				return
			}

			b.WriteString("|")
			for _, k := range keys {
				fmt.Fprintf(b, " %s |", markdownCell(k))
			}
			b.WriteString("\n|")
			for range keys {
				b.WriteString(" --- |")
			}
			b.WriteString("\n")
			for _, item := range x {
				m := item.(map[string]any)
				b.WriteString("|")
				for _, k := range keys {
					fmt.Fprintf(b, " %s |", markdownCell(m[k]))
				}
				b.WriteString("\n")
			}
		} else {
			for _, item := range x {
				fmt.Fprintf(b, "- %s\n", markdownCell(item))
			}
		}
		b.WriteString("\n")
	default:
		fmt.Fprintf(b, "%s\n\n", markdownCell(v))
	}
}
