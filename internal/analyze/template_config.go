package analyze

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/uknowpro/kubewiser/internal/manifest"
	"github.com/uknowpro/kubewiser/internal/scan"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation"
)

type NodePoolTemplate struct {
	Name              string `json:"name"`
	VMSize            string `json:"vm_size"`
	Count             int    `json:"node_count"`
	OSSKU             string `json:"os_sku"`
	KubernetesVersion string `json:"kubernetes_version"`
	ProviderVersion   string `json:"provider_version"`
}
type CanaryTemplate struct {
	Image        string            `json:"image"`
	Namespace    string            `json:"namespace"`
	Replicas     int               `json:"replicas"`
	CPU          string            `json:"cpu_request"`
	Memory       string            `json:"memory_request"`
	NodeSelector map[string]string `json:"node_selector,omitempty"`
}
type TemplateConfig struct {
	NodePool *NodePoolTemplate `json:"node_pool,omitempty"`
	Canary   *CanaryTemplate   `json:"canary,omitempty"`
}

func configuredTemplates(p ProjectProfile, files map[string]string) ([]string, error) {
	extra := []string{}
	if p.Templates == nil {
		return extra, nil
	}
	if c := p.Templates.NodePool; c != nil {
		if !regexp.MustCompile(`^[a-z][a-z0-9]{0,11}$`).MatchString(c.Name) || !regexp.MustCompile(`^[A-Za-z0-9_]+$`).MatchString(c.VMSize) || c.Count < 1 || c.Count > 1000 || !regexp.MustCompile(`^(AzureLinux|AzureLinux3|Ubuntu|Ubuntu2204|Ubuntu2404)$`).MatchString(c.OSSKU) || !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(c.ProviderVersion) {
			return nil, fmt.Errorf("invalid explicit node-pool settings or unpinned provider version")
		}
		if p.ClusterID == "" || strings.ContainsAny(p.ClusterID, "$%\"\n\r") {
			return nil, fmt.Errorf("valid cluster_id required for node-pool generation")
		}
		n, e := scan.Target(c.KubernetesVersion)
		if e != nil {
			return nil, e
		}
		from, _ := scan.Target(p.From)
		to, _ := scan.Target(p.To)
		if n < from || n > to {
			return nil, fmt.Errorf("node-pool version must be within project range; actual control-plane skew still requires verification")
		}
		files["nodepool.tf"] = strings.Replace(files["nodepool.tf"], `azurerm = { source = "hashicorp/azurerm" }`, `azurerm = { source = "hashicorp/azurerm", version = "`+c.ProviderVersion+`" }`, 1)
		values := map[string]any{"enable_canary_pool": false, "cluster_id": p.ClusterID, "pool_name": c.Name, "vm_size": c.VMSize, "node_count": c.Count, "os_sku": c.OSSKU, "kubernetes_version": c.KubernetesVersion}
		data, _ := json.MarshalIndent(values, "", "  ")
		files["nodepool.auto.tfvars.json"] = string(data) + "\n"
		extra = append(extra, "nodepool.auto.tfvars.json")
	}
	if c := p.Templates.Canary; c != nil {
		if c.Image == "" || strings.ContainsAny(c.Image, " \t\n\r") || !regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`).MatchString(c.Namespace) || len(c.Namespace) > 63 || c.Replicas < 1 || c.Replicas > 100 {
			return nil, fmt.Errorf("canary requires explicit image, namespace and 1–100 replicas")
		}
		for _, s := range []string{c.CPU, c.Memory} {
			q, e := resource.ParseQuantity(s)
			if e != nil || q.Sign() <= 0 {
				return nil, fmt.Errorf("canary requires positive CPU and memory requests")
			}
		}
		for key, value := range c.NodeSelector {
			if len(validation.IsQualifiedName(key)) > 0 || len(validation.IsValidLabelValue(value)) > 0 {
				return nil, fmt.Errorf("invalid canary node selector label")
			}
		}
		resources := map[string]any{"requests": map[string]any{"cpu": c.CPU, "memory": c.Memory}}
		values := map[string]any{"enabled": false, "namespace": c.Namespace, "image": c.Image, "replicas": c.Replicas, "nodeSelector": c.NodeSelector, "resources": resources}
		data, _ := json.MarshalIndent(values, "", "  ")
		files["chart/values.yaml"] = string(data) + "\n"
		obj := map[string]any{"apiVersion": "apps/v1", "kind": "Deployment", "metadata": map[string]any{"name": "kubewiser-canary-review", "namespace": c.Namespace}, "spec": map[string]any{"replicas": 0, "selector": map[string]any{"matchLabels": map[string]any{"app": "kubewiser-canary-review"}}, "template": map[string]any{"metadata": map[string]any{"labels": map[string]any{"app": "kubewiser-canary-review"}}, "spec": map[string]any{"automountServiceAccountToken": false, "containers": []any{map[string]any{"name": "canary", "image": c.Image, "resources": resources}}}}}}
		if len(c.NodeSelector) > 0 {
			manifest.Map(manifest.At(obj, "spec", "template", "spec"))["nodeSelector"] = c.NodeSelector
		}
		data, _ = json.MarshalIndent(obj, "", "  ")
		files["manifest.yaml"] = "# REVIEW: replicas deliberately remain zero; requested replica count is in chart/values.yaml.\n" + string(data) + "\n"
	}
	return extra, nil
}
