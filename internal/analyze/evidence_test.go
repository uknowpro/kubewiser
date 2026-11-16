package analyze

import (
	"context"
	"encoding/json"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/uknowpro/kubewiser/internal/cluster"
	"github.com/uknowpro/kubewiser/internal/manifest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAKSExportEvidence(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	p := projectFixture()
	p.From = "1.34"
	p.To = "1.35"
	p.ClusterID = "/subscriptions/example/resourceGroups/test/providers/Microsoft.ContainerService/managedClusters/cluster"
	raw := `{"id":"` + p.ClusterID + `/upgradeProfiles/default","controlPlaneProfile":{"kubernetesVersion":"1.34.2","upgrades":[{"kubernetesVersion":"1.35.10"},{"kubernetesVersion":"1.35.2"},{"kubernetesVersion":"1.36.0","isPreview":true}]}}`
	p.UpgradeEvidence = &AKSEvidence{CapturedAt: now.Format(time.RFC3339), Profile: json.RawMessage(raw)}
	r, e := ReviewAKSRoute(p, now)
	if e != nil || r.Status != "next_step_offered" || r.Next != "1.35.2" || len(r.Offered) != 2 {
		t.Fatal(r, e)
	}
	p.ClusterID += "wrong"
	r, e = ReviewAKSRoute(p, now)
	if e != nil || r.Status != "cluster_mismatch" {
		t.Fatal(r, e)
	}
	p.ClusterID = strings.TrimSuffix(p.ClusterID, "wrong")
	p.From = "1.34.1"
	r, e = ReviewAKSRoute(p, now)
	if e != nil || r.Status != "version_mismatch" {
		t.Fatal(r, e)
	}
	p.From = "1.34"
	r, e = ReviewAKSRoute(p, now.Add(25*time.Hour))
	if e != nil || r.Status != "stale" {
		t.Fatal(r, e)
	}
}

const celFixture = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata: {name: counters.example.org}
spec:
  group: example.org
  names: {kind: Counter, plural: counters}
  scope: Namespaced
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
              count:
                type: integer
                x-kubernetes-validations:
                - rule: self >= 0
                - rule: self >= oldSelf
---
apiVersion: example.org/v1
kind: Counter
metadata: {name: count, namespace: test}
spec: {count: 2}
`

func TestCELCreateAndUpdateRules(t *testing.T) {
	set := decode(t, celFixture)
	p, e := CELReview(context.Background(), set, nil)
	if e != nil || len(p.Actions) != 0 {
		t.Fatal(p, e)
	}
	if !strings.Contains(strings.Join(p.Unchecked, " "), "No matching old object") {
		t.Fatal("transition coverage omitted")
	}
	old := manifest.Resource{Object: JSONClone(set.Resources[1].Object)}
	manifest.Map(old.Object["spec"])["count"] = float64(3)
	p, e = CELReview(context.Background(), set, []manifest.Resource{old})
	if e != nil || len(p.Actions) != 1 {
		t.Fatal(p, e)
	}
	bad := decode(t, strings.Replace(celFixture, "count: 2", "count: -1", 1))
	p, e = CELReview(context.Background(), bad, nil)
	if e != nil || len(p.Actions) != 1 {
		t.Fatal(p, e)
	}
	invalid := decode(t, strings.Replace(celFixture, "self >= 0", "self >= INVALID", 1))
	p, e = CELReview(context.Background(), invalid, nil)
	if e != nil || len(p.Actions) == 0 {
		t.Fatal(p, e)
	}
}

func TestLocalCRDDefaultsAndNoneConversion(t *testing.T) {
	set := decode(t, celFixture)
	crdSpec := manifest.Map(set.Resources[0].Object["spec"])
	versions := manifest.Slice(crdSpec["versions"])
	v2 := JSONClone(manifest.Map(versions[0]))
	v2["name"] = "v2"
	crdSpec["versions"] = append(versions, v2)
	count := manifest.Map(manifest.At(manifest.Map(versions[0]), "schema", "openAPIV3Schema", "properties", "spec", "properties", "count"))
	count["default"] = float64(2)
	desired := set.Resources[1]
	delete(manifest.Map(desired.Object["spec"]), "count")
	observed := manifest.Resource{Object: JSONClone(desired.Object)}
	observed.Object["apiVersion"] = "example.org/v2"
	manifest.Map(observed.Object["spec"])["count"] = float64(2)
	snap := cluster.Snapshot{Resources: []manifest.Resource{set.Resources[0], observed}}
	d, e := CompareWithOptions(set, snap, "test", DiffOptions{CRDDefaults: true, CRDConversion: true})
	if e != nil || len(d.Changes) != 0 {
		t.Fatal(d, e)
	}
	if desired.Object["apiVersion"] != "example.org/v1" || manifest.Map(desired.Object["spec"])["count"] != nil {
		t.Fatal("input mutated")
	}
	crdSpec["conversion"] = map[string]any{"strategy": "Webhook"}
	d, e = CompareWithOptions(set, snap, "test", DiffOptions{CRDConversion: true})
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, c := range d.Changes {
		if c.Type == "conversion_unverified" {
			found = true
		}
	}
	if !found {
		t.Fatal("webhook conversion inferred")
	}
}
func TestServiceBackendMapping(t *testing.T) {
	service := `---
apiVersion: v1
kind: Service
metadata: {name: api, namespace: app}
spec:
  selector: {role: server}
  ports: [{port: 80, targetPort: web}]
`
	result, e := NetworkServicePath(context.Background(), decode(t, pathPods+service), "app/client", "app/api", "TCP", 80)
	if e != nil || len(result.Paths) != 1 || result.Paths[0].Port != 8080 || result.Paths[0].Destination != "app/server" {
		t.Fatal(result, e)
	}
}
func TestAddonCatalogAndOCIPlatform(t *testing.T) {
	set := decode(t, strings.Replace(pathPods, "example:test", "quay.io/jetstack/cert-manager-controller:v1.21.2", 1))
	actions := addonActions(set, 35, 37)
	if len(actions) != 1 || actions[0].Requirement != "required" {
		t.Fatal(actions)
	}
	p := projectFixture()
	p.TargetPlatform = &Platform{OS: "linux", Architecture: "arm64"}
	p.Images = []ImageEvidence{{Image: "quay.io/jetstack/cert-manager-controller:v1.21.2", Index: json.RawMessage(`{"schemaVersion":2,"manifests":[{"platform":{"os":"linux","architecture":"amd64"}}]}`)}}
	actions, _, e := runtimeActions(set, p)
	if e != nil {
		t.Fatal(e)
	}
	if len(actions) != 2 || actions[0].Requirement != "required" {
		t.Fatal(actions)
	}
}
func TestConfiguredTemplatesValidated(t *testing.T) {
	p := projectFixture()
	p.ClusterID = "/subscriptions/example/resourceGroups/test/providers/Microsoft.ContainerService/managedClusters/example"
	p.Templates = &TemplateConfig{NodePool: &NodePoolTemplate{Name: "canary", VMSize: "Standard_D4ps_v5", Count: 1, OSSKU: "Ubuntu", KubernetesVersion: "1.35.1", ProviderVersion: "5.7.0"}, Canary: &CanaryTemplate{Image: "example.invalid/canary:review", Namespace: "review", Replicas: 1, CPU: "100m", Memory: "128Mi", NodeSelector: map[string]string{"kubernetes.io/arch": "arm64"}}}
	root := t.TempDir()
	path := filepath.Join(root, "generated")
	files, e := Generate(p, path)
	if e != nil || len(files) != 9 {
		t.Fatal(files, e)
	}
	data, _ := os.ReadFile(filepath.Join(path, "nodepool.auto.tfvars.json"))
	var values map[string]any
	if e = json.Unmarshal(data, &values); e != nil || values["enable_canary_pool"] != false {
		t.Fatal(values, e)
	}
	manifestBytes, _ := os.ReadFile(filepath.Join(path, "manifest.yaml"))
	if !strings.Contains(string(manifestBytes), `"replicas": 0`) || !strings.Contains(string(manifestBytes), "arm64") {
		t.Fatal(string(manifestBytes))
	}
	p.Templates.Canary.NodeSelector["invalid key!"] = "value"
	badPath := filepath.Join(root, "invalid")
	if _, e = Generate(p, badPath); e == nil {
		t.Fatal("invalid scheduling label passed generated validation")
	}
	if _, e = os.Stat(badPath); !os.IsNotExist(e) {
		t.Fatal("wrote files before validation")
	}
}

func TestGeneratedTerraformSyntax(t *testing.T) {
	p := projectFixture()
	root := filepath.Join(t.TempDir(), "drafts")
	if _, e := Generate(p, root); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(filepath.Join(root, "nodepool.tf"))
	if e != nil {
		t.Fatal(e)
	}
	if _, diags := hclsyntax.ParseConfig(b, "nodepool.tf", hcl.InitialPos); diags.HasErrors() {
		t.Fatal(diags.Error())
	}
}
func TestExactPortFlowEvidence(t *testing.T) {
	p := PathReview{Source: "a/client", Destination: "a/api", Protocol: "TCP", Port: 443}
	AttachFlowEvidence(&p, []Flow{{Source: "a/client", Destination: "a/api", Protocol: "TCP", DestinationPort: 80}, {Source: "a/client", Destination: "a/api", Protocol: "TCP", DestinationPort: 443, Verdict: "FORWARDED"}})
	if len(p.Flows) != 1 || p.Flows[0].DestinationPort != 443 {
		t.Fatal(p)
	}
}
