package analyze

import (
	"encoding/json"
	"github.com/uknowpro/kubewiser/internal/manifest"
	"strings"
	"testing"
	"time"
)

func lifecycleTime(s string) time.Time { t, _ := time.Parse("2006-01-02", s); return t }
func TestLifecycleCatalogDatesAndSources(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range LifecycleCatalog() {
		if seen[r.ID] || !strings.HasPrefix(r.Source, "https://") || r.Replacement == "" {
			t.Fatal(r)
		}
		seen[r.ID] = true
		layout := "2006-01-02"
		if r.Precision == "month" {
			layout = "2006-01"
		}
		if _, err := time.Parse(layout, r.End); err != nil {
			t.Fatal(err)
		}
	}
	var r LifecycleRule
	for _, x := range LifecycleCatalog() {
		if x.ID == "aks-app-routing-nginx" {
			r = x
		}
	}
	for date, want := range map[string]string{"2026-10-31": "scheduled", "2026-11-01": "ending_this_month", "2026-11-30": "ending_this_month", "2026-12-01": "ended"} {
		if got := lifecycleStatus(r, lifecycleTime(date)); got != want {
			t.Fatal(date, got)
		}
	}
}
func TestLifecycleIdentificationAndManagedScope(t *testing.T) {
	set := decode(t, `apiVersion: v1
kind: Pod
metadata: {name: controller, namespace: ingress}
spec:
  containers:
  - {name: mirror, image: 'private.example/ingress-nginx/controller:test'}
  - {name: upstream, image: 'k8s.gcr.io/ingress-nginx/controller@sha256:abc'}
`)
	now := lifecycleTime("2026-11-17")
	p, e := Lifecycle(set, nil, now)
	if e != nil || len(p.Lifecycle.Findings) != 1 || p.Lifecycle.Findings[0].Status != "ended" {
		t.Fatal(p, e)
	}
	profile := projectFixture()
	profile.LifecycleComponents = []LifecycleComponent{{ID: "aks-app-routing-nginx", Resource: set.Resources[0].ID()}}
	p, e = Lifecycle(set, &profile, now)
	if e != nil || len(p.Lifecycle.Findings) != 1 || p.Lifecycle.Findings[0].Status != "ending_this_month" || p.Actions[0].Requirement != "conditional" {
		t.Fatal(p, e)
	}
	for image, want := range map[string]string{"private.example/ingress-nginx/controller:v1": "identification_required", "registry.k8s.io/ingress-nginx/controller:v1": "ended", "quay.io/kubernetes-ingress-controller/nginx-ingress-controller:0.30": "ended", "nginx/nginx-ingress:5.0": ""} {
		c := manifest.Map(manifest.Slice(manifest.At(set.Resources[0].Object, "spec", "containers"))[0])
		c["image"] = image
		manifest.Map(set.Resources[0].Object["spec"])["containers"] = []any{c}
		p, e = Lifecycle(set, nil, now)
		if e != nil {
			t.Fatal(e)
		}
		if want == "" {
			if len(p.Actions) != 0 {
				t.Fatal("F5 false positive", p)
			}
		} else if len(p.Lifecycle.Findings) != 1 || p.Lifecycle.Findings[0].Status != want {
			t.Fatal(image, p)
		}
	}
}
func TestLifecycleOSNeedsAKSAndRejectsInvalidInventory(t *testing.T) {
	set := decode(t, `apiVersion: v1
kind: Node
metadata: {name: worker}
status:
 nodeInfo: {osImage: 'Ubuntu 20.04.6 LTS'}
`)
	now := lifecycleTime("2026-11-17")
	p, _ := Lifecycle(set, nil, now)
	if len(p.Actions) != 0 {
		t.Fatal("inferred provider")
	}
	profile := projectFixture()
	p, e := Lifecycle(set, &profile, now)
	if e != nil || len(p.Actions) != 1 || p.Lifecycle.Findings[0].Status != "scheduled" {
		t.Fatal(p, e)
	}
	profile.LifecycleComponents = []LifecycleComponent{{ID: "imaginary", Resource: "worker"}}
	if _, e := Lifecycle(set, &profile, now); e == nil {
		t.Fatal("unknown component accepted")
	}
	if e := ValidateProfile(profile); e == nil {
		t.Fatal("invalid inventory accepted by profile")
	}
	profile.LifecycleComponents = []LifecycleComponent{{ID: "aks-kubenet", Resource: "network"}}
	p, e = Lifecycle(set, &profile, now)
	if e != nil || len(p.Lifecycle.Findings) != 2 {
		t.Fatal(p, e)
	}
	b, _ := json.Marshal(p)
	if !strings.Contains(string(b), `"end":"2028-03-31"`) {
		t.Fatal(string(b))
	}
}
