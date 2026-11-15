package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectCLIFormatsAndDraftConflict(t *testing.T) {
	dir := t.TempDir()
	profile := filepath.Join(dir, "profile.json")
	input := filepath.Join(dir, "input.yaml")
	os.WriteFile(profile, []byte(`{"schema_version":1,"provider":"aks","mode":"in-place","from_version":"1.21","to_version":"1.35","architecture_changes":[{"area":"cpu","current":"amd64","target":"arm64","reason":"<script>review</script>"}]}`), 0600)
	os.WriteFile(input, []byte("apiVersion: batch/v1beta1\nkind: CronJob\nmetadata: {name: old}\n"), 0600)
	for _, format := range []string{"json", "text", "markdown", "html"} {
		var out, err bytes.Buffer
		code := Run([]string{"plan", "aks", "--source", input, "--profile", profile, "--format", format}, &out, &err)
		if code != 0 || !strings.Contains(out.String(), "arm64") {
			t.Fatal(code, out.String(), err.String())
		}
		if format == "html" && strings.Contains(out.String(), "<script>review</script>") {
			t.Fatal("unescaped project data")
		}
		if format == "json" {
			var p map[string]any
			if e := json.Unmarshal(out.Bytes(), &p); e != nil || p["status"] != "review_required" {
				t.Fatal(p, e)
			}
		}
	}
	outdir := filepath.Join(dir, "drafts")
	var out, err bytes.Buffer
	if code := Run([]string{"generate", "--profile", profile, "--output", outdir}, &out, &err); code != 0 {
		t.Fatal(code, err.String())
	}
	if code := Run([]string{"generate", "--profile", profile, "--output", outdir}, &out, &err); code != 2 {
		t.Fatal("overwrite accepted", code)
	}
}
