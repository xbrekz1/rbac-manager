# rbac-manager

A Kubernetes operator for declarative RBAC management via a single custom resource.

[![CI](https://github.com/xbrekz1/rbac-manager/actions/workflows/ci.yml/badge.svg)](https://github.com/xbrekz1/rbac-manager/actions/workflows/ci.yml)
[![Release](https://github.com/xbrekz1/rbac-manager/actions/workflows/release.yml/badge.svg)](https://github.com/xbrekz1/rbac-manager/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/xbrekz1/rbac-manager)](https://goreportcard.com/report/github.com/xbrekz1/rbac-manager)

Instead of manually creating ServiceAccounts, Roles, and RoleBindings across namespaces — you declare an `AccessGrant`. The operator handles the rest, including guaranteed cleanup on deletion via Kubernetes finalizers.

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

---

## Features

- **Event-driven** — reacts to changes instantly via Kubernetes watch, no polling
- **Finalizer-based cleanup** — all RBAC resources are deleted when AccessGrant is removed, even if the operator was temporarily down
- **Self-healing** — periodically reconciles to restore resources deleted externally
- **9 predefined roles** — covering common access patterns out of the box
- **Custom rules** — full `PolicyRule` support when predefined roles aren't enough
- **ClusterWide mode** — one flag to switch from namespace-scoped to cluster-scoped
- **HA ready** — leader election for multi-replica deployments
- **Minimal image** — distroless base, non-root, read-only filesystem

---

## Predefined roles

| Role | Exec | Secrets | Deploy | Use case |
|------|:----:|:-------:|:------:|----------|
| `reader` | — | — | — | Stakeholders, dashboards — workloads visible, no logs or secrets |
| `viewer` | — | — | — | Monitoring teams, on-call — pods and logs |
| `developer` | ✓ | read | — | Developers — exec into pods, read configmaps and secrets |
| `developer-extended` | ✓ | read | — | Same as `developer` + namespace listing for [OpenLens](https://github.com/MuhammedKalkan/OpenLens) |
| `deployer` | — | — | ✓ | CI/CD pipelines — update deployments, services, jobs; no exec, no secrets |
| `debugger` | ✓ | — | — | Incident response — exec, logs, port-forward; read-only otherwise |
| `operator` | ✓ | read | ✓ | SRE teams — full workload management, ingresses, HPAs |
| `auditor` | — | read | — | Security reviews — read everything including secrets and RBAC rules |
| `maintainer` | ✓ | ✓ | ✓ | Tech leads — full access within the namespace |

---

## Installation

**Requirements:** Kubernetes 1.28+, Helm 3.x

```bash
helm install rbac-manager oci://ghcr.io/xbrekz1/charts/rbac-manager \
  --namespace rbac-manager \
  --create-namespace \
  --wait
```

From source:

```bash
git clone https://github.com/xbrekz1/rbac-manager && cd rbac-manager
helm install rbac-manager . --namespace rbac-manager --create-namespace --wait
```

---

## Usage

### Grant access

```bash
kubectl apply -f - <<EOF
apiVersion: rbacmanager.io/v1alpha1
kind: AccessGrant
metadata:
  name: alice
  namespace: rbac-manager
spec:
  role: developer
  namespaces: [backend-dev, backend-staging]
  serviceAccountName: alice-sa
EOF
```

```bash
kubectl get accessgrants -n rbac-manager
# NAME    ROLE        SERVICEACCOUNT   NAMESPACES                      PHASE    AGE
# alice   developer   alice-sa         [backend-dev backend-staging]   Active   3s
```

### Generate kubeconfig

```bash
cd access-permissions
task generate-kubeconfig ACCESSGRANT=alice
# ~/Downloads/kubeconfig-alice-sa.yaml
```

### Revoke access

```bash
kubectl delete accessgrant alice -n rbac-manager
# ServiceAccount, Roles, RoleBindings — all deleted immediately
```

---

## AccessGrant spec

```yaml
spec:
  # Predefined role (mutually exclusive with customRules)
  role: developer

  # Custom RBAC rules
  # customRules:
  #   - apiGroups: [""]
  #     resources: ["pods"]
  #     verbs: ["get", "list", "watch"]

  # Target namespaces (ignored when clusterWide: true)
  namespaces:
    - my-namespace

  # Create ClusterRole + ClusterRoleBinding instead of namespace-scoped resources
  clusterWide: false

  # ServiceAccount name (default: "rbac-<name>")
  serviceAccountName: my-sa

  # Labels and annotations propagated to all managed resources
  labels:
    team: backend
  annotations:
    owner: alice@example.com
    expires-at: "2026-12-31"
```

---

## Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| `image.tag` | Chart.AppVersion | Image tag |
| `operator.logLevel` | `info` | Log level: `debug`, `info`, `error` |
| `operator.leaderElection` | `false` | Enable for HA (multiple replicas) |
| `resources.limits.cpu` | `400m` | CPU limit |
| `resources.limits.memory` | `256Mi` | Memory limit |
| `metrics.port` | `8080` | Prometheus metrics port |

---

## Releasing

```bash
git tag v1.2.3
git push origin v1.2.3
```

The [release workflow](.github/workflows/release.yml) builds a multi-arch image (`linux/amd64`, `linux/arm64`), pushes it to `ghcr.io`, packages the Helm chart, and creates a GitHub Release.

---

## License

MIT
