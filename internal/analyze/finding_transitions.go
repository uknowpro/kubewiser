package analyze

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/uknowpro/kubewiser/internal/manifest"
	"github.com/uknowpro/kubewiser/internal/scan"
	"github.com/uknowpro/kubewiser/internal/schema"
)

type FindingTransition struct {
	Resource string `json:"resource"`
	Rule     string `json:"rule"`
	Field    string `json:"field"`
	Status   string `json:"status"`
	Before   string `json:"before"`
	After    string `json:"after"`
	Evidence string `json:"evidence"`
}
type trackedFinding struct {
	finding         scan.Finding
	identity, field string
	comparable      bool
}

// Container/env/volume indices are normalized by unique names. Other positional
// lists remain incomparable rather than falsely resolved after a reorder.
func stableFindingField(r manifest.Resource, path string) (string, bool) {
	if path == "" {
		return path, true
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	var current any = r.Object
	stable := []string{}
	for _, part := range parts {
		key := strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		if list, ok := current.([]any); ok {
			idx, e := strconv.Atoi(key)
			if e != nil || idx < 0 || idx >= len(list) {
				return path, false
			}
			item := manifest.Map(list[idx])
			name := manifest.String(item, "name")
			if name == "" {
				return path, false
			}
			count := 0
			for _, v := range list {
				if manifest.String(manifest.Map(v), "name") == name {
					count++
				}
			}
			if count != 1 {
				return path, false
			}
			stable = append(stable, "@name="+manifest.Escape(name))
			current = list[idx]
		} else {
			stable = append(stable, part)
			current = manifest.Map(current)[key]
		}
	}
	return "/" + strings.Join(stable, "/"), true
}
func trackFindings(ctx context.Context, before, after manifest.Set, target, ns string) ([]FindingTransition, error) {
	if ns == "" {
		ns = "default"
	}
	sets := []manifest.Set{before, after}
	objects := []map[string]manifest.Resource{{}, {}}
	supported := []map[string]bool{{}, {}}
	groups := []map[string][]trackedFinding{{}, {}}
	pdbMatched := []map[string]bool{{}, {}}
	for side, set := range sets {
		report, e := scan.Analyze(ctx, set, target, true)
		if e != nil {
			return nil, e
		}
		v, e := schema.NewVersion(target)
		if e != nil {
			return nil, e
		}
		if e = v.AddCRDs(set.Resources); e != nil {
			return nil, e
		}
		byID := map[string]manifest.Resource{}
		for _, r := range set.Resources {
			id := identity(r, ns)
			objects[side][id] = r
			byID[r.ID()] = r
			issues, known, e := v.Validate(r)
			supported[side][id] = known && e == nil && len(issues) == 0 && manifest.String(manifest.Map(r.Object["metadata"]), "name") != ""
		}
		cg, e := Cgroups(set, nil, target)
		if e != nil {
			return nil, e
		}
		for _, a := range cg.Actions {
			if a.ID == "KW-CGROUP-V1-REFERENCE" || a.ID == "KW-CGROUP-HOST-MOUNT" {
				report.Findings = append(report.Findings, scan.Finding{Rule: a.ID, Resource: a.Resource, Field: "", Message: a.Message})
			}
		}
		for _, row := range drainMatrix(set, ns) {
			if resource, ok := byID[row.Resource]; ok && manifest.String(resource.Object, "kind") == "PodDisruptionBudget" {
				pdbMatched[side][identity(resource, ns)] = strings.Contains(row.Current, "matching supplied workloads")
			}
			rule := ""
			switch row.Current {
			case "One desired replica":
				rule = "KW-DRAIN-SINGLE-REPLICA"
			case "Node-local volume dependency":
				rule = "KW-DRAIN-LOCAL-VOLUME"
			}
			if strings.Contains(row.Evidence, "declared budget allows no healthy Pod eviction") {
				rule = "KW-DRAIN-STATIC-BUDGET"
			}
			if rule != "" {
				message := row.Current
				if rule == "KW-DRAIN-STATIC-BUDGET" {
					message = "Declared PDB budget prevents healthy eviction at desired scale"
				}
				report.Findings = append(report.Findings, scan.Finding{Rule: rule, Resource: row.Resource, Message: message})
			}
		}
		for _, f := range report.Findings {
			r, ok := byID[f.Resource]
			id := identity(r, ns)
			field, stable := stableFindingField(r, f.Field)
			key := id + "\x00" + f.Rule + "\x00" + field + "\x00" + f.Message
			groups[side][key] = append(groups[side][key], trackedFinding{f, id, field, ok && stable})
		}
	}
	keys := []string{}
	for k := range groups[0] {
		keys = append(keys, k)
	}
	for k := range groups[1] {
		if _, ok := groups[0][k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	rows := []FindingTransition{}
	for _, key := range keys {
		a, b := groups[0][key], groups[1][key]
		f := trackedFinding{}
		if len(a) > 0 {
			f = a[0]
		} else {
			f = b[0]
		}
		row := FindingTransition{Resource: f.identity, Rule: f.finding.Rule, Field: f.field, Status: "cannot_compare", Before: "Not detected", After: "Not detected", Evidence: "Same target schema/rules and recommendation settings; raw values omitted"}
		if len(a) > 0 {
			row.Before = a[0].finding.Message
		}
		if len(b) > 0 {
			row.After = b[0].finding.Message
		}
		_, hasBefore := objects[0][f.identity]
		_, hasAfter := objects[1][f.identity]
		selectorChanged := f.finding.Rule == "KW-DRAIN-STATIC-BUDGET" && !reflect.DeepEqual(manifest.At(objects[0][f.identity].Object, "spec", "selector"), manifest.At(objects[1][f.identity].Object, "spec", "selector"))
		switch {
		case f.finding.Rule == "KW-DRAIN-STATIC-BUDGET" && (!pdbMatched[0][f.identity] || !pdbMatched[1][f.identity]):
			row.Evidence += "; PDB workload matching not established on both sides"
		case selectorChanged:
			row.Evidence += "; PDB selector changed, protection scope is not comparable"
		case !hasBefore || !hasAfter:
			row.Evidence += "; resource absent/renamed/group changed or scope differs"
		case len(a) > 1 || len(b) > 1 || !f.comparable:
			row.Evidence += "; ambiguous finding or positional list field"
		case len(a) > 0 && len(b) > 0:
			row.Status = "remaining"
		case len(a) == 0 && supported[0][f.identity]:
			row.Status = "introduced"
		case len(b) == 0 && supported[1][f.identity]:
			row.Status = "resolved"
		default:
			row.Evidence += "; counterpart schema not evaluated"
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		rows = append(rows, FindingTransition{Status: "cannot_compare", Before: "No tracked findings", After: "No tracked findings", Evidence: fmt.Sprintf("%s supported checks only; absence of findings is not compatibility certification", target)})
	}
	return rows, nil
}
