package analyze

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/uknowpro/kubewiser/internal/manifest"
	"github.com/uknowpro/kubewiser/internal/scan"
)

//go:embed lifecycle.json
var lifecycleData []byte

const LifecycleReviewed = "2026-11-17"

type LifecycleRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Scope       string `json:"scope"`
	End         string `json:"end"`
	Precision   string `json:"precision"`
	Replacement string `json:"replacement"`
	Source      string `json:"source"`
}
type LifecycleComponent struct {
	ID       string `json:"id"`
	Resource string `json:"resource"`
}
type LifecycleFinding struct {
	Component   string `json:"component"`
	Resource    string `json:"resource"`
	Status      string `json:"status"`
	End         string `json:"end"`
	Precision   string `json:"precision"`
	Scope       string `json:"scope"`
	Evidence    string `json:"evidence"`
	Replacement string `json:"replacement"`
	Source      string `json:"source"`
}
type LifecycleReport struct {
	Evaluated string             `json:"evaluated_at"`
	Reviewed  string             `json:"catalog_reviewed"`
	Findings  []LifecycleFinding `json:"findings"`
}

func LifecycleCatalog() []LifecycleRule {
	var rules []LifecycleRule
	_ = json.Unmarshal(lifecycleData, &rules)
	return rules
}
func validateLifecycleComponents(cs []LifecycleComponent) error {
	if len(cs) > 1000 {
		return fmt.Errorf("lifecycle inventory exceeds 1000 entries")
	}
	known := map[string]bool{}
	for _, r := range LifecycleCatalog() {
		known[r.ID] = true
	}
	seen := map[string]bool{}
	for _, c := range cs {
		if !known[c.ID] || strings.TrimSpace(c.Resource) == "" {
			return fmt.Errorf("lifecycle component requires a catalog id and resource")
		}
		key := c.ID + "\x00" + c.Resource
		if seen[key] {
			return fmt.Errorf("duplicate lifecycle component")
		}
		seen[key] = true
	}
	return nil
}
func lifecycleStatus(r LifecycleRule, now time.Time) string {
	today := now.UTC().Format("2006-01-02")
	if r.Precision == "month" {
		month := today[:7]
		if month > r.End {
			return "ended"
		}
		if month == r.End {
			return "ending_this_month"
		}
		return "scheduled"
	}
	if today >= r.End {
		return "ended"
	}
	return "scheduled"
}
func Lifecycle(set manifest.Set, profile *ProjectProfile, now time.Time) (Plan, error) {
	p := Plan{Kind: "lifecycle", Status: "partial", From: "local inputs", To: "support lifecycle", Actions: []Action{}, Steps: []string{}, Unchecked: []string{"Selected, pinned lifecycle catalog only; absence of findings is not proof of support. No lifecycle databases or vendors are contacted.", "Upstream retirement does not establish managed/vendor support coverage. Names, labels and mirror paths are hints only. Operator declarations are not independently verified.", "Missing Node OS, controller resources, redacted annotations and snapshot scope may hide dependencies. Kubernetes API removal is evaluated separately by version scans and upgrade plans."}}
	p.Lifecycle = &LifecycleReport{Evaluated: now.UTC().Format("2006-01-02"), Reviewed: LifecycleReviewed, Findings: []LifecycleFinding{}}
	if len(set.Issues) > 0 {
		return p, fmt.Errorf("lifecycle input contains parse errors")
	}
	rules := map[string]LifecycleRule{}
	for _, r := range LifecycleCatalog() {
		rules[r.ID] = r
	}
	seen := map[string]bool{}
	overrides := map[string]string{}
	if profile != nil {
		for _, c := range profile.LifecycleComponents {
			if c.ID == "aks-app-routing-nginx" {
				overrides["ingress-nginx\x00"+c.Resource] = c.ID
			}
			if c.ID == "aks-pod-identity" {
				overrides["aad-pod-identity\x00"+c.Resource] = c.ID
			}
		}
	}
	add := func(id, resource, evidence string, uncertain bool) {
		key := id + "\x00" + resource
		if overrides[key] != "" {
			return
		}
		if seen[key] {
			return
		}
		seen[key] = true
		r := rules[id]
		status := lifecycleStatus(r, now)
		req := "conditional"
		if uncertain {
			status = "identification_required"
		} else if status == "ended" {
			req = "required"
		}
		f := LifecycleFinding{Component: r.Name, Resource: resource, Status: status, End: r.End, Precision: r.Precision, Scope: r.Scope, Evidence: evidence, Replacement: r.Replacement, Source: r.Source}
		p.Lifecycle.Findings = append(p.Lifecycle.Findings, f)
		p.Actions = append(p.Actions, Action{ID: "KW-LIFECYCLE", Resource: resource, Requirement: req, Current: r.Name + " · " + status + " · " + r.End, Recommended: r.Replacement, Message: r.Scope + "; " + evidence, Verification: "Confirm component identity, support owner and current vendor policy; test replacement before cutover", Source: r.Source})
	}
	if profile != nil {
		if err := validateLifecycleComponents(profile.LifecycleComponents); err != nil {
			return p, err
		}
		for _, c := range profile.LifecycleComponents {
			add(c.ID, c.Resource, "Operator-declared component and support scope", false)
		}
	}
	for _, r := range set.Resources {
		id := r.ID()
		kind := manifest.String(r.Object, "kind")
		if profile != nil && profile.Provider == "aks" && kind == "Node" {
			os := strings.ToLower(manifest.String(manifest.Map(manifest.At(r.Object, "status", "nodeInfo")), "osImage"))
			if os == "ubuntu 20.04" || strings.HasPrefix(os, "ubuntu 20.04.") || strings.HasPrefix(os, "ubuntu 20.04 ") {
				add("aks-ubuntu2004", id, "Node status.osImage in declared AKS project", false)
			}
			if os == "azure linux 2.0" || strings.HasPrefix(os, "azure linux 2.0.") || os == "cbl-mariner/linux" {
				add("aks-azurelinux2", id, "Node OS signature; confirm exact OS version", os == "cbl-mariner/linux")
			}
		}
		api := manifest.String(r.Object, "apiVersion")
		if strings.HasPrefix(api, "aadpodidentity.k8s.io/") {
			add("aad-pod-identity", id, "AAD Pod Identity custom resource; upstream scope only", false)
		}
		spec, _ := scan.PodSpec(r)
		if spec == nil {
			continue
		}
		foundIngress := false
		mirrorHint := false
		for _, field := range []string{"containers", "initContainers", "ephemeralContainers"} {
			for _, c := range manifest.Slice(spec[field]) {
				image := manifest.String(manifest.Map(c), "image")
				repo := strings.SplitN(image, "@", 2)[0]
				if i := strings.LastIndex(repo, ":"); i > strings.LastIndex(repo, "/") {
					repo = repo[:i]
				}
				switch repo {
				case "registry.k8s.io/ingress-nginx/controller", "k8s.gcr.io/ingress-nginx/controller", "quay.io/kubernetes-ingress-controller/nginx-ingress-controller", "gcr.io/google_containers/nginx-ingress-controller":
					add("ingress-nginx", id, "Known upstream controller image repository; managed coverage unverified", false)
					foundIngress = true
				case "mcr.microsoft.com/oss/azure/aad-pod-identity/nmi", "mcr.microsoft.com/oss/azure/aad-pod-identity/mic":
					add("aad-pod-identity", id, "Known AAD Pod Identity image repository; upstream scope only", false)
				default:
					if strings.Contains(repo, "/ingress-nginx/") {
						mirrorHint = true
					}
				}
			}
		}
		if mirrorHint && !foundIngress {
			add("ingress-nginx", id, "Mirror-like image path only; identify actual product and support owner", true)
			foundIngress = true
		}
		labels := manifest.Map(manifest.At(r.Object, "metadata", "labels"))
		if !foundIngress && manifest.String(labels, "app.kubernetes.io/name") == "ingress-nginx" {
			add("ingress-nginx", id, "Controller label only; not proof of upstream identity", true)
		}
	}
	return p, nil
}
