package analyze

import (
	"fmt"
	"github.com/uknowpro/kubewiser/internal/manifest"
	"github.com/uknowpro/kubewiser/internal/scan"
	"strings"
)

// API changes must be addressed before the first affected step, not only the final target.
func findingDeadline(f scan.Finding, from, to int) string {
	deadline := to
	if f.Rule == "KW-API-REMOVED" || f.Rule == "KW-API-DEPRECATED" {
		for _, r := range scan.Rules() {
			if strings.HasPrefix(f.Resource, r.API+"/"+r.Kind+"/") {
				at := r.Removed
				if f.Rule == "KW-API-DEPRECATED" {
					at = r.Deprecated
				}
				if at > 0 && at < deadline {
					deadline = at
				}
				break
			}
		}
	}
	if deadline <= from {
		return "before starting (already affected)"
	}
	return fmt.Sprintf("1.%d", deadline)
}

func behaviorActions(set manifest.Set, from, to int) []Action {
	out := []Action{}
	add := func(id string, at int, message, verification, source string) {
		if from < at && to >= at {
			out = append(out, Action{ID: id, Requirement: "conditional", Message: message, Verification: verification, Source: source, Before: fmt.Sprintf("1.%d", at)})
		}
	}
	add("KW-BEHAVIOR-DOCKERSHIM", 24, "Dockershim is removed: review Docker socket mounts, runtime monitoring and image build dependencies", "Verify the configured CRI runtime and run runtime-dependent agents on a test node", "https://kubernetes.io/docs/tasks/administer-cluster/migrating-from-dockershim/")
	add("KW-BEHAVIOR-SA-TOKEN", 24, "ServiceAccount token Secrets are no longer automatically generated; review clients expecting them", "Test projected token rotation and external clients without copying credentials into reports", "https://kubernetes.io/docs/concepts/security/service-accounts/")
	add("KW-BEHAVIOR-PSP", 25, "PodSecurityPolicy is removed; validate replacement admission controls and namespace policy before upgrade", "Test allowed and rejected workloads in staging; do not assume equivalent policies", "https://kubernetes.io/docs/concepts/security/pod-security-policy/")
	for _, r := range set.Resources {
		spec, _ := scan.PodSpec(r)
		if spec == nil {
			continue
		}
		reasons := []string{}
		if spec["hostNetwork"] == true {
			reasons = append(reasons, "host networking")
		}
		if spec["runtimeClassName"] != nil {
			reasons = append(reasons, "RuntimeClass")
		}
		if len(manifest.Slice(manifest.At(spec, "securityContext", "sysctls"))) > 0 {
			reasons = append(reasons, "sysctls")
		}
		for _, v := range manifest.Slice(spec["volumes"]) {
			if manifest.Map(v)["hostPath"] != nil {
				reasons = append(reasons, "host filesystem")
				break
			}
		}
		for _, field := range []string{"containers", "initContainers"} {
			for _, v := range manifest.Slice(spec[field]) {
				if manifest.At(manifest.Map(v), "securityContext", "privileged") == true {
					reasons = append(reasons, "privileged container")
					break
				}
			}
		}
		if len(reasons) > 0 {
			out = append(out, Action{ID: "KW-NODE-DEPENDENCY", Resource: r.ID(), Requirement: "conditional", Message: "Node-dependent configuration: " + strings.Join(reasons, ", "), Verification: "Validate target OS image, kernel modules, cgroup mode, CRI runtime and CPU image architecture on a canary node", Source: "https://kubernetes.io/docs/concepts/architecture/nodes/", Before: fmt.Sprintf("1.%d", to)})
		}
	}
	return out
}
