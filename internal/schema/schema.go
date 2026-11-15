// Package schema validates against embedded, version-pinned official Kubernetes schemas.
package schema

import (
	"bytes"
	"compress/gzip"
	"embed"
	"encoding/json"
	"fmt"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/uknowpro/kubewiser/internal/manifest"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

//go:embed bundles/*.json.gz bundles/sources.json
var bundles embed.FS

type Source struct {
	Version   string `json:"version"`
	URL       string `json:"source"`
	SHA256    string `json:"source_sha256"`
	BundleSHA string `json:"bundle_sha256"`
	Reviewed  string `json:"reviewed"`
}

func Sources() []Source {
	var a []Source
	b, _ := bundles.ReadFile("bundles/sources.json")
	_ = json.Unmarshal(b, &a)
	return a
}

type Violation struct{ Path, Keyword string }
type Validator struct {
	compiler    *jsonschema.Compiler
	names       map[string]string
	compiled    map[string]*jsonschema.Schema
	Source      Source
	custom      map[string]*jsonschema.Schema
	Unsupported []string
}

func key(api, kind string) string { return api + "|" + kind }
func New(minor int) (*Validator, error) {
	b, e := bundles.ReadFile(fmt.Sprintf("bundles/v1.%d.0.json.gz", minor))
	if e != nil {
		return nil, fmt.Errorf("schema bundle unavailable")
	}
	gz, e := gzip.NewReader(bytes.NewReader(b))
	if e != nil {
		return nil, e
	}
	defer gz.Close()
	var root map[string]any
	if e = json.NewDecoder(gz).Decode(&root); e != nil {
		return nil, e
	}
	v := &Validator{names: map[string]string{}, compiled: map[string]*jsonschema.Schema{}, custom: map[string]*jsonschema.Schema{}}
	for _, s := range Sources() {
		if s.Version == fmt.Sprintf("v1.%d.0", minor) {
			v.Source = s
		}
	}
	defs := manifest.Map(root["definitions"])
	for name, raw := range defs {
		for _, item := range manifest.Slice(manifest.Map(raw)["x-kubernetes-group-version-kind"]) {
			g := manifest.Map(item)
			api := manifest.String(g, "version")
			if group := manifest.String(g, "group"); group != "" {
				api = group + "/" + api
			}
			v.names[key(api, manifest.String(g, "kind"))] = name
		}
	}
	if e = normalize(root, false); e != nil {
		return nil, e
	}
	root["$schema"] = "http://json-schema.org/draft-04/schema#"
	data, _ := json.Marshal(root)
	v.compiler = compiler()
	if e = v.compiler.AddResource("https://kubewiser.invalid/bundle.json", bytes.NewReader(data)); e != nil {
		return nil, e
	}
	return v, nil
}
func compiler() *jsonschema.Compiler {
	c := jsonschema.NewCompiler()
	c.Draft = jsonschema.Draft4
	c.LoadURL = func(string) (io.ReadCloser, error) { return nil, fmt.Errorf("external schema references disabled") }
	return c
}

// normalize adds strict properties and Kubernetes-specific int-or-string/nullability semantics.
func normalize(x any, preserve bool) error {
	switch x := x.(type) {
	case map[string]any:
		if ref, ok := x["$ref"].(string); ok && !strings.HasPrefix(ref, "#/") {
			return fmt.Errorf("external schema references disabled")
		}
		if x["x-kubernetes-preserve-unknown-fields"] == true {
			preserve = true
		}
		if x["x-kubernetes-int-or-string"] == true {
			x["anyOf"] = []any{map[string]any{"type": "integer"}, map[string]any{"type": "string"}}
			delete(x, "type")
		}
		// Native Quantity and IntOrString definitions can use an empty type and format.
		if x["format"] == "int-or-string" {
			x["anyOf"] = []any{map[string]any{"type": "integer"}, map[string]any{"type": "string"}}
			delete(x, "type")
		}
		if x["type"] == "" {
			delete(x, "type")
		}
		if _, ok := x["properties"]; ok && !preserve {
			if _, exists := x["additionalProperties"]; !exists {
				x["additionalProperties"] = false
			}
		}
		if x["nullable"] == true {
			if t, ok := x["type"].(string); ok {
				x["type"] = []string{t, "null"}
			}
			delete(x, "nullable")
		}
		for _, v := range x {
			if e := normalize(v, preserve); e != nil {
				return e
			}
		}
	case []any:
		for _, v := range x {
			if e := normalize(v, preserve); e != nil {
				return e
			}
		}
	}
	return nil
}
func (v *Validator) AddCRDs(rs []manifest.Resource) error {
	for _, r := range rs {
		if manifest.String(r.Object, "kind") != "CustomResourceDefinition" {
			continue
		}
		spec := manifest.Map(r.Object["spec"])
		group := manifest.String(spec, "group")
		kind := manifest.String(manifest.Map(spec["names"]), "kind")
		versions := manifest.Slice(spec["versions"])
		for _, item := range versions {
			ver := manifest.Map(item)
			if ver["served"] == false {
				continue
			}
			raw := manifest.Map(manifest.At(ver, "schema", "openAPIV3Schema"))
			if len(raw) == 0 {
				continue
			}
			data, _ := json.Marshal(raw)
			var root map[string]any
			_ = json.Unmarshal(data, &root)
			props := manifest.Map(root["properties"])
			root["properties"] = props
			for _, f := range []string{"apiVersion", "kind"} {
				props[f] = map[string]any{"type": "string"}
			}
			props["metadata"] = map[string]any{"type": "object"}
			if e := normalize(root, false); e != nil {
				return e
			}
			data, _ = json.Marshal(root)
			c := compiler()
			url := "https://kubewiser.invalid/crd.json"
			if e := c.AddResource(url, bytes.NewReader(data)); e != nil {
				return e
			}
			s, e := c.Compile(url)
			if e != nil {
				return fmt.Errorf("invalid CRD validation schema")
			}
			k := key(group+"/"+manifest.String(ver, "name"), kind)
			if _, exists := v.custom[k]; exists {
				return fmt.Errorf("duplicate CRD schema")
			}
			v.custom[k] = s
			v.Unsupported = append(v.Unsupported, "CRD CEL, defaulting, conversion, admission and metadata semantics are not evaluated")
		}
	}
	return nil
}
func (v *Validator) Validate(r manifest.Resource) ([]Violation, bool, error) {
	k := key(manifest.String(r.Object, "apiVersion"), manifest.String(r.Object, "kind"))
	s := v.custom[k]
	if s == nil {
		name, ok := v.names[k]
		if !ok {
			return nil, false, nil
		}
		s = v.compiled[name]
		if s == nil {
			var e error
			s, e = v.compiler.Compile("https://kubewiser.invalid/bundle.json#/definitions/" + name)
			if e != nil {
				return nil, true, fmt.Errorf("schema compilation failed for kind")
			}
			v.compiled[name] = s
		}
	}
	err := s.Validate(r.Object)
	if err == nil {
		return nil, true, nil
	}
	ve, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return nil, true, fmt.Errorf("validation failed")
	}
	var result []Violation
	var walk func(*jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(result) >= 100 {
			return
		}
		if len(e.Causes) == 0 {
			keyword := e.KeywordLocation
			if i := strings.LastIndex(keyword, "/"); i >= 0 {
				keyword = keyword[i+1:]
			}
			if _, e := strconv.Atoi(keyword); e == nil {
				keyword = "schema"
			}
			if keyword == "additionalProperties" || keyword == "required" {
				matches := regexp.MustCompile(`'([^']+)'`).FindAllStringSubmatch(e.Message, -1)
				if len(matches) > 0 {
					for _, m := range matches {
						result = append(result, Violation{e.InstanceLocation + "/" + manifest.Escape(m[1]), keyword})
					}
					return
				}
			}
			result = append(result, Violation{e.InstanceLocation, keyword})
		}
		for _, c := range e.Causes {
			walk(c)
		}
	}
	walk(ve)
	return result, true, nil
}

func (v *Validator) APIVersions() []string {
	seen := map[string]bool{}
	for k := range v.names {
		parts := strings.SplitN(k, "|", 2)
		seen[parts[0]] = true
		seen[parts[0]+"/"+parts[1]] = true
	}
	out := []string{}
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// HasGroupKind distinguishes a known built-in kind in another target API version from unknown CRDs.
func (v *Validator) HasGroupKind(api, kind string) bool {
	group := ""
	if i := strings.Index(api, "/"); i >= 0 {
		group = api[:i]
	}
	for k := range v.names {
		p := strings.SplitN(k, "|", 2)
		g := ""
		if i := strings.Index(p[0], "/"); i >= 0 {
			g = p[0][:i]
		}
		if g == group && p[1] == kind {
			return true
		}
	}
	return false
}
