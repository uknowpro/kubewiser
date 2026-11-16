package analyze

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/uknowpro/kubewiser/internal/manifest"
	"github.com/uknowpro/kubewiser/internal/scan"
	version "k8s.io/apimachinery/pkg/util/version"
)

type AKSEvidence struct {
	CapturedAt string          `json:"captured_at"`
	Profile    json.RawMessage `json:"profile"`
}
type AKSRoute struct {
	Status  string   `json:"status"`
	Current string   `json:"observed_version,omitempty"`
	Offered []string `json:"offered_versions"`
	Next    string   `json:"next_candidate,omitempty"`
	SHA256  string   `json:"evidence_sha256"`
	Note    string   `json:"note"`
}

func ReviewAKSRoute(p ProjectProfile, now time.Time) (AKSRoute, error) {
	r := AKSRoute{Status: "unknown", Offered: []string{}, Note: "Local exported evidence is not independently authenticated. Only the next offered step can be checked; future steps require new evidence."}
	if p.UpgradeEvidence == nil {
		return r, nil
	}
	evidence := p.UpgradeEvidence
	sum := sha256.Sum256(evidence.Profile)
	r.SHA256 = fmt.Sprintf("%x", sum)
	if p.ClusterID == "" {
		return r, fmt.Errorf("cluster_id is required to bind upgrade evidence to the project")
	}
	timestamp, e := time.Parse(time.RFC3339, evidence.CapturedAt)
	if e != nil {
		return r, fmt.Errorf("upgrade evidence needs an RFC3339 captured_at")
	}
	if timestamp.After(now.Add(5*time.Minute)) || now.Sub(timestamp) > 24*time.Hour {
		r.Status = "stale"
		return r, nil
	}
	var raw map[string]any
	if e = json.Unmarshal(evidence.Profile, &raw); e != nil {
		return r, fmt.Errorf("invalid AKS upgrade profile JSON")
	}
	id := strings.TrimSuffix(strings.TrimRight(manifest.String(raw, "id"), "/"), "/upgradeProfiles/default")
	if !strings.EqualFold(strings.TrimRight(p.ClusterID, "/"), id) {
		r.Status = "cluster_mismatch"
		return r, nil
	}
	root := raw
	if props, ok := raw["properties"].(map[string]any); ok {
		if raw["controlPlaneProfile"] != nil {
			return r, fmt.Errorf("ambiguous upgrade profile structure")
		}
		root = props
	}
	cp := manifest.Map(root["controlPlaneProfile"])
	r.Current = manifest.String(cp, "kubernetesVersion")
	current, e := version.ParseSemantic(r.Current)
	if e != nil {
		return r, fmt.Errorf("missing exact control-plane version in upgrade evidence")
	}
	from, e := scan.Target(p.From)
	if e != nil {
		return r, e
	}
	to, e := scan.Target(p.To)
	if e != nil {
		return r, e
	}
	if int(current.Minor()) != from || current.Major() != 1 {
		r.Status = "version_mismatch"
		return r, nil
	}
	if strings.Count(strings.TrimPrefix(p.From, "v"), ".") == 2 {
		f, e := version.ParseSemantic(p.From)
		if e != nil || !current.EqualTo(f) {
			r.Status = "version_mismatch"
			return r, nil
		}
	}
	seen := map[string]bool{}
	for _, item := range manifest.Slice(cp["upgrades"]) {
		u := manifest.Map(item)
		if u["isPreview"] == true {
			continue
		}
		v := manifest.String(u, "kubernetesVersion")
		parsed, e := version.ParseSemantic(v)
		if e != nil || parsed.Major() != 1 || parsed.PreRelease() != "" {
			continue
		}
		if !parsed.GreaterThan(current) || int(parsed.Minor()) > to {
			continue
		}
		if strings.Count(strings.TrimPrefix(p.To, "v"), ".") == 2 {
			target, e := version.ParseSemantic(p.To)
			if e != nil {
				return r, e
			}
			if parsed.GreaterThan(target) {
				continue
			}
		}
		if !seen[v] {
			r.Offered = append(r.Offered, v)
			seen[v] = true
		}
	}
	sort.Slice(r.Offered, func(i, j int) bool {
		a, _ := version.ParseSemantic(r.Offered[i])
		b, _ := version.ParseSemantic(r.Offered[j])
		return a.LessThan(b)
	})
	r.Status = "not_offered"
	if len(r.Offered) > 0 {
		r.Status = "next_step_offered"
		r.Next = r.Offered[0]
	}
	return r, nil
}
