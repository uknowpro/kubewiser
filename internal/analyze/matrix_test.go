package analyze

import (
	"context"
	"encoding/json"
	"github.com/uknowpro/kubewiser/internal/manifest"
	"strings"
	"testing"
)

func TestMatrixMissingEvidenceAndTargetValidation(t *testing.T) {
	ctx := context.Background()
	before, err := manifest.Load(ctx, "../../docs/examples/cgroup/workload.yaml")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Upgrade(ctx, before, "1.21", "1.36")
	if err != nil {
		t.Fatal(err)
	}
	for _, topic := range []string{"cgroup", "kernel", "runtime", "drivers"} {
		found := false
		for _, r := range p.Matrix {
			if r.Topic == topic {
				found = true
				if r.Status != "input_missing" {
					t.Fatal(r)
				}
			}
		}
		if !found {
			t.Fatal(topic)
		}
	}
	after, err := manifest.Load(ctx, "../../docs/examples/cgroup/workload.yaml")
	if err != nil {
		t.Fatal(err)
	}
	after.Resources[0].Object["spec"].(map[string]any)["replicas"] = "invalid-private-value"
	if err = CompareManifests(ctx, &p, before, after, "default"); err != nil {
		t.Fatal(err)
	}
	diff, invalid := false, false
	for _, r := range p.Matrix {
		if r.Topic == "manifest-field" {
			diff = true
		}
		if r.Topic == "target-validation" && r.Status == "action_required" {
			invalid = true
		}
	}
	if !diff || !invalid || p.AfterReport == nil {
		t.Fatal(p.Matrix)
	}
	data, _ := json.Marshal(p.Matrix)
	if strings.Contains(string(data), "invalid-private-value") {
		t.Fatal("raw value leaked")
	}
}
func TestMatrixCgroupValuesAndPrivacy(t *testing.T) {
	pools, err := ReadCgroups("../../docs/examples/cgroup/pools.json")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Cgroups(manifest.Set{}, pools, "1.36")
	if err != nil {
		t.Fatal(err)
	}
	BuildMatrix(&p)
	found := false
	for _, r := range p.Matrix {
		if r.Topic == "kernel" {
			found = true
			if r.Current != "unknown" || r.Target != "6.8.0-azure" || r.Status != "baseline_met" {
				t.Fatal(r)
			}
		}
	}
	if !found {
		t.Fatal("missing kernel")
	}
	data, _ := json.Marshal(p)
	if strings.Contains(string(data), pools[0].Current.MountInfo) {
		t.Fatal("mountinfo leaked")
	}
}
