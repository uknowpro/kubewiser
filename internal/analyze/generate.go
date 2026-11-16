package analyze

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/uknowpro/kubewiser/internal/manifest"
	"github.com/uknowpro/kubewiser/internal/scan"
	"os"
	"path/filepath"
)

// Generate writes review drafts into a newly created private directory only.
// It does not read credentials, copy workload manifests, or run any executables.
func Generate(p ProjectProfile, output string) ([]string, error) {
	if e := ValidateProfile(p); e != nil {
		return nil, e
	}
	a, e := scan.Target(p.From)
	if e != nil {
		return nil, e
	}
	b, e := scan.Target(p.To)
	if e != nil {
		return nil, e
	}
	if b <= a {
		return nil, fmt.Errorf("target must be newer than current version")
	}
	if output == "" {
		return nil, fmt.Errorf("new output directory required")
	}
	profile, _ := json.MarshalIndent(p, "", "  ")
	files := map[string]string{
		"project.json": string(profile) + "\n",
		"README.md": `# AKS project review drafts

These files are NOT an approved execution plan. No resources have been changed.

1. Run kubewiser plan aks with project.json and your local manifests/snapshot.
2. Verify the exact next version using fresh, cluster-specific AKS upgrade evidence.
3. Fill acceptance.md with owners, evidence, measured RPO/RTO and stop criteria.
4. The Terraform example adds a USER node pool to an existing cluster only. It does not import or upgrade the control plane. Pin a reviewed AzureRM provider version and examine a plan in your own workflow. Never change a Kubernetes version to the final target without checking control-plane skew.
5. The Helm chart is disabled by default. It is a disposable canary workload, not a converted production application. Supply an approved image and namespace before rendering. No image tags, identity permissions or sizing are guessed.
6. manifest.yaml is a disabled ReplicaSet canary (replicas: 0), not an application migration or a backup. Review image, labels and scheduling before enabling it.
7. Record restore-test evidence before upgrading. A Kubernetes downgrade is not the recovery plan. Do not retire old pools until acceptance.

Terraform, Helm and Azure commands are never executed by kubewiser. No credentials or source workload content are copied here.
`,
		"acceptance.md": `# Acceptance and recovery record

| Gate | Owner | Required evidence | Outcome |
| --- | --- | --- | --- |
| Requirements | TBD | Maintenance window, outage budget, RPO, RTO | Pending |
| Route | TBD | Fresh AKS offered upgrade, cluster/region, timestamp, support confirmation | Pending |
| Add-ons | TBD | Exact CNI/CSI/Ingress/webhook versions and vendor compatibility references | Pending |
| Recovery | TBD | Backup restore rehearsal, data checksum, measured RPO/RTO | Pending |
| Canary | TBD | CPU/OS/runtime compatibility, connectivity, storage and identity tests | Pending |
| Each version step | TBD | API validation, healthy nodes/pods, service error and latency budget | Pending |
| Architecture change | TBD | In-place support, separate rehearsal, rollback limitations | Pending |
| Closeout | TBD | Observation window, service owner sign-off, backup retention | Pending |

Stop on failed drain, unhealthy application, data-integrity failure or exceeded agreed budget. The recovery owner chooses the rehearsed recovery procedure; do not assume control-plane downgrade is available.
`,
		"nodepool.tf": `# REVIEW ONLY: opt-in additional canary user pool; existing AKS control plane is untouched.
terraform {
  required_providers {
    azurerm = { source = "hashicorp/azurerm" }
  }
}
# Pin the provider version and configure authentication in your existing Terraform workflow.
variable "enable_canary_pool" { default = false }
variable "cluster_id" { type = string }
variable "pool_name" { type = string }
variable "vm_size" { type = string }
variable "node_count" { type = number }
variable "os_sku" { type = string }
variable "kubernetes_version" { type = string }
resource "azurerm_kubernetes_cluster_node_pool" "canary" {
  count                 = var.enable_canary_pool ? 1 : 0
  name                  = var.pool_name
  kubernetes_cluster_id = var.cluster_id
  vm_size               = var.vm_size
  node_count            = var.node_count
  os_sku                = var.os_sku
  orchestrator_version  = var.kubernetes_version
  mode                  = "User"
  node_labels           = { "kubewiser-review" = "canary" }
  node_taints           = ["kubewiser-review=canary:NoSchedule"]
}
`,
		"chart/Chart.yaml":  "apiVersion: v2\nname: kubewiser-canary\nversion: 0.1.0\ntype: application\n",
		"chart/values.yaml": "enabled: false\nimage: ''\nreplicas: 1\nnodeSelector: {}\nresources: {}\n",
		"chart/templates/canary.yaml": `{{- if .Values.enabled }}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}-canary
  namespace: {{ .Values.namespace | default .Release.Namespace | quote }}
spec:
  replicas: {{ .Values.replicas }}
  selector:
    matchLabels:
      app.kubernetes.io/instance: {{ .Release.Name }}-canary
  template:
    metadata:
      labels:
        app.kubernetes.io/instance: {{ .Release.Name }}-canary
    spec:
      automountServiceAccountToken: false
      nodeSelector:
        {{- toYaml .Values.nodeSelector | nindent 8 }}
      tolerations:
        - key: kubewiser-review
          operator: Equal
          value: canary
          effect: NoSchedule
      containers:
        - name: canary
          image: {{ required "Set an approved canary image" .Values.image | quote }}
          resources:
            {{- toYaml .Values.resources | nindent 12 }}
{{- end }}
`,
		"manifest.yaml": `# REVIEW ONLY: replicas zero; replace the image and choose a namespace before use.
apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: kubewiser-canary-review
spec:
  replicas: 0
  selector:
    matchLabels:
      app: kubewiser-canary-review
  template:
    metadata:
      labels:
        app: kubewiser-canary-review
    spec:
      automountServiceAccountToken: false
      containers:
        - name: canary
          image: REPLACE_WITH_APPROVED_IMAGE
`,
	}
	extra, e := configuredTemplates(p, files)
	if e != nil {
		return nil, e
	}
	set, e := manifest.Decode(context.Background(), []byte(files["manifest.yaml"]), "generated-manifest")
	if e != nil || len(set.Issues) > 0 {
		return nil, fmt.Errorf("generated manifest could not be decoded")
	}
	validation, e := scan.Analyze(context.Background(), set, p.To, false)
	if e != nil || len(validation.Findings) > 0 {
		return nil, fmt.Errorf("generated manifest does not pass target schema validation")
	}

	if e = os.Mkdir(output, 0700); e != nil {
		return nil, e
	}
	names := []string{"README.md", "project.json", "acceptance.md", "nodepool.tf", "chart/Chart.yaml", "chart/values.yaml", "chart/templates/canary.yaml", "manifest.yaml"}
	names = append(names, extra...)
	for _, name := range names {
		path := filepath.Join(output, filepath.FromSlash(name))
		if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			return nil, e
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return nil, err
		}
		_, err = f.WriteString(files[name])
		closeErr := f.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
	}
	return names, nil
}
