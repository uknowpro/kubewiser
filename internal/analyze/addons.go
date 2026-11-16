package analyze

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/uknowpro/kubewiser/internal/manifest"
	"github.com/uknowpro/kubewiser/internal/scan"
	version "k8s.io/apimachinery/pkg/util/version"
)

//go:embed addons.json
var addonData []byte

type AddonRule struct {
	Name     string `json:"name"`
	Image    string `json:"image"`
	Version  string `json:"version"`
	Match    string `json:"match"`
	Min      int    `json:"min"`
	Max      int    `json:"max"`
	Source   string `json:"source"`
	Reviewed string `json:"reviewed"`
	Retired  bool   `json:"retired,omitempty"`
}

func AddonCatalog() []AddonRule {
	var rules []AddonRule
	_ = json.Unmarshal(addonData, &rules)
	return rules
}
func addonActions(set manifest.Set, from, to int) []Action {
	out := []Action{}
	rules := AddonCatalog()
	for _, r := range set.Resources {
		spec, _ := scan.PodSpec(r)
		if spec == nil {
			continue
		}
		for _, c := range manifest.Slice(spec["containers"]) {
			image := manifest.String(manifest.Map(c), "image")
			ref := strings.SplitN(image, "@", 2)[0]
			i := strings.LastIndex(ref, ":")
			if i < strings.LastIndex(ref, "/") {
				i = -1
			}
			repo, tag := ref, ""
			if i >= 0 {
				repo, tag = ref[:i], ref[i+1:]
			}
			known, matched := false, false
			for _, rule := range rules {
				if repo != rule.Image {
					continue
				}
				known = true
				v, e := version.ParseSemantic(tag)
				if e != nil || v.PreRelease() != "" {
					continue
				}
				match := strings.TrimPrefix(tag, "v") == rule.Version
				if rule.Match == "minor" {
					match = fmt.Sprintf("%d.%d", v.Major(), v.Minor()) == rule.Version
				}
				if !match {
					continue
				}
				matched = true
				req := "conditional"
				message := "The complete requested minor range is within the documented add-on matrix; validate patches, configuration and vendor exceptions"
				if from < rule.Min || to > rule.Max {
					req = "required"
					message = "The requested upgrade range extends outside the documented add-on matrix; schedule an add-on version change before affected steps"
				}
				if rule.Retired {
					req = "required"
					message = "ingress-nginx is retired; historical compatibility is not ongoing maintenance. Plan a supported replacement"
				}
				out = append(out, Action{ID: "KW-ADDON-CATALOG", Resource: r.ID(), Requirement: req, Current: rule.Name + " " + tag + " (image tag)", Recommended: fmt.Sprintf("Documented Kubernetes 1.%d–1.%d; reviewed %s", rule.Min, rule.Max, rule.Reviewed), Message: message, Verification: "Match the deployed image digest and vendor documentation; matrix coverage does not establish application or managed-provider support", Source: rule.Source})
				break
			}
			if known && !matched {
				if repo == "registry.k8s.io/ingress-nginx/controller" {
					out = append(out, Action{ID: "KW-ADDON-RETIRED", Resource: r.ID(), Requirement: "required", Message: "ingress-nginx is retired regardless of the unlisted image version", Verification: "Plan a supported replacement and verify traffic behavior in staging", Source: "https://kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/"})
				}
				out = append(out, Action{ID: "KW-ADDON-UNKNOWN", Resource: r.ID(), Requirement: "conditional", Message: "Recognized add-on image, but no matching version in the pinned compatibility catalog", Verification: "Provide vendor compatibility evidence for the exact image version; digest-only and unlisted tags are not inferred", Source: "https://kubernetes.io/docs/concepts/cluster-administration/addons/"})
			}
		}
	}
	return out
}
