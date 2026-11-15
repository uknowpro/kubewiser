// Package cli implements both kubewiser and kubewise commands.
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/uknowpro/kubewiser/internal/analyze"
	"github.com/uknowpro/kubewiser/internal/buildinfo"
	"github.com/uknowpro/kubewiser/internal/cluster"
	"github.com/uknowpro/kubewiser/internal/manifest"
	"github.com/uknowpro/kubewiser/internal/mcpserver"
	"github.com/uknowpro/kubewiser/internal/render"
	"github.com/uknowpro/kubewiser/internal/scan"
	"github.com/uknowpro/kubewiser/internal/schema"
	"io"
	"runtime"
	"strings"
	"time"
)

const help = `Commands: kubewiser or kubewise
  --version
  versions
  scan files --target-version VERSION [--recommend] PATH
  scan helm --target-version VERSION --release NAME --namespace NS [--values FILE] CHART
  scan snapshot --target-version VERSION SNAPSHOT
  scan network [--flows HUBBLE.jsonl] [--namespace NS] PATH
  scan network --snapshot FILE [--flows HUBBLE.jsonl]
  collect --kubeconfig FILE --context NAME --namespace NS --output snapshot.json
  diff --source PATH --snapshot FILE [--namespace NS]
  diff --chart DIR --target-version VERSION --release NAME --snapshot FILE [--values FILE]
  plan upgrade --source PATH --from VERSION --to VERSION
  plan upgrade --snapshot FILE --from VERSION --to VERSION
  plan migrate --source PATH --from eks --to aks
  mcp --input PATH [--snapshot FILE] [--chart DIR] [--values FILE] [--flows FILE]
Use --format json|text (default json for new commands); flags precede positional paths.
All analysis is offline except explicitly requested collect. No automatic downloads or apply.`

func Run(args []string, out, errOut io.Writer) int {
	return RunContext(context.Background(), args, out, errOut)
}
func RunContext(ctx context.Context, args []string, out, errOut io.Writer) int {
	fail := func(e error) int { fmt.Fprintln(errOut, e); return 2 }
	if len(args) == 1 && (args[0] == "--version" || args[0] == "version") {
		fmt.Fprintf(out, "kubewiser %s (commit %s; %s/%s)\n", buildinfo.Version, buildinfo.Commit, runtime.GOOS, runtime.GOARCH)
		return 0
	}
	if len(args) == 0 || (len(args) == 1 && (args[0] == "--help" || args[0] == "help")) {
		fmt.Fprintln(out, help)
		return 0
	}
	if args[0] == "versions" {
		if len(args) != 1 {
			return fail(fmt.Errorf("versions takes no arguments"))
		}
		return emit(out, errOut, schema.Sources(), "json", 0)
	}
	cmd := args[0]
	restArgs := args[1:]
	sub := ""
	if cmd == "scan" || cmd == "plan" {
		if len(restArgs) == 0 {
			return fail(fmt.Errorf("subcommand required"))
		}
		sub = restArgs[0]
		restArgs = restArgs[1:]
	}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(errOut)
	format := fs.String("format", "json", "json or text")
	target := fs.String("target-version", "", "target Kubernetes version")
	source := fs.String("source", "", "desired manifest path")
	snapshot := fs.String("snapshot", "", "snapshot file")
	ns := fs.String("namespace", "default", "namespace")
	recommend := fs.Bool("recommend", false, "include conditional recommendations")
	chart := fs.String("chart", "", "local chart directory")
	values := fs.String("values", "", "local Helm values file")
	release := fs.String("release", "kubewiser", "Helm release name")
	apis := fs.String("api-versions", "", "comma-separated Helm capabilities override")
	flows := fs.String("flows", "", "local Hubble JSON lines")
	from := fs.String("from", "", "source version/provider")
	to := fs.String("to", "", "target version/provider")
	input := fs.String("input", "", "operator-selected MCP files")
	kubeconfig := fs.String("kubeconfig", "", "explicit kubeconfig file")
	kubecontext := fs.String("context", "", "explicit cluster context")
	output := fs.String("output", "", "new output snapshot file")
	includeCluster := fs.Bool("include-cluster-resources", false, "include Nodes and StorageClasses")
	qps := fs.Float64("qps", 2, "GET requests per second, maximum 10")
	max := fs.Int("max-items", 5000, "maximum snapshot resources")
	timeout := fs.Duration("timeout", 2*time.Minute, "overall collection deadline")
	allowCluster := fs.Bool("allow-cluster", false, "explicitly enable scoped MCP collection")
	if e := fs.Parse(restArgs); e != nil {
		return 2
	}
	if *format != "json" && *format != "text" {
		return fail(fmt.Errorf("format must be json or text"))
	}
	collection := cluster.Options{Kubeconfig: *kubeconfig, Context: *kubecontext, Namespace: *ns, IncludeCluster: *includeCluster, QPS: *qps, MaxItems: *max, Timeout: *timeout}
	renderOpts := func() render.Options {
		a := []string{}
		if *apis != "" {
			a = strings.Split(*apis, ",")
		}
		return render.Options{Chart: *chart, Values: *values, Release: *release, Namespace: *ns, Target: *target, APIVersions: a}
	}
	load := func() (manifest.Set, error) {
		if (*source == "") == (*snapshot == "") {
			return manifest.Set{}, fmt.Errorf("provide exactly one of --source or --snapshot")
		}
		if *source != "" {
			return manifest.Load(ctx, *source)
		}
		s, e := cluster.Read(*snapshot)
		return manifest.Set{Resources: s.Resources}, e
	}
	switch cmd {
	case "scan":
		if sub == "network" {
			if fs.NArg() > 1 || (fs.NArg() == 0 && *snapshot == "") || (fs.NArg() == 1 && *snapshot != "") {
				return fail(fmt.Errorf("network requires one path or --snapshot"))
			}
		} else if fs.NArg() != 1 {
			return fail(fmt.Errorf("one input path required"))
		}
		switch sub {
		case "files":
			r, e := scan.ScanContext(ctx, fs.Arg(0), *target, *recommend)
			if e != nil {
				return fail(e)
			}
			code := 0
			if len(r.Findings) > 0 {
				code = 1
			}
			return emit(out, errOut, r, *format, code)
		case "snapshot":
			snap, e := cluster.Read(fs.Arg(0))
			if e != nil {
				return fail(e)
			}
			r, e := scan.Analyze(ctx, manifest.Set{Resources: snap.Resources}, *target, *recommend)
			if e != nil {
				return fail(e)
			}
			for _, g := range snap.Gaps {
				r.Unchecked = append(r.Unchecked, g.Resource+": "+g.Reason)
			}
			r.Unchecked = append(r.Unchecked, "Redacted fields may not be schema-validated")
			code := 0
			if len(r.Findings) > 0 {
				code = 1
			}
			return emit(out, errOut, r, *format, code)
		case "helm":
			*chart = fs.Arg(0)
			rr, e := render.Render(ctx, renderOpts())
			if e != nil {
				return fail(e)
			}
			r, e := scan.Analyze(ctx, rr.Set, *target, *recommend)
			if e != nil {
				return fail(e)
			}
			r.Unchecked = append(r.Unchecked, rr.Unchecked...)
			code := 0
			if len(r.Findings) > 0 {
				code = 1
			}
			return emit(out, errOut, map[string]any{"render": rr.Metadata(), "report": r}, *format, code)
		case "network":
			var set manifest.Set
			var e error
			var gaps []cluster.Gap
			if *snapshot != "" {
				snap, err := cluster.Read(*snapshot)
				if err != nil {
					return fail(err)
				}
				set = manifest.Set{Resources: snap.Resources}
				gaps = snap.Gaps
				explicit := false
				fs.Visit(func(f *flag.Flag) {
					if f.Name == "namespace" {
						explicit = true
					}
				})
				if !explicit {
					*ns = snap.Namespace
				}
			} else {
				set, e = manifest.Load(ctx, fs.Arg(0))
				if e != nil {
					return fail(e)
				}
			}
			r, e := analyze.NetworkScan(ctx, set, *flows, *ns)
			if e != nil {
				return fail(e)
			}
			for _, g := range gaps {
				r.Unchecked = append(r.Unchecked, g.Resource+": "+g.Reason)
			}
			return emit(out, errOut, r, *format, 0)
		default:
			return fail(fmt.Errorf("unknown scan subcommand"))
		}
	case "collect":
		if fs.NArg() != 0 || *output == "" || *kubecontext == "" || *kubeconfig == "" {
			return fail(fmt.Errorf("collect requires --kubeconfig --context --namespace --output"))
		}
		explicitNS := false
		fs.Visit(func(f *flag.Flag) {
			if f.Name == "namespace" {
				explicitNS = true
			}
		})
		if !explicitNS {
			return fail(fmt.Errorf("collect requires an explicit --namespace"))
		}
		s, e := cluster.Collect(ctx, collection)
		if e != nil {
			return fail(e)
		}
		if e = cluster.Write(*output, s); e != nil {
			return fail(e)
		}
		return emit(out, errOut, map[string]any{"output": *output, "resources": len(s.Resources), "requests": s.Requests, "gaps": s.Gaps, "redacted_fields": len(s.Redacted)}, *format, 0)
	case "diff":
		if fs.NArg() != 0 || *snapshot == "" || ((*source == "") == (*chart == "")) {
			return fail(fmt.Errorf("diff requires --snapshot and exactly one of --source or --chart"))
		}
		snap, e := cluster.Read(*snapshot)
		if e != nil {
			return fail(e)
		}
		explicitNS := false
		fs.Visit(func(f *flag.Flag) {
			if f.Name == "namespace" {
				explicitNS = true
			}
		})
		if !explicitNS {
			*ns = snap.Namespace
		}
		var set manifest.Set
		var rendered *render.Result
		if *chart != "" {
			rr, e := render.Render(ctx, renderOpts())
			if e != nil {
				return fail(e)
			}
			set = rr.Set
			rendered = &rr
		} else {
			set, e = manifest.Load(ctx, *source)
			if e != nil {
				return fail(e)
			}
		}
		d, e := analyze.Compare(set, snap, *ns)
		if e != nil {
			return fail(e)
		}
		code := 0
		if rendered != nil {
			d.Render = rendered.Metadata()
			d.Unchecked = append(d.Unchecked, rendered.Unchecked...)
		}
		if len(d.Changes) > 0 {
			code = 1
		}
		return emit(out, errOut, d, *format, code)
	case "plan":
		if fs.NArg() != 0 {
			return fail(fmt.Errorf("unexpected positional arguments"))
		}
		set, e := load()
		if e != nil {
			return fail(e)
		}
		var p analyze.Plan
		switch sub {
		case "upgrade":
			p, e = analyze.Upgrade(ctx, set, *from, *to)
		case "migrate":
			p, e = analyze.Migration(set, *from, *to)
		default:
			return fail(fmt.Errorf("unknown plan subcommand"))
		}
		if e != nil {
			return fail(e)
		}
		if *snapshot != "" {
			s, _ := cluster.Read(*snapshot)
			for _, gap := range s.Gaps {
				p.Unchecked = append(p.Unchecked, gap.Resource+": "+gap.Reason)
			}
			p.Unchecked = append(p.Unchecked, "Snapshot annotations and literal configuration values may be redacted")
		}
		return emit(out, errOut, p, *format, 0)
	case "mcp":
		if fs.NArg() != 0 || (*input == "" && *snapshot == "") {
			return fail(fmt.Errorf("mcp requires --input or --snapshot"))
		}
		c := mcpserver.Config{Input: *input, Snapshot: *snapshot, Chart: *chart, Values: *values, Flows: *flows, Namespace: *ns}
		if *allowCluster {
			explicitNS := false
			fs.Visit(func(f *flag.Flag) {
				if f.Name == "namespace" {
					explicitNS = true
				}
			})
			if !explicitNS {
				return fail(fmt.Errorf("MCP collection requires explicit --namespace"))
			}
			if *kubecontext == "" || *kubeconfig == "" {
				return fail(fmt.Errorf("MCP collection requires explicit kubeconfig/context"))
			}
			c.Cluster = &collection
		}
		if e := mcpserver.RunConfigured(ctx, c); e != nil {
			return fail(e)
		}
		return 0
	default:
		return fail(fmt.Errorf("unknown command; use --help"))
	}
}
func emit(out, errOut io.Writer, v any, format string, code int) int {
	if format == "text" {
		if r, ok := v.(scan.Report); ok {
			fmt.Fprintf(out, "Resources: %d | Schema-checked: %d | Findings: %d | Coverage: %s\n", r.Documents, r.SchemaChecked, len(r.Findings), r.Status)
			for _, f := range r.Findings {
				fmt.Fprintf(out, "%s %s:%d %s %s: %s\n", f.Rule, f.File, f.Line, f.Field, f.Requirement, f.Message)
			}
			for _, u := range r.Unchecked {
				fmt.Fprintf(out, "UNCHECKED: %s\n", u)
			}
			return code
		}
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if e := enc.Encode(v); e != nil {
		fmt.Fprintln(errOut, e)
		return 2
	}
	return code
}
