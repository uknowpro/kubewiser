# Inspection scope and operational controls

## Collection

`collect` requires an explicit kubeconfig, context, namespace and new output filename. Default: 2 GET requests/second, one request at a time, 200 items/page, 5,000 items total, 30 seconds/request and 2 minutes overall. Configurable ceilings: 10 requests/second, 10,000 items, 10 minutes. Responses are limited to 8 MiB each, 64 MiB cumulatively and 100 pages per resource type. Serialized snapshots also have a 64 MiB limit. RBAC errors, unavailable API versions, repeated page tokens and limits are recorded as gaps; a snapshot is partial, not an atomic point-in-time view.

The namespace allowlist contains Pods, Services, ConfigMaps, PVCs, ServiceAccounts, Deployments, StatefulSets, DaemonSets, CronJobs, Ingresses, NetworkPolicies and PDBs. `--include-cluster-resources` adds Nodes and StorageClasses. Secrets, other namespaces, CRDs and custom resources are never collected. Use a separately prepared least-privilege credential granting list permission only for the required resources and GET `/version`. Older servers may not provide every stable API path; these are gaps rather than silently converted resources.

TLS verification is required except loopback HTTP test servers. Exec/auth-provider plugins, kubeconfig proxies, environment proxies, cross-origin requests and redirects are disabled. Static token or certificate credentials are supported. API server DNS resolution may use your configured resolver. This is communication with your selected API server, not external reporting.

Snapshots remove annotations, ConfigMap data, literal environment values, commands and arguments, and record the omitted paths. This is targeted redaction, not a universal secret detector: labels, resource names, image references, status and other fields remain. ConfigMap contents are received from the API server before being removed. Output is created with exclusive creation and mode 0600 on POSIX; Windows permissions follow the directory ACL. Store snapshots accordingly. Existing files are never overwritten.

## Input and Helm limits

Manifest files: 16 MiB each, 64 MiB total, 1,000 files, 10,000 resources and nesting depth 100. YAML aliases and duplicate keys are rejected. List items are recursively checked. Reports retain file/document/field positions, input hashes and partial failures without raw parser excerpts.

Helm uses a local unpacked chart with vendored dependencies and a local optional values file. Symlinks and nonregular files are rejected; input/output sizes are bounded. No Helm installation or cluster connection is needed. `lookup` returns no live result, DNS functions are not enabled, hooks are only rendered, dependencies are never fetched. Values schema validation is intentionally skipped to prevent remote references. Rendered output is validated against the target Kubernetes schema. Release, namespace, chart version, values hash and capabilities are recorded. `--api-versions` replaces the embedded target capabilities when explicitly supplied. Charts can use nondeterministic template functions; identical input is not a guarantee of identical rendering. Template execution is in-process and is not a sandbox for hostile charts; cancellation is checked around rendering, not during every template instruction.

## Drift, upgrade and network interpretation

Drift excludes status, server metadata and redacted paths; containers/env/volumes are matched by name and volumeMounts by mountPath. Observed-only fields may be defaults or controller additions. Managed-field ownership and arbitrary CRD list semantics are not established. Different API versions require conversion and are reported separately. An observed-only resource is not a deletion recommendation; an absent resource may be outside snapshot scope.

Upgrade plans provide one-minor steps and target findings. Stage-specific vendor approval, addon matrices, HA component skew, audit logs of old API callers, feature gates, default/behavior changes and runtime application tests remain explicit follow-up work. Node-local paths, privileged settings, runtimeClass and sysctls are dependency signals, not proof of OS/kernel compatibility.

Network reports analyze policy isolation intent and configured exposure. They do not evaluate all allow-rule combinations, CNI enforcement, routing or cloud firewalls. Hubble JSON lines are existing local observations, aggregated by endpoints/verdict with observed timestamps. No agent or probe is started; missing observations do not mean a connection is unused or impossible. Flow input is limited to 16 MiB, 10,000 records and 1 MiB per line.

Migration plans inventory recognized dependencies for distinct `eks`, `aks` and `gke` providers. They do not provision infrastructure, generate apply-ready templates, transfer data or perform DNS cutover. Redacted snapshots lose provider annotations; original reviewed manifests give stronger migration evidence.
