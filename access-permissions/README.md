# access-permissions

Taskfile wrappers for kubeconfig generation using `kubeconfigctl`.

For full documentation see [docs/KUBECONFIG_GENERATION.md](../docs/KUBECONFIG_GENERATION.md).

---

## Quick start

```bash
# Generate kubeconfig for an AccessGrant
task generate ACCESSGRANT=john-dev

# Generate with 30-day token
task generate ACCESSGRANT=john-dev DURATION=720h

# List all AccessGrants
task list

# Test generated kubeconfig
task test ACCESSGRANT=john-dev

# Install kubeconfigctl CLI
task install-cli
```

## File structure

```
access-permissions/
├── Taskfile.yml                    # Task commands
├── scripts/
│   └── generate-kubeconfig.sh     # Legacy shell script
└── templates/
    └── kubeconfig-template.yaml   # Kubeconfig template
```
