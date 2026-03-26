#!/bin/bash

set -euo pipefail

# Logging functions
log_info() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') [INFO] $*" >&2
}

log_warn() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') [WARN] $*" >&2
}

log_error() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') [ERROR] $*" >&2
}

log_debug() {
    [[ "${LOG_LEVEL:-INFO}" == "DEBUG" ]] || return 0
    echo "$(date '+%Y-%m-%d %H:%M:%S') [DEBUG] $*" >&2
}

# Configuration
NAMESPACE=${NAMESPACE:-"rbac-manager"}
CHECK_INTERVAL=${CHECK_INTERVAL:-30}

log_info "Starting RBAC Manager Simple Bash Operator"
log_info "Namespace: $NAMESPACE"
log_info "Check interval: $CHECK_INTERVAL seconds"

# Check dependencies
log_info "Checking dependencies..."

if ! command -v jq &> /dev/null; then
    log_error "jq is not available"
    exit 1
fi
log_info "✅ jq is available"

if ! command -v kubectl &> /dev/null; then
    log_error "kubectl is not available"
    exit 1
fi
log_info "✅ kubectl is available"

# Test kubectl connection
log_info "Testing kubectl connection..."
if kubectl version --client &>/dev/null; then
    log_info "✅ kubectl client works"
else
    log_error "kubectl client failed"
    exit 1
fi

if kubectl get namespaces &>/dev/null; then
    log_info "✅ kubectl can connect to cluster"
else
    log_error "kubectl cannot connect to cluster"
    exit 1
fi

# Predefined roles
get_predefined_role_rules() {
    local role="$1"
    case "$role" in
        "viewer")
            echo '[
                {
                    "apiGroups": [""],
                    "resources": ["pods"],
                    "verbs": ["get", "list", "watch"]
                },
                {
                    "apiGroups": [""],
                    "resources": ["pods/log"],
                    "verbs": ["get", "list"]
                }
            ]'
            ;;
        "developer")
            echo '[
                {
                    "apiGroups": [""],
                    "resources": ["pods"],
                    "verbs": ["get", "list", "watch"]
                },
                {
                    "apiGroups": [""],
                    "resources": ["pods/log", "pods/exec"],
                    "verbs": ["get", "list", "create"]
                }
            ]'
            ;;
        "developer-openlens")
            echo '[
                {
                    "apiGroups": [""],
                    "resources": ["pods"],
                    "verbs": ["get", "list", "watch"]
                },
                {
                    "apiGroups": [""],
                    "resources": ["pods/log", "pods/exec"],
                    "verbs": ["get", "list", "create"]
                }
            ]'
            ;;
        "maintainer")
            echo '[
                {
                    "apiGroups": ["*"],
                    "resources": ["*"],
                    "verbs": ["*"]
                }
            ]'
            ;;
        *)
            log_error "Unknown predefined role: $role"
            return 1
            ;;
    esac
}

# Create ClusterRole for namespace viewing
create_namespace_clusterrole() {
    local cluster_role_name="$1"
    local sa_name="$2"
    local sa_namespace="$3"
    local access_grant_name="$4"
    local access_grant_namespace="$5"

    log_debug "Creating ClusterRole $cluster_role_name for namespace access"

    local output
    if output=$(cat <<EOF | kubectl apply -f - 2>&1
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: $cluster_role_name
  labels:
    rbacmanager.io/managed-by: rbac-manager-sh
    rbacmanager.io/access-grant: $access_grant_name
    rbacmanager.io/access-grant-namespace: $access_grant_namespace
rules:
- apiGroups:
  - ""
  resources:
  - namespaces
  verbs:
  - get
  - list
  - watch
EOF
); then
        if echo "$output" | grep -q "created"; then
            log_info "✅ Created ClusterRole $cluster_role_name"
        else
            log_debug "ClusterRole $cluster_role_name unchanged"
        fi
    else
        log_error "Failed to create ClusterRole $cluster_role_name: $output"
        return 1
    fi

    # Create ClusterRoleBinding
    local crb_name="$cluster_role_name"
    if output=$(cat <<EOF | kubectl apply -f - 2>&1
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: $crb_name
  labels:
    rbacmanager.io/managed-by: rbac-manager-sh
    rbacmanager.io/access-grant: $access_grant_name
    rbacmanager.io/access-grant-namespace: $access_grant_namespace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: $cluster_role_name
subjects:
- kind: ServiceAccount
  name: $sa_name
  namespace: $sa_namespace
EOF
); then
        if echo "$output" | grep -q "created"; then
            log_info "✅ Created ClusterRoleBinding $crb_name"
        else
            log_debug "ClusterRoleBinding $crb_name unchanged"
        fi
    else
        log_error "Failed to create ClusterRoleBinding $crb_name: $output"
        return 1
    fi
}

# Create ServiceAccount
create_service_account() {
    local sa_name="$1"
    local sa_namespace="$2"
    local access_grant_name="$3"

    log_info "Creating ServiceAccount $sa_name in $sa_namespace"

    if kubectl get sa "$sa_name" -n "$sa_namespace" &>/dev/null; then
        log_debug "ServiceAccount $sa_name already exists"
        return 0
    fi

    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: $sa_name
  namespace: $sa_namespace
  labels:
    rbacmanager.io/managed-by: rbac-manager-sh
    rbacmanager.io/access-grant: $access_grant_name
    rbacmanager.io/access-grant-namespace: $sa_namespace
EOF

    log_info "✅ Created ServiceAccount $sa_name"
}

# Create Role
create_role() {
    local role_name="$1"
    local target_namespace="$2"
    local rules_json="$3"
    local access_grant_name="$4"
    local access_grant_namespace="$5"

    log_debug "Creating Role $role_name in $target_namespace"

    # Check if namespace exists
    if ! kubectl get namespace "$target_namespace" &>/dev/null; then
        log_warn "Namespace $target_namespace does not exist, skipping"
        return 1
    fi

    # Convert JSON rules to YAML
    local rules_yaml
    rules_yaml=$(echo "$rules_json" | jq -r '.[] | "- apiGroups: [\((.apiGroups // [""]) | map("\"" + . + "\"") | join(", "))]
  resources: [\((.resources // []) | map("\"" + . + "\"") | join(", "))]
  verbs: [\((.verbs // []) | map("\"" + . + "\"") | join(", "))]"')

    local output
    if output=$(cat <<EOF | kubectl apply -f - 2>&1
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: $role_name
  namespace: $target_namespace
  labels:
    rbacmanager.io/managed-by: rbac-manager-sh
    rbacmanager.io/access-grant: $access_grant_name
    rbacmanager.io/access-grant-namespace: $access_grant_namespace
rules:
$rules_yaml
EOF
); then
        if echo "$output" | grep -q "created"; then
            log_info "✅ Created Role $role_name in $target_namespace"
        else
            log_debug "Role $role_name in $target_namespace unchanged"
        fi
    else
        log_error "Failed to create Role $role_name in $target_namespace: $output"
        return 1
    fi
}

# Create RoleBinding
create_role_binding() {
    local rb_name="$1"
    local target_namespace="$2"
    local role_name="$3"
    local sa_name="$4"
    local sa_namespace="$5"
    local access_grant_name="$6"
    local access_grant_namespace="$7"

    log_debug "Creating RoleBinding $rb_name in $target_namespace"

    local output
    if output=$(cat <<EOF | kubectl apply -f - 2>&1
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: $rb_name
  namespace: $target_namespace
  labels:
    rbacmanager.io/managed-by: rbac-manager-sh
    rbacmanager.io/access-grant: $access_grant_name
    rbacmanager.io/access-grant-namespace: $access_grant_namespace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: $role_name
subjects:
- kind: ServiceAccount
  name: $sa_name
  namespace: $sa_namespace
EOF
); then
        if echo "$output" | grep -q "created"; then
            log_info "✅ Created RoleBinding $rb_name in $target_namespace"
        else
            log_debug "RoleBinding $rb_name in $target_namespace unchanged"
        fi
    else
        log_error "Failed to create RoleBinding $rb_name in $target_namespace: $output"
        return 1
    fi
}

# Process AccessGrant
process_access_grant() {
    local access_grant="$1"

    log_debug "Processing AccessGrant: $access_grant"

    local name
    local namespace
    local spec
    local target_namespaces
    local role_name
    local sa_name
    local rules

    # Extract metadata
    name=$(echo "$access_grant" | jq -r '.metadata.name')
    namespace=$(echo "$access_grant" | jq -r '.metadata.namespace')
    spec=$(echo "$access_grant" | jq -r '.spec')

    log_info "📋 Processing AccessGrant: $name in namespace $namespace"

    # Get parameters
    target_namespaces=$(echo "$spec" | jq -r '.namespaces[]?' 2>/dev/null || echo "")
    role_name=$(echo "$spec" | jq -r '.role // empty')
    sa_name=$(echo "$spec" | jq -r ".serviceAccountName // \"rbac-$name\"")

    log_info "  - ServiceAccount: $sa_name"
    log_info "  - Role: $role_name"
    log_info "  - Target namespaces: $(echo "$target_namespaces" | tr '\n' ' ')"

    # Get rules
    if [[ -n "$role_name" && "$role_name" != "null" ]]; then
        log_info "Getting predefined role rules for: $role_name"
        rules=$(get_predefined_role_rules "$role_name")
        if [[ $? -ne 0 ]]; then
            log_error "Failed to get rules for role: $role_name"
            return 1
        fi
    else
        log_error "Only predefined roles are supported in this simple version"
        return 1
    fi

    # Create ServiceAccount
    create_service_account "$sa_name" "$namespace" "$name"

    # Create ClusterRole for namespace access (viewer and developer-openlens roles)
    if [[ "$role_name" == "viewer" || "$role_name" == "developer-openlens" ]]; then
        local cluster_role_name="rbac-$name-namespace-viewer"
        create_namespace_clusterrole "$cluster_role_name" "$sa_name" "$namespace" "$name" "$namespace"
    fi

    # Create Roles and RoleBindings for each namespace
    local created_count=0
    while IFS= read -r target_ns; do
        if [[ -n "$target_ns" ]]; then
            local role_resource_name="rbac-$name"
            local rb_resource_name="rbac-$name"

            if create_role "$role_resource_name" "$target_ns" "$rules" "$name" "$namespace"; then
                create_role_binding "$rb_resource_name" "$target_ns" "$role_resource_name" "$sa_name" "$namespace" "$name" "$namespace"
                ((created_count++))
            fi
        fi
    done <<< "$target_namespaces"

    if [[ $created_count -gt 0 ]]; then
        log_info "✅ AccessGrant $name processed successfully in $created_count namespaces"
        local processed_namespaces
        processed_namespaces=$(echo "$target_namespaces" | jq -R . | jq -sc .)
        kubectl patch accessgrant "$name" -n "$namespace" --subresource=status --type=merge \
            -p "{\"status\":{\"phase\":\"Active\",\"serviceAccount\":\"$sa_name\",\"namespaces\":$processed_namespaces,\"message\":\"Processed $created_count namespace(s)\"}}" \
            2>/dev/null || log_warn "Failed to update status for AccessGrant $name"
    else
        log_warn "⚠️ AccessGrant $name - no target namespaces found or accessible"
        kubectl patch accessgrant "$name" -n "$namespace" --subresource=status --type=merge \
            -p "{\"status\":{\"phase\":\"Failed\",\"message\":\"No target namespaces found or accessible\"}}" \
            2>/dev/null || log_warn "Failed to update status for AccessGrant $name"
    fi
}

# Cleanup orphaned resources
cleanup_orphaned_resources() {
    log_debug "🧹 Starting cleanup of orphaned resources..."

    # Get all existing AccessGrants as a reference
    local existing_grants
    existing_grants=$(kubectl get accessgrants -A -o json 2>/dev/null | jq -r '.items[] | "\(.metadata.name):\(.metadata.namespace)"' | sort)

    # Cleanup ServiceAccounts
    log_debug "Checking for orphaned ServiceAccounts..."
    local sa_list
    sa_list=$(kubectl get sa -A -l 'rbacmanager.io/managed-by=rbac-manager-sh' -o json 2>/dev/null | jq -r '.items[] | "\(.metadata.labels["rbacmanager.io/access-grant"]):\(.metadata.labels["rbacmanager.io/access-grant-namespace"]) \(.metadata.name) \(.metadata.namespace)"' 2>/dev/null || echo "")

    while IFS= read -r sa_info; do
        if [[ -n "$sa_info" ]]; then
            local grant_ref=$(echo "$sa_info" | awk '{print $1}')
            local sa_name=$(echo "$sa_info" | awk '{print $2}')
            local sa_namespace=$(echo "$sa_info" | awk '{print $3}')

            if [[ -n "$grant_ref" && "$grant_ref" != "null:null" ]]; then
                if ! echo "$existing_grants" | grep -q "^$grant_ref$"; then
                    log_info "🗑️ Deleting orphaned ServiceAccount: $sa_name in $sa_namespace"
                    kubectl delete sa "$sa_name" -n "$sa_namespace" 2>/dev/null || log_warn "Failed to delete SA $sa_name"
                fi
            fi
        fi
    done <<< "$sa_list"

    # Cleanup Roles
    log_debug "Checking for orphaned Roles..."
    local role_list
    role_list=$(kubectl get roles -A -l 'rbacmanager.io/managed-by=rbac-manager-sh' -o json 2>/dev/null | jq -r '.items[] | "\(.metadata.labels["rbacmanager.io/access-grant"]):\(.metadata.labels["rbacmanager.io/access-grant-namespace"]) \(.metadata.name) \(.metadata.namespace)"' 2>/dev/null || echo "")

    while IFS= read -r role_info; do
        if [[ -n "$role_info" ]]; then
            local grant_ref=$(echo "$role_info" | awk '{print $1}')
            local role_name=$(echo "$role_info" | awk '{print $2}')
            local role_namespace=$(echo "$role_info" | awk '{print $3}')

            if [[ -n "$grant_ref" && "$grant_ref" != "null:null" ]]; then
                if ! echo "$existing_grants" | grep -q "^$grant_ref$"; then
                    log_info "🗑️ Deleting orphaned Role: $role_name in $role_namespace"
                    kubectl delete role "$role_name" -n "$role_namespace" 2>/dev/null || log_warn "Failed to delete Role $role_name"
                fi
            fi
        fi
    done <<< "$role_list"

    # Cleanup RoleBindings
    log_debug "Checking for orphaned RoleBindings..."
    local rb_list
    rb_list=$(kubectl get rolebindings -A -l 'rbacmanager.io/managed-by=rbac-manager-sh' -o json 2>/dev/null | jq -r '.items[] | "\(.metadata.labels["rbacmanager.io/access-grant"]):\(.metadata.labels["rbacmanager.io/access-grant-namespace"]) \(.metadata.name) \(.metadata.namespace)"' 2>/dev/null || echo "")

    while IFS= read -r rb_info; do
        if [[ -n "$rb_info" ]]; then
            local grant_ref=$(echo "$rb_info" | awk '{print $1}')
            local rb_name=$(echo "$rb_info" | awk '{print $2}')
            local rb_namespace=$(echo "$rb_info" | awk '{print $3}')

            if [[ -n "$grant_ref" && "$grant_ref" != "null:null" ]]; then
                if ! echo "$existing_grants" | grep -q "^$grant_ref$"; then
                    log_info "🗑️ Deleting orphaned RoleBinding: $rb_name in $rb_namespace"
                    kubectl delete rolebinding "$rb_name" -n "$rb_namespace" 2>/dev/null || log_warn "Failed to delete RoleBinding $rb_name"
                fi
            fi
        fi
    done <<< "$rb_list"

    # Cleanup ClusterRoles
    log_debug "Checking for orphaned ClusterRoles..."
    local cr_list
    cr_list=$(kubectl get clusterroles -l 'rbacmanager.io/managed-by=rbac-manager-sh' -o json 2>/dev/null | jq -r '.items[] | "\(.metadata.labels["rbacmanager.io/access-grant"]):\(.metadata.labels["rbacmanager.io/access-grant-namespace"]) \(.metadata.name)"' 2>/dev/null || echo "")

    while IFS= read -r cr_info; do
        if [[ -n "$cr_info" ]]; then
            local grant_ref=$(echo "$cr_info" | awk '{print $1}')
            local cr_name=$(echo "$cr_info" | awk '{print $2}')

            if [[ -n "$grant_ref" && "$grant_ref" != "null:null" ]]; then
                if ! echo "$existing_grants" | grep -q "^$grant_ref$"; then
                    log_info "🗑️ Deleting orphaned ClusterRole: $cr_name"
                    kubectl delete clusterrole "$cr_name" 2>/dev/null || log_warn "Failed to delete ClusterRole $cr_name"
                fi
            fi
        fi
    done <<< "$cr_list"

    # Cleanup ClusterRoleBindings
    log_debug "Checking for orphaned ClusterRoleBindings..."
    local crb_list
    crb_list=$(kubectl get clusterrolebindings -l 'rbacmanager.io/managed-by=rbac-manager-sh' -o json 2>/dev/null | jq -r '.items[] | "\(.metadata.labels["rbacmanager.io/access-grant"]):\(.metadata.labels["rbacmanager.io/access-grant-namespace"]) \(.metadata.name)"' 2>/dev/null || echo "")

    while IFS= read -r crb_info; do
        if [[ -n "$crb_info" ]]; then
            local grant_ref=$(echo "$crb_info" | awk '{print $1}')
            local crb_name=$(echo "$crb_info" | awk '{print $2}')

            if [[ -n "$grant_ref" && "$grant_ref" != "null:null" ]]; then
                if ! echo "$existing_grants" | grep -q "^$grant_ref$"; then
                    log_info "🗑️ Deleting orphaned ClusterRoleBinding: $crb_name"
                    kubectl delete clusterrolebinding "$crb_name" 2>/dev/null || log_warn "Failed to delete ClusterRoleBinding $crb_name"
                fi
            fi
        fi
    done <<< "$crb_list"

    log_debug "✅ Cleanup of orphaned resources completed"
}

# Main reconciliation function
reconcile() {
    log_debug "🔄 Starting reconciliation..."

    # Get all AccessGrants
    local access_grants_json
    if access_grants_json=$(kubectl get accessgrants -A -o json 2>/dev/null); then
        local grants_count
        grants_count=$(echo "$access_grants_json" | jq '.items | length')

        if [[ "$grants_count" -gt 0 ]]; then
            log_debug "Found $grants_count AccessGrant(s)"
            # Process each AccessGrant
            echo "$access_grants_json" | jq -c '.items[]' | while IFS= read -r access_grant; do
                process_access_grant "$access_grant" || log_error "Failed to process AccessGrant"
            done
        fi
    else
        log_warn "Could not get AccessGrants - CRD might not be installed"
    fi

    # Cleanup orphaned resources
    cleanup_orphaned_resources

    log_debug "✅ Reconciliation completed"
}

# Health check function
health_check() {
    while true; do
        echo "OK" > /tmp/health
        sleep 10
    done
}

# Signal handlers
cleanup() {
    log_info "Shutting down RBAC Manager..."
    jobs -p | xargs -r kill 2>/dev/null || true
    exit 0
}

# Main function
main() {
    log_info "🚀 Starting RBAC Manager Simple Bash Operator"

    # Set up signal handlers
    trap cleanup SIGINT SIGTERM

    # Start health check in background
    health_check &
    local health_pid=$!
    log_info "Health check started with PID: $health_pid"

    # Perform initial reconciliation
    log_info "Performing initial reconciliation..."
    reconcile || log_error "Initial reconciliation failed, will retry in ${CHECK_INTERVAL}s"

    # Main loop - periodic reconciliation
    log_info "Starting main reconciliation loop (every ${CHECK_INTERVAL}s)..."
    while true; do
        sleep "$CHECK_INTERVAL"
        log_debug "⏰ Periodic reconciliation triggered"
        reconcile || log_error "Reconciliation failed, will retry in ${CHECK_INTERVAL}s"
    done
}

# Run main function
main "$@"
