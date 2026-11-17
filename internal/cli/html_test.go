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

func TestCompactVersionComparisonScope(t *testing.T) {
	var out, errs bytes.Buffer
	code := Run([]string{"plan", "upgrade", "--source", "../../docs/examples/manifest.yaml", "--from", "1.21", "--to", "1.36", "--format", "html", "--lang", "ko"}, &out, &errs)
	if code != 0 {
		t.Fatal(code, errs.String())
	}
	s := out.String()
	for _, want := range []string{`id="findings-table"`, `id="version-comparison"`, "As-Is 1.21", "To-Be 1.36", "batch/v1beta1", "KW-BEHAVIOR-DOCKERSHIM", "입력에서 미발견", "실제 영향 미확인", "상세 / 검증 방법"} {
		if !strings.Contains(s, want) {
			t.Fatal("missing", want)
		}
	}
	if strings.Contains(s, `<article`) {
		t.Fatal("card layout remains")
	}
	version := strings.Split(strings.Split(s, `id="version-comparison"`)[1], `</table>`)[0]
	if strings.Contains(version, "apps/v1beta1") {
		t.Fatal("pre-existing removal mislabeled as a new version change")
	}
	// A file scan without an As-Is version must not invent a version comparison.
	out.Reset()
	if err := writeHTML(&out, map[string]any{"findings": []any{}}, "en"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), `id="version-comparison"`) {
		t.Fatal("invented version range")
	}
}
