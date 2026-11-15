package cli

import (
	"bytes"
	"encoding/json"
	"github.com/uknowpro/kubewiser/internal/buildinfo"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.yaml")
	os.WriteFile(p, []byte("apiVersion: batch/v1beta1\nkind: CronJob\n"), 0600)
	var out, errs bytes.Buffer
	if code := Run([]string{"scan", "files", "--target-version", "1.25", "--format", "json", p}, &out, &errs); code != 1 {
		t.Fatal(code, errs.String())
	}
	if !json.Valid(out.Bytes()) {
		t.Fatal(out.String())
	}
	if code := Run([]string{"scan", "files", p}, &out, &errs); code != 2 {
		t.Fatal(code)
	}
}

func TestVersion(t *testing.T) {
	for _, arg := range []string{"--version", "version"} {
		var out, errs bytes.Buffer
		if code := Run([]string{arg}, &out, &errs); code != 0 {
			t.Fatal(code, errs.String())
		}
		if !strings.Contains(out.String(), "kubewiser "+buildinfo.Version+" (commit "+buildinfo.Commit) {
			t.Fatal(out.String())
		}
	}
}
