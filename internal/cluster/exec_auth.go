package cluster

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/client-go/rest"
	api "k8s.io/client-go/tools/clientcmd/api"
)

type credentialBuffer struct{ bytes.Buffer }

func (b *credentialBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 1<<20 {
		return 0, fmt.Errorf("credential output limit exceeded")
	}
	return b.Buffer.Write(p)
}

// The operator authorizes a specific local executable. Its network/child-process
// behavior is outside the API transport guard and is never enabled implicitly.
func execCredentials(ctx context.Context, cfg *rest.Config, plugin *api.ExecConfig, allowed string) error {
	if !filepath.IsAbs(allowed) {
		return fmt.Errorf("exec authentication requires an absolute --allow-exec-auth path")
	}
	info, e := os.Stat(allowed)
	if e != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("approved authentication executable must be a regular file")
	}
	if plugin.Command != allowed && plugin.Command != filepath.Base(allowed) {
		return fmt.Errorf("kubeconfig exec command does not match the approved executable")
	}
	if plugin.APIVersion != "client.authentication.k8s.io/v1" && plugin.APIVersion != "client.authentication.k8s.io/v1beta1" {
		return fmt.Errorf("unsupported ExecCredential API version")
	}
	if plugin.InteractiveMode == api.AlwaysExecInteractiveMode {
		return fmt.Errorf("interactive authentication is not supported; authenticate separately first")
	}
	if plugin.ProvideClusterInfo {
		return fmt.Errorf("exec provideClusterInfo is not supported")
	}
	// No stdin, no shell, bounded output and a deadline. Never forward token/error output.
	authCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(authCtx, allowed, plugin.Args...)
	cmd.WaitDelay = time.Second
	cmd.Env = os.Environ()
	for _, v := range plugin.Env {
		if v.Name == "KUBERNETES_EXEC_INFO" {
			continue
		}
		cmd.Env = append(cmd.Env, v.Name+"="+v.Value)
	}
	filtered := cmd.Env[:0]
	for _, v := range cmd.Env {
		if !strings.HasPrefix(v, "KUBERNETES_EXEC_INFO=") {
			filtered = append(filtered, v)
		}
	}
	cmd.Env = filtered
	infoJSON, _ := json.Marshal(map[string]any{"apiVersion": plugin.APIVersion, "kind": "ExecCredential", "spec": map[string]any{"interactive": false}})
	cmd.Env = append(cmd.Env, "KUBERNETES_EXEC_INFO="+string(infoJSON))
	output := &credentialBuffer{}
	cmd.Stdout = output
	cmd.Stderr = io.Discard
	if e = cmd.Run(); e != nil {
		return fmt.Errorf("approved authentication plugin failed, timed out or exceeded output limit")
	}
	var response struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Status     struct {
			Token       string     `json:"token"`
			Certificate string     `json:"clientCertificateData"`
			Key         string     `json:"clientKeyData"`
			Expires     *time.Time `json:"expirationTimestamp"`
		} `json:"status"`
	}
	if e = json.Unmarshal(output.Bytes(), &response); e != nil || response.APIVersion != plugin.APIVersion || response.Kind != "ExecCredential" {
		return fmt.Errorf("invalid authentication response")
	}
	if response.Status.Expires != nil && !response.Status.Expires.After(time.Now().Add(time.Minute)) {
		return fmt.Errorf("authentication credential expires too soon")
	}
	token, cert, key := response.Status.Token, response.Status.Certificate, response.Status.Key
	if (token == "") == (cert == "" || key == "") || (cert == "") != (key == "") {
		return fmt.Errorf("authentication must provide exactly one token or certificate/key pair")
	}
	if strings.ContainsAny(token, "\r\n") {
		return fmt.Errorf("invalid authentication token")
	}
	if cert != "" {
		if _, e = tls.X509KeyPair([]byte(cert), []byte(key)); e != nil {
			return fmt.Errorf("invalid authentication certificate pair")
		}
	}
	cfg.ExecProvider = nil
	cfg.AuthProvider = nil
	cfg.BearerTokenFile = ""
	cfg.Username = ""
	cfg.Password = ""
	cfg.TLSClientConfig.CertFile = ""
	cfg.TLSClientConfig.KeyFile = ""
	cfg.BearerToken = token
	cfg.TLSClientConfig.CertData = []byte(cert)
	cfg.TLSClientConfig.KeyData = []byte(key)
	return nil
}
