package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewPrioritizesAndComparesWithoutInventingSizing(t *testing.T) {
	value := map[string]any{"target_version": "1.37", "documents": 2, "schema_checked": 1, "findings": []any{
		map[string]any{"rule_id": "KW-REQUESTS", "requirement": "optional", "resource": "v1/Pod/ns/web"},
		map[string]any{"rule_id": "KW-API-REMOVED", "requirement": "required", "resource": "batch/v1beta1/CronJob/ns/job"},
	}, "unchecked": []any{"Admission not evaluated"}}
	r, ok := buildReview(value, "en")
	if !ok || r.Required != 1 || r.Optional != 1 || r.Items[0].Rule != "KW-API-REMOVED" {
		t.Fatal(r)
	}
	if r.Items[0].Current != "apiVersion: batch/v1beta1" || r.Items[0].Recommended != "batch/v1" {
		t.Fatal(r.Items[0])
	}
	if !strings.Contains(r.Items[1].Recommended, "Numeric sizing is not available") {
		t.Fatal(r.Items[1])
	}
	var out bytes.Buffer
	if e := writeHTML(&out, value, "ko"); e != nil {
		t.Fatal(e)
	}
	for _, s := range []string{"현재 상태", "권장 상태", "필수 조치", "batch/v1beta1", "batch/v1", "Admission not evaluated"} {
		if !strings.Contains(out.String(), s) {
			t.Fatal(s)
		}
	}
}
func TestDriftDoesNotRecommendBlindRevert(t *testing.T) {
	r, _ := buildReview(map[string]any{"changes": []any{map[string]any{"type": "observed_only_resource", "resource": "/Pod/ns/web"}}}, "en")
	if r.Required != 0 || !strings.Contains(r.Items[0].Action, "not deletion recommendations") {
		t.Fatal(r)
	}
}
func TestLanguageKeepsJSONAndExitCodes(t *testing.T) {
	p := filepath.Join(t.TempDir(), "legacy.yaml")
	os.WriteFile(p, []byte("apiVersion: batch/v1beta1\nkind: CronJob\nmetadata: {name: legacy}\n"), 0600)
	var en, ko, errs bytes.Buffer
	args := []string{"scan", "files", "--target-version", "1.37", "--format", "json"}
	if Run(append(append([]string{}, args...), p), &en, &errs) != 1 {
		t.Fatal(errs.String())
	}
	if Run(append(append([]string{}, args...), "--lang", "ko", p), &ko, &errs) != 1 {
		t.Fatal(errs.String())
	}
	if !bytes.Equal(en.Bytes(), ko.Bytes()) || !json.Valid(ko.Bytes()) {
		t.Fatal("JSON changed with language")
	}
	ko.Reset()
	if Run([]string{"scan", "files", "--target-version", "1.37", "--format", "text", "--lang", "ko", p}, &ko, &errs) != 1 || !strings.Contains(ko.String(), "권장:") {
		t.Fatal(ko.String(), errs.String())
	}
	if code := emit(brokenWriter{}, &errs, map[string]any{"findings": []any{}}, "text", 0); code != 2 {
		t.Fatal(code)
	}
}
func TestReviewRejectsUnsafeSourceLinks(t *testing.T) {
	var out bytes.Buffer
	writeHTML(&out, map[string]any{"findings": []any{map[string]any{"rule_id": "unknown", "source": "javascript:alert(1)", "message": "</pre><script>alert(1)</script>"}}})
	if strings.Contains(out.String(), `href="javascript:`) || strings.Contains(out.String(), "<script>") {
		t.Fatal(out.String())
	}
}
