// Package cluster collects bounded read-only snapshots from an explicitly selected API server.
package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/uknowpro/kubewiser/internal/manifest"
	"io"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Options struct {
	AllowExecAuth                  string
	Kubeconfig, Context, Namespace string
	IncludeCluster                 bool
	QPS                            float64
	MaxItems                       int
	Timeout                        time.Duration
}
type Gap struct {
	Resource string `json:"resource"`
	Reason   string `json:"reason"`
}
type Snapshot struct {
	Format        string              `json:"format"`
	Context       string              `json:"context"`
	Namespace     string              `json:"namespace"`
	ServerVersion string              `json:"server_version"`
	Started       string              `json:"started"`
	Finished      string              `json:"finished"`
	Resources     []manifest.Resource `json:"resources"`
	Gaps          []Gap               `json:"gaps"`
	Redacted      []string            `json:"redacted"`
	Requests      int                 `json:"requests"`
}
type endpoint struct {
	path, api, kind string
	cluster         bool
}

var endpoints = []endpoint{
	{"/apis/batch/v1/namespaces/%s/jobs", "batch/v1", "Job", false},
	{"/apis/apps/v1/namespaces/%s/replicasets", "apps/v1", "ReplicaSet", false},
	{"/apis/autoscaling/v2/namespaces/%s/horizontalpodautoscalers", "autoscaling/v2", "HorizontalPodAutoscaler", false},
	{"/api/v1/namespaces/%s/pods", "v1", "Pod", false}, {"/api/v1/namespaces/%s/services", "v1", "Service", false}, {"/api/v1/namespaces/%s/configmaps", "v1", "ConfigMap", false}, {"/api/v1/namespaces/%s/persistentvolumeclaims", "v1", "PersistentVolumeClaim", false}, {"/api/v1/namespaces/%s/serviceaccounts", "v1", "ServiceAccount", false},
	{"/apis/apps/v1/namespaces/%s/deployments", "apps/v1", "Deployment", false}, {"/apis/apps/v1/namespaces/%s/statefulsets", "apps/v1", "StatefulSet", false}, {"/apis/apps/v1/namespaces/%s/daemonsets", "apps/v1", "DaemonSet", false}, {"/apis/batch/v1/namespaces/%s/cronjobs", "batch/v1", "CronJob", false}, {"/apis/networking.k8s.io/v1/namespaces/%s/ingresses", "networking.k8s.io/v1", "Ingress", false}, {"/apis/networking.k8s.io/v1/namespaces/%s/networkpolicies", "networking.k8s.io/v1", "NetworkPolicy", false}, {"/apis/policy/v1/namespaces/%s/poddisruptionbudgets", "policy/v1", "PodDisruptionBudget", false},
	{"/api/v1/nodes", "v1", "Node", true}, {"/apis/storage.k8s.io/v1/storageclasses", "storage.k8s.io/v1", "StorageClass", true},
}

type guard struct {
	next   http.RoundTripper
	origin string
}

func (g guard) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method != "GET" || r.URL.Scheme+"://"+r.URL.Host != g.origin {
		return nil, fmt.Errorf("request outside read-only API origin")
	}
	return g.next.RoundTrip(r)
}
func Collect(ctx context.Context, o Options) (Snapshot, error) {
	s := Snapshot{Format: "kubewiser.snapshot.v1", Context: o.Context, Namespace: o.Namespace, Started: time.Now().UTC().Format(time.RFC3339Nano), Resources: []manifest.Resource{}, Gaps: []Gap{}, Redacted: []string{}}
	if o.Context == "" || o.Kubeconfig == "" || o.Namespace == "" || strings.ContainsAny(o.Namespace, "/\\%?#") {
		return s, fmt.Errorf("explicit kubeconfig, context and namespace required")
	}
	if o.QPS <= 0 || o.QPS > 10 || o.MaxItems < 1 || o.MaxItems > 10000 || o.Timeout <= 0 || o.Timeout > 10*time.Minute {
		return s, fmt.Errorf("invalid collection limits")
	}
	raw, e := clientcmd.LoadFromFile(o.Kubeconfig)
	if e != nil {
		return s, fmt.Errorf("cannot load kubeconfig")
	}
	if raw.Contexts[o.Context] == nil {
		return s, fmt.Errorf("context not found")
	}
	selected := raw.Contexts[o.Context]
	auth := raw.AuthInfos[selected.AuthInfo]
	if auth != nil && (auth.AuthProvider != nil || (auth.Exec != nil && o.AllowExecAuth == "")) {
		return s, fmt.Errorf("exec/auth-provider plugins are disabled; use static credentials")
	}
	cl := raw.Clusters[selected.Cluster]
	if cl == nil || cl.InsecureSkipTLSVerify || cl.ProxyURL != "" {
		return s, fmt.Errorf("verified TLS and no configured proxy are required")
	}
	cfg, e := clientcmd.NewNonInteractiveClientConfig(*raw, o.Context, &clientcmd.ConfigOverrides{}, nil).ClientConfig()
	if e != nil {
		return s, fmt.Errorf("invalid client configuration")
	}
	if auth != nil && auth.Exec != nil {
		if e = execCredentials(ctx, cfg, auth.Exec, o.AllowExecAuth); e != nil {
			return s, e
		}
		s.Gaps = append(s.Gaps, Gap{"authentication", "Explicit external authentication executable used; it may contact identity providers or launch child processes. Credentials are not stored in the snapshot"})
	}
	return collectConfig(ctx, cfg, o, s)
}
func collectConfig(ctx context.Context, cfg *rest.Config, o Options, s Snapshot) (Snapshot, error) {
	base, e := url.Parse(cfg.Host)
	if e != nil || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" || strings.Trim(base.Path, "/") != "" {
		return s, fmt.Errorf("API server must be an origin URL")
	}
	if base.Scheme != "https" {
		ip := net.ParseIP(base.Hostname())
		if base.Scheme != "http" || ip == nil || !ip.IsLoopback() {
			return s, fmt.Errorf("HTTPS required except loopback test servers")
		}
	}
	config := rest.CopyConfig(cfg)
	config.Proxy = func(*http.Request) (*url.URL, error) { return nil, nil }
	config.Timeout = 30 * time.Second
	config.UserAgent = "kubewiser-readonly"
	transport, e := rest.TransportFor(config)
	if e != nil {
		return s, fmt.Errorf("cannot initialize API transport")
	}
	client := &http.Client{Transport: guard{transport, base.Scheme + "://" + base.Host}, Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("redirects disabled") }}
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()
	ticker := time.NewTicker(time.Duration(float64(time.Second) / o.QPS))
	defer ticker.Stop()
	received := 0
	get := func(path string, q url.Values, dst any) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
		u := *base
		u.Path = path
		u.RawQuery = q.Encode()
		req, e := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		if e != nil {
			return e
		}
		req.Header.Set("Accept", "application/json")
		s.Requests++
		resp, e := client.Do(req)
		if e != nil {
			return fmt.Errorf("request failed or timed out")
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		b, e := io.ReadAll(io.LimitReader(resp.Body, 8<<20+1))
		if e != nil || len(b) > 8<<20 {
			return fmt.Errorf("response exceeds limit or unreadable")
		}
		received += len(b)
		if received > manifest.MaxTotal {
			return fmt.Errorf("total response byte limit reached")
		}
		if e = json.Unmarshal(b, dst); e != nil {
			return fmt.Errorf("invalid JSON response")
		}
		return nil
	}
	var ver map[string]any
	if e = get("/version", nil, &ver); e != nil {
		s.Gaps = append(s.Gaps, Gap{"version", e.Error()})
	} else {
		s.ServerVersion = manifest.String(ver, "gitVersion")
	}
	for _, ep := range endpointsFor(s.ServerVersion) {
		if ep.cluster && !o.IncludeCluster {
			continue
		}
		path := ep.path
		if !ep.cluster {
			path = fmt.Sprintf(ep.path, o.Namespace)
		}
		cont := ""
		seen := map[string]bool{}
		for page := 0; page < 100; page++ {
			if received > manifest.MaxTotal {
				s.Gaps = append(s.Gaps, Gap{ep.kind, "total response byte limit reached"})
				break
			}
			if len(s.Resources) >= o.MaxItems {
				s.Gaps = append(s.Gaps, Gap{ep.kind, "resource limit reached"})
				break
			}
			var list struct {
				Items    []map[string]any `json:"items"`
				Metadata struct {
					Continue string `json:"continue"`
				} `json:"metadata"`
			}
			q := url.Values{"limit": {"200"}}
			if cont != "" {
				q.Set("continue", cont)
			}
			if e = get(path, q, &list); e != nil {
				s.Gaps = append(s.Gaps, Gap{ep.kind, e.Error()})
				break
			}
			for _, obj := range list.Items {
				if len(s.Resources) >= o.MaxItems {
					s.Gaps = append(s.Gaps, Gap{ep.kind, "resource limit reached"})
					break
				}
				obj["apiVersion"] = ep.api
				obj["kind"] = ep.kind
				res := manifest.Resource{Object: obj, File: "cluster:" + ep.kind}
				redact(obj, "", res.ID(), &s.Redacted)
				s.Resources = append(s.Resources, res)
			}
			cont = list.Metadata.Continue
			if cont == "" {
				break
			}
			if seen[cont] || page == 99 {
				s.Gaps = append(s.Gaps, Gap{ep.kind, "pagination limit or repeated continuation"})
				break
			}
			seen[cont] = true
		}
	}
	s.Gaps = append(s.Gaps, Gap{"coverage", "Secrets, custom resources, unlisted kinds and other namespaces are not collected"})
	s.Finished = time.Now().UTC().Format(time.RFC3339Nano)
	return s, nil
}
func redact(m map[string]any, path, id string, redacted *[]string) {
	for k, v := range m {
		p := path + "/" + manifest.Escape(k)
		sensitive := k == "annotations" || k == "data" || k == "binaryData" || k == "stringData" || k == "command" || k == "args" || (k == "value" && strings.Contains(path, "/env/"))
		if sensitive {
			delete(m, k)
			*redacted = append(*redacted, id+"#"+p)
			continue
		}
		switch x := v.(type) {
		case map[string]any:
			redact(x, p, id, redacted)
		case []any:
			for i, a := range x {
				if child, ok := a.(map[string]any); ok {
					redact(child, fmt.Sprintf("%s/%d", p, i), id, redacted)
				}
			}
		}
	}
}
func Write(path string, s Snapshot) error {
	b, e := json.MarshalIndent(s, "", "  ")
	if e != nil {
		return e
	}
	if len(b) > manifest.MaxTotal {
		return fmt.Errorf("serialized snapshot exceeds size limit")
	}
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	if _, e = f.Write(b); e != nil {
		f.Close()
		os.Remove(path)
		return e
	}
	return f.Close()
}
func Read(path string) (Snapshot, error) {
	var s Snapshot
	b, e := manifest.ReadBounded(path, manifest.MaxTotal)
	if e != nil {
		return s, e
	}
	if e = json.Unmarshal(b, &s); e != nil {
		return s, fmt.Errorf("invalid snapshot JSON")
	}
	if s.Format != "kubewiser.snapshot.v1" {
		return s, fmt.Errorf("unsupported snapshot format")
	}
	if len(s.Resources) > manifest.MaxResources {
		return s, fmt.Errorf("snapshot resource limit")
	}
	return s, nil
}

// Select only known served versions from the reported minor. No speculative
// retries or discovery expansion occurs when version information is absent.
func endpointsFor(version string) []endpoint {
	out := append([]endpoint(nil), endpoints...)
	parts := strings.Split(strings.TrimPrefix(version, "v"), ".")
	if len(parts) < 2 || parts[0] != "1" {
		return out
	}
	minor, e := strconv.Atoi(parts[1])
	if e != nil || minor < 16 {
		return out
	}
	for i, ep := range out {
		replacement := ""
		switch ep.kind {
		case "CronJob":
			if minor < 21 {
				replacement = "batch/v1beta1"
			}
		case "Ingress":
			if minor < 19 {
				replacement = "networking.k8s.io/v1beta1"
			}
		case "PodDisruptionBudget":
			if minor < 21 {
				replacement = "policy/v1beta1"
			}
		case "HorizontalPodAutoscaler":
			if minor < 23 {
				replacement = "autoscaling/v2beta2"
			}
		}
		if replacement != "" {
			out[i].path = strings.Replace(ep.path, "/apis/"+ep.api+"/", "/apis/"+replacement+"/", 1)
			out[i].api = replacement
		}
	}
	return out
}
