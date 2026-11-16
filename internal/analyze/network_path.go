package analyze

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"

	"github.com/uknowpro/kubewiser/internal/manifest"
	core "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type PathReview struct {
	Flows       []Flow   `json:"matching_observed_flows,omitempty"`
	Source      string   `json:"source"`
	Destination string   `json:"destination"`
	Protocol    string   `json:"protocol"`
	Port        int      `json:"port"`
	Ingress     string   `json:"ingress"`
	Egress      string   `json:"egress"`
	Result      string   `json:"policy_intent"`
	Unchecked   []string `json:"unchecked"`
}

// Tri-state policy evaluation: unknown information must never become an allow.
type truth int

const (
	no truth = iota
	yes
	unknown
)

func and(a, b truth) truth {
	if a == no || b == no {
		return no
	}
	if a == unknown || b == unknown {
		return unknown
	}
	return yes
}
func or(a, b truth) truth {
	if a == yes || b == yes {
		return yes
	}
	if a == unknown || b == unknown {
		return unknown
	}
	return no
}
func state(t truth) string {
	if t == yes {
		return "allowed_by_policy"
	}
	if t == no {
		return "denied_by_policy"
	}
	return "unknown"
}
func matchSelector(s *meta.LabelSelector, ls map[string]string) truth {
	selector, e := meta.LabelSelectorAsSelector(s)
	if e != nil {
		return unknown
	}
	if selector.Matches(labels.Set(ls)) {
		return yes
	}
	return no
}
func decodeObject(obj map[string]any, v any) error {
	b, _ := json.Marshal(obj)
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	return d.Decode(v)
}
func NetworkPath(ctx context.Context, set manifest.Set, src, dst, protocol string, port int) (PathReview, error) {
	result := PathReview{Source: src, Destination: dst, Protocol: protocol, Port: port, Unchecked: []string{"Static NetworkPolicy intent for explicit Pod endpoints only. CNI enforcement, Service/NAT translation, DNS, routing, cloud firewalls and application listeners are not tested.", "Labels and Pod IPs reflect supplied files only; a complete namespace policy inventory is required. No probes or network calls are made."}}
	if len(set.Issues) > 0 {
		return result, fmt.Errorf("input parsing incomplete")
	}
	if port < 1 || port > 65535 || (protocol != "TCP" && protocol != "UDP" && protocol != "SCTP") {
		return result, fmt.Errorf("valid port and TCP/UDP/SCTP protocol required")
	}
	if len(strings.Split(src, "/")) != 2 || len(strings.Split(dst, "/")) != 2 {
		return result, fmt.Errorf("endpoints must be namespace/pod")
	}
	pods := map[string]core.Pod{}
	namespaces := map[string]map[string]string{}
	policies := []netv1.NetworkPolicy{}
	for _, r := range set.Resources {
		if e := ctx.Err(); e != nil {
			return result, e
		}
		switch manifest.String(r.Object, "kind") {
		case "Pod":
			var p core.Pod
			if e := decodeObject(r.Object, &p); e != nil {
				return result, fmt.Errorf("invalid Pod input: %w", e)
			}
			if p.Namespace == "" {
				p.Namespace = "default"
			}
			key := p.Namespace + "/" + p.Name
			if _, ok := pods[key]; ok {
				return result, fmt.Errorf("duplicate Pod identity")
			}
			pods[key] = p
		case "Namespace":
			var n core.Namespace
			if e := decodeObject(r.Object, &n); e != nil {
				return result, e
			}
			if _, ok := namespaces[n.Name]; ok {
				return result, fmt.Errorf("duplicate Namespace identity")
			}
			ls := n.Labels
			if ls == nil {
				ls = map[string]string{}
			}
			ls["kubernetes.io/metadata.name"] = n.Name
			namespaces[n.Name] = ls
		case "NetworkPolicy":
			if manifest.String(r.Object, "apiVersion") != "networking.k8s.io/v1" {
				continue
			}
			if manifest.At(r.Object, "spec", "podSelector") == nil {
				return result, fmt.Errorf("NetworkPolicy podSelector required")
			}
			var p netv1.NetworkPolicy
			if e := decodeObject(r.Object, &p); e != nil {
				return result, fmt.Errorf("invalid NetworkPolicy input: %w", e)
			}
			if p.Namespace == "" {
				p.Namespace = "default"
			}
			policies = append(policies, p)
		}
	}
	a, aok := pods[src]
	b, bok := pods[dst]
	if !aok || !bok {
		return result, fmt.Errorf("both explicit Pod endpoints must exist in inputs")
	}
	if a.Spec.HostNetwork || b.Spec.HostNetwork {
		result.Ingress = "unknown"
		result.Egress = "unknown"
		result.Result = "unknown"
		result.Unchecked = append(result.Unchecked, "hostNetwork behavior is CNI-specific")
		return result, nil
	}
	customPolicies := false
	for _, r := range set.Resources {
		kind := manifest.String(r.Object, "kind")
		api := manifest.String(r.Object, "apiVersion")
		if kind == "CiliumNetworkPolicy" || kind == "CiliumClusterwideNetworkPolicy" || kind == "GlobalNetworkPolicy" || (kind == "NetworkPolicy" && api != "networking.k8s.io/v1") {
			customPolicies = true
		}
	}
	ingress := direction(policies, b, a, b, true, namespaces, protocol, port)
	egress := direction(policies, a, b, b, false, namespaces, protocol, port)
	result.Ingress = state(ingress)
	result.Egress = state(egress)
	result.Result = state(and(ingress, egress))
	if customPolicies {
		result.Result = "unknown"
		result.Unchecked = append(result.Unchecked, "CNI-specific policy resources are present; their combined precedence and behavior are not evaluated")
	}
	return result, nil
}
func direction(policies []netv1.NetworkPolicy, selected, peer, dest core.Pod, ingress bool, namespaces map[string]map[string]string, protocol string, port int) truth {
	isolated := false
	allow := no
	for _, p := range policies {
		if p.Namespace != selected.Namespace {
			continue
		}
		applies := false
		if len(p.Spec.PolicyTypes) == 0 {
			applies = ingress || len(p.Spec.Egress) > 0
		} else {
			for _, t := range p.Spec.PolicyTypes {
				if t != netv1.PolicyTypeIngress && t != netv1.PolicyTypeEgress {
					return unknown
				}
				if (ingress && t == netv1.PolicyTypeIngress) || (!ingress && t == netv1.PolicyTypeEgress) {
					applies = true
				}
			}
		}
		if !applies {
			continue
		}
		matches := matchSelector(&p.Spec.PodSelector, selected.Labels)
		if matches == unknown {
			return unknown
		}
		if matches == no {
			continue
		}
		isolated = true
		if ingress {
			for _, r := range p.Spec.Ingress {
				allow = or(allow, and(matchPorts(r.Ports, dest, protocol, port), matchPeers(r.From, p.Namespace, peer, namespaces)))
			}
		} else {
			for _, r := range p.Spec.Egress {
				allow = or(allow, and(matchPorts(r.Ports, dest, protocol, port), matchPeers(r.To, p.Namespace, peer, namespaces)))
			}
		}
	}
	if !isolated {
		return yes
	}
	return allow
}
func matchPorts(ports []netv1.NetworkPolicyPort, dest core.Pod, protocol string, port int) truth {
	if len(ports) == 0 {
		return yes
	}
	result := no
	for _, p := range ports {
		proto := core.ProtocolTCP
		if p.Protocol != nil {
			proto = *p.Protocol
		}
		if string(proto) != protocol {
			continue
		}
		if p.Port == nil {
			if p.EndPort != nil {
				result = or(result, unknown)
			} else {
				return yes
			}
			continue
		}
		if p.Port.Type == intstr.String {
			if p.EndPort != nil {
				result = or(result, unknown)
				continue
			}
			for _, c := range dest.Spec.Containers {
				for _, cp := range c.Ports {
					cpProto := cp.Protocol
					if cpProto == "" {
						cpProto = core.ProtocolTCP
					}
					if cp.Name == p.Port.StrVal && string(cpProto) == protocol && int(cp.ContainerPort) == port {
						return yes
					}
				}
			}
		} else {
			start := int(p.Port.IntVal)
			end := start
			if p.EndPort != nil {
				end = int(*p.EndPort)
			}
			if start < 1 || end > 65535 || end < start {
				result = or(result, unknown)
				continue
			}
			if port >= start && port <= end {
				return yes
			}
		}
	}
	return result
}
func matchPeers(peers []netv1.NetworkPolicyPeer, ns string, pod core.Pod, namespaces map[string]map[string]string) truth {
	if len(peers) == 0 {
		return yes
	}
	result := no
	for _, p := range peers {
		m := yes
		if p.IPBlock != nil {
			if p.PodSelector != nil || p.NamespaceSelector != nil {
				result = or(result, unknown)
				continue
			}
			prefix, e := netip.ParsePrefix(p.IPBlock.CIDR)
			ip, ie := netip.ParseAddr(pod.Status.PodIP)
			if e != nil || ie != nil {
				result = or(result, unknown)
				continue
			}
			if !prefix.Contains(ip) {
				m = no
			}
			for _, ex := range p.IPBlock.Except {
				x, e := netip.ParsePrefix(ex)
				if e != nil {
					m = and(m, unknown)
				} else if x.Contains(ip) {
					m = no
				}
			}
		} else {
			if p.NamespaceSelector != nil {
				if len(p.NamespaceSelector.MatchLabels) == 0 && len(p.NamespaceSelector.MatchExpressions) == 0 {
					m = yes
				} else if ls, ok := namespaces[pod.Namespace]; ok {
					m = matchSelector(p.NamespaceSelector, ls)
				} else {
					m = unknown
				}
			} else if p.PodSelector != nil && pod.Namespace != ns {
				m = no
			}
			if p.PodSelector != nil {
				m = and(m, matchSelector(p.PodSelector, pod.Labels))
			}
		}
		result = or(result, m)
	}
	return result
}

func AttachFlowEvidence(path *PathReview, flows []Flow) {
	for _, flow := range flows {
		if flow.Source == path.Source && flow.Destination == path.Destination && flow.Protocol == path.Protocol && flow.DestinationPort == path.Port {
			path.Flows = append(path.Flows, flow)
		}
	}
	path.Unchecked = append(path.Unchecked, "Flow observations are matched by endpoint, protocol and destination port. Their timestamps and capture coverage must be reviewed; they do not establish current or future reachability")
}
