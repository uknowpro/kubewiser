// Package render renders local Helm charts without a Kubernetes client or dependency downloads.
package render

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/uknowpro/kubewiser/internal/manifest"
	"github.com/uknowpro/kubewiser/internal/scan"
	"github.com/uknowpro/kubewiser/internal/schema"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type Options struct {
	Chart, Values, Release, Namespace, Target string
	APIVersions                               []string
}
type Result struct {
	Set          manifest.Set `json:"-"`
	ChartName    string       `json:"chart"`
	ChartVersion string       `json:"chart_version"`
	Release      string       `json:"release"`
	Namespace    string       `json:"namespace"`
	Target       string       `json:"target_version"`
	ValuesSHA    string       `json:"values_sha256,omitempty"`
	APIVersions  []string     `json:"api_versions"`
	Unchecked    []string     `json:"unchecked"`
}

var renderMu sync.Mutex

func Render(ctx context.Context, o Options) (Result, error) {
	// Helm can log raw merged values through the standard logger. Serialize rendering
	// and suppress those library diagnostics; CLI errors are returned separately.
	renderMu.Lock()
	defer renderMu.Unlock()
	oldWriter := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(oldWriter)

	r := Result{Release: o.Release, Namespace: o.Namespace, Target: o.Target, APIVersions: o.APIVersions, Unchecked: []string{"lookup returns no live data; Helm values.schema.json validation is skipped to prevent remote schema resolution", "Capabilities use explicit target version/API overrides; no cluster discovery, hooks execution or dependency downloads"}}
	if o.Chart == "" || o.Release == "" || o.Namespace == "" {
		return r, fmt.Errorf("local chart, release and namespace required")
	}
	st, e := os.Stat(o.Chart)
	if e != nil || !st.IsDir() {
		return r, fmt.Errorf("chart must be a local unpacked directory")
	}
	total := int64(0)
	e = filepath.WalkDir(o.Chart, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("chart symlinks are not accepted")
		}
		if !d.IsDir() {
			st, e := d.Info()
			if e != nil {
				return e
			}
			if !st.Mode().IsRegular() {
				return fmt.Errorf("chart contains nonregular file")
			}
			total += st.Size()
			if st.Size() > manifest.MaxFile || total > manifest.MaxTotal {
				return fmt.Errorf("chart exceeds size limits")
			}
		}
		return ctx.Err()
	})
	if e != nil {
		return r, e
	}
	c, e := loader.Load(o.Chart)
	if e != nil {
		return r, fmt.Errorf("cannot load local chart")
	}
	if e = checkDeps(c); e != nil {
		return r, e
	}
	r.ChartName = c.Metadata.Name
	r.ChartVersion = c.Metadata.Version
	vals := map[string]any{}
	if o.Values != "" {
		b, e := manifest.ReadBounded(o.Values, manifest.MaxFile)
		if e != nil {
			return r, e
		}
		r.ValuesSHA = fmt.Sprintf("%x", sha256.Sum256(b))
		v, e := chartutil.ReadValues(b)
		if e != nil {
			return r, fmt.Errorf("invalid values YAML")
		}
		vals = v
	}
	_, err := scan.Target(o.Target)
	if err != nil {
		return r, err
	}
	validator, err := schema.NewVersion(o.Target)
	if err != nil {
		return r, err
	}
	caps := chartutil.DefaultCapabilities.Copy()
	caps.APIVersions = chartutil.VersionSet(validator.APIVersions())
	kv, e := chartutil.ParseKubeVersion(o.Target)
	if e != nil {
		return r, fmt.Errorf("invalid target version")
	}
	caps.KubeVersion = *kv
	if len(o.APIVersions) > 0 {
		caps.APIVersions = chartutil.VersionSet(o.APIVersions)
	}
	if e = chartutil.ProcessDependencies(c, vals); e != nil {
		return r, fmt.Errorf("chart dependency conditions invalid")
	}
	values, e := chartutil.ToRenderValuesWithSchemaValidation(c, vals, chartutil.ReleaseOptions{Name: o.Release, Namespace: o.Namespace, Revision: 1, IsInstall: true}, caps, true)
	if e != nil {
		return r, fmt.Errorf("cannot construct render values")
	}
	if e = ctx.Err(); e != nil {
		return r, e
	}
	output, e := engine.Render(c, values)
	if e != nil {
		return r, fmt.Errorf("Helm rendering failed; inspect the chart locally (template diagnostics suppressed)")
	}
	keys := []string{}
	for k := range output {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	r.APIVersions = []string(caps.APIVersions)
	r.Set = manifest.Set{Resources: []manifest.Resource{}, Issues: []manifest.Issue{}, Inputs: []manifest.Input{}}
	size := 0
	for _, k := range keys {
		if strings.HasSuffix(k, "NOTES.txt") {
			continue
		}
		b := []byte(output[k])
		size += len(b)
		if size > manifest.MaxTotal {
			return r, fmt.Errorf("render output limit exceeded")
		}
		s, e := manifest.Decode(ctx, b, "helm:"+k)
		if e != nil {
			return r, e
		}
		r.Set.Resources = append(r.Set.Resources, s.Resources...)
		r.Set.Inputs = append(r.Set.Inputs, s.Inputs...)
		r.Set.Issues = append(r.Set.Issues, s.Issues...)
	}
	for _, f := range c.CRDObjects() {
		s, e := manifest.Decode(ctx, f.File.Data, "helm:"+f.Filename)
		if e != nil {
			return r, e
		}
		r.Set.Resources = append(r.Set.Resources, s.Resources...)
		r.Set.Inputs = append(r.Set.Inputs, s.Inputs...)
		r.Set.Issues = append(r.Set.Issues, s.Issues...)
	}
	if len(r.Set.Resources) > manifest.MaxResources {
		return r, fmt.Errorf("render resource limit exceeded")
	}
	return r, nil
}
func checkDeps(c *chart.Chart) error {
	for _, want := range c.Metadata.Dependencies {
		found := false
		for _, got := range c.Dependencies() {
			if got.Name() == want.Name {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("chart dependencies must be vendored locally")
		}
	}
	for _, child := range c.Dependencies() {
		if e := checkDeps(child); e != nil {
			return e
		}
	}
	return nil
}
func (r Result) Metadata() map[string]any {
	b, _ := json.Marshal(r)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}
