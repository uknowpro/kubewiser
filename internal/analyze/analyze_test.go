package analyze

import (
	"context"
	"encoding/json"
	"github.com/uknowpro/kubewiser/internal/cluster"
	"github.com/uknowpro/kubewiser/internal/manifest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func decode(t *testing.T, text string) manifest.Set {
	t.Helper()
	s, e := manifest.Decode(context.Background(), []byte(text), "input")
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestDriftListsDefaultsAndSecrets(t *testing.T) {
	s := decode(t, `apiVersion: v1
kind: Pod
metadata: {name: test, namespace: ns}
spec:
  containers:
  - name: a
    image: example:a
    env: [{name: TOKEN, value: TOPSECRET}]
  - name: b
    image: example:b
`)
	live := manifest.Resource{Object: JSONClone(s.Resources[0].Object)}
	spec := manifest.Map(live.Object["spec"])
	a := spec["containers"].([]any)
	env := manifest.Map(a[0])["env"].([]any)
	delete(manifest.Map(env[0]), "value")
	spec["containers"] = []any{a[1], a[0]}
	live.Object["status"] = map[string]any{"phase": "Running"}
	manifest.Map(live.Object["metadata"])["uid"] = "ignored"
	snapshot := cluster.Snapshot{Resources: []manifest.Resource{live}, Redacted: []string{live.ID() + "#/spec/containers/0/env/0/value"}}
	d, e := Compare(s, snapshot, "ns")
	if e != nil || len(d.Changes) != 0 {
		t.Fatal(d, e)
	}
	manifest.Map(a[0])["image"] = "new"
	d, e = Compare(s, snapshot, "ns")
	if e != nil || len(d.Changes) != 1 {
		t.Fatal(d, e)
	}
	b, _ := json.Marshal(d)
	if strings.Contains(string(b), "TOPSECRET") || strings.Contains(string(b), "example:a") {
		t.Fatal("raw values leaked")
	}
}
func TestUpgradeAndMigration(t *testing.T) {
	s := decode(t, "apiVersion: batch/v1beta1\nkind: CronJob\nmetadata: {name: old}\n")
	p, e := Upgrade(context.Background(), s, "1.24", "1.26")
	if e != nil || len(p.Steps) != 2 || len(p.Actions) < 2 {
		t.Fatal(p, e)
	}
	if _, e = Upgrade(context.Background(), s, "1.26", "1.25"); e == nil {
		t.Fatal("downgrade accepted")
	}
	s = decode(t, "apiVersion: v1\nkind: PersistentVolumeClaim\nmetadata: {name: data}\n")
	p, e = Migration(s, "eks", "aks")
	if e != nil || len(p.Actions) != 1 || p.Actions[0].ID != "KW-MIGRATE-DATA" {
		t.Fatal(p, e)
	}
}
func TestNetworkPoliciesAndFlows(t *testing.T) {
	s := decode(t, `apiVersion: v1
kind: Pod
metadata: {name: app, namespace: ns, labels: {app: test}}
spec: {containers: []}
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: isolated, namespace: ns}
spec:
  podSelector: {matchLabels: {app: test}}
  policyTypes: [Ingress]
  ingress: []
`)
	path := filepath.Join(t.TempDir(), "flow.jsonl")
	os.WriteFile(path, []byte(`{"flow":{"source":{"namespace":"ns","pod_name":"app"},"destination":{"namespace":"ns","pod_name":"db"},"verdict":"FORWARDED","time":"2026-11-15T00:00:00Z"}}`+"\n"), 0600)
	n, e := NetworkScan(context.Background(), s, path, "ns")
	if e != nil || len(n.Isolation) != 1 || !n.Isolation[0].Ingress || n.Isolation[0].Egress || len(n.Flows) != 1 {
		t.Fatal(n, e)
	}
}

func TestFlowTimestampOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flows.jsonl")
	var lines []string
	for _, stamp := range []string{"2026-11-15T09:00:00+09:00", "2026-11-15T00:00:00.5Z", "2026-11-14T23:59:59Z"} {
		lines = append(lines, `{"source":{"pod_name":"a"},"destination":{"pod_name":"b"},"verdict":"FORWARDED","time":"`+stamp+`"}`)
	}
	os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0600)
	n, e := NetworkScan(context.Background(), manifest.Set{}, path, "default")
	if e != nil || len(n.Flows) != 1 || n.Flows[0].First != "2026-11-14T23:59:59Z" || n.Flows[0].Last != "2026-11-15T00:00:00.5Z" {
		t.Fatal(n, e)
	}
}
