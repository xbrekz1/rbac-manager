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

# Configuration
RBAC_NAMESPACE="rbac-manager"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RBAC_MANAGER_DIR="$SCRIPT_DIR/.."

log_info "🚀 Установка системы управления доступами RBAC Manager"

# Check dependencies
log_info "Проверка зависимостей..."

# Check kubectl
if ! command -v kubectl &> /dev/null; then
    log_error "kubectl не установлен"
    exit 1
fi
log_success "✅ kubectl доступен"

# Check helm
if ! command -v helm &> /dev/null; then
    log_error "helm не установлен"
    exit 1
fi
log_success "✅ helm доступен"

# Check task
if ! command -v task &> /dev/null; then
    log_warn "⚠️ task (Taskfile) не установлен"
    log_info "Установите task из https://taskfile.dev/"
    log_info "Или используйте скрипты напрямую из scripts/"
else
    log_success "✅ task доступен"
fi

# Check cluster connection
log_info "Проверка подключения к кластеру..."
if ! kubectl cluster-info &>/dev/null; then
    log_error "Нет подключения к кластеру Kubernetes"
    log_info "Убедитесь что kubectl настроен правильно"
    exit 1
fi
log_success "✅ Подключение к кластеру работает"

# Check RBAC Manager directory
if [[ ! -d "$RBAC_MANAGER_DIR" ]]; then
    log_error "Директория RBAC Manager не найдена: $RBAC_MANAGER_DIR"
    exit 1
fi
log_success "✅ RBAC Manager найден"

# Create namespace
log_info "Создание namespace $RBAC_NAMESPACE..."
if kubectl create namespace "$RBAC_NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - &>/dev/null; then
    log_success "✅ Namespace $RBAC_NAMESPACE готов"
else
    log_error "Не удалось создать namespace"
    exit 1
fi

# Install RBAC Manager
log_info "Установка RBAC Manager..."
cd "$RBAC_MANAGER_DIR"

if helm upgrade --install rbac-manager . --namespace "$RBAC_NAMESPACE" --wait --timeout=300s; then
    log_success "✅ RBAC Manager установлен"
else
    log_error "Не удалось установить RBAC Manager"
    exit 1
fi

# Wait for operator to be ready
log_info "Ожидание готовности оператора..."
if kubectl wait --for=condition=available deployment/rbac-manager -n "$RBAC_NAMESPACE" --timeout=120s; then
    log_success "✅ RBAC Manager оператор готов"
else
    log_warn "⚠️ Таймаут ожидания готовности оператора"
fi

# Check operator logs
log_info "Проверка логов оператора..."
if kubectl logs -n "$RBAC_NAMESPACE" -l app.kubernetes.io/name=rbac-manager --tail=5 | grep -q "Starting RBAC Manager"; then
    log_success "✅ Оператор запущен корректно"
else
    log_warn "⚠️ Возможные проблемы с запуском оператора"
    log_info "Проверьте логи: kubectl logs -n $RBAC_NAMESPACE -l app.kubernetes.io/name=rbac-manager"
fi

# Return to original directory
cd "$SCRIPT_DIR"

# Make scripts executable
log_info "Настройка прав доступа к скриптам..."
chmod +x scripts/generate-kubeconfig.sh
log_success "✅ Скрипты настроены"

# Test with example user (dry-run)
log_info "Тестирование создания пользователя (dry-run)..."
if kubectl apply --dry-run=client -f users/john-doe-viewer.yaml &>/dev/null; then
    log_success "✅ Примеры пользователей корректны"
else
    log_warn "⚠️ Возможные проблемы с примерами пользователей"
fi

# Installation summary
log_success "🎉 Установка завершена!"
echo ""
log_info "📋 Что установлено:"
log_info "  ✅ RBAC Manager оператор в namespace: $RBAC_NAMESPACE"
log_info "  ✅ Custom Resource Definition (AccessGrant)"
log_info "  ✅ Система автоматической генерации KUBECONFIG"
log_info "  ✅ Примеры пользователей в директории users/"

echo ""
log_info "🚀 Быстрый старт:"
echo ""

if command -v task &> /dev/null; then
    echo "  # Посмотреть доступные команды"
    echo "  task --list"
    echo ""
    echo "  # Создать нового пользователя"
    echo "  task generate-user-template USER=vasya-petrov EMAIL=vasya@company.com ROLE=developer"
    echo "  # Отредактировать файл users/vasya-petrov.yaml"
    echo "  task create-user USER=vasya-petrov"
    echo ""
    echo "  # Проверить статус системы"
    echo "  task status"
    echo ""
    echo "  # Показать справку"
    echo "  task help"
else
    echo "  # Создать пользователя вручную:"
    echo "  kubectl apply -f users/john-doe-viewer.yaml"
    echo "  ./scripts/generate-kubeconfig.sh john-doe-viewer-sa"
    echo ""
    echo "  # Установите task для удобной автоматизации:"
    echo "  https://taskfile.dev/"
fi

echo ""
log_info "📁 KUBECONFIG файлы будут сохраняться в: ~/Downloads/"
log_info "📖 Подробная документация: README.md"

echo ""
log_info "🔍 Проверка статуса:"
kubectl get pods -n "$RBAC_NAMESPACE"

echo ""
log_success "✨ Система готова к работе!"
