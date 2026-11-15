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

func TestCollectRequiresExplicitScope(t *testing.T) {
	var out, errs bytes.Buffer
	code := Run([]string{"collect", "--kubeconfig", "missing", "--context", "staging", "--output", "unused.json"}, &out, &errs)
	if code != 2 || !strings.Contains(errs.String(), "explicit --namespace") {
		t.Fatal(code, errs.String())
	}
}

func TestSnapshotNetworkAndDiffNamespace(t *testing.T) {
	dir := t.TempDir()
	desired := filepath.Join(dir, "desired.yaml")
	snapshot := filepath.Join(dir, "snapshot.json")
	os.WriteFile(desired, []byte("apiVersion: v1\nkind: Service\nmetadata: {name: web}\nspec: {type: LoadBalancer}\n"), 0600)
	os.WriteFile(snapshot, []byte(`{"format":"kubewiser.snapshot.v1","namespace":"team","resources":[{"object":{"apiVersion":"v1","kind":"Service","metadata":{"name":"web","namespace":"team"},"spec":{"type":"LoadBalancer"}}}],"gaps":[{"resource":"Pod","reason":"HTTP 403"}]}`), 0600)
	var out, errs bytes.Buffer
	if code := Run([]string{"scan", "network", "--snapshot", snapshot}, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	if !strings.Contains(out.String(), "HTTP 403") || !strings.Contains(out.String(), "LoadBalancer") {
		t.Fatal(out.String())
	}
	out.Reset()
	errs.Reset()
	code := Run([]string{"diff", "--source", desired, "--snapshot", snapshot}, &out, &errs)
	if code == 2 || strings.Contains(out.String(), "not_observed") {
		t.Fatal(code, out.String(), errs.String())
	}
}
