package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }
func TestMarkdownEscapingAndCoverage(t *testing.T) {
	var out bytes.Buffer
	err := writeMarkdown(&out, map[string]any{"findings": []any{map[string]any{"resource": "a|b\n<script>x</script>", "message": "[link](https://example.org)"}}, "unchecked": []string{"Admission not evaluated"}, "count": 1})
	if err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{"# Inspection review", "a\\|b<br>&lt;script&gt;", "\\[link\\]", "Admission not evaluated", `"count": 1`} {
		if !strings.Contains(s, want) {
			t.Fatal(want, s)
		}
	}
	if strings.Contains(s, "<script>") {
		t.Fatal(s)
	}
}
func TestMarkdownExitCodes(t *testing.T) {
	var out, errs bytes.Buffer
	if code := emit(&out, &errs, map[string]any{"changes": []any{}}, "markdown", 1); code != 1 {
		t.Fatal(code)
	}
	if code := emit(brokenWriter{}, &errs, map[string]any{}, "markdown", 0); code != 2 {
		t.Fatal(code)
	}
	out.Reset()
	if code := Run([]string{"versions", "--format", "markdown"}, &out, &errs); code != 0 || !strings.Contains(out.String(), "v1.37.0") {
		t.Fatal(code, out.String(), errs.String())
	}
}
