// Package cli implements both kubewiser and kubewise commands.
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/uknowpro/kubewiser/internal/buildinfo"
	"github.com/uknowpro/kubewiser/internal/mcpserver"
	"github.com/uknowpro/kubewiser/internal/scan"
	"io"
	"runtime"
)

func Run(args []string, out, errOut io.Writer) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "version") {
		fmt.Fprintf(out, "kubewiser %s (commit %s; %s/%s)\n", buildinfo.Version, buildinfo.Commit, runtime.GOOS, runtime.GOARCH)
		return 0
	}

	if len(args) > 0 && args[0] == "mcp" {
		fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
		fs.SetOutput(errOut)
		input := fs.String("input", "", "operator-selected manifest path")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if *input == "" || fs.NArg() != 0 {
			fmt.Fprintln(errOut, "usage: kubewiser mcp --input PATH")
			return 2
		}
		if _, err := scan.Files(*input); err != nil {
			fmt.Fprintln(errOut, err)
			return 2
		}
		if err := mcpserver.Run(context.Background(), *input); err != nil {
			fmt.Fprintln(errOut, err)
			return 2
		}
		return 0
	}

	if len(args) == 1 && (args[0] == "--help" || args[0] == "help") {
		fmt.Fprintln(out, "Commands: kubewiser or kubewise (equivalent)\nkubewiser --version\nkubewiser mcp --input PATH\nkubewiser scan files --target-version 1.32 [--format json|text] PATH")
		return 0
	}
	if len(args) < 2 || args[0] != "scan" || args[1] != "files" {
		fmt.Fprintln(errOut, "usage: kubewiser scan files --target-version 1.32 [--format json|text] PATH")
		return 2
	}
	fs := flag.NewFlagSet("files", flag.ContinueOnError)
	fs.SetOutput(errOut)
	target := fs.String("target-version", "", "target Kubernetes version (required)")
	format := fs.String("format", "text", "json or text")
	if err := fs.Parse(args[2:]); err != nil {
		return 2
	}
	if fs.NArg() != 1 || (*format != "json" && *format != "text") {
		fmt.Fprintln(errOut, "provide one path and format json or text; flags precede path")
		return 2
	}
	r, err := scan.Scan(fs.Arg(0), *target)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if *format == "json" {
		e := json.NewEncoder(out)
		e.SetIndent("", "  ")
		if err := e.Encode(r); err != nil {
			fmt.Fprintln(errOut, err)
			return 2
		}
	} else {
		fmt.Fprintf(out, "Documents: %d | Findings: %d | Coverage: %s\n", r.Documents, len(r.Findings), r.Status)
		for _, f := range r.Findings {
			fmt.Fprintf(out, "%s %s:%d document=%d %s: %s\n", f.Rule, f.File, f.Line, f.Document, f.Field, f.Message)
		}
		for _, u := range r.Unchecked {
			fmt.Fprintf(out, "UNCHECKED: %s\n", u)
		}
	}
	if len(r.Findings) > 0 {
		return 1
	}
	return 0
}
