package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/uknowpro/kubewiser/internal/manifest"
	"k8s.io/client-go/rest"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCollectionPaginationRedactionAndRBAC(t *testing.T) {
	pages := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Error("non-GET request")
		}
		if strings.Contains(r.URL.Path, "secrets") {
			t.Error("secret endpoint")
		}
		switch r.URL.Path {
		case "/version":
			fmt.Fprint(w, `{"gitVersion":"v1.32.0"}`)
		case "/api/v1/namespaces/test/pods":
			pages++
			if r.URL.Query().Get("limit") != "200" {
				t.Error("missing pagination")
			}
			if r.URL.Query().Get("continue") == "" {
				fmt.Fprint(w, `{"metadata":{"continue":"next"},"items":[{"metadata":{"name":"a","namespace":"test","annotations":{"token":"TOPSECRET"}},"spec":{"containers":[{"name":"c","env":[{"name":"PASSWORD","value":"TOPSECRET"}],"args":["TOPSECRET"]}]}}]}`)
			} else {
				fmt.Fprint(w, `{"items":[{"metadata":{"name":"b","namespace":"test"}}]}`)
			}
		default:
			w.WriteHeader(403)
		}
	}))
	defer server.Close()
	o := Options{Namespace: "test", QPS: 10000, MaxItems: 10, Timeout: time.Second}
	s, e := collectConfig(context.Background(), &rest.Config{Host: server.URL}, o, Snapshot{Format: "kubewiser.snapshot.v1"})
	if e != nil || len(s.Resources) != 2 || pages != 2 || len(s.Gaps) == 0 {
		t.Fatal(s, e)
	}
	b, _ := json.Marshal(s)
	if strings.Contains(string(b), "TOPSECRET") {
		t.Fatal("secret leaked")
	}
	if len(s.Redacted) < 3 {
		t.Fatal(s.Redacted)
	}
	p := filepath.Join(t.TempDir(), "snapshot.json")
	if e = Write(p, s); e != nil {
		t.Fatal(e)
	}
	if e = Write(p, s); e == nil {
		t.Fatal("overwrote snapshot")
	}
	if _, e = Read(p); e != nil {
		t.Fatal(e)
	}
}
func TestRejectExecPlugin(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config")
	os.WriteFile(p, []byte("apiVersion: v1\nkind: Config\nclusters:\n- name: c\n  cluster: {server: 'https://127.0.0.1:1'}\ncontexts:\n- name: c\n  context: {cluster: c, user: u}\nusers:\n- name: u\n  user:\n    exec: {command: never-run, apiVersion: client.authentication.k8s.io/v1}\n"), 0600)
	_, e := Collect(context.Background(), Options{Kubeconfig: p, Context: "c", Namespace: "test", QPS: 2, MaxItems: 10, Timeout: time.Second})
	if e == nil || !strings.Contains(e.Error(), "plugins") {
		t.Fatal(e)
	}
}
func TestGuardRejectsWritesAndOtherOrigins(t *testing.T) {
	g := guard{origin: "https://example.org"}
	for _, x := range []struct{ method, url string }{{"POST", "https://example.org/api"}, {"GET", "https://elsewhere.org"}} {
		req, _ := http.NewRequest(x.method, x.url, nil)
		if _, e := g.RoundTrip(req); e == nil {
			t.Fatal("request allowed")
		}
	}
}
func TestRedaction(t *testing.T) {
	m := map[string]any{"kind": "ConfigMap", "data": map[string]any{"password": "x"}}
	var fields []string
	redact(m, "", "id", &fields)
	if manifest.Map(m)["data"] != nil || len(fields) != 1 {
		t.Fatal(m)
	}
}
