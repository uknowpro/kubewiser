package analyze

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uknowpro/kubewiser/internal/cluster"
	"github.com/uknowpro/kubewiser/internal/manifest"
	"github.com/uknowpro/kubewiser/internal/render"
)

func TestCRDListSemanticsAndOwnership(t *testing.T) {
	s := decode(t, `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata: {name: widgets.example.org}
spec:
  group: example.org
  names: {kind: Widget, plural: widgets}
  scope: Cluster
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              items:
                type: array
                x-kubernetes-list-type: map
                x-kubernetes-list-map-keys: [id]
                items:
                  type: object
                  properties:
                    id: {type: string}
                    weight: {type: integer}
              tags:
                type: array
                x-kubernetes-list-type: set
                items: {type: string}
---
apiVersion: example.org/v1
kind: Widget
metadata: {name: sample}
spec:
  items: [{id: PRIVATE_KEY_A, weight: 1}, {id: PRIVATE_KEY_B, weight: 2}]
  tags: [a, b]
`)
	live := []manifest.Resource{{Object: JSONClone(s.Resources[0].Object)}, {Object: JSONClone(s.Resources[1].Object)}}
	spec := manifest.Map(live[1].Object["spec"])
	items := manifest.Slice(spec["items"])
	spec["items"] = []any{items[1], items[0]}
	spec["tags"] = []any{"b", "a"}
	snapshot := cluster.Snapshot{Resources: live, Namespace: "unrelated"}
	d, e := Compare(s, snapshot, "")
	if e != nil || len(d.Changes) != 0 {
		t.Fatal(d, e)
	}
	manifest.Map(items[0])["weight"] = float64(3)
	fields := map[string]any{"f:spec": map[string]any{"f:items": map[string]any{`k:{"id":"PRIVATE_KEY_A"}`: map[string]any{"f:weight": map[string]any{}}}}}
	manifest.Map(live[1].Object["metadata"])["managedFields"] = []any{map[string]any{"manager": "controller", "apiVersion": "example.org/v1", "operation": "Update", "fieldsType": "FieldsV1", "fieldsV1": fields}}
	d, e = Compare(s, snapshot, "")
	if e != nil || len(d.Changes) != 1 || len(d.Changes[0].FieldManagers) != 1 {
		t.Fatal(d, e)
	}
	b, _ := json.Marshal(d)
	if strings.Contains(string(b), "PRIVATE_KEY") {
		t.Fatal("map keys leaked")
	}
	spec["items"] = []any{items[0], items[0]}
	d, e = Compare(s, snapshot, "")
	if e != nil || !strings.Contains(strings.Join(d.Unchecked, " "), "Invalid CRD map-list") {
		t.Fatal(d, e)
	}
}
func TestEmptyManagedFieldsNeverOwnEverything(t *testing.T) {
	for _, fields := range []any{nil, map[string]any{}, "invalid"} {
		r := manifest.Resource{Object: map[string]any{"apiVersion": "v1", "metadata": map[string]any{"managedFields": []any{map[string]any{"manager": "bad", "fieldsType": "FieldsV1", "apiVersion": "v1", "fieldsV1": fields}}}}}
		owners, _ := fieldManagers(r, "/spec/image", false)
		if len(owners) != 0 {
			t.Fatal(owners)
		}
	}
}
func projectFixture() ProjectProfile {
	return ProjectProfile{Version: 1, Provider: "aks", Mode: "in-place", From: "1.21", To: "1.35", Changes: []ArchitectureChange{{Area: "cpu", Current: "amd64", Target: "arm64", Reason: "application requirement"}}}
}
func TestProjectGatesAndFirstAffectedVersion(t *testing.T) {
	s := decode(t, "apiVersion: batch/v1beta1\nkind: CronJob\nmetadata: {name: old}\n")
	p, e := Project(context.Background(), s, projectFixture())
	if e != nil {
		t.Fatal(e)
	}
	if p.Status != "review_required" || len(p.Steps) != 19 {
		t.Fatal(p)
	}
	found := map[string]Action{}
	for _, a := range p.Actions {
		found[a.ID] = a
	}
	if found["KW-API-REMOVED"].Before != "1.25" || found["KW-BEHAVIOR-DOCKERSHIM"].Before != "1.24" || found["KW-AKS-ROUTE"].Requirement != "required" || found["KW-ARCH-CPU"].Recommended != "arm64" {
		t.Fatal(found)
	}
	if found["KW-PROJECT-REQUIREMENT"].ID == "" {
		t.Fatal("missing requirements silently accepted")
	}
	p2 := projectFixture()
	p2.To = "1.20"
	if _, e = Project(context.Background(), s, p2); e == nil {
		t.Fatal("downgrade accepted")
	}
}
func TestProfileStrictAndGeneration(t *testing.T) {
	root := t.TempDir()
	profile := projectFixture()
	b, _ := json.Marshal(profile)
	file := filepath.Join(root, "project.json")
	os.WriteFile(file, b, 0600)
	if _, e := ReadProfile(file); e != nil {
		t.Fatal(e)
	}
	os.WriteFile(file, append(b, []byte(" {}")...), 0600)
	if _, e := ReadProfile(file); e == nil {
		t.Fatal("trailing object accepted")
	}
	os.WriteFile(file, []byte(`{"schema_version":1,"provider":"aks","mode":"in-place","typo":true}`), 0600)
	if _, e := ReadProfile(file); e == nil {
		t.Fatal("unknown field accepted")
	}
	output := filepath.Join(root, "drafts")
	names, e := Generate(profile, output)
	if e != nil || len(names) != 8 {
		t.Fatal(names, e)
	}
	if _, e = Generate(profile, output); e == nil {
		t.Fatal("existing directory overwritten")
	}
	rr, e := render.Render(context.Background(), render.Options{Chart: filepath.Join(output, "chart"), Release: "review", Namespace: "test", Target: "1.35"})
	if e != nil || len(rr.Set.Resources) != 0 {
		t.Fatal("disabled chart", rr, e)
	}
	values := filepath.Join(root, "values.yaml")
	os.WriteFile(values, []byte("enabled: true\nimage: example.invalid/canary:test\n"), 0600)
	rr, e = render.Render(context.Background(), render.Options{Chart: filepath.Join(output, "chart"), Values: values, Release: "review", Namespace: "test", Target: "1.35"})
	if e != nil || len(rr.Set.Resources) != 1 {
		t.Fatal(rr, e)
	}
	spec := manifest.Map(manifest.At(rr.Set.Resources[0].Object, "spec", "template", "spec"))
	if spec["automountServiceAccountToken"] != false {
		t.Fatal("token mounted")
	}
}

const pathPods = `apiVersion: v1
kind: Pod
metadata: {name: client, namespace: app, labels: {role: client}}
spec: {containers: [{name: app, image: example:test}]}
---
apiVersion: v1
kind: Pod
metadata: {name: server, namespace: app, labels: {role: server}}
spec:
  containers:
  - name: app
    image: example:test
    ports: [{name: web, containerPort: 8080}]
`

func TestNetworkPathUnionIntersectionNamedPorts(t *testing.T) {
	policy := `---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: server, namespace: app}
spec:
  podSelector: {matchLabels: {role: server}}
  policyTypes: [Ingress]
  ingress:
  - from:
    - podSelector: {matchLabels: {role: client}}
    ports: [{port: web}]
`
	for _, tc := range []struct {
		port int
		want string
	}{{8080, "allowed_by_policy"}, {8081, "denied_by_policy"}} {
		r, e := NetworkPath(context.Background(), decode(t, pathPods+policy), "app/client", "app/server", "TCP", tc.port)
		if e != nil || r.Result != tc.want {
			t.Fatal(r, e)
		}
	}
	deny := `---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: client, namespace: app}
spec:
  podSelector: {matchLabels: {role: client}}
  policyTypes: [Egress]
  egress: []
`
	r, e := NetworkPath(context.Background(), decode(t, pathPods+policy+deny), "app/client", "app/server", "TCP", 8080)
	if e != nil || r.Result != "denied_by_policy" || r.Ingress != "allowed_by_policy" {
		t.Fatal(r, e)
	}
	allow := strings.Replace(strings.Replace(deny, "egress: []", "egress: [{}]", 1), "name: client", "name: client-allow", 1)
	r, e = NetworkPath(context.Background(), decode(t, pathPods+policy+deny+allow), "app/client", "app/server", "TCP", 8080)
	if e != nil || r.Result != "allowed_by_policy" {
		t.Fatal(r, e)
	}
}
func TestNetworkPathMissingNamespaceEvidence(t *testing.T) {
	policy := `---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: server, namespace: app}
spec:
  podSelector: {matchLabels: {role: server}}
  ingress:
  - from:
    - namespaceSelector: {matchLabels: {team: trusted}}
`
	r, e := NetworkPath(context.Background(), decode(t, pathPods+policy), "app/client", "app/server", "TCP", 8080)
	if e != nil || r.Result != "unknown" {
		t.Fatal(r, e)
	}
	ns := "---\napiVersion: v1\nkind: Namespace\nmetadata: {name: app, labels: {team: trusted}}\n"
	r, e = NetworkPath(context.Background(), decode(t, pathPods+policy+ns), "app/client", "app/server", "TCP", 8080)
	if e != nil || r.Result != "allowed_by_policy" {
		t.Fatal(r, e)
	}
}

func TestAddonEvidenceRange(t *testing.T) {
	profile := projectFixture()
	profile.Addons = []AddonEvidence{{Name: "CNI", Version: "example", MinKubernetes: "1.28", MaxKubernetes: "1.35", Source: "https://example.invalid/matrix", Reviewed: "2026-11-16"}}
	p, e := Project(context.Background(), manifest.Set{}, profile)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, a := range p.Actions {
		if a.ID == "KW-ADDON-EVIDENCE" {
			found = true
			if a.Requirement != "required" {
				t.Fatal(a)
			}
		}
	}
	if !found {
		t.Fatal("missing matrix check")
	}
	profile.Addons[0].Source = "file:///etc/passwd"
	if e = ValidateProfile(profile); e == nil {
		t.Fatal("unsafe source accepted")
	}
}

func BenchmarkDriftTenThousand(b *testing.B) {
	set := manifest.Set{}
	snapshot := cluster.Snapshot{}
	for i := 0; i < 10000; i++ {
		r := manifest.Resource{Object: map[string]any{"apiVersion": "v1", "kind": "Service", "metadata": map[string]any{"name": fmt.Sprint("service-", i), "namespace": "bench"}, "spec": map[string]any{"type": "ClusterIP"}}}
		set.Resources = append(set.Resources, r)
		snapshot.Resources = append(snapshot.Resources, manifest.Resource{Object: JSONClone(r.Object)})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d, e := Compare(set, snapshot, "bench")
		if e != nil || len(d.Changes) != 0 {
			b.Fatal(d, e)
		}
	}
}

func TestNetworkPathRangesIPExceptionsAndHostNetwork(t *testing.T) {
	pods := strings.Replace(pathPods, "spec: {containers: [{name: app, image: example:test}]}", "status: {podIP: 10.0.0.10}\nspec: {containers: [{name: app, image: example:test}]}", 1)
	policy := `---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: server, namespace: app}
spec:
  podSelector: {matchLabels: {role: server}}
  ingress:
  - from:
    - ipBlock: {cidr: 10.0.0.0/24, except: [10.0.0.128/25]}
    ports: [{port: 8000, endPort: 8100}]
`
	r, e := NetworkPath(context.Background(), decode(t, pods+policy), "app/client", "app/server", "TCP", 8080)
	if e != nil || r.Result != "allowed_by_policy" {
		t.Fatal(r, e)
	}
	r, e = NetworkPath(context.Background(), decode(t, strings.Replace(pods, "10.0.0.10", "10.0.0.200", 1)+policy), "app/client", "app/server", "TCP", 8080)
	if e != nil || r.Result != "denied_by_policy" {
		t.Fatal(r, e)
	}
	r, e = NetworkPath(context.Background(), decode(t, strings.Replace(pods, "spec: {containers:", "spec: {hostNetwork: true, containers:", 1)+policy), "app/client", "app/server", "TCP", 8080)
	if e != nil || r.Result != "unknown" {
		t.Fatal(r, e)
	}
}
