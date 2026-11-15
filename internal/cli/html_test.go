package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestStandaloneHTMLEscapesInputs(t *testing.T) {
	var out, errs bytes.Buffer
	report := map[string]any{"findings": []any{map[string]any{"resource": "<img src=x onerror=alert(1)>", "message": "a & b"}}, "unchecked": []string{"Admission not evaluated"}}
	if code := emit(&out, &errs, report, "html", 1); code != 1 {
		t.Fatal(code)
	}
	s := out.String()
	for _, want := range []string{"<!doctype html>", "Current state", "Recommended state", "&lt;img src=x onerror=alert(1)&gt;", "a &amp; b", "Admission not evaluated", "default-src 'none'"} {
		if !strings.Contains(s, want) {
			t.Fatal(want, s)
		}
	}
	for _, bad := range []string{"<img", "<script", "<link"} {
		if strings.Contains(s, bad) {
			t.Fatal(bad, s)
		}
	}
	if code := emit(brokenWriter{}, &errs, report, "html", 0); code != 2 {
		t.Fatal(code)
	}
	out.Reset()
	if code := Run([]string{"versions", "--format", "html"}, &out, &errs); code != 0 || !strings.Contains(out.String(), "v1.37.0") {
		t.Fatal(code, errs.String())
	}
}
