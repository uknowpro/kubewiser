package analyze

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/uknowpro/kubewiser/internal/manifest"
	"github.com/uknowpro/kubewiser/internal/scan"
	core "k8s.io/api/core/v1"
	policy "k8s.io/api/policy/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func typed(v any, dst any) bool {
	b, e := json.Marshal(v)
	return e == nil && json.Unmarshal(b, dst) == nil
}
func resourceNamespace(r manifest.Resource, fallback string) string {
	n := manifest.String(manifest.Map(r.Object["metadata"]), "namespace")
	if n == "" {
		n = fallback
	}
	return n
}

// Only supported hard node constraints are evaluated. A match is a candidate,
// never a scheduler feasibility verdict (capacity, volumes and pod affinity remain).
func nodeCandidate(spec core.PodSpec, n core.Node) truth {
	if n.Spec.Unschedulable {
		return no
	}
	if spec.NodeName != "" && spec.NodeName != n.Name {
		return no
	}
	if !labels.SelectorFromSet(spec.NodeSelector).Matches(labels.Set(n.Labels)) {
		return no
	}
	if spec.Affinity != nil && spec.Affinity.NodeAffinity != nil && spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		result := no
		for _, term := range spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
			t := yes
			if len(term.MatchExpressions)+len(term.MatchFields) == 0 {
				t = no
			}
			for _, r := range term.MatchExpressions {
				t = and(t, nodeRequirement(r, n.Labels))
			}
			for _, r := range term.MatchFields {
				if r.Key != "metadata.name" || (r.Operator != core.NodeSelectorOpIn && r.Operator != core.NodeSelectorOpNotIn) || len(r.Values) != 1 {
					t = and(t, unknown)
				} else {
					t = and(t, nodeRequirement(r, map[string]string{"metadata.name": n.Name}))
				}
			}
			result = or(result, t)
		}
		if result != yes {
			return result
		}
	}
	for _, taint := range n.Spec.Taints {
		if taint.Effect != core.TaintEffectNoSchedule && taint.Effect != core.TaintEffectNoExecute {
			continue
		}
		result := no
		for _, tol := range spec.Tolerations {
			if (tol.Effect != "" && tol.Effect != taint.Effect) || (tol.Key != "" && tol.Key != taint.Key) {
				continue
			}
			switch tol.Operator {
			case core.TolerationOpExists:
				result = yes
			case "", core.TolerationOpEqual:
				if tol.Key == "" {
					result = or(result, unknown)
				} else if tol.Value == taint.Value {
					result = yes
				}
			default:
				result = or(result, unknown)
			}
		}
		if result != yes {
			return result
		}
	}
	return yes
}
func nodeRequirement(r core.NodeSelectorRequirement, values map[string]string) truth {
	ops := map[core.NodeSelectorOperator]selection.Operator{core.NodeSelectorOpIn: selection.In, core.NodeSelectorOpNotIn: selection.NotIn, core.NodeSelectorOpExists: selection.Exists, core.NodeSelectorOpDoesNotExist: selection.DoesNotExist, core.NodeSelectorOpGt: selection.GreaterThan, core.NodeSelectorOpLt: selection.LessThan}
	op, ok := ops[r.Operator]
	if !ok {
		return unknown
	}
	req, e := labels.NewRequirement(r.Key, op, r.Values)
	if e != nil {
		return unknown
	}
	if req.Matches(labels.Set(values)) {
		return yes
	}
	return no
}
func poolForNode(n core.Node, pools []CgroupPool) (string, bool) {
	matches := []string{}
	for _, p := range pools {
		if len(p.NodeSelector) > 0 && labels.SelectorFromSet(p.NodeSelector).Matches(labels.Set(n.Labels)) {
			matches = append(matches, p.Name)
		}
	}
	if len(matches) == 1 {
		return matches[0], true
	}
	if len(matches) > 1 {
		return "ambiguous pool", false
	}
	for _, key := range []string{"kubernetes.azure.com/agentpool", "agentpool", "eks.amazonaws.com/nodegroup", "cloud.google.com/gke-nodepool"} {
		if v := n.Labels[key]; v != "" {
			matches = append(matches, v)
		}
	}
	name := ""
	for _, v := range matches {
		if name != "" && name != v {
			return "ambiguous pool", false
		}
		name = v
	}
	if name != "" {
		for _, p := range pools {
			if p.Name == name && len(p.NodeSelector) > 0 {
				return "unidentified pool", false
			}
		}
		return name, true
	}
	return "unidentified pool", false
}
func workloadMatrix(set manifest.Set, pools []CgroupPool, report *CgroupReport, actions []Action) []MatrixRow {
	rows := []MatrixRow{}
	nodes := []core.Node{}
	for _, r := range set.Resources {
		if manifest.String(r.Object, "kind") == "Node" {
			var n core.Node
			if typed(r.Object, &n) && n.Name != "" {
				nodes = append(nodes, n)
			}
		}
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })
	for i := 1; i < len(nodes); i++ {
		if nodes[i-1].Name == nodes[i].Name {
			return []MatrixRow{{Topic: "workload-pool", Status: "input_missing", Current: "Duplicate Node identities", Target: "Unknown", Evidence: "Node inventory is ambiguous", Action: "Supply one record per Node"}}
		}
	}
	checks := 0
	for _, r := range set.Resources {
		spec, _ := scan.PodSpec(r)
		if spec == nil {
			continue
		}
		var pod core.PodSpec
		if !typed(spec, &pod) {
			rows = append(rows, MatrixRow{Topic: "workload-pool", Resource: r.ID(), Status: "input_missing", Current: "Invalid Pod specification", Target: "Unknown", Action: "Resolve schema errors"})
			continue
		}
		groups := map[string][]string{}
		observed := manifest.String(r.Object, "kind") == "Pod" && pod.NodeName != ""
		uncertain := false
		for _, n := range nodes {
			checks++
			if checks > 100000 {
				rows = append(rows, MatrixRow{Topic: "workload-pool", Status: "input_missing", Current: "Review truncated", Target: "Unknown", Evidence: "100000 workload/node comparisons reached", Action: "Split input scope; unreviewed workloads are not compatible by default"})
				return rows
			}
			match := nodeCandidate(pod, n)
			if observed {
				if pod.NodeName == n.Name {
					match = yes
				} else {
					match = no
				}
			}
			if match == unknown {
				uncertain = true
			}
			if match != yes {
				continue
			}
			pool, _ := poolForNode(n, pools)
			groups[pool] = append(groups[pool], n.Name)
		}
		if len(groups) == 0 {
			message := "No candidate among supplied Nodes; inventory may be incomplete"
			if observed {
				message = "Bound node absent from supplied inventory"
			}
			if len(nodes) == 0 {
				message = "No usable Node inventory supplied"
			}
			if uncertain {
				message = "Some hard constraints could not be evaluated"
			}
			rows = append(rows, MatrixRow{Topic: "workload-pool", Resource: r.ID(), Current: message, Target: "Unknown", Status: "input_missing", Action: "Supply Nodes; validate capacity, topology and storage separately"})
			continue
		}
		keys := []string{}
		for k := range groups {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, pool := range keys {
			current := "candidate pool: " + pool
			if observed {
				current = "supplied nodeName binding: " + pool
			}
			status, target := "input_missing", "Pool cgroup target not established"
			if report != nil {
				for _, cg := range report.Pools {
					if cg.Pool == pool && pool != "ambiguous pool" && pool != "unidentified pool" {
						status = "review_required"
						target = cg.Current + " → " + cg.Target
						if cg.Status == "mode_change" || cg.Status == "hybrid_review" || cg.Status == "conflicting_evidence" {
							status = "action_required"
						}
						if cg.Status == "unknown" {
							status = "input_missing"
						}
						for _, a := range actions {
							if a.Resource == r.ID() && a.ID == "KW-CGROUP-V1-REFERENCE" && cg.Target == "v2" {
								status = "action_required"
								target += "; workload references v1 resource files"
							}
						}
					}
				}
			}
			rows = append(rows, MatrixRow{Topic: "workload-pool", Resource: r.ID(), Current: current, Target: target, Status: status, Evidence: "Supplied Nodes: " + strings.Join(groups[pool], ", "), Action: "Current inventory only; candidates are not target scheduling guarantees. Verify capacity, pod affinity, topology, volume constraints, finite tolerations and canary behavior"})
		}
	}
	return rows
}

func AttachReadiness(p *Plan, set manifest.Set, namespace string) {
	if namespace == "" {
		namespace = "default"
	}
	p.Drain = drainMatrix(set, namespace)
	filtered := p.Actions[:0]
	for _, a := range p.Actions {
		if !strings.HasPrefix(a.ID, "KW-DRAIN-") {
			filtered = append(filtered, a)
		}
	}
	p.Actions = filtered
	for _, r := range p.Drain {
		req := "conditional"
		if r.Status == "action_required" {
			req = "required"
		}
		p.Actions = append(p.Actions, Action{ID: "KW-DRAIN-REVIEW", Resource: r.Resource, Requirement: req, Current: r.Current, Recommended: r.Target, Message: r.Evidence, Verification: r.Action, Source: "https://kubernetes.io/docs/tasks/run-application/configure-pdb/"})
	}
}
func drainMatrix(set manifest.Set, namespace string) []MatrixRow {
	rows := []MatrixRow{}
	comparisons := 0
	add := func(id, current, target, status, evidence string) {
		rows = append(rows, MatrixRow{Topic: "drain", Resource: id, Current: current, Target: target, Status: status, Evidence: evidence, Action: "Offline risk review only. Recheck live PDB status, healthy replicas and remaining-node capacity before an explicitly authorized drain"})
	}
	for _, r := range set.Resources {
		kind := manifest.String(r.Object, "kind")
		spec, _ := scan.PodSpec(r)
		if kind == "Deployment" || kind == "StatefulSet" || kind == "ReplicaSet" {
			count := int32(1)
			var deployment struct {
				Spec struct {
					Replicas *int32 `json:"replicas"`
				} `json:"spec"`
			}
			if !typed(r.Object, &deployment) {
				continue
			}
			if deployment.Spec.Replicas != nil {
				count = *deployment.Spec.Replicas
			}
			if count == 1 {
				add(r.ID(), "One desired replica", "Review temporary capacity and disruption tolerance", "review_required", "Single replica may interrupt service during node replacement; readiness and actual traffic not established")
			}
		}
		if kind == "Pod" {
			var pod core.Pod
			if !typed(r.Object, &pod) {
				continue
			}
			if len(pod.OwnerReferences) == 0 {
				add(r.ID(), "Pod has no owner reference in supplied input", "Review unmanaged Pod handling", "review_required", "Drain may require explicit force for unmanaged Pods; missing metadata is not proof of unmanaged ownership")
			}
			if pod.Spec.NodeName != "" {
				candidates := 0
				nodeCount := 0
				for _, nr := range set.Resources {
					comparisons++
					if comparisons > 100000 {
						add("workloads", "Review truncated", "Split input scope", "input_missing", "100000 drain inventory comparisons reached")
						return rows
					}
					if manifest.String(nr.Object, "kind") != "Node" {
						continue
					}
					var n core.Node
					if !typed(nr.Object, &n) {
						continue
					}
					nodeCount++
					if n.Name == pod.Spec.NodeName {
						continue
					}
					copy := pod.Spec
					copy.NodeName = ""
					if nodeCandidate(copy, n) == yes {
						candidates++
					}
				}
				if nodeCount > 0 && candidates == 0 {
					add(r.ID(), "No alternative candidate among supplied Nodes", "Review rescheduling before drain", "review_required", "Evaluates hard node affinity, selectors, cordon and untolerated hard taints only; inventory and scheduler constraints may be incomplete")
				}
			}
		}
		if spec != nil {
			for _, v := range manifest.Slice(spec["volumes"]) {
				m := manifest.Map(v)
				if m["emptyDir"] != nil || m["hostPath"] != nil {
					add(r.ID(), "Node-local volume dependency", "Review data loss or node affinity", "review_required", "emptyDir/hostPath cannot be assumed to move with the Pod; volume values omitted")
					break
				}
			}
		}
		if kind != "PodDisruptionBudget" {
			continue
		}
		var pdb policy.PodDisruptionBudget
		if !typed(r.Object, &pdb) || ((pdb.Spec.MinAvailable == nil) == (pdb.Spec.MaxUnavailable == nil)) {
			add(r.ID(), "Invalid or missing PDB budget", "Provide exactly one valid budget", "input_missing", "Budget could not be evaluated")
			continue
		}
		api := manifest.String(r.Object, "apiVersion")
		if api != "policy/v1" && api != "policy/v1beta1" {
			add(r.ID(), "Unsupported PDB API", "Validate API", "input_missing", "Selector semantics unknown")
			continue
		}
		selector, e := meta.LabelSelectorAsSelector(pdb.Spec.Selector)
		if e != nil {
			add(r.ID(), "Invalid selector", "Fix selector", "input_missing", "PDB selector is invalid")
			continue
		}
		if pdb.Spec.Selector == nil || (api == "policy/v1beta1" && len(pdb.Spec.Selector.MatchLabels)+len(pdb.Spec.Selector.MatchExpressions) == 0) {
			add(r.ID(), "Selector matches no Pods", "Verify intended protection", "review_required", "nil selectors select none; policy/v1beta1 empty selectors also select none")
			continue
		}
		matched := []string{}
		var sole manifest.Resource
		for _, w := range set.Resources {
			comparisons++
			if comparisons > 100000 {
				add("workloads", "Review truncated", "Split input scope", "input_missing", "100000 drain inventory comparisons reached")
				return rows
			}
			sp, base := scan.PodSpec(w)
			if sp == nil || resourceNamespace(w, namespace) != resourceNamespace(r, namespace) {
				continue
			}
			var lm map[string]string
			var raw any
			if base == "/spec" {
				raw = manifest.At(w.Object, "metadata", "labels")
			} else if manifest.String(w.Object, "kind") == "CronJob" {
				raw = manifest.At(w.Object, "spec", "jobTemplate", "spec", "template", "metadata", "labels")
			} else {
				raw = manifest.At(w.Object, "spec", "template", "metadata", "labels")
			}
			if raw != nil && !typed(raw, &lm) {
				continue
			}
			if selector.Matches(labels.Set(lm)) {
				matched = append(matched, w.ID())
				sole = w
			}
		}
		current := "No matching workload in supplied scope"
		if len(matched) > 0 {
			current = fmt.Sprintf("%d matching supplied workloads", len(matched))
		}
		status, evidence := "review_required", "Selected workloads: "+strings.Join(matched, ", ")+"; actual namespace completeness not established"
		budget := pdb.Spec.MinAvailable
		if budget == nil {
			budget = pdb.Spec.MaxUnavailable
		}
		value, err := intstr.GetScaledValueFromIntOrPercent(budget, 100, true)
		if err != nil || value < 0 || (budget.Type == intstr.String && value > 100) {
			add(r.ID(), "Invalid budget", "Validate PDB schema", "input_missing", "Budget value not recognized")
			continue
		}
		if len(matched) == 1 {
			kind := manifest.String(sole.Object, "kind")
			if kind == "Deployment" || kind == "StatefulSet" || kind == "ReplicaSet" {
				var ctl struct {
					Spec struct {
						Replicas *int32 `json:"replicas"`
					} `json:"spec"`
				}
				if typed(sole.Object, &ctl) {
					replicas := int32(1)
					if ctl.Spec.Replicas != nil {
						replicas = *ctl.Spec.Replicas
					}
					if replicas > 0 {
						scaled, err := intstr.GetScaledValueFromIntOrPercent(budget, int(replicas), true)
						if err == nil {
							allowance := scaled
							if pdb.Spec.MinAvailable != nil {
								allowance = int(replicas) - scaled
							}
							if allowance < 0 {
								allowance = 0
							}
							evidence += fmt.Sprintf("; if this is the sole selected controller and all %d desired replicas are healthy, calculated budget allows %d disruptions (percentages round up)", replicas, allowance)
							if allowance == 0 {
								evidence += "; static zero-budget risk; confirm complete selector scope"
							}
						}
					}
				}
			}
		}
		zero := pdb.Spec.MaxUnavailable != nil && value == 0
		all := pdb.Spec.MinAvailable != nil && budget.Type == intstr.String && value == 100
		if len(matched) > 0 && (zero || all) {
			status = "action_required"
			evidence += "; declared budget allows no healthy Pod eviction at desired scale. Scaling, rollout state and unhealthyPodEvictionPolicy require separate checks"
		}
		rawstatus := manifest.Map(r.Object["status"])
		rawmeta := manifest.Map(r.Object["metadata"])
		if rawstatus["disruptionsAllowed"] != nil && rawstatus["observedGeneration"] != nil && rawmeta["generation"] != nil && pdb.Generation > 0 && pdb.Status.ObservedGeneration == pdb.Generation {
			if pdb.Status.DisruptionsAllowed <= 0 {
				status = "action_required"
				evidence += "; supplied generation-matched status has no disruption allowance (freshness unverified; unhealthy Pod exceptions may apply)"
			} else {
				evidence += fmt.Sprintf("; supplied allowance %d is not a future drain guarantee", pdb.Status.DisruptionsAllowed)
			}
		} else {
			evidence += "; current disruption allowance unavailable or stale-generation"
		}
		add(r.ID(), current, "Validate eviction budget and current health", status, evidence)
	}
	if len(rows) == 0 {
		add("workloads", "No supported drain risk detected", "Verify full inventory", "input_missing", "No complete drain feasibility assessment is performed")
	}
	return rows
}

func validPoolSelector(selector map[string]string) bool {
	if len(selector) > 64 {
		return false
	}
	_, err := meta.LabelSelectorAsSelector(&meta.LabelSelector{MatchLabels: selector})
	return err == nil
}
