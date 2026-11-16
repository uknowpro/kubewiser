package analyze

import (
	"context"
	"fmt"
	"github.com/uknowpro/kubewiser/internal/manifest"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sort"
	"strings"
)

type ServicePaths struct {
	Source    string       `json:"source"`
	Service   string       `json:"service"`
	Paths     []PathReview `json:"backend_paths"`
	Unchecked []string     `json:"unchecked"`
}

func NetworkServicePath(ctx context.Context, set manifest.Set, src, dst, protocol string, port int) (ServicePaths, error) {
	result := ServicePaths{Source: src, Service: dst, Paths: []PathReview{}, Unchecked: []string{"Backend selection and targetPort mapping use supplied Service and Pod declarations. This is not Service/DNS/NAT, readiness, EndpointSlice or load-balancer reachability verification.", "External firewall, routing and CNI dataplane enforcement require external evidence; no probes or cloud API calls are performed."}}
	if len(set.Issues) > 0 {
		return result, fmt.Errorf("input parsing incomplete")
	}
	if port < 1 || port > 65535 || (protocol != "TCP" && protocol != "UDP" && protocol != "SCTP") {
		return result, fmt.Errorf("valid service port and protocol required")
	}
	if len(strings.Split(dst, "/")) != 2 {
		return result, fmt.Errorf("service must be namespace/name")
	}
	var service *core.Service
	for _, r := range set.Resources {
		if manifest.String(r.Object, "kind") != "Service" {
			continue
		}
		var s core.Service
		if e := decodeObject(r.Object, &s); e != nil {
			return result, e
		}
		if s.Namespace == "" {
			s.Namespace = "default"
		}
		if s.Namespace+"/"+s.Name == dst {
			if service != nil {
				return result, fmt.Errorf("duplicate Service identity")
			}
			service = &s
		}
	}
	if service == nil {
		return result, fmt.Errorf("Service not found in inputs")
	}
	if service.Spec.Type == core.ServiceTypeExternalName || len(service.Spec.Selector) == 0 {
		result.Unchecked = append(result.Unchecked, "Service has no local selector or uses ExternalName; backend policy intent is unknown")
		return result, nil
	}
	var selected *core.ServicePort
	for _, p := range service.Spec.Ports {
		proto := p.Protocol
		if proto == "" {
			proto = core.ProtocolTCP
		}
		if int(p.Port) == port && string(proto) == protocol {
			copy := p
			if selected != nil {
				return result, fmt.Errorf("ambiguous Service port")
			}
			selected = &copy
		}
	}
	if selected == nil {
		return result, fmt.Errorf("requested Service port/protocol not found")
	}
	for _, r := range set.Resources {
		if e := ctx.Err(); e != nil {
			return result, e
		}
		if manifest.String(r.Object, "kind") != "Pod" {
			continue
		}
		var pod core.Pod
		if e := decodeObject(r.Object, &pod); e != nil {
			return result, e
		}
		if pod.Namespace == "" {
			pod.Namespace = "default"
		}
		if pod.Namespace != service.Namespace || !labels.SelectorFromSet(service.Spec.Selector).Matches(labels.Set(pod.Labels)) {
			continue
		}
		if len(result.Paths) >= 200 {
			return result, fmt.Errorf("service backend limit exceeded (200)")
		}
		target := int(selected.TargetPort.IntVal)
		if target == 0 && selected.TargetPort.Type != intstr.String {
			target = port
		}
		if selected.TargetPort.Type == intstr.String {
			target = 0
			for _, c := range pod.Spec.Containers {
				for _, cp := range c.Ports {
					proto := cp.Protocol
					if proto == "" {
						proto = core.ProtocolTCP
					}
					if cp.Name == selected.TargetPort.StrVal && string(proto) == protocol {
						target = int(cp.ContainerPort)
					}
				}
			}
		}
		dest := pod.Namespace + "/" + pod.Name
		if target == 0 {
			result.Paths = append(result.Paths, PathReview{Source: src, Destination: dest, Protocol: protocol, Ingress: "unknown", Egress: "unknown", Result: "unknown", Unchecked: []string{"Named targetPort cannot be resolved in this Pod"}})
			continue
		}
		path, e := NetworkPath(ctx, set, src, dest, protocol, target)
		if e != nil {
			return result, e
		}
		result.Paths = append(result.Paths, path)
	}
	sort.Slice(result.Paths, func(i, j int) bool { return result.Paths[i].Destination < result.Paths[j].Destination })
	if len(result.Paths) == 0 {
		result.Unchecked = append(result.Unchecked, "No matching Pod backends supplied; absence does not prove that the live Service has no endpoints")
	}
	return result, nil
}
