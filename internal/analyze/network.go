package analyze

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/uknowpro/kubewiser/internal/manifest"
	"github.com/uknowpro/kubewiser/internal/scan"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sort"
	"strings"
	"time"
)

type Exposure struct {
	Resource string `json:"resource"`
	Type     string `json:"type"`
	Evidence string `json:"evidence"`
}
type Isolation struct {
	Resource string `json:"resource"`
	Ingress  bool   `json:"ingress_isolated_by_policy"`
	Egress   bool   `json:"egress_isolated_by_policy"`
}
type Flow struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Verdict     string `json:"verdict"`
	Count       int    `json:"count"`
	First       string `json:"first_observed"`
	Last        string `json:"last_observed"`
}
type Network struct {
	Exposures []Exposure  `json:"exposures"`
	Isolation []Isolation `json:"isolation"`
	Flows     []Flow      `json:"observed_flows"`
	Unchecked []string    `json:"unchecked"`
}

func NetworkScan(ctx context.Context, set manifest.Set, flowFile, defaultNS string) (Network, error) {
	n := Network{Exposures: []Exposure{}, Isolation: []Isolation{}, Flows: []Flow{}, Unchecked: []string{"Policy isolation is not proof of packet filtering: CNI enforcement, allow rules, cloud firewalls, routing and DNS are not evaluated", "No probes, port scans or agents are started; no observed traffic does not mean unused or blocked"}}
	if len(set.Issues) > 0 {
		return n, fmt.Errorf("input parsing incomplete")
	}
	policies := []manifest.Resource{}
	for _, r := range set.Resources {
		kind := manifest.String(r.Object, "kind")
		if kind == "NetworkPolicy" {
			policies = append(policies, r)
		}
		if kind == "Ingress" {
			n.Exposures = append(n.Exposures, Exposure{r.ID(), "Ingress", "configured route, not reachability tested"})
		}
		if kind == "Service" {
			t := manifest.String(manifest.Map(r.Object["spec"]), "type")
			if t == "LoadBalancer" || t == "NodePort" {
				n.Exposures = append(n.Exposures, Exposure{r.ID(), t, "configured exposure, not reachability tested"})
			}
		}
	}
	namespace := func(m map[string]any) string {
		s := manifest.String(manifest.Map(m["metadata"]), "namespace")
		if s == "" {
			s = defaultNS
		}
		return s
	}
	for _, r := range set.Resources {
		if e := ctx.Err(); e != nil {
			return n, e
		}
		spec, _ := scan.PodSpec(r)
		if spec == nil {
			continue
		}
		meta := manifest.Map(r.Object["metadata"])
		switch manifest.String(r.Object, "kind") {
		case "CronJob":
			meta = manifest.Map(manifest.At(r.Object, "spec", "jobTemplate", "spec", "template", "metadata"))
		case "Pod":
		default:
			meta = manifest.Map(manifest.At(r.Object, "spec", "template", "metadata"))
		}
		ls := labels.Set{}
		for k, v := range manifest.Map(meta["labels"]) {
			if s, ok := v.(string); ok {
				ls[k] = s
			}
		}
		row := Isolation{Resource: r.ID()}
		for _, p := range policies {
			if namespace(p.Object) != namespace(r.Object) {
				continue
			}
			ps := manifest.Map(p.Object["spec"])
			if _, ok := ps["podSelector"].(map[string]any); !ok {
				return n, fmt.Errorf("NetworkPolicy podSelector must be an object")
			}
			b, _ := json.Marshal(ps["podSelector"])
			var sel metav1.LabelSelector
			if e := json.Unmarshal(b, &sel); e != nil {
				return n, fmt.Errorf("invalid NetworkPolicy selector")
			}
			selector, e := metav1.LabelSelectorAsSelector(&sel)
			if e != nil {
				return n, fmt.Errorf("invalid NetworkPolicy selector")
			}
			if !selector.Matches(ls) {
				continue
			}
			types := manifest.Slice(ps["policyTypes"])
			if len(types) == 0 {
				row.Ingress = true
				if len(manifest.Slice(ps["egress"])) > 0 {
					row.Egress = true
				}
			} else {
				for _, t := range types {
					if t == "Ingress" {
						row.Ingress = true
					}
					if t == "Egress" {
						row.Egress = true
					}
				}
			}
		}
		n.Isolation = append(n.Isolation, row)
	}
	if flowFile != "" {
		b, e := manifest.ReadBounded(flowFile, manifest.MaxFile)
		if e != nil {
			return n, e
		}
		scanner := bufio.NewScanner(bytes.NewReader(b))
		scanner.Buffer(make([]byte, 4096), 1<<20)
		flows := map[string]*Flow{}
		line := 0
		for scanner.Scan() {
			if e := ctx.Err(); e != nil {
				return n, e
			}
			if strings.TrimSpace(scanner.Text()) == "" {
				continue
			}
			line++
			if line > 10000 {
				return n, fmt.Errorf("flow record limit exceeded")
			}
			var obj map[string]any
			if json.Unmarshal(scanner.Bytes(), &obj) != nil {
				return n, fmt.Errorf("invalid flow JSON at line %d", line)
			}
			f := manifest.Map(obj["flow"])
			if len(f) == 0 {
				f = obj
			}
			endpoint := func(name, ip string) string {
				m := manifest.Map(f[name])
				ns, pod := manifest.String(m, "namespace"), manifest.String(m, "pod_name")
				if pod != "" {
					return ns + "/" + pod
				}
				return manifest.String(manifest.Map(f["IP"]), ip)
			}
			src, dst := endpoint("source", "source"), endpoint("destination", "destination")
			verdict := manifest.String(f, "verdict")
			if src == "" || dst == "" || verdict == "" {
				return n, fmt.Errorf("unsupported Hubble flow at line %d", line)
			}
			key := src + "\x00" + dst + "\x00" + verdict
			row := flows[key]
			if row == nil {
				row = &Flow{Source: src, Destination: dst, Verdict: verdict}
				flows[key] = row
			}
			row.Count++
			ts := manifest.String(f, "time")
			if ts != "" {
				stamp, err := time.Parse(time.RFC3339Nano, ts)
				if err != nil {
					return n, fmt.Errorf("invalid flow timestamp at line %d", line)
				}
				first, _ := time.Parse(time.RFC3339Nano, row.First)
				last, _ := time.Parse(time.RFC3339Nano, row.Last)
				if row.First == "" || stamp.Before(first) {
					row.First = stamp.UTC().Format(time.RFC3339Nano)
				}
				if row.Last == "" || stamp.After(last) {
					row.Last = stamp.UTC().Format(time.RFC3339Nano)
				}
			}
		}
		if e = scanner.Err(); e != nil {
			return n, fmt.Errorf("flow line exceeds limit")
		}
		keys := []string{}
		for k := range flows {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			n.Flows = append(n.Flows, *flows[k])
		}
	}
	return n, nil
}
