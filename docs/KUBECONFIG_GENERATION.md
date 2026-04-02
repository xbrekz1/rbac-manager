# kubeconfigctl — Kubeconfig Generation

`kubeconfigctl` is a CLI tool for generating kubeconfig files for users managed by rbac-manager. It finds the ServiceAccount created by an `AccessGrant`, issues a time-bound token, and packages everything into a ready-to-use kubeconfig file.

---

## Prerequisites

- `kubectl` configured and pointing to the target cluster
- rbac-manager installed and running
- Permissions to create ServiceAccount tokens (`kubectl create token`)

---

## Installation

### Homebrew (macOS / Linux) — recommended

```bash
brew install xbrekz1/rbac-manager/kubeconfigctl 
```

### Download pre-built binary

Pre-built binaries for macOS, Linux, and Windows are attached to every [GitHub Release](https://github.com/xbrekz1/rbac-manager/releases).

```bash
# macOS (Apple Silicon)
curl -fsSL https://github.com/xbrekz1/rbac-manager/releases/latest/download/kubeconfigctl-darwin-arm64.tar.gz | tar -xz
sudo mv kubeconfigctl /usr/local/bin/

# macOS (Intel)
curl -fsSL https://github.com/xbrekz1/rbac-manager/releases/latest/download/kubeconfigctl-darwin-amd64.tar.gz | tar -xz
sudo mv kubeconfigctl /usr/local/bin/

# Linux (amd64)
curl -fsSL https://github.com/xbrekz1/rbac-manager/releases/latest/download/kubeconfigctl-linux-amd64.tar.gz | tar -xz
sudo mv kubeconfigctl /usr/local/bin/
```

### Build from source

```bash
go install github.com/xbrekz1/rbac-manager/cmd/kubeconfigctl@latest
```

Verify:

```bash
kubeconfigctl --version
```

---

## Quick Start

**1. Create an AccessGrant:**

```yaml
apiVersion: rbacmanager.io/v1alpha1
kind: AccessGrant
metadata:
  name: john-dev
  namespace: rbac-manager
spec:
  role: developer
  namespaces: [backend-dev, backend-staging]
  serviceAccountName: john-dev-sa
```

```bash
kubectl apply -f john-accessgrant.yaml
```

**2. Wait for Active status:**

```bash
kubectl get accessgrant john-dev -n rbac-manager
# NAME       ROLE        SERVICEACCOUNT   PHASE    AGE
# john-dev   developer   john-dev-sa      Active   5s
```

**3. Generate kubeconfig:**

```bash
kubeconfigctl generate john-dev
# Saved to: ~/Downloads/kubeconfig-john-dev.yaml
```

**4. Use it:**

```bash
export KUBECONFIG=~/Downloads/kubeconfig-john-dev.yaml
kubectl auth whoami
kubectl get pods -n backend-dev
```

---

## Commands

### `generate`

Generate a kubeconfig file for an AccessGrant.

```bash
kubeconfigctl generate <name> [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `-n, --namespace` | `rbac-manager` | Namespace of the AccessGrant |
| `-d, --duration` | `8760h` | Token validity duration (e.g. `1h`, `720h`, `8760h`) |
| `-o, --output` | `~/Downloads` | Output directory |
| `--default-namespace` | `default` | Default namespace set in the kubeconfig context |
| `--cluster-name` | auto-detect | Cluster name in kubeconfig (default: current context) |
| `--server` | auto-detect | Kubernetes API server URL (default: current context) |
| `--kubeconfig` | `~/.kube/config` | Path to your admin kubeconfig |

**Examples:**

```bash
# Default: 1-year token, saved to ~/Downloads
kubeconfigctl generate john-dev

# 30-day token
kubeconfigctl generate john-dev --duration 720h

# Custom output directory and default namespace
kubeconfigctl generate john-dev -o /tmp --default-namespace backend-dev

# AccessGrant in a non-default namespace
kubeconfigctl generate john-dev -n my-namespace
```

The command outputs two files:
- `kubeconfig-<name>.yaml` — the kubeconfig (permissions `0600`)
- `kubeconfig-<name>-instructions.txt` — usage instructions and security notes

---

### `list`

List AccessGrants and their status.

```bash
kubeconfigctl list [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `-n, --namespace` | `rbac-manager` | Namespace to list |
| `-A, --all-namespaces` | `false` | List across all namespaces |

**Examples:**

```bash
# List in default namespace
kubeconfigctl list

# List in a specific namespace
kubeconfigctl list -n production

# List across all namespaces
kubeconfigctl list -A
```

**Output:**

```
NAME       SERVICEACCOUNT   ROLE        PHASE
john-dev   john-dev-sa      developer   Active
alice      alice-sa         viewer      Active
ci-bot     ci-bot-sa        deployer    Active
```

---

## Token Duration

Tokens are issued via `kubectl create token` and expire automatically. Choose duration based on use case:

| Duration | Use case |
|----------|----------|
| `1h` – `24h` | Temporary / incident response access |
| `720h` (30d) | Developer access — recommended rotation |
| `2160h` (90d) | Extended developer access |
| `8760h` (1y) | Default — long-lived CI/CD or service accounts |
| `17520h` (2y) | CI/CD pipelines with infrequent rotation |

A token becomes invalid when:
- Its duration expires
- The AccessGrant is deleted
- The ServiceAccount is deleted

To renew, simply generate a new kubeconfig:

```bash
kubeconfigctl generate john-dev
```

The old token continues to work until it expires — there is no invalidation on regeneration.

---

## Revoking Access

**Immediately (delete everything):**

```bash
kubectl delete accessgrant john-dev -n rbac-manager
# ServiceAccount, Roles, RoleBindings — all removed by rbac-manager
```

**Delete only the ServiceAccount (token stops working, RBAC stays):**

```bash
kubectl delete serviceaccount john-dev-sa -n rbac-manager
```

---

## Common Examples

### Developer with 30-day access

```bash
kubectl apply -f - <<EOF
apiVersion: rbacmanager.io/v1alpha1
kind: AccessGrant
metadata:
  name: alice-dev
  namespace: rbac-manager
spec:
  role: developer
  namespaces: [development, staging]
  serviceAccountName: alice-dev-sa
EOF

kubeconfigctl generate alice-dev --duration 720h
```

### CI/CD pipeline

```bash
kubectl apply -f - <<EOF
apiVersion: rbacmanager.io/v1alpha1
kind: AccessGrant
metadata:
  name: gitlab-ci
  namespace: rbac-manager
spec:
  role: deployer
  namespaces: [production]
  serviceAccountName: gitlab-ci-sa
EOF

kubeconfigctl generate gitlab-ci --duration 8760h -o /tmp/ci
```

### Incident responder (short-lived)

```bash
kubectl apply -f - <<EOF
apiVersion: rbacmanager.io/v1alpha1
kind: AccessGrant
metadata:
  name: oncall-debugger
  namespace: rbac-manager
spec:
  role: debugger
  namespaces: [production]
EOF

kubeconfigctl generate oncall-debugger --duration 4h
# Revoke when done:
kubectl delete accessgrant oncall-debugger -n rbac-manager
```

### Cluster-wide admin access

```bash
kubectl apply -f - <<EOF
apiVersion: rbacmanager.io/v1alpha1
kind: AccessGrant
metadata:
  name: platform-bot
  namespace: rbac-manager
spec:
  role: cluster-admin
  clusterWide: true
  serviceAccountName: platform-bot-sa
EOF

kubeconfigctl generate platform-bot
```

---

## Security

**Do:**
- Use short-lived tokens for human users (30–90 days)
- Set permissions `600` on kubeconfig files (done automatically)
- Transfer kubeconfig files via encrypted channels (not Slack, unencrypted email)
- Audit active AccessGrants regularly: `kubeconfigctl list -A`
- Delete old kubeconfig files when no longer needed

**Don't:**
- Commit kubeconfig files to git repositories
- Share tokens via Slack, Teams, or unencrypted email
- Use `cluster-admin` without `clusterWide: true`
- Reuse one AccessGrant/kubeconfig for multiple people

---

## Troubleshooting

### `AccessGrant not found`

```bash
# Check the namespace
kubeconfigctl list -A
kubectl get accessgrants -A
```

### `AccessGrant is not in Active phase`

```bash
# Check status and events
kubectl describe accessgrant <name> -n rbac-manager
kubectl logs -n rbac-manager -l app.kubernetes.io/name=rbac-manager
```

### `failed to create token`

```bash
# Verify permissions
kubectl auth can-i create serviceaccounts/token -n rbac-manager

# Test manually
kubectl create token <sa-name> -n rbac-manager --duration=1h
```

### `Unauthorized` when using kubeconfig

```bash
# Check AccessGrant and ServiceAccount still exist
kubectl get accessgrant <name> -n rbac-manager
kubectl get serviceaccount <sa-name> -n rbac-manager

# Regenerate kubeconfig with a fresh token
kubeconfigctl generate <name>
```

### `Permission denied` on resources

```bash
# Check what role the AccessGrant has
kubectl get accessgrant <name> -n rbac-manager -o jsonpath='{.spec.role}'

# List permissions
export KUBECONFIG=~/Downloads/kubeconfig-<name>.yaml
kubectl auth can-i --list
```

---

## Legacy: Taskfile

The `access-permissions/Taskfile.yml` provides task-based wrappers around `kubeconfigctl` for teams that prefer `task`:

```bash
cd access-permissions

task generate ACCESSGRANT=john-dev           # generate kubeconfig
task generate ACCESSGRANT=john-dev DURATION=720h
task list                                     # list AccessGrants
task test ACCESSGRANT=john-dev                # verify kubeconfig works
task batch-generate                           # generate for all AccessGrants
task cleanup-kubeconfigs                      # delete generated files
```

---

## References

- [rbac-manager README](../README.md)
- [Kubernetes RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
- [kubectl create token](https://kubernetes.io/docs/reference/generated/kubectl/kubectl-commands#-em-token-em-)
