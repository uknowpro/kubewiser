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
	if err != nil || len(list.Tools) != 6 {
		t.Fatal(list, err)
	}
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "scan_files", Arguments: map[string]any{"target_version": "1.25"}})
	if err != nil || res.IsError || res.StructuredContent == nil {
		t.Fatal(res, err)
	}
}

func TestAKSProfileToolScope(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.yaml")
	profile := filepath.Join(dir, "profile.json")
	os.WriteFile(input, []byte("apiVersion: v1\nkind: Service\nmetadata: {name: web}\n"), 0600)
	os.WriteFile(profile, []byte(`{"schema_version":1,"provider":"aks","mode":"in-place","from_version":"1.21","to_version":"1.35"}`), 0600)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, b := mcp.NewInMemoryTransports()
	ss, e := NewConfigured(Config{Input: input, Profile: profile}).Connect(ctx, a, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	cs, e := client.Connect(ctx, b, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer cs.Close()
	list, e := cs.ListTools(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, tool := range list.Tools {
		if tool.Name == "plan_aks" {
			found = true
		}
		if tool.Name == "generate" || tool.Name == "collect_cluster" {
			t.Fatal("unexpected write/cluster tool")
		}
	}
	if !found {
		t.Fatal("no project tool")
	}
	result, e := cs.CallTool(ctx, &mcp.CallToolParams{Name: "plan_aks", Arguments: map[string]any{}})
	if e != nil || result.IsError || result.StructuredContent == nil {
		t.Fatal(result, e)
	}
}
