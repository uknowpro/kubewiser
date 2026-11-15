package analyze

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/uknowpro/kubewiser/internal/manifest"
)

type FieldManager struct {
	Manager    string `json:"manager"`
	Operation  string `json:"operation"`
	APIVersion string `json:"api_version"`
}
type resourceSchema struct {
	root    map[string]any
	cluster bool
}

func driftSchemas(rs []manifest.Resource) (map[string]resourceSchema, error) {
	out := map[string]resourceSchema{}
	for _, r := range rs {
		if manifest.String(r.Object, "kind") != "CustomResourceDefinition" {
			continue
		}
		spec := manifest.Map(r.Object["spec"])
		group := manifest.String(spec, "group")
		kind := manifest.String(manifest.Map(spec["names"]), "kind")
		if group == "" || kind == "" {
			return nil, fmt.Errorf("CRD group and kind required for drift semantics")
		}
		for _, v := range manifest.Slice(spec["versions"]) {
			ver := manifest.Map(v)
			if ver["served"] == false {
				continue
			}
			root := manifest.Map(manifest.At(ver, "schema", "openAPIV3Schema"))
			if len(root) == 0 {
				continue
			}
			key := group + "/" + manifest.String(ver, "name") + "|" + kind
			if _, ok := out[key]; ok {
				return nil, fmt.Errorf("duplicate CRD drift schema")
			}
			out[key] = resourceSchema{root: root, cluster: spec["scope"] == "Cluster"}
		}
	}
	return out, nil
}
func schemaKey(r manifest.Resource) string {
	return manifest.String(r.Object, "apiVersion") + "|" + manifest.String(r.Object, "kind")
}
func keyedCRD(a []any, keys []string) (map[string]any, bool) {
	out := map[string]any{}
	if len(keys) == 0 {
		return nil, false
	}
	for _, v := range a {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, false
		}
		id := map[string]any{}
		for _, key := range keys {
			value, ok := m[key]
			if !ok {
				return nil, false
			}
			switch value.(type) {
			case string, float64, bool:
			default:
				return nil, false
			}
			id[key] = value
		}
		k := privateKey(id)
		if _, ok := out[k]; ok {
			return nil, false
		}
		out[k] = v
	}
	return out, true
}
func privateKey(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return fmt.Sprintf("@%x", sum[:12])
}
func canonicalSet(a []any) ([]string, bool) {
	out := []string{}
	seen := map[string]bool{}
	for _, v := range a {
		b, e := json.Marshal(v)
		if e != nil {
			return nil, false
		}
		k := string(b)
		if seen[k] {
			return nil, false
		}
		seen[k] = true
		out = append(out, k)
	}
	sort.Strings(out)
	return out, true
}
func childSchema(s map[string]any, k string) map[string]any {
	if c, ok := manifest.Map(s["properties"])[k].(map[string]any); ok {
		return c
	}
	return manifest.Map(s["additionalProperties"])
}

// ManagedFields are hints about recorded field management, not an explanation
// of why a difference exists or authority to overwrite another manager.
func fieldManagers(r manifest.Resource, path string, custom bool) ([]FieldManager, bool) {
	owners := []FieldManager{}
	complete := true
	for _, raw := range manifest.Slice(manifest.At(r.Object, "metadata", "managedFields")) {
		m := manifest.Map(raw)
		if manifest.String(m, "subresource") != "" {
			continue
		}
		if m["fieldsType"] != "FieldsV1" || m["apiVersion"] != r.Object["apiVersion"] {
			complete = false
			continue
		}
		matched := false
		visits := 0
		var walk func(map[string]any, string, int)
		walk = func(node map[string]any, p string, depth int) {
			visits++
			if visits > 10000 || depth > 100 {
				complete = false
				return
			}
			if len(node) == 0 && (p == path || strings.HasPrefix(path, p+"/") || strings.HasPrefix(p, path+"/")) {
				matched = true
			}
			for k, raw := range node {
				if k == "." {
					if p == path || strings.HasPrefix(p, path+"/") {
						matched = true
					}
					continue
				}
				token := ""
				switch {
				case strings.HasPrefix(k, "f:"):
					token = strings.TrimPrefix(k, "f:")
				case strings.HasPrefix(k, "k:"):
					var id map[string]any
					if json.Unmarshal([]byte(k[2:]), &id) != nil {
						complete = false
						continue
					}
					token = privateKey(id)
					if !custom && len(id) == 1 {
						for _, key := range []string{"name", "mountPath"} {
							if v, ok := id[key].(string); ok {
								token = v
							}
						}
					}
				default:
					complete = false
					continue
				}
				child, ok := raw.(map[string]any)
				if !ok {
					complete = false
					continue
				}
				walk(child, p+"/"+manifest.Escape(token), depth+1)
			}
		}
		fields, ok := m["fieldsV1"].(map[string]any)
		if !ok {
			complete = false
			continue
		}
		if len(fields) == 0 {
			continue
		}
		walk(fields, "", 0)
		if matched {
			o := FieldManager{manifest.String(m, "manager"), manifest.String(m, "operation"), manifest.String(m, "apiVersion")}
			found := false
			for _, x := range owners {
				if x == o {
					found = true
				}
			}
			if !found {
				owners = append(owners, o)
			}
		}
	}
	sort.Slice(owners, func(i, j int) bool {
		return owners[i].Manager+owners[i].Operation < owners[j].Manager+owners[j].Operation
	})
	return owners, complete
}
