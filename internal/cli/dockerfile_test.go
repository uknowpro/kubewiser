package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDockerfileCLI(t *testing.T) {
	dir := t.TempDir()
	docker := filepath.Join(dir, "Dockerfile")
	manifest := filepath.Join(dir, "pod.yaml")
	os.WriteFile(docker, []byte("FROM openjdk:11.0.15\nCMD cat /sys/fs/cgroup/memory/memory.limit_in_bytes\n"), 0600)
	os.WriteFile(manifest, []byte("apiVersion: v1\nkind: Pod\nmetadata: {name: demo}\nspec:\n  containers:\n  - name: app\n    image: private/app:1\n"), 0600)
	for _, format := range []string{"json", "html", "markdown", "text"} {
		var out, errs bytes.Buffer
		code := Run([]string{"scan", "cgroup", "--dockerfile", docker, "--format", format, "--lang", "ko"}, &out, &errs)
		if code != 1 {
			t.Fatal(code, errs.String())
		}
		if !strings.Contains(out.String(), "KW-DOCKERFILE-BASE") {
			t.Fatal(out.String())
		}
		if format == "json" && !json.Valid(out.Bytes()) {
			t.Fatal("invalid json")
		}
		if format == "html" && (!strings.Contains(out.String(), "Dockerfile · 앱 런타임") || !strings.Contains(out.String(), "review-matrix")) {
			t.Fatal("missing matrix")
		}
	}
	var out, errs bytes.Buffer
	if code := Run([]string{"plan", "upgrade", "--source", manifest, "--from", "1.21", "--to", "1.36", "--dockerfile", docker}, &out, &errs); code != 0 || !strings.Contains(out.String(), "application_sources") {
		t.Fatal(code, errs.String(), out.String())
	}
	out.Reset()
	errs.Reset()
	if code := Run([]string{"scan", "files", "--dockerfile", docker}, &out, &errs); code != 2 {
		t.Fatal(code)
	}
}

func TestDockerfileAutomaticCLI(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM eclipse-temurin:11.0.15\n"), 0600)
	os.WriteFile(filepath.Join(dir, "pod.yaml"), []byte("apiVersion: v1\nkind: Pod\nmetadata: {name: demo}\nspec:\n  containers:\n  - name: app\n    image: private/app:1\n"), 0600)
	for _, source := range []string{dir, filepath.Join(dir, "pod.yaml")} {
		for _, args := range [][]string{{"scan", "cgroup", "--source", source}, {"plan", "upgrade", "--source", source, "--from", "1.21", "--to", "1.36"}} {
			var out, errs bytes.Buffer
			code := Run(args, &out, &errs)
			if code == 2 || !strings.Contains(out.String(), "KW-DOCKERFILE-BASE") {
				t.Fatal(code, errs.String(), out.String())
			}
		}
	}
}
