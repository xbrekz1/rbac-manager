# Выдача доступов в Kubernetes

Быстрая выдача прав доступа к кластеру для разработчиков.

## Роли

| Роль                   | Что может                                      | Видит все NS |
| ---------------------- | ---------------------------------------------- | ------------ |
| **viewer**             | Смотреть поды, логи, все namespace             | ✅           |
| **developer**          | Как viewer + заходить в поды (`kubectl exec`)  | ❌           |
| **developer-openlens** | Как developer + видит все namespace (OpenLens) | ✅           |
| **maintainer**         | Полный доступ к указанным namespace            | ❌           |

## Выбор роли

### Developer vs Developer-OpenLens

- **developer** - базовая роль разработчика, видит только назначенные namespace
- **developer-openlens** - для работы с OpenLens/Lens, видит все namespace в кластере

⚠️ **Безопасность**: `developer` безопаснее, `developer-openlens` только при необходимости OpenLens

## Как выдать доступ

### 1. Создайте файл `users/имя-пользователя.yaml`

**Базовый developer (рекомендуется):**

```yaml
apiVersion: rbacmanager.io/v1alpha1
kind: AccessGrant
metadata:
  name: vasya-petrov
  namespace: rbac-manager
spec:
  namespaces:
    - default
    - development
  role: developer
  serviceAccountName: vasya-petrov-sa
```

**Developer для OpenLens:**

```yaml
apiVersion: rbacmanager.io/v1alpha1
kind: AccessGrant
metadata:
  name: vasya-petrov-lens
  namespace: rbac-manager
spec:
  namespaces:
    - default
    - development
  role: developer-openlens
  serviceAccountName: vasya-petrov-lens-sa
```

### 2. Примените и создайте KUBECONFIG

```bash
kubectl apply -f users/vasya-petrov.yaml
task generate-kubeconfig ACCESSGRANT=vasya-petrov
```

Готово! KUBECONFIG появится в `~/Downloads/`

### 3. Отдайте файлы пользователю

- `kubeconfig-vasya-petrov-sa.yaml` - основной файл
- `kubeconfig-vasya-petrov-sa-instructions.txt` - инструкция

## Команды

```bash
# Создать KUBECONFIG
task generate-kubeconfig ACCESSGRANT=имя-пользователя

# Проверить права
task test-access-grant ACCESSGRANT=имя-пользователя
```

## Примеры

Готовые примеры в папке `users/`:

- `john-doe-viewer.yaml` - только просмотр
- `jane-smith-developer.yaml` - базовый разработчик
- `jane-smith-developer-openlens.yaml` - разработчик для OpenLens
- `admin-maintainer.yaml` - администратор

Больше примеров: [EXAMPLES.md](EXAMPLES.md)

## Как пользователю работать

```bash
export KUBECONFIG=kubeconfig-vasya-petrov-sa.yaml
kubectl get pods
```

______________________________________________________________________

**Документация оператора**: [../README.md](../README.md)
