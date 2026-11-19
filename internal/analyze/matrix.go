package analyze

import (
	"context"
	"fmt"
	"github.com/uknowpro/kubewiser/internal/cluster"
	"github.com/uknowpro/kubewiser/internal/manifest"
	"github.com/uknowpro/kubewiser/internal/scan"
	"strings"
)

// MatrixRow separates inspection coverage from a compatibility verdict.
// It intentionally contains no raw manifest values.
type MatrixRow struct {
	Topic    string `json:"topic"`
	Resource string `json:"resource,omitempty"`
	Current  string `json:"current"`
	Target   string `json:"target"`
	Status   string `json:"status"`
	Evidence string `json:"evidence"`
	Action   string `json:"action"`
}

func BuildMatrix(p *Plan) {
	p.Matrix = []MatrixRow{}
	add := func(topic, current, target, status, evidence, action string) {
		p.Matrix = append(p.Matrix, MatrixRow{Topic: topic, Current: current, Target: target, Status: status, Evidence: evidence, Action: action})
	}
	add("manifest", "Supplied current manifests", "Recommendation only; no proposed manifests supplied", "input_missing", "No before/after field comparison performed", "Supply --after PATH to compare proposed manifests and validate them against the target version")
	topics := []struct{ name, prefix, status, action string }{
		{"api", "KW-API-", "not_detected", "Review removed APIs and API clients; absence in supplied manifests is not proof of absence"},
		{"schema", "KW-SCHEMA", "not_detected", "Review target schema findings and exact bundled schema version; admission remains separate"},
		{"defaults", "KW-BEHAVIOR-", "analyzer_unsupported", "Review selected behavior changes; exhaustive default/feature-gate comparison is not implemented"},
		{"lifecycle", "KW-LIFECYCLE", "not_detected", "Verify components outside the selected lifecycle catalog"},
		{"ingress", "KW-ARCH-INGRESS", "analyzer_unsupported", "Validate target controller annotations, TLS and routing; general controller conversion is not implemented"},
		{"storage", "KW-ARCH-STORAGE", "analyzer_unsupported", "Verify CSI, access modes, zones and restore behavior"},
		{"cpu", "KW-IMAGE-PLATFORM", "input_missing", "Provide target platform and local OCI evidence in an AKS profile; test native dependencies"},
		{"os", "KW-ARCH-OS", "input_missing", "Provide current/target OS architecture declarations and verify vendor support"},
		{"application", "KW-CGROUP-V1-", "execution_required", "Test memory/OOM, CPU throttling, application runtimes and agents on target nodes"},
	}
	for _, t := range topics {
		status := t.status
		evidence := "No matching finding; coverage is partial"
		n := 0
		for _, a := range p.Actions {
			if strings.HasPrefix(a.ID, t.prefix) {
				n++
				if a.Requirement == "required" {
					status = "action_required"
				} else if status != "action_required" {
					status = "review_required"
				}
			}
		}
		if n > 0 {
			evidence = fmt.Sprintf("%d related findings; see inspection details", n)
		}
		current, target := "Not established from supplied inputs", "Target requirements require evidence"
		if t.name == "api" || t.name == "schema" || t.name == "defaults" {
			current, target = p.From, p.To
		}
		for _, a := range p.Actions {
			if strings.HasPrefix(a.ID, t.prefix) {
				if a.Current != "" {
					current = a.Current
				}
				if a.Recommended != "" {
					target = a.Recommended
				}
				break
			}
		}
		add(t.name, current, target, status, evidence, t.action)
	}
	if p.Cgroup == nil || len(p.Cgroup.Pools) == 0 {
		for _, topic := range []string{"cgroup", "kernel", "runtime", "drivers"} {
			add(topic, "unknown", "unknown", "input_missing", "No current/target node-pool evidence supplied", "Supply --cgroup-evidence or AKS profile cgroup_pools; OS/Kubernetes versions do not establish cgroup mode")
		}
	} else {
		for _, pool := range p.Cgroup.Pools {
			status := "review_required"
			switch pool.Status {
			case "unknown":
				status = "input_missing"
			case "conflicting_evidence", "mode_change", "hybrid_review":
				status = "action_required"
			case "same_mode":
				status = "execution_required"
			}
			p.Matrix = append(p.Matrix, MatrixRow{Topic: "cgroup", Resource: pool.Pool, Current: pool.Current, Target: pool.Target, Status: status, Evidence: pool.CurrentEvidence + "; " + pool.TargetEvidence, Action: strings.Join(pool.Checks, "; ")})
			for _, topic := range []string{"kernel", "runtime", "drivers"} {
				status := "input_missing"
				evidence := []string{}
				for _, check := range pool.Checks {
					_, message, _ := strings.Cut(check, ": ")
					message = strings.ToLower(message)
					match := false
					switch topic {
					case "kernel":
						match = strings.HasPrefix(message, "target kernel")
					case "runtime":
						match = strings.HasPrefix(message, "target runtime") || strings.HasPrefix(message, "runtime meets")
					case "drivers":
						match = strings.Contains(message, "driver")
					}
					if !match {
						continue
					}
					evidence = append(evidence, check)
					if strings.HasPrefix(check, "required:") {
						status = "action_required"
					} else if status != "action_required" && strings.HasPrefix(check, "evidence_meets_minimum:") {
						status = "baseline_met"
					}
				}
				current, target := "", ""
				switch topic {
				case "kernel":
					current, target = pool.CurrentConfig.Kernel, pool.TargetConfig.Kernel
				case "runtime":
					current = strings.TrimSpace(pool.CurrentConfig.Runtime + " " + pool.CurrentConfig.RuntimeVersion)
					target = strings.TrimSpace(pool.TargetConfig.Runtime + " " + pool.TargetConfig.RuntimeVersion)
				case "drivers":
					current = pool.CurrentConfig.KubeletDriver + " / " + pool.CurrentConfig.RuntimeDriver
					target = pool.TargetConfig.KubeletDriver + " / " + pool.TargetConfig.RuntimeDriver
				}
				if strings.TrimSpace(current) == "" || current == " / " {
					current = "unknown"
				}
				if strings.TrimSpace(target) == "" || target == " / " {
					target = "unknown"
				}
				p.Matrix = append(p.Matrix, MatrixRow{Topic: topic, Resource: pool.Pool, Current: current, Target: target, Status: status, Evidence: strings.Join(evidence, "; "), Action: "Baseline checks do not certify exact provider or application compatibility"})
			}
		}
	}
}

func CompareManifests(ctx context.Context, p *Plan, before, after manifest.Set, namespace string) error {
	if len(before.Issues) > 0 || len(after.Issues) > 0 {
		return fmt.Errorf("before/after manifests contain parse errors")
	}
	report, err := scan.Analyze(ctx, after, p.To, true)
	if err != nil {
		return err
	}
	diff, err := Compare(after, cluster.Snapshot{Namespace: namespace, Resources: before.Resources}, namespace)
	if err != nil {
		return err
	}
	p.AfterReport = &report
	BuildMatrix(p)
	p.Matrix[0] = MatrixRow{Topic: "manifest", Current: fmt.Sprintf("%d resources", len(before.Resources)), Target: fmt.Sprintf("%d proposed resources; schema %s", len(after.Resources), report.Schema.Version), Status: "review_required", Evidence: fmt.Sprintf("%d structural differences; %d target findings", len(diff.Changes), len(report.Findings)), Action: "Proposed configuration, not an applied state. Values omitted; defaults, admission and runtime behavior require separate validation"}
	for _, c := range diff.Changes {
		current, target := "present", "changed"
		switch c.Type {
		case "not_observed":
			current = "absent from supplied current set"
			target = "added in proposal"
		case "observed_only_resource":
			current = "present"
			target = "absent from proposal (scope must be checked)"
		case "observed_only_field":
			target = "removed from proposal"
		case "desired_only_field":
			current = "absent"
			target = "added in proposal"
		case "conversion_unverified":
			target = "API version changed; semantic conversion unverified"
		}
		p.Matrix = append(p.Matrix, MatrixRow{Topic: "manifest-field", Resource: c.Resource + "#" + c.Path, Current: current, Target: target, Status: "review_required", Evidence: "Structural comparison; raw values omitted", Action: "Review proposed change and target validation findings"})
	}
	for _, f := range report.Findings {
		status := "review_required"
		if f.Requirement == "required" {
			status = "action_required"
		}
		p.Matrix = append(p.Matrix, MatrixRow{Topic: "target-validation", Resource: f.Resource + "#" + f.Field, Current: "Current findings remain in inspection table", Target: f.Message, Status: status, Evidence: f.Rule, Action: "Resolve or verify this finding in the proposed manifest"})
	}
	p.Unchecked = append(p.Unchecked, diff.Unchecked...)
	return nil
}
