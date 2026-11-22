package analyze

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uknowpro/kubewiser/internal/manifest"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func readinessSet(t *testing.T, body string) manifest.Set {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input.yaml")
	if e := os.WriteFile(path, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
	set, e := manifest.Load(context.Background(), path)
	if e != nil || len(set.Issues) > 0 {
		t.Fatal(e, set.Issues)
	}
	return set
}

const readinessFixture = `apiVersion: v1
kind: Node
metadata:
  name: n1
  labels: {agentpool: pool1, disk: ssd}
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: app, namespace: team}
spec:
  replicas: 1
  selector: {matchLabels: {app: web}}
  template:
    metadata: {labels: {app: web}}
    spec:
      nodeSelector: {disk: ssd}
      containers:
      - name: web
        image: example.invalid/web
        args: [memory.limit_in_bytes]
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata: {name: budget, namespace: team}
spec:
  maxUnavailable: 0
  selector: {matchLabels: {app: web}}
`

func TestReadinessPoolAndDrain(t *testing.T) {
	set := readinessSet(t, readinessFixture)
	p, e := Upgrade(context.Background(), set, "1.21", "1.36")
	if e != nil {
		t.Fatal(e)
	}
	pools := []CgroupPool{{Name: "pool1", Current: CgroupSide{Mode: "v1", Evidence: "test"}, Target: CgroupSide{Mode: "v2", Evidence: "test"}}}
	if e = AttachCgroups(&p, set, pools, "1.36"); e != nil {
		t.Fatal(e)
	}
	BuildMatrix(&p)
	if len(p.Workloads) != 1 || p.Workloads[0].Status != "action_required" || !strings.Contains(p.Workloads[0].Current, "candidate") {
		t.Fatal(p.Workloads)
	}
	found := false
	for _, r := range p.Drain {
		if strings.Contains(r.Resource, "PodDisruptionBudget") && r.Status == "action_required" {
			found = true
		}
	}
	if !found {
		t.Fatal(p.Drain)
	}
	p2, e := Upgrade(context.Background(), readinessSet(t, strings.ReplaceAll(readinessFixture, "namespace: team", "namespace: other")), "1.21", "1.36")
	if e != nil {
		t.Fatal(e)
	}
	if len(p2.Drain) == 0 {
		t.Fatal("missing risks")
	}
}
func TestNodeHardConstraints(t *testing.T) {
	node := core.Node{ObjectMeta: meta.ObjectMeta{Name: "n", Labels: map[string]string{"disk": "ssd", "generation": "3"}}}
	spec := core.PodSpec{NodeSelector: map[string]string{"disk": "ssd"}}
	if nodeCandidate(spec, node) != yes {
		t.Fatal("selector")
	}
	spec.NodeSelector["disk"] = "hdd"
	if nodeCandidate(spec, node) != no {
		t.Fatal("selector mismatch")
	}
	spec.NodeSelector = nil
	spec.Affinity = &core.Affinity{NodeAffinity: &core.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &core.NodeSelector{NodeSelectorTerms: []core.NodeSelectorTerm{{MatchExpressions: []core.NodeSelectorRequirement{{Key: "disk", Operator: core.NodeSelectorOpIn, Values: []string{"hdd"}}}}, {MatchExpressions: []core.NodeSelectorRequirement{{Key: "generation", Operator: core.NodeSelectorOpGt, Values: []string{"2"}}}}}}}}
	if nodeCandidate(spec, node) != yes {
		t.Fatal("OR required terms")
	}
	node.Spec.Taints = []core.Taint{{Key: "dedicated", Value: "x", Effect: core.TaintEffectNoSchedule}}
	if nodeCandidate(spec, node) != no {
		t.Fatal("taint ignored")
	}
	spec.Tolerations = []core.Toleration{{Key: "dedicated", Value: "x"}}
	if nodeCandidate(spec, node) != yes {
		t.Fatal("default Equal")
	}
	node.Spec.Unschedulable = true
	if nodeCandidate(spec, node) != no {
		t.Fatal("cordon ignored")
	}
}
func TestPDBSemanticsAndScope(t *testing.T) {
	for _, tc := range []struct {
		name, replace, with string
		required            bool
	}{
		{"zero", "maxUnavailable: 0", "maxUnavailable: 0", true},
		{"hundred", "maxUnavailable: 0", "minAvailable: 100%", true},
		{"round-up", "maxUnavailable: 0", "maxUnavailable: 30%", false},
		{"different-label", "selector: {matchLabels: {app: web}}", "selector: {matchLabels: {app: other}}", false},
		{"beta-empty", "apiVersion: policy/v1", "apiVersion: policy/v1beta1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := strings.ReplaceAll(readinessFixture, tc.replace, tc.with)
			if tc.name == "beta-empty" {
				body = strings.Replace(body, "  selector: {matchLabels: {app: web}}\n", "  selector: {}\n", 1)
				body = strings.ReplaceAll(body, "  selector: {matchLabels: {app: web}}", "  selector: {}")
			}
			rows := drainMatrix(readinessSet(t, body), "default")
			found := false
			for _, r := range rows {
				if strings.Contains(r.Resource, "PodDisruptionBudget") {
					found = true
					if (r.Status == "action_required") != tc.required {
						t.Fatal(r)
					}
					if tc.name == "round-up" && !strings.Contains(r.Evidence, "allows 1 disruptions") {
						t.Fatal(r)
					}
				}
			}
			if !found {
				t.Fatal("no PDB")
			}
		})
	}
	body := strings.Replace(readinessFixture, "metadata: {name: budget, namespace: team}", "metadata: {name: budget, namespace: other}", 1)
	for _, r := range drainMatrix(readinessSet(t, body), "default") {
		if strings.Contains(r.Resource, "PodDisruptionBudget") && r.Status == "action_required" {
			t.Fatal("cross namespace match")
		}
	}
}
func TestObservedBindingAndAmbiguousPools(t *testing.T) {
	set := readinessSet(t, `apiVersion: v1
kind: Node
metadata: {name: n, labels: {disk: ssd}}
spec: {unschedulable: true}
---
apiVersion: v1
kind: Pod
metadata: {name: p}
spec:
  nodeName: n
  containers: [{name: app, image: example.invalid/a}]
`)
	pools := []CgroupPool{{Name: "a", NodeSelector: map[string]string{"disk": "ssd"}}, {Name: "b", NodeSelector: map[string]string{"disk": "ssd"}}}
	rows := workloadMatrix(set, pools, nil, nil)
	if len(rows) != 1 || !strings.Contains(rows[0].Current, "supplied nodeName binding: ambiguous pool") || rows[0].Status != "input_missing" {
		t.Fatal(rows)
	}
}
func TestFindingProgressAndRename(t *testing.T) {
	before := readinessSet(t, readinessFixture)
	after := readinessSet(t, strings.ReplaceAll(strings.ReplaceAll(readinessFixture, "replicas: 1", "replicas: 2"), "args: [memory.limit_in_bytes]", "args: [memory.max]"))
	rows, e := trackFindings(context.Background(), before, after, "1.36", "default")
	if e != nil {
		t.Fatal(e)
	}
	resolved := map[string]bool{}
	for _, r := range rows {
		if r.Status == "resolved" {
			resolved[r.Rule] = true
		}
	}
	if !resolved["KW-CGROUP-V1-REFERENCE"] || !resolved["KW-DRAIN-SINGLE-REPLICA"] {
		t.Fatal(rows)
	}
	renamed := readinessSet(t, strings.ReplaceAll(readinessFixture, "name: app,", "name: renamed,"))
	rows, e = trackFindings(context.Background(), before, renamed, "1.36", "default")
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range rows {
		if strings.Contains(r.Resource, "Deployment") && r.Status != "cannot_compare" {
			t.Fatal("rename marked resolved", r)
		}
	}
}
func TestFindingStableNamedLists(t *testing.T) {
	before := readinessSet(t, `apiVersion: v1
kind: Pod
metadata: {name: app}
spec:
  containers:
  - {name: a, image: example.invalid/a}
  - {name: b, image: example.invalid/b}
`)
	after := readinessSet(t, `apiVersion: v1
kind: Pod
metadata: {name: app}
spec:
  containers:
  - {name: b, image: example.invalid/b}
  - {name: a, image: example.invalid/a}
`)
	rows, e := trackFindings(context.Background(), before, after, "1.36", "default")
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range rows {
		if r.Rule == "KW-REQUESTS" && r.Status != "remaining" {
			t.Fatal(r)
		}
	}
}

func TestPDBStatusGenerationAndNumericBudget(t *testing.T) {
	for _, tc := range []struct {
		generation string
		required   bool
	}{{"1", false}, {"2", true}} {
		body := strings.Replace(readinessFixture, "maxUnavailable: 0", "maxUnavailable: 1", 1)
		body = strings.Replace(body, "metadata: {name: budget, namespace: team}", "metadata: {name: budget, namespace: team, generation: 2}", 1)
		// observed generation 2 is current; generation 1 is stale.
		body += "status: {observedGeneration: " + tc.generation + ", disruptionsAllowed: 0}\n"
		for _, r := range drainMatrix(readinessSet(t, body), "default") {
			if strings.Contains(r.Resource, "PodDisruptionBudget") && (r.Status == "action_required") != tc.required {
				t.Fatal("wrong generation handling", r)
			}
		}
	}
	body := strings.Replace(readinessFixture, "maxUnavailable: 0", "minAvailable: 1", 1)
	found := false
	for _, r := range drainMatrix(readinessSet(t, body), "default") {
		if strings.Contains(r.Resource, "PodDisruptionBudget") {
			found = strings.Contains(r.Evidence, "allows 0 disruptions")
		}
	}
	if !found {
		t.Fatal("numeric budget not calculated")
	}
}
func TestProgressIntroducedAndInvalidCounterpart(t *testing.T) {
	plain := `apiVersion: v1
kind: Pod
metadata: {name: app}
spec:
  containers:
  - name: app
    image: example.invalid/a
    resources: {requests: {cpu: 100m}}
`
	before := readinessSet(t, plain)
	after := readinessSet(t, strings.Replace(plain, "    resources: {requests: {cpu: 100m}}\n", "", 1))
	rows, e := trackFindings(context.Background(), before, after, "1.36", "default")
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, r := range rows {
		if r.Rule == "KW-REQUESTS" {
			found = true
			if r.Status != "introduced" {
				t.Fatal(r)
			}
		}
	}
	if !found {
		t.Fatal("missing new finding")
	}
	bad := readinessSet(t, strings.Replace(plain, "image: example.invalid/a", "image: 123", 1))
	rows, e = trackFindings(context.Background(), after, bad, "1.36", "default")
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range rows {
		if r.Rule == "KW-REQUESTS" && r.Status != "cannot_compare" {
			t.Fatal("invalid counterpart considered resolved", r)
		}
	}
}

func TestPDBFindingProgress(t *testing.T) {
	before := readinessSet(t, readinessFixture)
	after := readinessSet(t, strings.Replace(readinessFixture, "maxUnavailable: 0", "maxUnavailable: 1", 1))
	rows, e := trackFindings(context.Background(), before, after, "1.36", "default")
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, r := range rows {
		if r.Rule == "KW-DRAIN-STATIC-BUDGET" {
			found = true
			if r.Status != "resolved" {
				t.Fatal(r)
			}
		}
	}
	if !found {
		t.Fatal("no PDB progress")
	}
	after = readinessSet(t, strings.ReplaceAll(readinessFixture, "selector: {matchLabels: {app: web}}", "selector: {matchLabels: {app: other}}"))
	rows, e = trackFindings(context.Background(), before, after, "1.36", "default")
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range rows {
		if r.Rule == "KW-DRAIN-STATIC-BUDGET" && r.Status != "cannot_compare" {
			t.Fatal("changed selector marked resolved", r)
		}
	}
}
