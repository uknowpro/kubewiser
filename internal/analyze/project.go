package analyze

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/uknowpro/kubewiser/internal/manifest"
	"github.com/uknowpro/kubewiser/internal/scan"
	"io"
	"net/url"
	"strings"
	"time"
)

const aksUpgradeSource = "https://learn.microsoft.com/en-us/azure/aks/upgrade-aks-control-plane"

type ArchitectureChange struct {
	Area    string `json:"area"`
	Current string `json:"current"`
	Target  string `json:"target"`
	Reason  string `json:"reason"`
}

// AddonEvidence is a local operator-supplied vendor-matrix excerpt, never fetched at runtime.
type AddonEvidence struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	MinKubernetes string `json:"min_kubernetes"`
	MaxKubernetes string `json:"max_kubernetes"`
	Source        string `json:"source"`
	Reviewed      string `json:"reviewed"`
}
type ProjectProfile struct {
	LifecycleComponents []LifecycleComponent `json:"lifecycle_components,omitempty"`
	TargetPlatform      *Platform            `json:"target_platform,omitempty"`
	Images              []ImageEvidence      `json:"image_indexes,omitempty"`
	Templates           *TemplateConfig      `json:"templates,omitempty"`
	ClusterID           string               `json:"cluster_id,omitempty"`
	UpgradeEvidence     *AKSEvidence         `json:"upgrade_evidence,omitempty"`
	Addons              []AddonEvidence      `json:"addons,omitempty"`
	Version             int                  `json:"schema_version"`
	Provider            string               `json:"provider"`
	Mode                string               `json:"mode"`
	From                string               `json:"from_version"`
	To                  string               `json:"to_version"`
	MaintenanceWindow   string               `json:"maintenance_window"`
	MaxDowntime         string               `json:"max_downtime"`
	RPO                 string               `json:"rpo"`
	RTO                 string               `json:"rto"`
	Changes             []ArchitectureChange `json:"architecture_changes,omitempty"`
}

func ReadProfile(path string) (ProjectProfile, error) {
	var p ProjectProfile
	b, e := manifest.ReadBounded(path, 1<<20)
	if e != nil {
		return p, e
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e = d.Decode(&p); e != nil {
		return p, fmt.Errorf("invalid project profile: %w", e)
	}
	if e = d.Decode(new(any)); e != io.EOF {
		return p, fmt.Errorf("profile must contain one JSON object")
	}
	return p, ValidateProfile(p)
}
func ValidateProfile(p ProjectProfile) error {
	if err := validateLifecycleComponents(p.LifecycleComponents); err != nil {
		return err
	}
	if p.Version != 1 || p.Provider != "aks" || p.Mode != "in-place" {
		return fmt.Errorf("profile requires schema_version 1, provider aks, mode in-place")
	}
	if len(p.Addons) > 100 {
		return fmt.Errorf("at most 100 add-on entries allowed")
	}
	for _, a := range p.Addons {
		lo, e := scan.Target(a.MinKubernetes)
		if e != nil {
			return fmt.Errorf("invalid add-on minimum: %w", e)
		}
		hi, e := scan.Target(a.MaxKubernetes)
		if e != nil || hi < lo {
			return fmt.Errorf("invalid add-on support range")
		}
		u, e := url.Parse(a.Source)
		if e != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || a.Name == "" || a.Version == "" || a.Reviewed == "" {
			return fmt.Errorf("add-on evidence requires name, version, source URL and reviewed date")
		}
	}
	if len(p.Changes) > 50 {
		return fmt.Errorf("at most 50 architecture changes allowed")
	}
	seen := map[string]bool{}
	for _, c := range p.Changes {
		switch c.Area {
		case "network", "nodepool", "cpu", "os", "ingress", "storage", "identity":
		default:
			return fmt.Errorf("unsupported architecture area %q", c.Area)
		}
		if seen[c.Area] {
			return fmt.Errorf("duplicate architecture area %q", c.Area)
		}
		seen[c.Area] = true
		if strings.TrimSpace(c.Current) == "" || strings.TrimSpace(c.Target) == "" || strings.TrimSpace(c.Reason) == "" {
			return fmt.Errorf("architecture changes require current, target and reason")
		}
	}
	return nil
}

func Project(ctx context.Context, set manifest.Set, profile ProjectProfile) (Plan, error) {
	if e := ValidateProfile(profile); e != nil {
		return Plan{}, e
	}
	p, e := Upgrade(ctx, set, profile.From, profile.To)
	if e != nil {
		return p, e
	}
	life, lifeErr := Lifecycle(set, &profile, time.Now().UTC())
	if lifeErr != nil {
		return p, lifeErr
	}
	p.Lifecycle = life.Lifecycle
	filtered := p.Actions[:0]
	for _, action := range p.Actions {
		if action.ID != "KW-LIFECYCLE" {
			filtered = append(filtered, action)
		}
	}
	p.Actions = append(filtered, life.Actions...)
	runtimeChecks, runtimeGaps, err := runtimeActions(set, profile)
	if err != nil {
		return p, err
	}
	p.Actions = append(p.Actions, runtimeChecks...)
	p.Unchecked = append(p.Unchecked, runtimeGaps...)
	p.Kind = "aks-project"
	p.Project = &profile
	p.Status = "review_required"
	if profile.UpgradeEvidence != nil {
		route, err := ReviewAKSRoute(profile, time.Now().UTC())
		if err != nil {
			return p, err
		}
		p.Route = &route
		req := "required"
		if route.Status == "next_step_offered" {
			req = "conditional"
		}
		p.Actions = append(p.Actions, Action{ID: "KW-AKS-EVIDENCE", Requirement: req, Current: route.Current, Recommended: route.Next, Message: "Saved AKS upgrade profile: " + route.Status, Verification: route.Note, Source: aksUpgradeSource})
	}
	p.Unchecked = append(p.Unchecked, "Profile values are operator declarations, not live observations. A minor-version sequence is NOT a verified AKS upgrade route.", "Provider availability, support policy, quota, regional capacity and architecture transitions require fresh cluster-specific evidence. No Azure API is contacted.")
	p.Actions = append([]Action{{ID: "KW-AKS-ROUTE", Requirement: "required", Message: "Confirm the next offered upgrade for this exact AKS cluster; do not execute the entire mathematical minor sequence", Verification: "Review a fresh az aks get-upgrades result and AKS support guidance before each step. If the required path is unavailable, stop and resolve with the provider or assess a replacement cluster", Source: aksUpgradeSource, Before: profile.From}, {ID: "KW-AKS-RECOVERY", Requirement: "required", Message: "Prepare and rehearse recovery before the first control-plane change; do not assume Kubernetes downgrade is a rollback", Verification: "Record backup restore evidence, application rollback limits, recovery owner and measured RPO/RTO", Source: "https://learn.microsoft.com/en-us/azure/backup/azure-kubernetes-service-backup-overview", Before: profile.From}, {ID: "KW-AKS-DRAIN", Requirement: "required", Message: "Validate PDBs, replicas, topology, drain time, surge IP addresses and compute quota before rotating nodes", Verification: "Drain a canary node in staging and measure service errors, latency and storage attachment; stop if the agreed budget is exceeded", Source: "https://learn.microsoft.com/en-us/azure/aks/upgrade-options"}}, p.Actions...)
	for _, x := range []struct{ name, value string }{{"maintenance_window", profile.MaintenanceWindow}, {"max_downtime", profile.MaxDowntime}, {"rpo", profile.RPO}, {"rto", profile.RTO}} {
		if strings.TrimSpace(x.value) == "" || strings.EqualFold(x.value, "unknown") {
			p.Actions = append(p.Actions, Action{ID: "KW-PROJECT-REQUIREMENT", Resource: x.name, Requirement: "required", Message: "Project acceptance requirement is not defined", Verification: "Agree and record a measurable value with the service owner before execution", Source: aksUpgradeSource})
		}
	}
	if len(profile.Addons) == 0 {
		p.Unchecked = append(p.Unchecked, "No local add-on compatibility evidence supplied")
	}
	lo, _ := scan.Target(profile.From)
	hi, _ := scan.Target(profile.To)
	for _, a := range profile.Addons {
		amin, _ := scan.Target(a.MinKubernetes)
		amax, _ := scan.Target(a.MaxKubernetes)
		msg := "Operator-supplied compatibility range covers the requested minor sequence; verify patch-specific requirements and staging results"
		req := "conditional"
		if amin > lo || amax < hi {
			req = "required"
			msg = "One add-on version does not cover the complete upgrade range; schedule compatible add-on versions at the affected steps"
		}
		p.Actions = append(p.Actions, Action{ID: "KW-ADDON-EVIDENCE", Resource: a.Name, Requirement: req, Current: a.Version + " (declared)", Recommended: a.MinKubernetes + " through " + a.MaxKubernetes + " (supplied support range)", Message: msg, Verification: "Recheck vendor evidence dated " + a.Reviewed + "; test before each step. This range is user-provided, not independently verified", Source: a.Source})
	}
	guidance := map[string]string{
		"network":  "Verify the exact source/target network plugin, policy engine, pod/service CIDRs, subnet capacity, DNS and firewall paths. Confirm whether this transition is supported in-place; stop if it requires replacement.",
		"nodepool": "Plan a separate canary node pool, labels/taints, topology, quota and controlled drain. Preserve the previous pool until acceptance where supported.",
		"cpu":      "Verify multi-architecture images and native libraries for every workload and daemon. Test scheduling on a matching canary pool before draining old nodes.",
		"os":       "Test the target OS image, kernel, cgroup, CRI, filesystem and privileged agents on a canary pool; verify provider OS/version support.",
		"ingress":  "Verify controller/chart compatibility, ingress classes, TLS, health probes, client IP behavior and DNS. Test representative traffic before changing routes.",
		"storage":  "Verify CSI driver, storage classes, zones, access modes, reclaim policy and restore tests. Configuration changes do not move volume data.",
		"identity": "Verify OIDC issuer, workload federation, service accounts, SDK token handling and least-privilege access using an isolated canary workload.",
	}
	for _, c := range profile.Changes {
		if c.Current == c.Target {
			continue
		}
		p.Actions = append(p.Actions, Action{ID: "KW-ARCH-" + strings.ToUpper(c.Area), Resource: c.Area, Requirement: "conditional", Message: c.Reason, Current: c.Current, Recommended: c.Target, Verification: guidance[c.Area], Source: "https://learn.microsoft.com/en-us/azure/aks/what-is-aks"})
	}
	p.Steps = append([]string{"Gate 1: inventory and approve requirements, upgrade route, add-on compatibility and recovery evidence", "Gate 2: rehearse version and architecture changes separately in staging; capture baseline and acceptance metrics"}, p.Steps...)
	p.Steps = append(p.Steps, "Gate 3: perform only the next provider-supported control-plane step, then compatible canary nodes; validate before continuing", "Gate 4: stop on PDB/drain failure, unhealthy workloads, data integrity failure or exceeded error/downtime budget; use the rehearsed recovery runbook", "Gate 5: complete the observation window and owner sign-off before retiring old pools or backups")
	return p, nil
}
