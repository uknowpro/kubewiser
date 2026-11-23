package analyze

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func dockerPlan(t *testing.T, source string) Plan {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Dockerfile")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	p := Plan{}
	if err := AttachDockerfile(&p, path); err != nil {
		t.Fatal(err)
	}
	BuildMatrix(&p)
	return p
}
func TestDockerJavaBaseline(t *testing.T) {
	for _, tt := range []struct{ image, status string }{
		{"openjdk:8u371-jre", "review_required"}, {"eclipse-temurin:8u372-jre", "execution_required"},
		{"docker.io/library/openjdk:11.0.15", "review_required"}, {"openjdk:11.0.16", "execution_required"},
		{"eclipse-temurin:17-jre", "execution_required"}, {"openjdk:8", "input_missing"}, {"openjdk:11", "input_missing"},
		{"private.example/openjdk:8u222", "input_missing"}, {"${BASE}", "input_missing"}, {"builder", "input_missing"},
		{"openjdk@sha256:abc", "input_missing"}, {"openjdk:11.0.15-openj9", "input_missing"}, {"node:12", "input_missing"},
	} {
		t.Run(tt.image, func(t *testing.T) {
			_, _, status, _ := dockerJavaBaseline(tt.image)
			if status != tt.status {
				t.Fatal(status)
			}
		})
	}
}
func TestDockerfileStagesAndHints(t *testing.T) {
	p := dockerPlan(t, "\ufeffFROM openjdk:8u222 AS builder\r\nRUN echo secret-do-not-emit && \\\r\n cat /sys/fs/cgroup/memory/memory.limit_in_bytes\r\nFROM eclipse-temurin:17-jre\r\nCOPY --from=builder /app.jar /app.jar\r\nENV JAVA_TOOL_OPTIONS=\"-XX:-UseContainerSupport\"\r\n")
	bases, hints, options, coverage := 0, 0, 0, 0
	for _, a := range p.Actions {
		switch a.ID {
		case "KW-DOCKERFILE-BASE":
			bases++
		case "KW-DOCKERFILE-V1-REFERENCE":
			hints++
			if !strings.Contains(a.Resource, "stage 1, line 2") {
				t.Fatal(a)
			}
		case "KW-DOCKERFILE-JAVA-OPTIONS":
			options++
			if !strings.Contains(a.Resource, "stage 2") {
				t.Fatal(a)
			}
		case "KW-DOCKERFILE-CONTENTS":
			coverage++
		}
	}
	if bases != 2 || hints != 1 || options != 1 || coverage != 1 {
		t.Fatal(p.Actions)
	}
	b, _ := json.Marshal(p)
	if strings.Contains(string(b), "secret-do-not-emit") {
		t.Fatal("raw command leaked")
	}
	rows := 0
	for _, r := range p.Matrix {
		if r.Topic == "dockerfile" {
			rows++
		}
	}
	if rows != len(p.ApplicationSources) || rows == 0 {
		t.Fatal("missing matrix rows")
	}
}
func TestDockerfileUnsupportedNotInterpreted(t *testing.T) {
	for _, s := range []string{
		"FROM alpine\nRUN <<EOF\nFROM openjdk:8u222\nEOF\n",
		"# escape=`\nFROM openjdk:8u222\n",
		"# syntax=private/frontend\nFROM openjdk:8u222\n",
	} {
		p := dockerPlan(t, s)
		if len(p.ApplicationSources) != 1 || p.ApplicationSources[0].Status != "analyzer_unsupported" {
			t.Fatal(p)
		}
	}
	p := dockerPlan(t, "# syntax=docker/dockerfile:1\n# RUN cat memory.limit_in_bytes\nARG BASE=openjdk:8u222\nFROM $BASE\n")
	if len(p.Actions) != 1 || p.ApplicationSources[0].Status != "input_missing" {
		t.Fatal(p)
	}
}
func TestDockerfileInvalidAndLimits(t *testing.T) {
	for _, s := range []string{"", "RUN true\n", "FROM alpine\nRUN echo \\\n", "FROM alpine\n\x00", strings.Repeat("x", (1<<20)+1), "FROM alpine\n" + strings.Repeat("RUN true\n", 1001)} {
		path := filepath.Join(t.TempDir(), "Dockerfile")
		os.WriteFile(path, []byte(s), 0600)
		if AttachDockerfile(&Plan{}, path) == nil {
			t.Fatal("expected invalid/limit failure")
		}
	}
}

func TestDockerfileDiscoveryScope(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"Dockerfile", "service/Dockerfile.prod", "service/app.Containerfile", ".git/Dockerfile", "node_modules/Dockerfile", "vendor/Dockerfile", "deploy/input.yaml"} {
		path := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(path), 0700)
		os.WriteFile(path, []byte("FROM openjdk:8u222\n"), 0600)
	}
	paths, err := discoverDockerfiles(context.Background(), root)
	if err != nil || len(paths) != 3 {
		t.Fatal(paths, err)
	}
	paths, err = discoverDockerfiles(context.Background(), filepath.Join(root, "deploy/input.yaml"))
	if err != nil || len(paths) != 0 {
		t.Fatal("must not search parent", paths, err)
	}
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "Dockerfile"), []byte("FROM alpine\n"), 0600)
	if err = os.Symlink(outside, filepath.Join(root, "external")); err == nil {
		paths, err = discoverDockerfiles(context.Background(), root)
		if err != nil || len(paths) != 3 {
			t.Fatal(paths, err)
		}
	}
	p := Plan{}
	if err = AttachDockerfiles(context.Background(), &p, root, ""); err != nil || len(p.ApplicationSources) != 3 {
		t.Fatal(p, err)
	}
	p = Plan{}
	if err = AttachDockerfiles(context.Background(), &p, "", ""); err != nil || p.ApplicationSources[0].Status != "input_missing" {
		t.Fatal(p, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = AttachDockerfiles(ctx, &Plan{}, root, ""); err == nil {
		t.Fatal("cancellation ignored")
	}
}
func TestDockerfileDiscoveryBudgetAndInvalid(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 33; i++ {
		os.WriteFile(filepath.Join(root, fmt.Sprintf("Dockerfile.%02d", i)), []byte("FROM alpine\n"), 0600)
	}
	paths, err := discoverDockerfiles(context.Background(), root)
	if err == nil || len(paths) != 32 {
		t.Fatal(len(paths), err)
	}
	p := Plan{}
	if err = AttachDockerfiles(context.Background(), &p, root, ""); err != nil {
		t.Fatal(err)
	}
	if p.ApplicationSources[0].Status != "input_missing" {
		t.Fatal("truncation must remain visible")
	}
	root = t.TempDir()
	os.WriteFile(filepath.Join(root, "Dockerfile"), []byte("invalid"), 0600)
	p = Plan{}
	if err = AttachDockerfiles(context.Background(), &p, root, ""); err != nil || p.ApplicationSources[0].Status != "input_missing" {
		t.Fatal(p, err)
	}
}
