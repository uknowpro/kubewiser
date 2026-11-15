// Package mcpserver exposes the same offline engine to local AI clients.
package mcpserver

import (
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/uknowpro/kubewiser/internal/buildinfo"
	"github.com/uknowpro/kubewiser/internal/scan"
)

type Args struct {
	Target string `json:"target_version" jsonschema:"Kubernetes target version from 1.16 through 1.32"`
}

// The operator fixes the input at startup; tool arguments cannot select arbitrary paths.
func New(path string) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "kubewiser", Version: buildinfo.Version}, nil)
	mcp.AddTool(s, &mcp.Tool{Name: "scan_files", Description: "Inspect the operator-selected local manifests. Partial rule coverage only; no cluster access or mutations."}, func(ctx context.Context, req *mcp.CallToolRequest, a Args) (*mcp.CallToolResult, scan.Report, error) {
		if err := ctx.Err(); err != nil {
			return nil, scan.Report{}, err
		}
		report, err := scan.Scan(path, a.Target)
		return nil, report, err
	})
	return s
}
func Run(ctx context.Context, path string) error { return New(path).Run(ctx, &mcp.StdioTransport{}) }
