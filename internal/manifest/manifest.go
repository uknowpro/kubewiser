// Package manifest loads bounded YAML/JSON inputs without network access.
package manifest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const MaxFile = 16 << 20
const MaxTotal = 64 << 20
const MaxResources = 10000

type Resource struct {
	Object    map[string]any `json:"object"`
	File      string         `json:"file"`
	Document  int            `json:"document"`
	Line      int            `json:"line"`
	Locations map[string]int `json:"-"`
}
type Issue struct {
	File     string `json:"file"`
	Document int    `json:"document"`
	Line     int    `json:"line"`
	Code     string `json:"code"`
}
type Input struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}
type Set struct {
	Resources []Resource `json:"resources"`
	Issues    []Issue    `json:"issues"`
	Inputs    []Input    `json:"inputs"`
}

func String(m map[string]any, key string) string { s, _ := m[key].(string); return s }
func Map(v any) map[string]any {
	m, _ := v.(map[string]any)
	if m == nil {
		return map[string]any{}
	}
	return m
}
func Slice(v any) []any { a, _ := v.([]any); return a }
func At(m map[string]any, keys ...string) any {
	var v any = m
	for _, k := range keys {
		v = Map(v)[k]
	}
	return v
}
func (r Resource) ID() string {
	m := Map(r.Object["metadata"])
	return String(r.Object, "apiVersion") + "/" + String(r.Object, "kind") + "/" + String(m, "namespace") + "/" + String(m, "name")
}
func Escape(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1") }
func Files(path string) ([]string, error) {
	info, e := os.Stat(path)
	if e != nil {
		return nil, e
	}
	if !info.IsDir() {
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("input must be a regular file")
		}
		return []string{path}, nil
	}
	var paths []string
	e = filepath.WalkDir(path, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			if p != path && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(p)) {
		case ".yaml", ".yml", ".json":
			paths = append(paths, p)
		}
		if len(paths) > 1000 {
			return fmt.Errorf("input file limit exceeded")
		}
		return nil
	})
	sort.Strings(paths)
	if e == nil && len(paths) == 0 {
		e = fmt.Errorf("no manifest files")
	}
	return paths, e
}
func ReadBounded(path string, max int64) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}
	b, e := io.ReadAll(io.LimitReader(f, max+1))
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > max {
		return nil, fmt.Errorf("input size limit exceeded")
	}
	return b, nil
}
func Load(ctx context.Context, path string) (Set, error) {
	s := Set{Resources: []Resource{}, Issues: []Issue{}, Inputs: []Input{}}
	paths, e := Files(path)
	if e != nil {
		return s, e
	}
	total := 0
	for _, p := range paths {
		if e := ctx.Err(); e != nil {
			return s, e
		}
		b, e := ReadBounded(p, MaxFile)
		if e != nil {
			s.Issues = append(s.Issues, Issue{File: p, Code: "read_or_size_limit"})
			continue
		}
		total += len(b)
		if total > MaxTotal {
			return s, fmt.Errorf("total input size limit exceeded")
		}
		part, e := Decode(ctx, b, p)
		if e != nil {
			return s, e
		}
		s.Resources = append(s.Resources, part.Resources...)
		s.Issues = append(s.Issues, part.Issues...)
		s.Inputs = append(s.Inputs, part.Inputs...)
		if len(s.Resources) > MaxResources {
			return s, fmt.Errorf("resource limit exceeded")
		}
	}
	return s, nil
}
func Decode(ctx context.Context, b []byte, file string) (Set, error) {
	s := Set{Resources: []Resource{}, Issues: []Issue{}, Inputs: []Input{{file, fmt.Sprintf("%x", sha256.Sum256(b))}}}
	if len(b) > MaxFile {
		return s, fmt.Errorf("input size limit exceeded")
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	for doc := 1; doc <= MaxResources+1; doc++ {
		if e := ctx.Err(); e != nil {
			return s, e
		}
		var n yaml.Node
		e := dec.Decode(&n)
		if e == io.EOF {
			break
		}
		if e != nil {
			s.Issues = append(s.Issues, Issue{file, doc, 0, "yaml_syntax"})
			break
		}
		if doc > MaxResources {
			return s, fmt.Errorf("document limit exceeded")
		}
		if len(n.Content) == 0 || n.Content[0].Tag == "!!null" {
			continue
		}
		v := n.Content[0]
		loc := map[string]int{}
		if !walk(v, "", 0, loc) {
			s.Issues = append(s.Issues, Issue{file, doc, v.Line, "aliases_or_depth_limit"})
			continue
		}
		var obj map[string]any
		if v.Kind != yaml.MappingNode || v.Decode(&obj) != nil {
			s.Issues = append(s.Issues, Issue{file, doc, v.Line, "mapping_or_duplicate_keys"})
			continue
		}
		raw, e := json.Marshal(obj)
		if e != nil {
			s.Issues = append(s.Issues, Issue{file, doc, v.Line, "non_json_yaml"})
			continue
		}
		if e = json.Unmarshal(raw, &obj); e != nil {
			return s, e
		}
		flatten(&s, Resource{obj, file, doc, v.Line, loc}, 0)
		if len(s.Resources) > MaxResources {
			return s, fmt.Errorf("resource limit exceeded")
		}
	}
	return s, nil
}
func walk(n *yaml.Node, path string, depth int, loc map[string]int) bool {
	if depth > 100 || n.Kind == yaml.AliasNode {
		return false
	}
	loc[path] = n.Line
	if n.Kind == yaml.MappingNode {
		for i := 0; i < len(n.Content); i += 2 {
			if n.Content[i].Tag != "!!str" {
				return false
			}
			if !walk(n.Content[i+1], path+"/"+Escape(n.Content[i].Value), depth+1, loc) {
				return false
			}
		}
	} else {
		for i, c := range n.Content {
			if !walk(c, path+"/"+strconv.Itoa(i), depth+1, loc) {
				return false
			}
		}
	}
	return true
}
func flatten(s *Set, r Resource, depth int) {
	if depth > 50 {
		s.Issues = append(s.Issues, Issue{r.File, r.Document, r.Line, "list_depth_limit"})
		return
	}
	if String(r.Object, "kind") != "List" {
		s.Resources = append(s.Resources, r)
		return
	}
	items, ok := r.Object["items"].([]any)
	if !ok {
		s.Issues = append(s.Issues, Issue{r.File, r.Document, r.Line, "list_items_required"})
		return
	}
	for i, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			s.Issues = append(s.Issues, Issue{r.File, r.Document, r.Line, "list_item_mapping"})
			continue
		}
		prefix := "/items/" + strconv.Itoa(i)
		loc := map[string]int{}
		for p, l := range r.Locations {
			if p == prefix || strings.HasPrefix(p, prefix+"/") {
				loc[strings.TrimPrefix(p, prefix)] = l
			}
		}
		flatten(s, Resource{m, r.File, r.Document, loc[""], loc}, depth+1)
	}
}
