package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCLI(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.yaml")
	os.WriteFile(p, []byte("apiVersion: batch/v1beta1\nkind: CronJob\n"), 0600)
	var out, errs bytes.Buffer
	if code := run([]string{"scan", "files", "--target-version", "1.25", "--format", "json", p}, &out, &errs); code != 1 {
		t.Fatal(code, errs.String())
	}
	if !json.Valid(out.Bytes()) {
		t.Fatal(out.String())
	}
	if code := run([]string{"scan", "files", p}, &out, &errs); code != 2 {
		t.Fatal(code)
	}
}
