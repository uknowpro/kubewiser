// Package analyze provides evidence-based offline drift, upgrade and migration plans.
package analyze

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/uknowpro/kubewiser/internal/cluster"
	"github.com/uknowpro/kubewiser/internal/manifest"
	"github.com/uknowpro/kubewiser/internal/scan"
	"reflect"
	"sort"
	"strings"
	"time"
)

type Change struct {
	FieldManagers []FieldManager `json:"field_managers,omitempty"`
	Resource      string         `json:"resource"`
	Path          string         `json:"path"`
	Type          string         `json:"type"`
	Message       string         `json:"message"`
}
type Diff struct {
	Render    map[string]any `json:"render,omitempty"`
	Changes   []Change       `json:"changes"`
	Unchecked []string       `json:"unchecked"`
}

var clusterKinds = map[string]bool{"Node": true, "Namespace": true, "StorageClass": true, "ClusterRole": true, "ClusterRoleBinding": true, "CustomResourceDefinition": true, "PersistentVolume": true}

func identity(r manifest.Resource, ns string) string {
	api := manifest.String(r.Object, "apiVersion")
	group := ""
	if i := strings.Index(api, "/"); i >= 0 {
		group = api[:i]
	}
	meta := manifest.Map(r.Object["metadata"])
	kind := manifest.String(r.Object, "kind")
	n := manifest.String(meta, "namespace")
	if n == "" && !clusterKinds[kind] {
		n = ns
	}
	return group + "/" + kind + "/" + n + "/" + manifest.String(meta, "name")
}
func Compare(desired manifest.Set, s cluster.Snapshot, ns string) (Diff, error) {
	return CompareWithOptions(desired, s, ns, DiffOptions{})
}
func CompareWithOptions(desired manifest.Set, s cluster.Snapshot, ns string, options DiffOptions) (Diff, error) {
	d := Diff{Changes: []Change{}, Unchecked: []string{"Observed-only fields may be defaults or controller additions; no server mutation or dry-run was performed", "CRD list semantics use supplied local schemas; webhook conversion is not performed; optional local None conversion is explicitly reported; sensitive paths are omitted"}}
	if len(desired.Issues) > 0 {
		return d, fmt.Errorf("desired inputs contain parse errors; resolve them before drift comparison")
	}
	if ns == "" {
		ns = s.Namespace
	}
	schemas, err := driftSchemas(desired.Resources)
	if err != nil {
		return d, err
	}
	identify := func(r manifest.Resource) string {
		if sc, ok := schemas[schemaKey(r)]; ok && sc.cluster {
			return identity(r, "")
		}
		return identity(r, ns)
	}
	d.Unchecked = append(d.Unchecked, "Recorded field managers are ownership hints, not proof of drift cause or permission to overwrite; unsupported managedFields encodings are not inferred")

	live := map[string]manifest.Resource{}
	for _, r := range s.Resources {
		k := identify(r)
		if _, ok := live[k]; ok {
			return d, fmt.Errorf("duplicate observed resource identity")
		}
		live[k] = r
	}
	seen := map[string]bool{}
	for _, r := range desired.Resources {
		id := identify(r)
		if seen[id] {
			return d, fmt.Errorf("duplicate desired resource identity")
		}
		seen[id] = true
		if manifest.String(manifest.Map(r.Object["metadata"]), "name") == "" {
			d.Unchecked = append(d.Unchecked, "Unnamed resource cannot be matched: "+r.File)
			continue
		}
		got, ok := live[id]
		if !ok {
			d.Changes = append(d.Changes, Change{Resource: id, Path: "", Type: "not_observed", Message: "Not found in this snapshot; check namespace, collection scope and RBAC gaps before concluding deletion"})
			continue
		}
		observedID := got.ID()
		sc, custom := schemas[schemaKey(r)]
		resourceNS := ns
		if clusterKinds[manifest.String(r.Object, "kind")] || (custom && sc.cluster) {
			resourceNS = ""
		}
		r = cloneForDiff(r, resourceNS)
		got = cloneForDiff(got, resourceNS)
		if options.CRDConversion && custom && r.Object["apiVersion"] != got.Object["apiVersion"] {
			if observedSchema, ok := schemas[schemaKey(got)]; ok && observedSchema.convertNone && sc.convertNone {
				got.Object["apiVersion"] = r.Object["apiVersion"]
				d.Unchecked = append(d.Unchecked, "Local CRD strategy None conversion simulated for "+id+"; pruning, admission and webhook transformations are not simulated")
			}
		}
		if manifest.String(r.Object, "apiVersion") != manifest.String(got.Object, "apiVersion") {
			d.Changes = append(d.Changes, Change{Resource: id, Path: "/apiVersion", Type: "conversion_unverified", Message: "API versions differ; semantic comparison requires conversion"})
			continue
		}
		skip := map[string]bool{}
		for _, x := range s.Redacted {
			prefix := observedID + "#"
			if strings.HasPrefix(x, prefix) {
				skip[strings.TrimPrefix(x, prefix)] = true
			}
		}
		start := len(d.Changes)
		if options.CRDDefaults && custom {
			if e := defaultCRD(r.Object, sc.root); e != nil {
				return d, e
			}
			if e := defaultCRD(got.Object, sc.root); e != nil {
				return d, e
			}
			d.Unchecked = append(d.Unchecked, "Local CRD defaults normalized for "+id+"; this is not evidence of server admission/defaulting behavior")
		}
		compare(&d, id, "", r.Object, got.Object, skip, sc.root, custom)
		for n := start; n < len(d.Changes); n++ {
			owners, complete := fieldManagers(got, d.Changes[n].Path, custom)
			d.Changes[n].FieldManagers = owners
			if !complete {
				d.Unchecked = append(d.Unchecked, "Some field management metadata could not be mapped: "+id)
			}
		}

	}
	for id := range live {
		if !seen[id] {
			d.Changes = append(d.Changes, Change{Resource: id, Path: "", Type: "observed_only_resource", Message: "Resource is outside the supplied desired set; this is not a deletion recommendation"})
		}
	}
	for _, g := range s.Gaps {
		d.Unchecked = append(d.Unchecked, g.Resource+": "+g.Reason)
	}
	sort.Slice(d.Changes, func(i, j int) bool {
		a, b := d.Changes[i], d.Changes[j]
		return a.Resource+a.Path+a.Type < b.Resource+b.Path+b.Type
	})
	return d, nil
}
func compare(d *Diff, id, path string, a, b any, skip map[string]bool, schema map[string]any, custom bool) {
	if skip[path] || path == "/status" || (len(skip) > 0 && (strings.HasSuffix(path, "/args") || strings.HasSuffix(path, "/command") || (strings.Contains(path, "/env/") && strings.HasSuffix(path, "/value")))) {
		return
	}
	for _, f := range []string{"uid", "resourceVersion", "generation", "creationTimestamp", "managedFields", "selfLink"} {
		if path == "/metadata/"+f {
			return
		}
	}
	am, aok := a.(map[string]any)
	bm, bok := b.(map[string]any)
	if aok && bok {
		keys := map[string]bool{}
		for k := range am {
			keys[k] = true
		}
		for k := range bm {
			keys[k] = true
		}
		for k := range keys {
			p := path + "/" + manifest.Escape(k)
			if skip[p] || p == "/status" || (len(skip) > 0 && (strings.HasSuffix(p, "/args") || strings.HasSuffix(p, "/command") || (strings.Contains(p, "/env/") && strings.HasSuffix(p, "/value")))) {
				continue
			}
			if strings.HasPrefix(p, "/metadata/") {
				switch k {
				case "uid", "resourceVersion", "generation", "creationTimestamp", "managedFields", "selfLink":
					continue
				}
			}
			av, ae := am[k]
			bv, be := bm[k]
			if !ae {
				d.Changes = append(d.Changes, Change{Resource: id, Path: p, Type: "observed_only_field", Message: "May be a default or controller-managed addition; value omitted"})
			} else if !be {
				d.Changes = append(d.Changes, Change{Resource: id, Path: p, Type: "desired_only_field", Message: "Declared field not present in snapshot; value omitted"})
			} else {
				compare(d, id, p, av, bv, skip, childSchema(schema, k), custom)
			}
		}
		return
	}
	aa, aok := a.([]any)
	bb, bok := b.([]any)
	if aok && bok {
		if custom {
			switch manifest.String(schema, "x-kubernetes-list-type") {
			case "map":
				keys := []string{}
				for _, v := range manifest.Slice(schema["x-kubernetes-list-map-keys"]) {
					if k, ok := v.(string); ok {
						keys = append(keys, k)
					}
				}
				x, xok := keyedCRD(aa, keys)
				y, yok := keyedCRD(bb, keys)
				if xok && yok {
					compare(d, id, path, x, y, skip, map[string]any{"additionalProperties": schema["items"]}, true)
					return
				}
				d.Unchecked = append(d.Unchecked, "Invalid CRD map-list keys or duplicates at "+id+path)
			case "set":
				x, xok := canonicalSet(aa)
				y, yok := canonicalSet(bb)
				if xok && yok {
					if !reflect.DeepEqual(x, y) {
						d.Changes = append(d.Changes, Change{Resource: id, Path: path, Type: "changed", Message: "Set membership differs; values omitted"})
					}
					return
				}
				d.Unchecked = append(d.Unchecked, "Invalid CRD set-list at "+id+path)
			}
		} else {
			field := path[strings.LastIndex(path, "/")+1:]
			key := ""
			switch field {
			case "containers", "initContainers", "ephemeralContainers", "env", "volumes":
				key = "name"
			case "volumeMounts":
				key = "mountPath"
			}
			if key != "" {
				x, xok := keyed(aa, key)
				y, yok := keyed(bb, key)
				if xok && yok {
					compare(d, id, path, x, y, skip, nil, false)
					return
				}
			}
		}
	}
	if !reflect.DeepEqual(a, b) {
		d.Changes = append(d.Changes, Change{Resource: id, Path: path, Type: "changed", Message: "Declared and observed values differ; values omitted"})
	}
}
func keyed(a []any, key string) (map[string]any, bool) {
	m := map[string]any{}
	for _, v := range a {
		s := manifest.String(manifest.Map(v), key)
		if s == "" {
			return nil, false
		}
		if _, ok := m[s]; ok {
			return nil, false
		}
		m[s] = v
	}
	return m, true
}

type Action struct {
	Current      string `json:"current,omitempty"`
	Recommended  string `json:"recommended,omitempty"`
	ID           string `json:"id"`
	Resource     string `json:"resource"`
	Requirement  string `json:"requirement"`
	Message      string `json:"message"`
	Verification string `json:"verification"`
	Source       string `json:"source"`
	Before       string `json:"before_version,omitempty"`
}
type Plan struct {
	Cgroup    *CgroupReport    `json:"cgroup,omitempty"`
	Lifecycle *LifecycleReport `json:"lifecycle,omitempty"`
	Route     *AKSRoute        `json:"aks_route,omitempty"`
	Project   *ProjectProfile  `json:"project,omitempty"`
	Status    string           `json:"status,omitempty"`
	Kind      string           `json:"kind"`
	From      string           `json:"from"`
	To        string           `json:"to"`
	Steps     []string         `json:"steps"`
	Actions   []Action         `json:"actions"`
	Unchecked []string         `json:"unchecked"`
	Report    *scan.Report     `json:"report,omitempty"`
}

func Upgrade(ctx context.Context, set manifest.Set, from, to string) (Plan, error) {
	p := Plan{Kind: "upgrade", From: from, To: to, Steps: []string{}, Actions: []Action{}}
	a, e := scan.Target(from)
	if e != nil {
		return p, e
	}
	b, e := scan.Target(to)
	if e != nil {
		return p, e
	}
	if b <= a {
		return p, fmt.Errorf("target must be newer than current minor version")
	}
	report, e := scan.Analyze(ctx, set, to, true)
	if e != nil {
		return p, e
	}
	p.Report = &report
	p.Unchecked = append([]string{}, report.Unchecked...)
	for n := a + 1; n <= b; n++ {
		p.Steps = append(p.Steps, fmt.Sprintf("1.%d: verify provider/add-on prerequisites, upgrade control plane, then compatible node components, then validate workloads", n))
	}
	for _, f := range report.Findings {
		p.Actions = append(p.Actions, Action{ID: f.Rule, Resource: f.Resource, Requirement: f.Requirement, Message: f.Message, Verification: "Re-run target validation and test workload behavior in staging", Source: f.Source, Before: findingDeadline(f, a, b)})
	}
	for _, res := range set.Resources {
		if manifest.String(res.Object, "kind") != "Node" {
			continue
		}
		raw := manifest.String(manifest.Map(manifest.At(res.Object, "status", "nodeInfo")), "kubeletVersion")
		n, e := scan.Target(strings.SplitN(strings.SplitN(raw, "+", 2)[0], "-", 2)[0])
		if e != nil {
			p.Unchecked = append(p.Unchecked, "Node kubelet version unavailable: "+res.ID())
			continue
		}
		maxSkew := 2
		if b >= 28 {
			maxSkew = 3
		}
		if n > b || b-n > maxSkew {
			p.Actions = append(p.Actions, Action{ID: "KW-KUBELET-SKEW", Resource: res.ID(), Requirement: "required", Message: "Kubelet version is outside the target control-plane skew window", Verification: "Plan compatible kubelet versions at every minor upgrade step", Source: "https://kubernetes.io/releases/version-skew-policy/", Before: to})
		}
	}
	if e := AttachCgroups(&p, set, nil, to); e != nil {
		return p, e
	}
	p.Actions = append(p.Actions, behaviorActions(set, a, b)...)
	p.Actions = append(p.Actions, addonActions(set, a, b)...)
	life, err := Lifecycle(set, nil, time.Now().UTC())
	if err != nil {
		return p, err
	}
	p.Lifecycle = life.Lifecycle
	p.Actions = append(p.Actions, life.Actions...)
	p.Unchecked = append(p.Unchecked, life.Unchecked...)
	p.Actions = append(p.Actions, Action{ID: "KW-ADDON-MATRIX", Requirement: "conditional", Message: "Verify CNI, CSI, ingress, admission webhooks and managed-provider upgrade support for each step", Verification: "Use vendor version matrices and staging tests; these dependencies are not inferred from Kubernetes schemas", Source: "https://kubernetes.io/releases/version-skew-policy/"})
	p.Unchecked = append(p.Unchecked, "Audit logs/API client usage, add-on release matrices, actual node kernel/cgroup and image runtime dependencies are not automatically established")
	return p, nil
}
func Migration(set manifest.Set, from, to string) (Plan, error) {
	p := Plan{Kind: "migration", From: from, To: to, Steps: []string{"Inventory platform dependencies", "Provision and validate target infrastructure", "Migrate stateful data and workload identity", "Stage workloads, verify traffic and cutover"}, Actions: []Action{}, Unchecked: []string{"No cloud API calls or infrastructure mutation are performed", "IAM permissions, firewall/routing, storage data movement, DNS cutover and provider version support require separate verification"}}
	valid := map[string]bool{"eks": true, "aks": true, "gke": true}
	if !valid[from] || !valid[to] || from == to {
		return p, fmt.Errorf("distinct --from/--to providers required: eks, aks, gke")
	}
	if len(set.Issues) > 0 {
		return p, fmt.Errorf("input parsing incomplete")
	}
	sources := map[string]string{"eks": "https://docs.aws.amazon.com/eks/latest/userguide/what-is-eks.html", "aks": "https://learn.microsoft.com/en-us/azure/aks/what-is-aks", "gke": "https://cloud.google.com/kubernetes-engine/docs/concepts/kubernetes-engine-overview"}
	add := func(r manifest.Resource, id, req, msg string) {
		p.Actions = append(p.Actions, Action{ID: id, Resource: r.ID(), Requirement: req, Message: msg, Verification: "Validate target provider configuration and test in staging before cutover", Source: sources[to]})
	}
	for _, r := range set.Resources {
		kind := manifest.String(r.Object, "kind")
		switch kind {
		case "Service":
			if manifest.At(r.Object, "spec", "type") == "LoadBalancer" {
				add(r, "KW-MIGRATE-LB", "required", "Recreate provider load balancer configuration, annotations and health checks; verify client IP and exposure")
			}
		case "Ingress":
			add(r, "KW-MIGRATE-INGRESS", "conditional", "Verify ingress class/controller, TLS integration, public DNS and target load balancing")
		case "StorageClass":
			add(r, "KW-MIGRATE-STORAGE", "required", "Map provisioner, storage parameters, topology and reclaim policy to target CSI driver")
		case "PersistentVolumeClaim", "PersistentVolume":
			add(r, "KW-MIGRATE-DATA", "required", "Plan backup/restore or replication and verify data integrity; manifests alone do not migrate volume contents")
		case "ServiceAccount":
			add(r, "KW-MIGRATE-IDENTITY", "conditional", "Review workload identity/IAM mapping and least-privilege target permissions")
		}
		spec, _ := scan.PodSpec(r)
		if spec != nil {
			selector := manifest.Map(spec["nodeSelector"])
			for k := range selector {
				if strings.Contains(k, "eks.amazonaws.com") || strings.Contains(k, "cloud.google.com") || strings.Contains(k, "kubernetes.azure.com") {
					add(r, "KW-MIGRATE-NODESELECTOR", "required", "Replace provider-specific node selection constraints")
					break
				}
			}
			for _, v := range manifest.Slice(spec["volumes"]) {
				if manifest.Map(v)["hostPath"] != nil {
					add(r, "KW-MIGRATE-HOSTPATH", "required", "Replace or reproduce node-local filesystem dependency and validate data persistence")
					break
				}
			}
		}
		annotations := manifest.Map(manifest.At(r.Object, "metadata", "annotations"))
		for k := range annotations {
			if strings.Contains(k, "aws") || strings.Contains(k, "azure") || strings.Contains(k, "gke") || strings.Contains(k, "google") {
				add(r, "KW-MIGRATE-ANNOTATION", "conditional", "Review provider-specific annotations against target controller semantics")
				break
			}
		}
	}
	if len(p.Actions) == 0 {
		p.Unchecked = append(p.Unchecked, "No recognized platform dependencies found; this does not establish portability")
	}
	return p, nil
}
func JSONClone(m map[string]any) map[string]any {
	b, _ := json.Marshal(m)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return out
}
