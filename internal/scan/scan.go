// Package scan combines offline official schemas, version rules and conditional recommendations.
package scan

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/uknowpro/kubewiser/internal/manifest"
	"github.com/uknowpro/kubewiser/internal/schema"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Finding struct {
	Rule        string `json:"rule_id"`
	File        string `json:"file"`
	Document    int    `json:"document"`
	Line        int    `json:"line"`
	Resource    string `json:"resource"`
	Field       string `json:"field"`
	Requirement string `json:"requirement"`
	Severity    string `json:"severity"`
	Evidence    string `json:"evidence"`
	Message     string `json:"message"`
	Source      string `json:"source,omitempty"`
}
type Input = manifest.Input
type Report struct {
	SchemaVersion string        `json:"schema_version"`
	RulesVersion  string        `json:"rules_version"`
	Target        string        `json:"target_version"`
	Status        string        `json:"status"`
	Documents     int           `json:"documents"`
	Inputs        []Input       `json:"inputs"`
	Findings      []Finding     `json:"findings"`
	Unchecked     []string      `json:"unchecked"`
	Schema        schema.Source `json:"schema"`
	SchemaChecked int           `json:"schema_checked"`
}
type Rule struct {
	API         string `json:"api"`
	Kind        string `json:"kind"`
	Deprecated  int    `json:"deprecated"`
	Removed     int    `json:"removed"`
	Replacement string `json:"replacement"`
	Source      string `json:"source"`
	Reviewed    string `json:"reviewed"`
}

//go:embed rules.json
var ruleData []byte

func Rules() []Rule { var a []Rule; _ = json.Unmarshal(ruleData, &a); return a }

var version = regexp.MustCompile(`^v?1\.(\d+)(?:\.\d+)?$`)

func Target(s string) (int, error) {
	m := version.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("target must be 1.MINOR or 1.MINOR.PATCH")
	}
	n, e := strconv.Atoi(m[1])
	if e != nil || n < 16 || n > 37 {
		return 0, fmt.Errorf("bundled Kubernetes minor versions: 1.16 through 1.37")
	}
	return n, nil
}
func Files(path string) ([]string, error) { return manifest.Files(path) }
func Scan(path, target string) (Report, error) {
	return ScanContext(context.Background(), path, target, false)
}
func ScanContext(ctx context.Context, path, target string, recommend bool) (Report, error) {
	s, e := manifest.Load(ctx, path)
	if e != nil {
		return Report{}, e
	}
	return Analyze(ctx, s, target, recommend)
}
func Analyze(ctx context.Context, set manifest.Set, target string, recommend bool) (Report, error) {
	r := Report{SchemaVersion: "2", RulesVersion: "2026-11-15", Target: target, Status: "partial", Documents: len(set.Resources), Inputs: set.Inputs, Findings: []Finding{}, Unchecked: []string{"Admission, CEL, controller behavior, feature-gate availability and application runtime compatibility are not evaluated", "Schemas are pinned to minor .0 releases; patch-specific changes are not inferred", "OpenAPI validation does not replace Kubernetes admission or server semantic validation"}}
	minor, e := Target(target)
	if e != nil {
		return r, e
	}
	v, e := schema.New(minor)
	if e != nil {
		return r, e
	}
	r.Schema = v.Source
	if e = v.AddCRDs(set.Resources); e != nil {
		return r, e
	}
	for _, i := range set.Issues {
		r.Findings = append(r.Findings, Finding{Rule: "KW-INPUT", File: i.File, Document: i.Document, Line: i.Line, Requirement: "required", Severity: "error", Evidence: "confirmed", Message: i.Code})
		r.Unchecked = append(r.Unchecked, "One or more inputs could not be fully inspected")
	}
	rules := Rules()
	for _, res := range set.Resources {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		api, kind := manifest.String(res.Object, "apiVersion"), manifest.String(res.Object, "kind")
		if api == "" || kind == "" {
			r.Add(res, "KW-REQUIRED", "", "required", "error", "confirmed", "apiVersion and kind must be nonempty strings", r.Schema.URL)
			continue
		}
		removed := false
		for _, rule := range rules {
			if rule.API != api || rule.Kind != kind {
				continue
			}
			if rule.Removed > 0 && minor >= rule.Removed {
				removed = true
				r.Add(res, "KW-API-REMOVED", "/apiVersion", "required", "error", "confirmed", fmt.Sprintf("Not served since 1.%d; migrate to %s", rule.Removed, rule.Replacement), rule.Source)
			} else if rule.Deprecated > 0 && minor >= rule.Deprecated {
				r.Add(res, "KW-API-DEPRECATED", "/apiVersion", "conditional", "warning", "confirmed", fmt.Sprintf("Deprecated since 1.%d, removed in 1.%d; prepare %s", rule.Deprecated, rule.Removed, rule.Replacement), rule.Source)
			}
		}
		violations, known, e := v.Validate(res)
		if e != nil {
			r.Unchecked = append(r.Unchecked, "Schema could not be compiled for "+res.ID())
			continue
		}
		if known {
			r.SchemaChecked++
			for _, issue := range violations {
				r.Add(res, "KW-SCHEMA", issue.Path, "required", "error", "confirmed", "Schema constraint failed: "+issue.Keyword, r.Schema.URL)
			}
		} else if !removed && v.HasGroupKind(api, kind) {
			r.Add(res, "KW-API-UNAVAILABLE", "/apiVersion", "required", "error", "confirmed", "This built-in API version is absent from the target schema bundle; check introduction/removal and feature gates", r.Schema.URL)
		} else if !removed {
			r.Unchecked = append(r.Unchecked, "No target schema for "+res.ID())
		}
		if recommend {
			recommendations(&r, res)
		}
	}
	r.Unchecked = append(r.Unchecked, v.Unsupported...)
	sort.Strings(r.Unchecked)
	r.Unchecked = unique(r.Unchecked)
	return r, nil
}
func unique(in []string) []string {
	out := []string{}
	for _, s := range in {
		if len(out) == 0 || out[len(out)-1] != s {
			out = append(out, s)
		}
	}
	return out
}
func (r *Report) Add(res manifest.Resource, rule, field, requirement, severity, evidence, message, source string) {
	line := res.Locations[field]
	if line == 0 {
		line = res.Line
	}
	r.Findings = append(r.Findings, Finding{rule, res.File, res.Document, line, res.ID(), field, requirement, severity, evidence, message, source})
}
func PodSpec(res manifest.Resource) (map[string]any, string) {
	switch manifest.String(res.Object, "kind") {
	case "Pod":
		return manifest.Map(res.Object["spec"]), "/spec"
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job":
		return manifest.Map(manifest.At(res.Object, "spec", "template", "spec")), "/spec/template/spec"
	case "CronJob":
		return manifest.Map(manifest.At(res.Object, "spec", "jobTemplate", "spec", "template", "spec")), "/spec/jobTemplate/spec/template/spec"
	}
	return nil, ""
}
func recommendations(r *Report, res manifest.Resource) {
	p, base := PodSpec(res)
	if p == nil {
		return
	}
	doc := "https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/"
	for i, raw := range manifest.Slice(p["containers"]) {
		c := manifest.Map(raw)
		path := base + "/containers/" + strconv.Itoa(i)
		if len(manifest.Map(manifest.At(c, "resources", "requests"))) == 0 {
			r.Add(res, "KW-REQUESTS", path+"/resources/requests", "optional", "warning", "confirmed", "Set workload-appropriate resource requests after measuring usage", doc)
		}
		if c["readinessProbe"] == nil {
			r.Add(res, "KW-READINESS", path+"/readinessProbe", "conditional", "warning", "confirmed", "If serving traffic, define a readiness probe appropriate to the application", "https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/")
		}
		if manifest.At(c, "securityContext", "privileged") == true {
			r.Add(res, "KW-PRIVILEGED", path+"/securityContext/privileged", "conditional", "warning", "confirmed", "Review privileged container requirement and node kernel/OS dependency", "https://kubernetes.io/docs/concepts/security/pod-security-standards/")
		}
	}
	for i, raw := range manifest.Slice(p["volumes"]) {
		if manifest.Map(raw)["hostPath"] != nil {
			r.Add(res, "KW-HOSTPATH", base+"/volumes/"+strconv.Itoa(i)+"/hostPath", "conditional", "warning", "confirmed", "Host filesystem dependency requires node OS/path verification before upgrade or migration", "https://kubernetes.io/docs/concepts/storage/volumes/#hostpath")
		}
	}
	for _, f := range []string{"hostNetwork", "hostPID", "hostIPC"} {
		if p[f] == true {
			r.Add(res, "KW-HOST-NAMESPACE", base+"/"+f, "conditional", "warning", "confirmed", "Host namespace dependency requires node and network compatibility testing", "https://kubernetes.io/docs/concepts/security/pod-security-standards/")
		}
	}
	if len(manifest.Slice(manifest.At(p, "securityContext", "sysctls"))) > 0 || p["runtimeClassName"] != nil {
		r.Add(res, "KW-NODE-DEPENDENCY", base, "conditional", "warning", "confirmed", "RuntimeClass or sysctl settings require target node kernel/runtime verification", "https://kubernetes.io/docs/tasks/administer-cluster/sysctl-cluster/")
	}
}
func HasErrors(r Report) bool {
	for _, f := range r.Findings {
		if f.Severity == "error" {
			return true
		}
	}
	return false
}
func VersionDoc(target, path string) string {
	n, e := Target(target)
	if e != nil {
		return ""
	}
	return fmt.Sprintf("https://v1-%d.docs.kubernetes.io/%s", n, strings.TrimPrefix(path, "/"))
}
