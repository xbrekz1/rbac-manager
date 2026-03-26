# rbac-manager

A production-grade Kubernetes operator for declarative RBAC management.

[![CI](https://github.com/xbrekz1/rbac-manager/actions/workflows/ci.yml/badge.svg)](https://github.com/xbrekz1/rbac-manager/actions/workflows/ci.yml)
[![Release](https://github.com/xbrekz1/rbac-manager/actions/workflows/release.yml/badge.svg)](https://github.com/xbrekz1/rbac-manager/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/xbrekz1/rbac-manager)](https://goreportcard.com/report/github.com/xbrekz1/rbac-manager)

---

## Overview

Managing Kubernetes RBAC by hand doesn't scale. Every new team member or service account means creating a ServiceAccount, writing Role rules, creating a RoleBinding — and repeating that across every namespace. And when someone leaves, you have to remember to clean it all up.

**rbac-manager** solves this with a single Custom Resource:

```yaml
apiVersion: rbacmanager.io/v1alpha1
kind: AccessGrant
metadata:
  name: john-developer
  namespace: rbac-manager
spec:
  role: developer
  namespaces: [backend-dev, backend-staging]
  serviceAccountName: john-dev-sa
```

Apply it — the operator creates the ServiceAccount, Roles, and RoleBindings. Delete it — everything is cleaned up instantly via Kubernetes finalizers.

---

## How it works

```
kubectl apply -f access-grant.yaml
        │
        ▼
┌─────────────────┐     watches      ┌──────────────────────┐
│   AccessGrant   │ ◄─────────────── │   rbac-manager       │
│   (your YAML)   │                  │   (Go operator)      │
└─────────────────┘                  └──────────┬───────────┘
                                                │ creates
                                    ┌───────────▼───────────┐
                                    │  ServiceAccount        │
                                    │  Role (per namespace)  │
                                    │  RoleBinding           │
                                    │  ClusterRole (if need) │
                                    └───────────────────────┘
```

The operator watches for AccessGrant changes and reconciles immediately — no polling, no delay.

---

## Features

- **Instant reconciliation** — event-driven via Kubernetes watch, not polling
- **Guaranteed cleanup** — finalizers ensure all RBAC resources are deleted when AccessGrant is removed, even if the operator was down
- **9 predefined roles** — from read-only reader to full maintainer
- **Custom rules** — bring your own RBAC PolicyRules when predefined roles aren't enough
- **ClusterWide mode** — create ClusterRole + ClusterRoleBinding with one flag
- **HA ready** — leader election support for multi-replica deployments
- **Self-healing** — periodic reconciliation detects and restores externally deleted resources
- **Prometheus metrics** — built-in via controller-runtime on `:8080`
- **Minimal footprint** — distroless container image, non-root, read-only filesystem

---

## Predefined roles

| Role | Exec | Secrets | Deploy | Description |
|------|:----:|:-------:|:------:|-------------|
| `reader` | — | — | — | View workloads only — no logs, no secrets. For stakeholders and dashboards. |
| `viewer` | — | — | — | View pods and logs. For monitoring teams and on-call rotation. |
| `developer` | ✓ | read | — | Debug access: exec into pods, read configmaps and secrets. |
| `developer-extended` | ✓ | read | — | Same as `developer` + namespace listing for [OpenLens](https://github.com/MuhammedKalkan/OpenLens) navigation. |
| `deployer` | — | — | ✓ | Update deployments, services, jobs. For CI/CD pipelines (GitLab CI, GitHub Actions). |
| `debugger` | ✓ | — | — | Exec, logs, port-forward. No config changes. For incident response. |
| `operator` | ✓ | read | ✓ | Full workload management for SRE teams. Restart pods, scale deployments, manage ingresses. |
| `auditor` | — | read | — | Read everything including secrets and RBAC rules. For security audits and compliance. |
| `maintainer` | ✓ | ✓ | ✓ | Full access to all resources in the namespace. For tech leads and platform engineers. |

> **Tip:** Use `customRules` when no predefined role fits your use case exactly.

---

## Installation

### Prerequisites

- Kubernetes 1.28+
- Helm 3.x

### Install with Helm

```bash
# Add the repository
helm install rbac-manager oci://ghcr.io/batonogov/charts/rbac-manager \
  --namespace rbac-manager \
  --create-namespace \
  --wait
```

### Install from source

```bash
git clone https://github.com/xbrekz1/rbac-manager
cd rbac-manager

kubectl create namespace rbac-manager
helm install rbac-manager . --namespace rbac-manager --wait
```

### Verify

```bash
kubectl get pods -n rbac-manager
# NAME                            READY   STATUS    RESTARTS   AGE
# rbac-manager-6d9f8b7c5d-x4k2p  1/1     Running   0          30s

kubectl get crd accessgrants.rbacmanager.io
# NAME                            CREATED AT
# accessgrants.rbacmanager.io     2026-01-01T00:00:00Z
```

---

## Usage

### 1. Create an AccessGrant

```bash
kubectl apply -f - <<EOF
apiVersion: rbacmanager.io/v1alpha1
kind: AccessGrant
metadata:
  name: john-developer
  namespace: rbac-manager
spec:
  role: developer
  namespaces:
    - backend-dev
    - backend-staging
  serviceAccountName: john-dev-sa
EOF
```

### 2. Check the status

```bash
kubectl get accessgrants -n rbac-manager
# NAME             ROLE        SERVICEACCOUNT   NAMESPACES                      PHASE    AGE
# john-developer   developer   john-dev-sa      [backend-dev backend-staging]   Active   5s
```

### 3. Generate a kubeconfig for the user

```bash
cd access-permissions
task generate-kubeconfig ACCESSGRANT=john-developer
# Kubeconfig saved to ~/Downloads/kubeconfig-john-dev-sa.yaml
```

### 4. Test access

```bash
task test-access-grant ACCESSGRANT=john-developer
```

### 5. Revoke access

```bash
kubectl delete accessgrant john-developer -n rbac-manager
# All managed resources (ServiceAccount, Roles, RoleBindings) are deleted immediately.
```

---

## AccessGrant spec reference

```yaml
apiVersion: rbacmanager.io/v1alpha1
kind: AccessGrant
metadata:
  name: example
  namespace: rbac-manager
spec:
  # Predefined role name (mutually exclusive with customRules)
  role: developer

  # Custom RBAC rules (mutually exclusive with role)
  # customRules:
  #   - apiGroups: [""]
  #     resources: ["pods", "services"]
  #     verbs: ["get", "list", "watch"]

  # Target namespaces for Role + RoleBinding
  namespaces:
    - my-namespace
    - another-namespace

  # Create ClusterRole + ClusterRoleBinding instead of namespace-scoped resources
  clusterWide: false

  # ServiceAccount name (defaults to "rbac-<accessgrant-name>")
  serviceAccountName: my-sa

  # Extra labels added to all managed resources
  labels:
    team: backend
    environment: staging

  # Extra annotations added to all managed resources
  annotations:
    owner: john@example.com
    expires-at: "2026-12-31"
```

---

## Examples

### Developer access for a team member

```yaml
apiVersion: rbacmanager.io/v1alpha1
kind: AccessGrant
metadata:
  name: alice-backend
  namespace: rbac-manager
spec:
  role: developer
  namespaces: [backend-dev, backend-staging]
  serviceAccountName: alice-sa
  labels:
    team: backend
```

### CI/CD pipeline (GitLab Runner, GitHub Actions)

```yaml
apiVersion: rbacmanager.io/v1alpha1
kind: AccessGrant
metadata:
  name: gitlab-ci
  namespace: rbac-manager
spec:
  role: deployer          # Can update deployments — cannot exec or read secrets
  namespaces: [production, staging]
  serviceAccountName: gitlab-runner-sa
```

### Temporary audit access with expiry annotation

```yaml
apiVersion: rbacmanager.io/v1alpha1
kind: AccessGrant
metadata:
  name: security-audit-q1
  namespace: rbac-manager
  annotations:
    expires-at: "2026-04-01"   # Reminder — not enforced automatically
    ticket: "SEC-4821"
spec:
  role: auditor
  clusterWide: true
  serviceAccountName: security-auditor-sa
```

### Custom rules when no predefined role fits

```yaml
apiVersion: rbacmanager.io/v1alpha1
kind: AccessGrant
metadata:
  name: prometheus-scraper
  namespace: rbac-manager
spec:
  customRules:
    - apiGroups: [""]
      resources: ["pods", "services", "endpoints"]
      verbs: ["get", "list", "watch"]
    - apiGroups: ["extensions", "networking.k8s.io"]
      resources: ["ingresses"]
      verbs: ["get", "list", "watch"]
  namespaces: [monitoring, production]
  serviceAccountName: prometheus-sa
```

### OpenLens / Lens access

```yaml
apiVersion: rbacmanager.io/v1alpha1
kind: AccessGrant
metadata:
  name: alice-openlens
  namespace: rbac-manager
spec:
  role: developer-extended  # Adds namespace listing required for OpenLens sidebar
  namespaces: [backend-dev, frontend-dev]
  serviceAccountName: alice-lens-sa
```

---

## Configuration

Key `values.yaml` options:

| Parameter | Default | Description |
|-----------|---------|-------------|
| `image.repository` | `ghcr.io/xbrekz1/rbac-manager` | Container image |
| `image.tag` | `""` (uses Chart.AppVersion) | Image tag |
| `replicaCount` | `1` | Number of replicas |
| `operator.logLevel` | `info` | Log level (`debug`, `info`, `error`) |
| `operator.leaderElection` | `false` | Enable leader election for HA |
| `resources.limits.cpu` | `400m` | CPU limit |
| `resources.limits.memory` | `256Mi` | Memory limit |
| `metrics.enabled` | `true` | Expose Prometheus metrics on `:8080` |

---

## Useful commands

```bash
# List all AccessGrants across all namespaces
kubectl get accessgrants -A

# Watch operator logs
kubectl logs -n rbac-manager -l app.kubernetes.io/name=rbac-manager -f

# List all RBAC resources managed by rbac-manager
kubectl get roles,rolebindings,clusterroles,clusterrolebindings \
  -A -l rbacmanager.io/managed-by=rbac-manager

# List managed ServiceAccounts
kubectl get sa -A -l rbacmanager.io/managed-by=rbac-manager
```

---

## Development

```bash
# Build
go build ./...

# Vet
go vet ./...

# Build Docker image
docker build -t rbac-manager:dev .

# Install CRD locally (requires cluster access)
kubectl apply -f templates/crd.yaml
```

### Publishing a release

```bash
git tag v1.2.3
git push origin v1.2.3
```

The [release workflow](.github/workflows/release.yml) will automatically:
1. Build and push the Docker image to `ghcr.io` (linux/amd64 + linux/arm64)
2. Package and push the Helm chart to `ghcr.io` as an OCI artifact
3. Create a GitHub Release with release notes

---

## License

MIT
