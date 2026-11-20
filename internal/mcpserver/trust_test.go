package mcpserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/uknowpro/kubewiser/internal/cluster"
)

func trustClient(t *testing.T, c Config) (context.Context, *mcp.ClientSession) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	a, b := mcp.NewInMemoryTransports()
	ss, err := NewConfigured(c).Connect(ctx, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "trust-test", Version: "1"}, nil).Connect(ctx, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return ctx, cs
}
func trustWrite(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
func trustConfig(t *testing.T) Config {
	return Config{Input: "../../docs/examples/desired.yaml", Snapshot: "../../docs/examples/snapshot.json", Chart: "../../docs/examples/chart", Namespace: "team", Profile: trustWrite(t, filepath.Join(t.TempDir(), "profile.json"), `{"schema_version":1,"provider":"aks","mode":"in-place","from_version":"1.21","to_version":"1.36"}`)}
}

var trustCases = []struct {
	name string
	args map[string]any
}{
	{"scan_files", map[string]any{"target_version": "1.36"}},
	{"scan_cgroup", map[string]any{}},
	{"scan_lifecycle", map[string]any{}},
	{"plan_upgrade", map[string]any{"from_version": "1.21", "target_version": "1.36"}},
	{"plan_migrate", map[string]any{"from_provider": "eks", "to_provider": "aks"}},
	{"scan_network", map[string]any{}},
	{"scan_cel", map[string]any{}},
	{"plan_aks", map[string]any{}},
	{"compare_drift", map[string]any{}},
	{"scan_helm", map[string]any{"target_version": "1.36"}},
}

func TestAllOfflineToolsThroughMCP(t *testing.T) {
	c := trustConfig(t)
	ctx, cs := trustClient(t, c)
	listed, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != len(trustCases) {
		t.Fatalf("every tool needs a named call test: %d tools, %d cases", len(listed.Tools), len(trustCases))
	}
	called := map[string]bool{}
	for _, tc := range trustCases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
			if err != nil || r.IsError || r.StructuredContent == nil {
				t.Fatalf("%s: %#v %v", tc.name, r, err)
			}
			called[tc.name] = true
		})
	}
	for _, tool := range listed.Tools {
		if !called[tool.Name] {
			t.Errorf("untested tool %s", tool.Name)
		}
	}
	// Optional capabilities cannot be requested by the model if the operator did not configure them.
	ctx2, limited := trustClient(t, Config{Input: c.Input})
	for _, name := range []string{"plan_aks", "scan_helm", "compare_drift", "collect_cluster", "generate"} {
		r, e := limited.CallTool(ctx2, &mcp.CallToolParams{Name: name, Arguments: map[string]any{}})
		if e == nil && (r == nil || !r.IsError) {
			t.Fatalf("unconfigured tool accepted: %s", name)
		}
	}
}
func TestAllOfflineToolFailures(t *testing.T) {
	c := Config{Input: "missing-manifest", Snapshot: "missing-snapshot", Chart: "missing-chart", Profile: "missing-profile"}
	ctx, cs := trustClient(t, c)
	for _, tc := range trustCases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
			if err == nil && (r == nil || !r.IsError) {
				t.Fatalf("missing input accepted: %s", tc.name)
			}
		})
	}
}
func TestToolsListExplicitBooleanHints(t *testing.T) {
	c := trustConfig(t)
	c.Cluster = &cluster.Options{}
	ctx, cs := trustClient(t, c)
	r, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Tools) != 11 {
		t.Fatal(len(r.Tools))
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Tools []struct {
			Name        string         `json:"name"`
			Annotations map[string]any `json:"annotations"`
		}
	}
	if err = json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	for _, tool := range wire.Tools {
		expected := map[string]bool{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": tool.Name == "collect_cluster"}
		for key, want := range expected {
			got, ok := tool.Annotations[key].(bool)
			if !ok || got != want {
				t.Errorf("%s %s: %#v want %v", tool.Name, key, tool.Annotations[key], want)
			}
		}
	}
}
func TestModelCannotOverrideOperatorScope(t *testing.T) {
	c := trustConfig(t)
	ctx, cs := trustClient(t, c)
	for _, tc := range trustCases {
		t.Run(tc.name, func(t *testing.T) {
			args := map[string]any{}
			for k, v := range tc.args {
				args[k] = v
			}
			args["input"] = "/not-authorized"
			args["kubeconfig"] = "/not-authorized"
			args["allow_cluster"] = true
			r, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tc.name, Arguments: args})
			if err == nil && (r == nil || !r.IsError) {
				t.Fatal("unexpected scope arguments accepted")
			}
		})
	}
}
func TestCollectClusterThroughMCP(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "GET" {
			t.Error("write request")
		}
		if r.URL.Path != "/version" && !strings.Contains(r.URL.Path, "/namespaces/team/") {
			t.Error("outside namespace", r.URL.Path)
		}
		if strings.Contains(r.URL.Path, "secrets") {
			t.Error("secret endpoint")
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/version" {
			fmt.Fprint(w, `{"gitVersion":"v1.36.0"}`)
		} else if strings.HasSuffix(r.URL.Path, "/pods") {
			fmt.Fprint(w, `{"items":[{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p","namespace":"team","annotations":{"token":"PRIVATE_SENTINEL"}},"spec":{"containers":[{"name":"c","image":"example.invalid/test","args":["PRIVATE_SENTINEL"],"env":[{"name":"TOKEN","value":"PRIVATE_SENTINEL"}]}]}}]}`)
		} else {
			fmt.Fprint(w, `{"items":[]}`)
		}
	}))
	defer server.Close()
	ca := base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}))
	config := trustWrite(t, filepath.Join(t.TempDir(), "kubeconfig"), fmt.Sprintf("apiVersion: v1\nkind: Config\nclusters:\n- name: fixture\n  cluster:\n    server: %s\n    certificate-authority-data: %s\ncontexts:\n- name: fixture\n  context: {cluster: fixture, user: fixture}\nusers:\n- name: fixture\n  user: {}\n", server.URL, ca))
	c := Config{Cluster: &cluster.Options{Kubeconfig: config, Context: "fixture", Namespace: "team", QPS: 10, MaxItems: 100, Timeout: 10 * time.Second}}
	ctx, cs := trustClient(t, c)
	r, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "collect_cluster", Arguments: map[string]any{}})
	if err != nil || r.IsError || calls.Load() == 0 {
		t.Fatalf("%#v %v", r, err)
	}
	data, _ := json.Marshal(r)
	if strings.Contains(string(data), "PRIVATE_SENTINEL") || !strings.Contains(string(data), "kubewiser.snapshot.v1") {
		t.Fatal("redaction or snapshot failed")
	}
	before := calls.Load()
	r, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "collect_cluster", Arguments: map[string]any{"namespace": "other"}})
	if err == nil && (r == nil || !r.IsError) {
		t.Fatal("scope override accepted")
	}
	if calls.Load() != before {
		t.Fatal("rejected arguments made requests")
	}
}
func TestCollectClusterRejectsExecAndInvalidConfig(t *testing.T) {
	for _, options := range []cluster.Options{{}, {AllowExecAuth: "/must-not-run"}} {
		ctx, cs := trustClient(t, Config{Cluster: &options})
		r, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "collect_cluster", Arguments: map[string]any{}})
		if err == nil && (r == nil || !r.IsError) {
			t.Fatal("invalid/exec config accepted")
		}
		if options.AllowExecAuth != "" {
			data, _ := json.Marshal(r)
			if !strings.Contains(string(data), "does not execute authentication plugins") {
				t.Fatalf("wrong rejection: %s %v", data, err)
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := RunConfigured(ctx, Config{Cluster: &cluster.Options{AllowExecAuth: "/must-not-run"}}); err == nil || !strings.Contains(err.Error(), "does not execute authentication plugins") {
		t.Fatal("stdio allowed exec auth or wrong rejection", err)
	}
}
