package cluster

import (
	"context"
	"fmt"
	"k8s.io/client-go/rest"
	api "k8s.io/client-go/tools/clientcmd/api"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAuthHelper(t *testing.T) {
	mode := os.Getenv("KW_AUTH_HELPER")
	if mode == "" {
		return
	}
	if mode == "hang" {
		time.Sleep(time.Minute)
		os.Exit(1)
	}
	if mode == "bad" {
		fmt.Fprint(os.Stderr, "SENSITIVE_TOKEN")
		fmt.Print("SENSITIVE_TOKEN")
		os.Exit(2)
	}
	fmt.Print(`{"apiVersion":"client.authentication.k8s.io/v1","kind":"ExecCredential","status":{"token":"test-token-never-persist"}}`)
	os.Exit(0)
}
func TestApprovedExecAuth(t *testing.T) {
	executable, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	plugin := &api.ExecConfig{Command: filepath.Base(executable), APIVersion: "client.authentication.k8s.io/v1", InteractiveMode: api.NeverExecInteractiveMode, Args: []string{"-test.run=^TestAuthHelper$"}, Env: []api.ExecEnvVar{{Name: "KW_AUTH_HELPER", Value: "ok"}}}
	cfg := &rest.Config{}
	if e = execCredentials(context.Background(), cfg, plugin, executable); e != nil || cfg.BearerToken != "test-token-never-persist" || cfg.ExecProvider != nil {
		t.Fatal("credential extraction failed", e)
	}
	if e = execCredentials(context.Background(), cfg, plugin, "relative"); e == nil {
		t.Fatal("relative executable accepted")
	}
	plugin.Command = "different"
	if e = execCredentials(context.Background(), cfg, plugin, executable); e == nil {
		t.Fatal("wrong executable accepted")
	}
	plugin.Command = filepath.Base(executable)
	plugin.Env[0].Value = "bad"
	e = execCredentials(context.Background(), cfg, plugin, executable)
	if e == nil || strings.Contains(e.Error(), "SENSITIVE_TOKEN") {
		t.Fatal("plugin output leaked", e)
	}
	plugin.Env[0].Value = "hang"
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if e = execCredentials(ctx, cfg, plugin, executable); e == nil {
		t.Fatal("cancellation ignored")
	}
}
