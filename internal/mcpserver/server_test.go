package mcpserver

import (
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"os"
	"path/filepath"
	"testing"
)

func TestTool(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.yaml")
	os.WriteFile(p, []byte("apiVersion: batch/v1beta1\nkind: CronJob\n"), 0600)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, b := mcp.NewInMemoryTransports()
	ss, err := New(p).Connect(ctx, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	cs, err := c.Connect(ctx, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	list, err := cs.ListTools(ctx, nil)
	if err != nil || len(list.Tools) != 1 {
		t.Fatal(list, err)
	}
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "scan_files", Arguments: map[string]any{"target_version": "1.25"}})
	if err != nil || res.IsError || res.StructuredContent == nil {
		t.Fatal(res, err)
	}
}
