package analyze

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/uknowpro/kubewiser/internal/manifest"
	"github.com/uknowpro/kubewiser/internal/scan"
)

const cgroupSource = "https://kubernetes.io/docs/concepts/architecture/cgroups/"

type CgroupSide struct {
	Mode           string `json:"mode,omitempty"`
	Scope          string `json:"scope,omitempty"`
	Evidence       string `json:"evidence,omitempty"`
	RootFS         string `json:"root_fs_type,omitempty"`
	MountInfo      string `json:"mountinfo,omitempty"`
	Kernel         string `json:"kernel,omitempty"`
	Runtime        string `json:"runtime,omitempty"`
	RuntimeVersion string `json:"runtime_version,omitempty"`
	KubeletDriver  string `json:"kubelet_driver,omitempty"`
	RuntimeDriver  string `json:"runtime_driver,omitempty"`
}
type CgroupPool struct {
	Name    string     `json:"name"`
	Current CgroupSide `json:"current"`
	Target  CgroupSide `json:"target"`
}
type CgroupComparison struct {
	CurrentConfig   CgroupSide `json:"current_config"`
	TargetConfig    CgroupSide `json:"target_config"`
	Pool            string     `json:"pool"`
	Current         string     `json:"current"`
	Target          string     `json:"target"`
	Status          string     `json:"status"`
	CurrentEvidence string     `json:"current_evidence"`
	TargetEvidence  string     `json:"target_evidence"`
	Checks          []string   `json:"checks"`
}
type CgroupReport struct {
	Reviewed string             `json:"reviewed"`
	Pools    []CgroupComparison `json:"pools"`
}

func ReadCgroups(path string) ([]CgroupPool, error) {
	b, e := manifest.ReadBounded(path, 1<<20)
	if e != nil {
		return nil, e
	}
	var pools []CgroupPool
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e = d.Decode(&pools); e != nil {
		return nil, fmt.Errorf("invalid cgroup evidence JSON")
	}
	if d.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("cgroup evidence requires one JSON array")
	}
	return pools, ValidateCgroups(pools)
}
func ValidateCgroups(pools []CgroupPool) error {
	if len(pools) > 100 {
		return fmt.Errorf("at most 100 cgroup pools allowed")
	}
	seen := map[string]bool{}
	for _, p := range pools {
		if strings.TrimSpace(p.Name) == "" || seen[p.Name] {
			return fmt.Errorf("cgroup pool names must be nonempty and unique")
		}
		seen[p.Name] = true
		for _, s := range []CgroupSide{p.Current, p.Target} {
			if s.Mode != "" && s.Mode != "unknown" && s.Mode != "v1" && s.Mode != "v2" && s.Mode != "hybrid" {
				return fmt.Errorf("cgroup mode must be v1, v2, hybrid or unknown")
			}
			if s.Scope != "" && s.Scope != "node" && s.Scope != "container" {
				return fmt.Errorf("cgroup scope must be node or container")
			}
			if len(s.MountInfo) > 256<<10 {
				return fmt.Errorf("mountinfo evidence too large")
			}
			if s.Mode != "" && s.Mode != "unknown" && strings.TrimSpace(s.Evidence) == "" {
				return fmt.Errorf("declared cgroup mode requires evidence description")
			}
			for _, driver := range []string{s.KubeletDriver, s.RuntimeDriver} {
				if driver != "" && driver != "unknown" && driver != "systemd" && driver != "cgroupfs" {
					return fmt.Errorf("invalid cgroup driver")
				}
			}
		}
	}
	return nil
}

// Evidence is supplied, never collected by executing node commands. Container
// mount namespaces and tmpfs alone cannot establish a node's active hierarchy.
func cgroupMode(s CgroupSide) (string, string, error) {
	inferred := "unknown"
	note := "No usable node-scope evidence"
	if s.Scope == "node" {
		v1, v2 := false, false
		if s.MountInfo != "" {
			for _, line := range strings.Split(strings.TrimSpace(s.MountInfo), "\n") {
				a, z, ok := strings.Cut(line, " - ")
				pre, post := strings.Fields(a), strings.Fields(z)
				if !ok || len(pre) < 6 || len(post) < 3 {
					return "", "", fmt.Errorf("invalid node mountinfo record")
				}
				path := pre[4]
				if path != "/sys/fs/cgroup" && !strings.HasPrefix(path, "/sys/fs/cgroup/") {
					continue
				}
				if post[0] == "cgroup" {
					v1 = true
				}
				if post[0] == "cgroup2" {
					v2 = true
				}
			}
			switch {
			case v1 && v2:
				inferred = "hybrid"
			case v1:
				inferred = "v1"
			case v2:
				inferred = "v2"
			}
			hash := sha256.Sum256([]byte(s.MountInfo))
			note = fmt.Sprintf("Supplied node mountinfo sha256:%x; node identity/freshness not independently verified", hash)
		}
		if strings.TrimSpace(s.RootFS) == "cgroup2fs" {
			if inferred == "v1" {
				return "conflict", "Node root filesystem and mountinfo disagree", nil
			}
			if inferred == "unknown" {
				inferred = "v2"
				note = "Supplied node /sys/fs/cgroup stat result cgroup2fs; not independently verified"
			}
		}
		if strings.TrimSpace(s.RootFS) == "tmpfs" && inferred == "v2" {
			return "conflict", "Node root filesystem and mountinfo disagree", nil
		}
	}
	if s.Mode != "" && s.Mode != "unknown" {
		if inferred != "unknown" && inferred != s.Mode {
			return "conflict", "Declared mode conflicts with supplied node evidence", nil
		}
		if inferred == "unknown" {
			return s.Mode, "Operator declaration: " + s.Evidence + "; not independently verified", nil
		}
	}
	return inferred, note, nil
}
func versionAtLeast(value string, major, minor int) (bool, bool) {
	m := regexp.MustCompile(`^v?([0-9]+)\.([0-9]+)(?:\.[0-9]+)?(?:[-+].*)?$`).FindStringSubmatch(value)
	if m == nil {
		return false, false
	}
	a, e := strconv.Atoi(m[1])
	b, e2 := strconv.Atoi(m[2])
	if e != nil || e2 != nil {
		return false, false
	}
	return a > major || (a == major && b >= minor), true
}
func Cgroups(set manifest.Set, pools []CgroupPool, target string) (Plan, error) {
	p := Plan{Kind: "cgroup", Status: "partial", From: "current node pools", To: "target node pools", Actions: []Action{}, Steps: []string{}, Unchecked: []string{"Cgroup results compare supplied evidence/declarations, not live nodes. OS or Kubernetes versions alone never establish the active cgroup mode.", "Equal modes or minimum prerequisites do not certify application compatibility. Test memory limits/OOM, CPU throttling, agents and application runtime versions on target nodes.", "Workload hints are not mapped to individual pools; scheduling and node-pool membership require validation. Redacted snapshots can hide commands, environment values and host paths."}, Cgroup: &CgroupReport{Reviewed: "2026-11-17", Pools: []CgroupComparison{}}}
	if len(set.Issues) > 0 {
		return p, fmt.Errorf("cgroup inputs contain parse errors")
	}
	if e := ValidateCgroups(pools); e != nil {
		return p, e
	}
	minor := 0
	if target != "" {
		var e error
		minor, e = scan.Target(target)
		if e != nil {
			return p, e
		}
	}
	add := func(rule, res, req, current, recommended, message string) {
		p.Actions = append(p.Actions, Action{ID: rule, Resource: res, Requirement: req, Current: current, Recommended: recommended, Message: message, Verification: "Verify supplied node evidence and test CPU/memory enforcement, OOM behavior and monitoring on a canary; no commands are executed", Source: cgroupSource})
	}
	targetV2 := false
	for _, pool := range pools {
		current, ce, e := cgroupMode(pool.Current)
		if e != nil {
			return p, e
		}
		next, te, e := cgroupMode(pool.Target)
		if e != nil {
			return p, e
		}
		row := CgroupComparison{Pool: pool.Name, Current: current, Target: next, CurrentEvidence: ce, TargetEvidence: te, Checks: []string{}}
		row.CurrentConfig, row.TargetConfig = pool.Current, pool.Target
		row.CurrentConfig.MountInfo, row.TargetConfig.MountInfo = "", ""
		req := "conditional"
		switch {
		case current == "conflict" || next == "conflict":
			row.Status = "conflicting_evidence"
			req = "required"
		case current == "unknown" || next == "unknown":
			row.Status = "unknown"
		case current == "hybrid" || next == "hybrid":
			row.Status = "hybrid_review"
			req = "required"
		case current == next:
			row.Status = "same_mode"
		default:
			row.Status = "mode_change"
			req = "required"
		}
		add("KW-CGROUP-MODE", pool.Name, req, current, next, "Cgroup comparison: "+row.Status+"; same_mode is not an application compatibility verdict")
		check := func(status, msg string) {
			row.Checks = append(row.Checks, status+": "+msg)
			if status != "evidence_meets_minimum" {
				r := "conditional"
				if status == "required" {
					r = "required"
				}
				add("KW-CGROUP-PREREQUISITE", pool.Name, r, next, "Validated target prerequisites", msg)
			}
		}
		if next == "unknown" && minor >= 35 {
			check("unknown", "Kubernetes 1.35+ refuses cgroup v1 by default; establish target mode and effective failCgroupV1/provider policy")
		}
		if next == "v1" && minor >= 35 {
			check("required", "Kubernetes 1.35+ defaults to refusing cgroup v1; verify effective failCgroupV1 and provider policy. Plan v2; a temporary override is not a compatibility guarantee")
		}
		if pool.Target.KubeletDriver != "" && pool.Target.KubeletDriver != "unknown" && pool.Target.RuntimeDriver != "" && pool.Target.RuntimeDriver != "unknown" && pool.Target.KubeletDriver != pool.Target.RuntimeDriver {
			check("required", "Effective kubelet/runtime cgroup drivers must agree")
		}
		if next == "v2" {
			targetV2 = true
			if ok, known := versionAtLeast(pool.Target.Kernel, 5, 8); !known {
				check("unknown", "Target kernel version missing or unrecognized")
			} else if !ok {
				check("required", "Target kernel below the documented cgroup v2 baseline 5.8; validate vendor support/backports")
			} else {
				check("evidence_meets_minimum", "Target kernel meets generic 5.8 baseline; not a vendor support guarantee")
			}
			major, min := 0, 0
			switch pool.Target.Runtime {
			case "containerd":
				major, min = 1, 4
			case "cri-o":
				major, min = 1, 20
			}
			if major == 0 {
				check("unknown", "Target runtime support not established; recognized baselines are containerd and cri-o")
			} else if ok, known := versionAtLeast(pool.Target.RuntimeVersion, major, min); !known {
				check("unknown", "Target runtime version missing or unrecognized")
			} else if !ok {
				check("required", "Target runtime below documented cgroup v2 baseline")
			} else {
				check("evidence_meets_minimum", "Runtime meets generic v2 baseline; exact Kubernetes/provider compatibility remains unchecked")
			}
			kd, rd := pool.Target.KubeletDriver, pool.Target.RuntimeDriver
			if kd == "cgroupfs" || rd == "cgroupfs" {
				check("required", "Target v2 requires effective systemd drivers for kubelet and runtime; declared drivers are incompatible")
			} else if kd != "systemd" || rd != "systemd" {
				check("unknown", "Effective target kubelet/runtime drivers not both supplied as systemd")
			} else {
				check("evidence_meets_minimum", "Declared kubelet and runtime drivers both systemd")
			}
			if current != "v2" {
				check("required", "v2 changes hierarchy and resource files (memory.limit_in_bytes to memory.max, cpu.cfs_* to cpu.max); migrate direct readers and validate JVM, Node.js, Go automaxprocs and monitoring/security agents where used; their versions are not inferred from image names")
			}
		}
		p.Cgroup.Pools = append(p.Cgroup.Pools, row)
	}
	if len(pools) == 0 {
		add("KW-CGROUP-MODE", "node pools", "conditional", "unknown", "Provide current and target cgroup evidence", "No node-pool cgroup comparison evidence supplied")
		if minor >= 35 {
			add("KW-CGROUP-PREREQUISITE", "node pools", "conditional", "Target mode unknown", "Confirm v2 or explicitly validated provider exception", "Kubernetes 1.35+ refuses cgroup v1 by default; verify effective failCgroupV1 without inferring actual node mode")
		}
	}
	for _, r := range set.Resources {
		spec, _ := scan.PodSpec(r)
		if spec == nil {
			continue
		}
		v1ref, host := false, false
		for _, v := range manifest.Slice(spec["volumes"]) {
			path := manifest.String(manifest.Map(manifest.Map(v)["hostPath"]), "path")
			if path == "/" || path == "/sys" || path == "/sys/fs" || path == "/sys/fs/cgroup" || strings.HasPrefix(path, "/sys/fs/cgroup/") {
				host = true
			}
		}
		for _, field := range []string{"containers", "initContainers", "ephemeralContainers"} {
			for _, c := range manifest.Slice(spec[field]) {
				m := manifest.Map(c)
				values := []string{}
				for _, f := range []string{"command", "args"} {
					for _, v := range manifest.Slice(m[f]) {
						if s, ok := v.(string); ok {
							values = append(values, s)
						}
					}
				}
				for _, v := range manifest.Slice(m["env"]) {
					values = append(values, manifest.String(manifest.Map(v), "value"))
				}
				for _, s := range values {
					for _, token := range []string{"memory.limit_in_bytes", "memory.usage_in_bytes", "cpu.cfs_quota_us", "cpu.cfs_period_us", "cpu.shares"} {
						if strings.Contains(s, token) {
							v1ref = true
						}
					}
				}
			}
		}
		if v1ref {
			req := "conditional"
			if targetV2 {
				req = "required"
			}
			add("KW-CGROUP-V1-REFERENCE", r.ID(), req, "Configuration references cgroup v1-specific resource files (raw values omitted)", "Review and migrate readers for the selected target hierarchy", "Static text reference, not proof of executed code; verify workload scheduling and actual access")
		}
		if host {
			add("KW-CGROUP-HOST-MOUNT", r.ID(), "conditional", "Host mount can expose cgroup files (path omitted)", "Validate monitoring/security agents and direct cgroup access on target nodes", "A mount is an access capability, not proof that the application reads cgroups")
		}
	}
	return p, nil
}
func AttachCgroups(p *Plan, set manifest.Set, pools []CgroupPool, target string) error {
	r, e := Cgroups(set, pools, target)
	if e != nil {
		return e
	}
	p.Cgroup = r.Cgroup
	filtered := p.Actions[:0]
	for _, a := range p.Actions {
		if !strings.HasPrefix(a.ID, "KW-CGROUP-") {
			filtered = append(filtered, a)
		}
	}
	p.Actions = append(filtered, r.Actions...)
	for _, u := range r.Unchecked {
		found := false
		for _, old := range p.Unchecked {
			if old == u {
				found = true
				break
			}
		}
		if !found {
			p.Unchecked = append(p.Unchecked, u)
		}
	}
	return nil
}
