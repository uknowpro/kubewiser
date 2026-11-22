// Package mcpserver exposes scoped inspection tools. File and cluster scope is fixed by the operator.
package mcpserver

import (
	"context"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/uknowpro/kubewiser/internal/analyze"
	"github.com/uknowpro/kubewiser/internal/buildinfo"
	"github.com/uknowpro/kubewiser/internal/cluster"
	"github.com/uknowpro/kubewiser/internal/manifest"
	"github.com/uknowpro/kubewiser/internal/render"
	"github.com/uknowpro/kubewiser/internal/scan"
	"time"
)

type Config struct {
	Input, Snapshot, Chart, Values, Flows, Namespace, Profile string
	Cluster                                                   *cluster.Options
}
type Args struct {
	Target    string `json:"target_version" jsonschema:"Kubernetes target minor version from 1.16 through 1.37"`
	Recommend bool   `json:"recommend,omitempty"`
}
type UpgradeArgs struct {
	From string `json:"from_version"`
	To   string `json:"target_version"`
}
type MigrateArgs struct {
	From string `json:"from_provider"`
	To   string `json:"to_provider"`
}
type Empty struct{}

func New(path string) *mcp.Server { return NewConfigured(Config{Input: path, Namespace: "default"}) }
func NewConfigured(c Config) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "kubewiser", Version: buildinfo.Version}, nil)
	load := func(ctx context.Context) (manifest.Set, error) {
		if c.Input != "" {
			return manifest.Load(ctx, c.Input)
		}
		if c.Snapshot != "" {
			snap, e := cluster.Read(c.Snapshot)
			if e != nil {
				return manifest.Set{}, e
			}
			return manifest.Set{Resources: snap.Resources}, nil
		}
		return manifest.Set{}, fmt.Errorf("operator did not configure inputs")
	}
	snapshotGaps := func() []string {
		if c.Input != "" || c.Snapshot == "" {
			return nil
		}
		snap, err := cluster.Read(c.Snapshot)
		if err != nil {
			return []string{"Snapshot coverage unavailable"}
		}
		gaps := []string{"Snapshot fields are redacted; missing kinds and fields cannot establish compatibility"}
		for _, g := range snap.Gaps {
			gaps = append(gaps, g.Resource+": "+g.Reason)
		}
		return gaps
	}
	mcp.AddTool(s, tool("scan_files", "Offline pinned official-schema and API rule validation. No remote schema loading.", false), func(ctx context.Context, req *mcp.CallToolRequest, a Args) (*mcp.CallToolResult, scan.Report, error) {
		set, e := load(ctx)
		if e != nil {
			return nil, scan.Report{}, e
		}
		r, e := scan.Analyze(ctx, set, a.Target, a.Recommend)
		r.Unchecked = append(r.Unchecked, snapshotGaps()...)
		return nil, r, e
	})
	mcp.AddTool(s, tool("scan_cgroup", "Compare current/target cgroups from the operator-fixed AKS profile and inspect supplied workload references; no node commands.", false), func(ctx context.Context, req *mcp.CallToolRequest, a Empty) (*mcp.CallToolResult, analyze.Plan, error) {
		set, e := load(ctx)
		if e != nil {
			return nil, analyze.Plan{}, e
		}
		var pools []analyze.CgroupPool
		target := ""
		if c.Profile != "" {
			p, e := analyze.ReadProfile(c.Profile)
			if e != nil {
				return nil, analyze.Plan{}, e
			}
			pools = p.CgroupPools
			target = p.To
		}
		r, e := analyze.Cgroups(set, pools, target)
		r.Unchecked = append(r.Unchecked, snapshotGaps()...)
		return nil, r, e
	})
	mcp.AddTool(s, tool("scan_lifecycle", "Offline support lifecycle check using the pinned catalog and operator-fixed inputs/profile; no vendor calls.", false), func(ctx context.Context, req *mcp.CallToolRequest, a Empty) (*mcp.CallToolResult, analyze.Plan, error) {
		set, e := load(ctx)
		if e != nil {
			return nil, analyze.Plan{}, e
		}
		var p *analyze.ProjectProfile
		if c.Profile != "" {
			v, e := analyze.ReadProfile(c.Profile)
			if e != nil {
				return nil, analyze.Plan{}, e
			}
			p = &v
		}
		r, e := analyze.Lifecycle(set, p, time.Now().UTC())
		r.Unchecked = append(r.Unchecked, snapshotGaps()...)
		return nil, r, e
	})
	mcp.AddTool(s, tool("plan_upgrade", "Offline version-step actions from the configured inputs; includes unchecked dependencies.", false), func(ctx context.Context, req *mcp.CallToolRequest, a UpgradeArgs) (*mcp.CallToolResult, analyze.Plan, error) {
		set, e := load(ctx)
		if e != nil {
			return nil, analyze.Plan{}, e
		}
		r, e := analyze.Upgrade(ctx, set, a.From, a.To)
		if e == nil {
			analyze.AttachReadiness(&r, set, c.Namespace)
			analyze.BuildMatrix(&r)
		}
		r.Unchecked = append(r.Unchecked, snapshotGaps()...)
		return nil, r, e
	})
	mcp.AddTool(s, tool("plan_migrate", "Offline EKS/AKS/GKE dependency checklist; no cloud calls, data movement or provisioning.", false), func(ctx context.Context, req *mcp.CallToolRequest, a MigrateArgs) (*mcp.CallToolResult, analyze.Plan, error) {
		set, e := load(ctx)
		if e != nil {
			return nil, analyze.Plan{}, e
		}
		r, e := analyze.Migration(set, a.From, a.To)
		r.Unchecked = append(r.Unchecked, snapshotGaps()...)
		return nil, r, e
	})
	mcp.AddTool(s, tool("scan_network", "Analyze configured exposure/policy isolation and operator-selected Hubble JSON; no active probes.", false), func(ctx context.Context, req *mcp.CallToolRequest, a Empty) (*mcp.CallToolResult, analyze.Network, error) {
		set, e := load(ctx)
		if e != nil {
			return nil, analyze.Network{}, e
		}
		r, e := analyze.NetworkScan(ctx, set, c.Flows, c.Namespace)
		r.Unchecked = append(r.Unchecked, snapshotGaps()...)
		return nil, r, e
	})
	if c.Input != "" {
		mcp.AddTool(s, tool("scan_cel", "Evaluate supplied CRD CEL rules locally with bounded cost. Optional old snapshot is operator-fixed; no admission or conversion calls.", false), func(ctx context.Context, req *mcp.CallToolRequest, a Empty) (*mcp.CallToolResult, analyze.Plan, error) {
			set, e := manifest.Load(ctx, c.Input)
			if e != nil {
				return nil, analyze.Plan{}, e
			}
			var old []manifest.Resource
			if c.Snapshot != "" {
				snap, e := cluster.Read(c.Snapshot)
				if e != nil {
					return nil, analyze.Plan{}, e
				}
				old = snap.Resources
			}
			p, e := analyze.CELReview(ctx, set, old)
			if c.Snapshot != "" {
				p.Unchecked = append(p.Unchecked, "Old snapshot may be redacted or incomplete")
			}
			return nil, p, e
		})
	}
	if c.Profile != "" {
		mcp.AddTool(s, tool("plan_aks", "Offline AKS project review using operator-fixed profile and inputs. No Azure calls or writes.", false), func(ctx context.Context, req *mcp.CallToolRequest, a Empty) (*mcp.CallToolResult, analyze.Plan, error) {
			set, e := load(ctx)
			if e != nil {
				return nil, analyze.Plan{}, e
			}
			profile, e := analyze.ReadProfile(c.Profile)
			if e != nil {
				return nil, analyze.Plan{}, e
			}
			p, e := analyze.Project(ctx, set, profile)
			if e == nil {
				analyze.AttachReadiness(&p, set, c.Namespace)
				analyze.BuildMatrix(&p)
			}
			p.Unchecked = append(p.Unchecked, snapshotGaps()...)
			return nil, p, e
		})
	}
	if c.Snapshot != "" && c.Input != "" {
		mcp.AddTool(s, tool("compare_drift", "Compare operator-selected desired files and snapshot; values omitted and redacted fields excluded.", false), func(ctx context.Context, req *mcp.CallToolRequest, a Empty) (*mcp.CallToolResult, analyze.Diff, error) {
			set, e := load(ctx)
			if e != nil {
				return nil, analyze.Diff{}, e
			}
			snap, e := cluster.Read(c.Snapshot)
			if e != nil {
				return nil, analyze.Diff{}, e
			}
			r, e := analyze.Compare(set, snap, c.Namespace)
			return nil, r, e
		})
	}
	if c.Chart != "" {
		mcp.AddTool(s, tool("scan_helm", "Render the configured local chart with fixed values and no cluster or dependency downloads, then inspect.", false), func(ctx context.Context, req *mcp.CallToolRequest, a Args) (*mcp.CallToolResult, scan.Report, error) {
			rr, e := render.Render(ctx, render.Options{Chart: c.Chart, Values: c.Values, Release: "kubewiser", Namespace: c.Namespace, Target: a.Target})
			if e != nil {
				return nil, scan.Report{}, e
			}
			r, e := scan.Analyze(ctx, rr.Set, a.Target, a.Recommend)
			r.Unchecked = append(r.Unchecked, rr.Unchecked...)
			return nil, r, e
		})
	}
	if c.Cluster != nil {
		mcp.AddTool(s, tool("collect_cluster", "Explicitly enabled bounded GET-only collection of the operator-fixed context and namespace. Returns a redacted snapshot to the client.", true), func(ctx context.Context, req *mcp.CallToolRequest, a Empty) (*mcp.CallToolResult, cluster.Snapshot, error) {
			if c.Cluster.AllowExecAuth != "" {
				return nil, cluster.Snapshot{}, fmt.Errorf("MCP collection does not execute authentication plugins; use static credentials or collect explicitly with the CLI")
			}
			r, e := cluster.Collect(ctx, *c.Cluster)
			return nil, r, e
		})
	}
	return s
}
func Run(ctx context.Context, path string) error {
	return RunConfigured(ctx, Config{Input: path, Namespace: "default"})
}
func RunConfigured(ctx context.Context, c Config) error {
	if c.Cluster != nil && c.Cluster.AllowExecAuth != "" {
		return fmt.Errorf("MCP collection does not execute authentication plugins; use static credentials or collect explicitly with the CLI")
	}
	return NewConfigured(c).Run(ctx, &mcp.StdioTransport{})
}

// Hints describe handlers, not an authorization boundary. Explicit false
// pointers survive SDK JSON omitempty; tests verify tools/list wire output.
func tool(name, description string, openWorld bool) *mcp.Tool {
	destructive := false
	return &mcp.Tool{Name: name, Description: description, Annotations: &mcp.ToolAnnotations{
		ReadOnlyHint: true, DestructiveHint: &destructive, IdempotentHint: true, OpenWorldHint: &openWorld,
	}}
}
