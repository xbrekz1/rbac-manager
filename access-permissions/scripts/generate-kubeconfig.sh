#!/bin/bash

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $*"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $*"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*"
}

# Default values
DEFAULT_TOKEN_DURATION="8760h"  # 1 year
DEFAULT_DOWNLOADS_DIR="$HOME/Downloads"
RBAC_NAMESPACE="rbac-manager"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATE_DIR="$SCRIPT_DIR/../templates"

# Help function
show_help() {
    cat << EOF
Генератор KUBECONFIG для RBAC Manager

ИСПОЛЬЗОВАНИЕ:
    $(basename "$0") [OPTIONS] SERVICE_ACCOUNT_NAME

ПАРАМЕТРЫ:
    SERVICE_ACCOUNT_NAME    Имя ServiceAccount для которого генерировать KUBECONFIG

ОПЦИИ:
    -n, --namespace NAMESPACE      Namespace где находится ServiceAccount (по умолчанию: rbac-manager)
    -d, --duration DURATION        Срок действия токена (по умолчанию: 8760h = 1 год)
    -o, --output DIR              Директория для сохранения (по умолчанию: ~/Downloads)
    -c, --cluster-name NAME       Имя кластера в KUBECONFIG (по умолчанию: из текущего контекста)
    -s, --server URL              URL сервера API (по умолчанию: из текущего контекста)
    --default-namespace NAMESPACE  Namespace по умолчанию в контексте (по умолчанию: default)
    --context-name NAME           Имя контекста (по умолчанию: SERVICE_ACCOUNT_NAME-context)
    -h, --help                    Показать эту справку

ПРИМЕРЫ:
    # Простая генерация для ServiceAccount
    $(basename "$0") monitoring-viewer-sa

    # С кастомными параметрами
    $(basename "$0") -d 720h -o /tmp monitoring-viewer-sa

    # Для конкретного namespace
    $(basename "$0") -n production --default-namespace production prod-admin-sa

EOF
}

# Parse arguments
parse_args() {
    local namespace="$RBAC_NAMESPACE"
    local duration="$DEFAULT_TOKEN_DURATION"
    local output_dir="$DEFAULT_DOWNLOADS_DIR"
    local cluster_name=""
    local server_url=""
    local default_namespace="default"
    local context_name=""
    local sa_name=""

    while [[ $# -gt 0 ]]; do
        case $1 in
            -n|--namespace)
                namespace="$2"
                shift 2
                ;;
            -d|--duration)
                duration="$2"
                shift 2
                ;;
            -o|--output)
                output_dir="$2"
                shift 2
                ;;
            -c|--cluster-name)
                cluster_name="$2"
                shift 2
                ;;
            -s|--server)
                server_url="$2"
                shift 2
                ;;
            --default-namespace)
                default_namespace="$2"
                shift 2
                ;;
            --context-name)
                context_name="$2"
                shift 2
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            -*)
                log_error "Неизвестная опция: $1"
                show_help
                exit 1
                ;;
            *)
                if [[ -z "$sa_name" ]]; then
                    sa_name="$1"
                else
                    log_error "Слишком много аргументов: $1"
                    show_help
                    exit 1
                fi
                shift
                ;;
        esac
    done

    if [[ -z "$sa_name" ]]; then
        log_error "Не указано имя ServiceAccount"
        show_help
        exit 1
    fi

    # Set defaults based on sa_name
    if [[ -z "$context_name" ]]; then
        context_name="${sa_name}-context"
    fi

    # Export variables for use in other functions
    export SA_NAMESPACE="$namespace"
    export SA_NAME="$sa_name"
    export TOKEN_DURATION="$duration"
    export OUTPUT_DIR="$output_dir"
    export CLUSTER_NAME="$cluster_name"
    export SERVER_URL="$server_url"
    export DEFAULT_NAMESPACE="$default_namespace"
    export CONTEXT_NAME="$context_name"
}

# Get cluster info from current context
get_cluster_info() {
    log_info "Получение информации о кластере..."

    # Get current context if cluster name not provided
    if [[ -z "$CLUSTER_NAME" ]]; then
        CLUSTER_NAME=$(kubectl config current-context 2>/dev/null || echo "kubernetes")
        log_info "Используется имя кластера: $CLUSTER_NAME"
    fi

    # Get server URL if not provided
    if [[ -z "$SERVER_URL" ]]; then
        SERVER_URL=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}' 2>/dev/null)
        if [[ -z "$SERVER_URL" ]]; then
            log_error "Не удалось получить URL сервера API"
            exit 1
        fi
        log_info "Используется сервер API: $SERVER_URL"
    fi

    # Get CA data
    CA_DATA=$(kubectl config view --raw --minify --flatten -o jsonpath='{.clusters[0].cluster.certificate-authority-data}' 2>/dev/null)
    if [[ -z "$CA_DATA" ]]; then
        log_error "Не удалось получить certificate-authority-data"
        exit 1
    fi

    export SERVER_URL CA_DATA
}

# Check if ServiceAccount exists
check_service_account() {
    log_info "Проверка ServiceAccount $SA_NAME в namespace $SA_NAMESPACE..."

    if ! kubectl get serviceaccount "$SA_NAME" -n "$SA_NAMESPACE" &>/dev/null; then
        log_error "ServiceAccount $SA_NAME не найден в namespace $SA_NAMESPACE"
        log_info "Доступные ServiceAccount:"
        kubectl get serviceaccount -n "$SA_NAMESPACE" --no-headers | awk '{print "  - " $1}'
        exit 1
    fi

    log_success "ServiceAccount $SA_NAME найден"
}

# Generate token for ServiceAccount
generate_token() {
    log_info "Генерация токена для ServiceAccount $SA_NAME (срок действия: $TOKEN_DURATION)..."

    local token
    if ! token=$(kubectl create token "$SA_NAME" -n "$SA_NAMESPACE" --duration="$TOKEN_DURATION" 2>/dev/null); then
        log_error "Не удалось создать токен для ServiceAccount $SA_NAME"
        log_info "Возможные причины:"
        log_info "  - ServiceAccount не существует"
        log_info "  - Недостаточно прав для создания токена"
        log_info "  - Некорректный формат duration"
        exit 1
    fi

    if [[ -z "$token" ]]; then
        log_error "Получен пустой токен"
        exit 1
    fi

    log_success "Токен успешно создан"
    export SERVICE_ACCOUNT_TOKEN="$token"
}

# Generate KUBECONFIG file
generate_kubeconfig() {
    log_info "Генерация KUBECONFIG файла..."

    local template_file="$TEMPLATE_DIR/kubeconfig-template.yaml"
    if [[ ! -f "$template_file" ]]; then
        log_error "Шаблон KUBECONFIG не найден: $template_file"
        exit 1
    fi

    # Create output directory if it doesn't exist
    mkdir -p "$OUTPUT_DIR"

    local output_file="$OUTPUT_DIR/kubeconfig-$SA_NAME.yaml"
    local temp_file=$(mktemp)

    # Replace placeholders in template
    sed -e "s|{{CA_DATA}}|$CA_DATA|g" \
        -e "s|{{CLUSTER_SERVER}}|$SERVER_URL|g" \
        -e "s|{{CLUSTER_NAME}}|$CLUSTER_NAME|g" \
        -e "s|{{DEFAULT_NAMESPACE}}|$DEFAULT_NAMESPACE|g" \
        -e "s|{{USER_NAME}}|$SA_NAME|g" \
        -e "s|{{CONTEXT_NAME}}|$CONTEXT_NAME|g" \
        -e "s|{{SERVICE_ACCOUNT_TOKEN}}|$SERVICE_ACCOUNT_TOKEN|g" \
        "$template_file" > "$temp_file"

    # Move to final location
    mv "$temp_file" "$output_file"

    log_success "KUBECONFIG сохранен: $output_file"
    export KUBECONFIG_FILE="$output_file"
}

# Test generated KUBECONFIG
test_kubeconfig() {
    log_info "Тестирование сгенерированного KUBECONFIG..."

    # Test basic connectivity
    if kubectl --kubeconfig="$KUBECONFIG_FILE" auth whoami &>/dev/null; then
        log_success "✅ Аутентификация работает"
    else
        log_warn "⚠️ Не удалось проверить аутентификацию"
    fi

    # Test namespace access
    if kubectl --kubeconfig="$KUBECONFIG_FILE" get namespaces &>/dev/null; then
        log_success "✅ Доступ к namespaces работает"
    else
        log_warn "⚠️ Нет доступа к просмотру namespaces"
    fi

    # Test pod access in default namespace
    if kubectl --kubeconfig="$KUBECONFIG_FILE" get pods -n "$DEFAULT_NAMESPACE" &>/dev/null; then
        log_success "✅ Доступ к подам в namespace $DEFAULT_NAMESPACE работает"
    else
        log_warn "⚠️ Нет доступа к подам в namespace $DEFAULT_NAMESPACE"
    fi

    # Show permissions summary
    log_info "Краткая информация о правах:"
    local permissions_info=$(kubectl --kubeconfig="$KUBECONFIG_FILE" auth can-i --list 2>/dev/null | head -10 || echo "Не удалось получить список прав")
    echo "$permissions_info"
}

# Generate usage instructions
generate_instructions() {
    local instructions_file="$OUTPUT_DIR/kubeconfig-$SA_NAME-instructions.txt"

    cat > "$instructions_file" << EOF
=======================================================
KUBECONFIG для ServiceAccount: $SA_NAME
=======================================================

Файл конфигурации: kubeconfig-$SA_NAME.yaml
Дата создания: $(date)
Срок действия токена: $TOKEN_DURATION
Кластер: $CLUSTER_NAME ($SERVER_URL)
Namespace по умолчанию: $DEFAULT_NAMESPACE

ИСПОЛЬЗОВАНИЕ:
--------------

1. Экспорт KUBECONFIG:
   export KUBECONFIG=$(pwd)/kubeconfig-$SA_NAME.yaml

2. Или использование с флагом:
   kubectl --kubeconfig=kubeconfig-$SA_NAME.yaml get pods

3. Проверка подключения:
   kubectl auth whoami
   kubectl get namespaces

БЕЗОПАСНОСТЬ:
------------

⚠️  ВАЖНО: Этот файл содержит токен доступа к кластеру!
   - Не передавайте файл по незащищенным каналам
   - Не коммитьте в git репозитории
   - Удалите файл после использования или истечения срока действия

📋 Информация о правах доступа:
   - Просмотр и выполнение команд зависят от роли ServiceAccount
   - За подробностями обратитесь к администратору кластера

ПОДДЕРЖКА:
----------

При возникновении проблем обратитесь к администратору кластера.
ServiceAccount: $SA_NAME
Namespace: $SA_NAMESPACE

=======================================================
EOF

    log_success "Инструкции сохранены: $instructions_file"
}

# Main function
main() {
    log_info "🚀 Запуск генератора KUBECONFIG для RBAC Manager"

    # Parse command line arguments
    parse_args "$@"

    # Show configuration
    log_info "Конфигурация:"
    log_info "  ServiceAccount: $SA_NAME"
    log_info "  Namespace: $SA_NAMESPACE"
    log_info "  Token duration: $TOKEN_DURATION"
    log_info "  Output directory: $OUTPUT_DIR"
    log_info "  Default namespace: $DEFAULT_NAMESPACE"

    # Check dependencies
    log_info "Проверка зависимостей..."
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl не установлен"
        exit 1
    fi

    if ! kubectl cluster-info &>/dev/null; then
        log_error "Нет подключения к кластеру Kubernetes"
        exit 1
    fi

    # Get cluster information
    get_cluster_info

    # Check if ServiceAccount exists
    check_service_account

    # Generate token
    generate_token

    # Generate KUBECONFIG
    generate_kubeconfig

    # Test generated KUBECONFIG
    test_kubeconfig

    # Generate usage instructions
    generate_instructions

    # Summary
    log_success "🎉 KUBECONFIG успешно сгенерирован!"
    log_info "📁 Файлы сохранены в: $OUTPUT_DIR"
    log_info "   - kubeconfig-$SA_NAME.yaml (основной файл)"
    log_info "   - kubeconfig-$SA_NAME-instructions.txt (инструкции)"
    log_info ""
    log_info "Для использования выполните:"
    echo "  export KUBECONFIG=$OUTPUT_DIR/kubeconfig-$SA_NAME.yaml"
    echo "  kubectl auth whoami"
}

# Run main function with all arguments
main "$@"
