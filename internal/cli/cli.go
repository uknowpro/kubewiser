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
  versions [--format json|markdown|html] [--lang en|ko]
  scan files --target-version VERSION [--recommend] PATH
  scan cel --source PATH [--snapshot OLD_SNAPSHOT]
  scan helm --target-version VERSION --release NAME --namespace NS [--values FILE] CHART
  scan snapshot --target-version VERSION SNAPSHOT
  scan network [--flows HUBBLE.jsonl] [--namespace NS] PATH
  scan network --snapshot FILE [--flows HUBBLE.jsonl]
  scan path --source PATH --source-pod NS/NAME --destination-pod NS/NAME --port PORT [--protocol TCP]
  scan cgroup --source PATH [--cgroup-evidence FILE] [--target-version VERSION]
  scan lifecycle --source PATH [--profile FILE] [--format html]
  collect --kubeconfig FILE --context NAME --namespace NS --output snapshot.json
  diff --source PATH --snapshot FILE [--namespace NS]
  diff --chart DIR --target-version VERSION --release NAME --snapshot FILE [--values FILE]
  plan upgrade --source PATH --from VERSION --to VERSION [--after PATH] [--cgroup-evidence FILE]
  plan upgrade --snapshot FILE --from VERSION --to VERSION
  plan migrate --source PATH --from eks --to aks
  plan aks --source PATH --profile project.json
  generate --profile project.json --output NEW_DIRECTORY
  mcp --input PATH [--snapshot FILE] [--chart DIR] [--values FILE] [--flows FILE]
Use --format json|text|markdown|html (default json); flags precede positional paths.
Readable scan/plan/diff reports support --lang en|ko (default en).
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
		flags := flag.NewFlagSet("versions", flag.ContinueOnError)
		flags.SetOutput(errOut)
		format := flags.String("format", "json", "json, markdown or html")
		lang := flags.String("lang", "en", "en or ko for readable reports")
		if e := flags.Parse(args[1:]); e != nil {
			return 2
		}
		if (*lang != "en" && *lang != "ko") || flags.NArg() != 0 || (*format != "json" && *format != "markdown" && *format != "html") {
			return fail(fmt.Errorf("versions accepts --format json|markdown|html"))
		}
		return emit(out, errOut, schema.Sources(), *format, 0, *lang)
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
	format := fs.String("format", "json", "json, text, markdown or html")
	lang := fs.String("lang", "en", "en or ko for readable reports; JSON unchanged")
	target := fs.String("target-version", "", "target Kubernetes version")
	sourcePod := fs.String("source-pod", "", "source namespace/pod")
	destinationService := fs.String("destination-service", "", "destination namespace/service")
	destinationPod := fs.String("destination-pod", "", "destination namespace/pod")
	port := fs.Int("port", 0, "destination port")
	protocol := fs.String("protocol", "TCP", "TCP, UDP or SCTP")
	crdDefaults := fs.Bool("crd-defaults", false, "normalize local CRD defaults during diff")
	crdConversion := fs.Bool("crd-conversion", false, "simulate local CRD strategy None conversion during diff")
	after := fs.String("after", "", "optional proposed manifest path for plan upgrade/aks comparison")
	cgroupEvidence := fs.String("cgroup-evidence", "", "local JSON current/target node-pool cgroup evidence")
	profile := fs.String("profile", "", "local AKS project profile JSON")
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
	allowExec := fs.String("allow-exec-auth", "", "explicit absolute authentication executable; may contact identity services")
	allowCluster := fs.Bool("allow-cluster", false, "explicitly enable scoped MCP collection")
	if e := fs.Parse(restArgs); e != nil {
		return 2
	}
	if *after != "" && !(cmd == "plan" && (sub == "upgrade" || sub == "aks")) {
		return fail(fmt.Errorf("--after requires plan upgrade or plan aks"))
	}
	if *cgroupEvidence != "" && !((cmd == "scan" && sub == "cgroup") || (cmd == "plan" && sub == "upgrade")) {
		return fail(fmt.Errorf("--cgroup-evidence is supported by scan cgroup and plan upgrade; use cgroup_pools in AKS profiles"))
	}
	if *format != "json" && *format != "text" && *format != "markdown" && *format != "html" {
		return fail(fmt.Errorf("format must be json, text, markdown or html"))
	}
	if *lang != "en" && *lang != "ko" {
		return fail(fmt.Errorf("lang must be en or ko"))
	}
	collection := cluster.Options{AllowExecAuth: *allowExec, Kubeconfig: *kubeconfig, Context: *kubecontext, Namespace: *ns, IncludeCluster: *includeCluster, QPS: *qps, MaxItems: *max, Timeout: *timeout}
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
	case "generate":
		if fs.NArg() != 0 || *profile == "" || *output == "" {
			return fail(fmt.Errorf("generate requires --profile and a new --output directory"))
		}
		p, e := analyze.ReadProfile(*profile)
		if e != nil {
			return fail(e)
		}
		files, e := analyze.Generate(p, *output)
		if e != nil {
			return fail(e)
		}
		return emit(out, errOut, map[string]any{"status": "review_drafts", "output": *output, "files": files, "unchecked": []string{"Drafts require review; no infrastructure commands executed"}}, *format, 0, *lang)
	case "scan":
		if sub == "cgroup" {
			if fs.NArg() != 0 {
				return fail(fmt.Errorf("scan cgroup uses --source or --snapshot"))
			}
			set, e := load()
			if e != nil {
				return fail(e)
			}
			var pools []analyze.CgroupPool
			if *cgroupEvidence != "" {
				pools, e = analyze.ReadCgroups(*cgroupEvidence)
				if e != nil {
					return fail(e)
				}
			}
			p, e := analyze.Cgroups(set, pools, *target)
			if e != nil {
				return fail(e)
			}
			code := 0
			if len(p.Actions) > 0 {
				code = 1
			}
			return emit(out, errOut, p, *format, code, *lang)
		}
		if sub == "lifecycle" {
			if fs.NArg() != 0 {
				return fail(fmt.Errorf("scan lifecycle uses --source or --snapshot"))
			}
			set, e := load()
			if e != nil {
				return fail(e)
			}
			var project *analyze.ProjectProfile
			if *profile != "" {
				v, e := analyze.ReadProfile(*profile)
				if e != nil {
					return fail(e)
				}
				project = &v
			}
			p, e := analyze.Lifecycle(set, project, time.Now().UTC())
			if e != nil {
				return fail(e)
			}
			code := 0
			if len(p.Actions) > 0 {
				code = 1
			}
			return emit(out, errOut, p, *format, code, *lang)
		}
		if sub == "cel" {
			if *source == "" || fs.NArg() != 0 {
				return fail(fmt.Errorf("scan cel requires --source and optional --snapshot old values"))
			}
			set, e := manifest.Load(ctx, *source)
			if e != nil {
				return fail(e)
			}
			var old []manifest.Resource
			if *snapshot != "" {
				snap, e := cluster.Read(*snapshot)
				if e != nil {
					return fail(e)
				}
				old = snap.Resources
			}
			p, e := analyze.CELReview(ctx, set, old)
			if e != nil {
				return fail(e)
			}
			if *snapshot != "" {
				p.Unchecked = append(p.Unchecked, "Old snapshot may be redacted or incomplete; missing values do not establish transition compatibility")
			}
			code := 0
			if len(p.Actions) > 0 {
				code = 1
			}
			return emit(out, errOut, p, *format, code, *lang)
		}
		if sub == "path" {
			if fs.NArg() != 0 {
				return fail(fmt.Errorf("scan path uses --source or --snapshot"))
			}
			set, e := load()
			if e != nil {
				return fail(e)
			}
			if *destinationService != "" {
				if *destinationPod != "" {
					return fail(fmt.Errorf("choose destination-pod or destination-service"))
				}
				r, e := analyze.NetworkServicePath(ctx, set, *sourcePod, *destinationService, *protocol, *port)
				if e != nil {
					return fail(e)
				}
				if *flows != "" {
					observations, e := analyze.NetworkScan(ctx, set, *flows, *ns)
					if e != nil {
						return fail(e)
					}
					for i := range r.Paths {
						analyze.AttachFlowEvidence(&r.Paths[i], observations.Flows)
					}
				}

				if *snapshot != "" {
					r.Unchecked = append(r.Unchecked, "Snapshot coverage and redaction may omit policy/endpoint evidence")
				}
				return emit(out, errOut, r, *format, 0, *lang)
			}
			r, e := analyze.NetworkPath(ctx, set, *sourcePod, *destinationPod, *protocol, *port)
			if e != nil {
				return fail(e)
			}
			if *flows != "" {
				observations, e := analyze.NetworkScan(ctx, set, *flows, *ns)
				if e != nil {
					return fail(e)
				}
				analyze.AttachFlowEvidence(&r, observations.Flows)
			}

			if *snapshot != "" {
				snap, e := cluster.Read(*snapshot)
				if e != nil {
					return fail(e)
				}
				for _, g := range snap.Gaps {
					r.Unchecked = append(r.Unchecked, g.Resource+": "+g.Reason)
				}
			}
			return emit(out, errOut, r, *format, 0, *lang)
		}
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
			return emit(out, errOut, r, *format, code, *lang)
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
			return emit(out, errOut, r, *format, code, *lang)
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
			return emit(out, errOut, map[string]any{"render": rr.Metadata(), "report": r}, *format, code, *lang)
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
			return emit(out, errOut, r, *format, 0, *lang)
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
		return emit(out, errOut, map[string]any{"output": *output, "resources": len(s.Resources), "requests": s.Requests, "gaps": s.Gaps, "redacted_fields": len(s.Redacted)}, *format, 0, *lang)
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
		d, e := analyze.CompareWithOptions(set, snap, *ns, analyze.DiffOptions{CRDDefaults: *crdDefaults, CRDConversion: *crdConversion})
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
		return emit(out, errOut, d, *format, code, *lang)
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
			if e == nil && *cgroupEvidence != "" {
				var pools []analyze.CgroupPool
				pools, e = analyze.ReadCgroups(*cgroupEvidence)
				if e == nil {
					e = analyze.AttachCgroups(&p, set, pools, *to)
				}
			}
		case "aks":
			var project analyze.ProjectProfile
			project, e = analyze.ReadProfile(*profile)
			if e == nil {
				p, e = analyze.Project(ctx, set, project)
			}
		case "migrate":
			p, e = analyze.Migration(set, *from, *to)
		default:
			return fail(fmt.Errorf("unknown plan subcommand"))
		}
		if e != nil {
			return fail(e)
		}
		if sub == "upgrade" || sub == "aks" {
			analyze.BuildMatrix(&p)
			if *after != "" {
				proposed, err := manifest.Load(ctx, *after)
				if err != nil {
					return fail(err)
				}
				if err = analyze.CompareManifests(ctx, &p, set, proposed, *ns); err != nil {
					return fail(err)
				}
			}
		}
		if *snapshot != "" {
			s, _ := cluster.Read(*snapshot)
			for _, gap := range s.Gaps {
				p.Unchecked = append(p.Unchecked, gap.Resource+": "+gap.Reason)
			}
			p.Unchecked = append(p.Unchecked, "Snapshot annotations and literal configuration values may be redacted")
		}
		return emit(out, errOut, p, *format, 0, *lang)
	case "mcp":
		if fs.NArg() != 0 || (*input == "" && *snapshot == "") {
			return fail(fmt.Errorf("mcp requires --input or --snapshot"))
		}
		c := mcpserver.Config{Input: *input, Snapshot: *snapshot, Chart: *chart, Values: *values, Flows: *flows, Namespace: *ns, Profile: *profile}
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
func emit(out, errOut io.Writer, v any, format string, code int, langs ...string) int {
	lang := language(langs)
	if format == "html" {
		if e := writeHTML(out, v, lang); e != nil {
			fmt.Fprintln(errOut, e)
			return 2
		}
		return code
	}
	if format == "markdown" {
		if e := writeMarkdown(out, v, lang); e != nil {
			fmt.Fprintln(errOut, e)
			return 2
		}
		return code
	}
	if format == "text" {
		data, e := json.Marshal(v)
		if e != nil {
			fmt.Fprintln(errOut, e)
			return 2
		}
		var value any
		if e = json.Unmarshal(data, &value); e != nil {
			fmt.Fprintln(errOut, e)
			return 2
		}
		if handled, e := writeTextReview(out, value, lang); handled {
			if e != nil {
				fmt.Fprintln(errOut, e)
				return 2
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
