package analyze

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/uknowpro/kubewiser/internal/manifest"
	"github.com/uknowpro/kubewiser/internal/scan"
	"strings"
)

type ImageEvidence struct {
	Image string          `json:"image"`
	Index json.RawMessage `json:"index"`
}
type Platform struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
}

func runtimeActions(set manifest.Set, p ProjectProfile) ([]Action, []string, error) {
	actions := []Action{}
	unchecked := []string{"OCI platform descriptors are local evidence, not image execution or kernel/module compatibility tests. No registry calls or pulls occur."}
	if p.TargetPlatform == nil {
		return actions, unchecked, nil
	}
	platform := p.TargetPlatform
	if (platform.OS != "linux" && platform.OS != "windows") || (platform.Architecture != "amd64" && platform.Architecture != "arm64" && platform.Architecture != "386") {
		return nil, nil, fmt.Errorf("unsupported target platform")
	}
	indexes := map[string]ImageEvidence{}
	for _, e := range p.Images {
		if e.Image == "" {
			return nil, nil, fmt.Errorf("image evidence requires an image reference")
		}
		if _, ok := indexes[e.Image]; ok {
			return nil, nil, fmt.Errorf("duplicate image evidence")
		}
		indexes[e.Image] = e
	}
	seen := map[string]bool{}
	for _, r := range set.Resources {
		if manifest.String(r.Object, "kind") == "Node" {
			ni := manifest.Map(manifest.At(r.Object, "status", "nodeInfo"))
			actions = append(actions, Action{ID: "KW-NODE-RUNTIME-EVIDENCE", Resource: r.ID(), Requirement: "conditional", Current: fmt.Sprintf("OS=%s; arch=%s; kernel=%s; runtime=%s", manifest.String(ni, "operatingSystem"), manifest.String(ni, "architecture"), manifest.String(ni, "kernelVersion"), manifest.String(ni, "containerRuntimeVersion")), Recommended: platform.OS + "/" + platform.Architecture, Message: "Observed node runtime inventory; verify target OS image and application dependencies", Verification: "Validate kernel modules, cgroup mode, native libraries, storage and agents on a target-platform canary", Source: "https://kubernetes.io/docs/concepts/architecture/nodes/"})
		}
		spec, _ := scan.PodSpec(r)
		if spec == nil {
			continue
		}
		for _, field := range []string{"containers", "initContainers", "ephemeralContainers"} {
			for _, c := range manifest.Slice(spec[field]) {
				image := manifest.String(manifest.Map(c), "image")
				if image == "" || seen[image] {
					continue
				}
				seen[image] = true
				a := Action{ID: "KW-IMAGE-PLATFORM", Resource: r.ID(), Requirement: "conditional", Current: image, Recommended: platform.OS + "/" + platform.Architecture, Message: "No matching local OCI image-index evidence; platform compatibility is unknown", Verification: "Provide the exact OCI index and bind it to the deployed digest; run workload tests on the target platform", Source: "https://github.com/opencontainers/image-spec/blob/main/image-index.md"}
				if evidence, ok := indexes[image]; ok {
					var index struct {
						Schema    int    `json:"schemaVersion"`
						MediaType string `json:"mediaType"`
						Manifests []struct {
							Platform Platform `json:"platform"`
						} `json:"manifests"`
					}
					if e := json.Unmarshal(evidence.Index, &index); e != nil || index.Schema != 2 || len(index.Manifests) == 0 {
						return nil, nil, fmt.Errorf("image evidence must contain an OCI/Docker image index with platform descriptors")
					}
					sum := sha256.Sum256(evidence.Index)
					hash := fmt.Sprintf("sha256:%x", sum)
					if strings.Contains(image, "@") && !strings.HasSuffix(image, "@"+hash) {
						a.Requirement = "required"
						a.Message = "Supplied OCI index bytes do not match the referenced image digest"
					} else {
						found := false
						for _, d := range index.Manifests {
							if d.Platform == *platform {
								found = true
							}
						}
						if found {
							a.Message = "The supplied index advertises the requested platform; this does not verify binaries, node kernel or runtime behavior"
						} else {
							a.Requirement = "required"
							a.Message = "The supplied index does not advertise the requested platform"
						}
						if !strings.Contains(image, "@") {
							a.Message += "; tag-only evidence is not immutable"
						}
					}
				}
				actions = append(actions, a)
			}
		}
	}
	return actions, unchecked, nil
}
