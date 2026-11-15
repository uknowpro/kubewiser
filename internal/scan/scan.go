// Package scan provides offline manifest checks without cluster access.
package scan

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Finding struct {
	Rule        string `json:"rule_id"`
	File        string `json:"file"`
	Document    int    `json:"document"`
	Line        int    `json:"line"`
	Field       string `json:"field"`
	Requirement string `json:"requirement"`
	Evidence    string `json:"evidence"`
	Message     string `json:"message"`
	Source      string `json:"source,omitempty"`
}
type Input struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}
type Report struct {
	SchemaVersion string    `json:"schema_version"`
	RulesVersion  string    `json:"rules_version"`
	Target        string    `json:"target_version"`
	Status        string    `json:"status"`
	Documents     int       `json:"documents"`
	Inputs        []Input   `json:"inputs"`
	Findings      []Finding `json:"findings"`
	Unchecked     []string  `json:"unchecked"`
}

var version = regexp.MustCompile(`^1\.(\d+)(?:\.\d+)?$`)

func Target(s string) (int, error) {
	m := version.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("target version must be 1.MINOR or 1.MINOR.PATCH")
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n < 16 || n > 32 {
		return 0, fmt.Errorf("initial rule coverage supports targets 1.16 through 1.32 only")
	}
	return n, nil
}

type removal struct {
	API, Kind   string
	Minor       int
	Replacement string
}

var removals = []removal{
	{"extensions/v1beta1", "Deployment", 16, "apps/v1"},
	{"apps/v1beta1", "Deployment", 16, "apps/v1"},
	{"apps/v1beta2", "Deployment", 16, "apps/v1"},
	{"extensions/v1beta1", "Ingress", 22, "networking.k8s.io/v1; review backend and pathType fields"},
	{"networking.k8s.io/v1beta1", "Ingress", 22, "networking.k8s.io/v1; review backend and pathType fields"},
	{"apiextensions.k8s.io/v1beta1", "CustomResourceDefinition", 22, "apiextensions.k8s.io/v1; review schema requirements"},
	{"batch/v1beta1", "CronJob", 25, "batch/v1"},
	{"policy/v1beta1", "PodDisruptionBudget", 25, "policy/v1; review empty selector semantics"},
	{"policy/v1beta1", "PodSecurityPolicy", 25, "Pod Security Admission or an alternative policy solution"},
	{"autoscaling/v2beta1", "HorizontalPodAutoscaler", 25, "autoscaling/v2; review metric fields"},
	{"autoscaling/v2beta2", "HorizontalPodAutoscaler", 26, "autoscaling/v2; review metric fields"},
}

// Files returns a deterministic set of YAML/JSON inputs. Directory symlinks are not followed.
func Files(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{path}, nil
	}
	var paths []string
	err = filepath.WalkDir(path, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			if p != path && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		switch strings.ToLower(filepath.Ext(p)) {
		case ".yaml", ".yml", ".json":
			paths = append(paths, p)
		}
		return nil
	})
	sort.Strings(paths)
	if err == nil && len(paths) == 0 {
		err = fmt.Errorf("no manifest files found")
	}
	return paths, err
}
func Scan(path, target string) (Report, error) {
	r := Report{SchemaVersion: "1", RulesVersion: "initial-1", Target: target, Status: "partial", Inputs: []Input{}, Findings: []Finding{}, Unchecked: []string{"Full Kubernetes/CRD schema validation", "API removals outside the bundled rule table", "Admission policies and controller behavior", "Helm source templates (render them first)", "Live cluster, network, OS and application compatibility"}}
	minor, err := Target(target)
	if err != nil {
		return r, err
	}
	paths, err := Files(path)
	if err != nil {
		return r, err
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return r, fmt.Errorf("read %s: %w", p, err)
		}
		r.Inputs = append(r.Inputs, Input{p, fmt.Sprintf("%x", sha256.Sum256(data))})
		dec := yaml.NewDecoder(bytes.NewReader(data))
		for doc := 1; ; doc++ {
			var n yaml.Node
			err := dec.Decode(&n)
			if err == io.EOF {
				break
			}
			if err != nil {
				r.add("KW-YAML", p, doc, 0, "", "Invalid YAML syntax; inspect source locally")
				break
			}
			if len(n.Content) == 0 {
				continue
			}
			v := n.Content[0]
			if v.Tag == "!!null" {
				continue
			}
			r.Documents++
			// Decode catches duplicate mapping keys. Never expose parser diagnostics containing values.
			var decoded any
			if err := v.Decode(&decoded); err != nil {
				r.add("KW-YAML", p, doc, v.Line, "", "Invalid mapping or duplicate keys; inspect source locally")
				continue
			}
			if v.Kind != yaml.MappingNode {
				r.add("KW-OBJECT", p, doc, v.Line, "", "Manifest must be a mapping")
				continue
			}
			fields := map[string]*yaml.Node{}
			for i := 0; i < len(v.Content); i += 2 {
				fields[v.Content[i].Value] = v.Content[i+1]
			}
			for _, key := range []string{"apiVersion", "kind"} {
				x := fields[key]
				if x == nil || x.Tag != "!!str" || x.Value == "" {
					r.add("KW-REQUIRED", p, doc, v.Line, key, "Required nonempty string is missing or invalid")
				}
			}
			api, kind := fields["apiVersion"], fields["kind"]
			if api == nil || kind == nil {
				continue
			}
			if kind.Value == "List" {
				r.add("KW-LIST", p, doc, kind.Line, "items", "List items are not inspected; provide individual manifests")
				r.Findings[len(r.Findings)-1].Requirement = "review"
			}
			for _, rule := range removals {
				if rule.API == api.Value && rule.Kind == kind.Value && minor >= rule.Minor {
					r.add("KW-API-REMOVED", p, doc, api.Line, "apiVersion", fmt.Sprintf("API not served since 1.%d; migrate to %s", rule.Minor, rule.Replacement))
					r.Findings[len(r.Findings)-1].Source = "https://kubernetes.io/docs/reference/using-api/deprecation-guide/"
				}
			}
		}
	}
	return r, nil
}
func (r *Report) add(rule, file string, doc, line int, field, msg string) {
	r.Findings = append(r.Findings, Finding{Rule: rule, File: file, Document: doc, Line: line, Field: field, Requirement: "required", Evidence: "confirmed", Message: msg})
}
