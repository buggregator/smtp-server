# Этап 4: Обновление RPC и зависимостей

## Описание
Удаление устаревших RPC методов управления воркерами и настройка Endure зависимостей для автоматической инжекции Jobs RPC клиента.

## Файлы для изменения

### 1. `rpc.go` — удаление worker методов

**Текущий код:** строки 21-98

**Удалить методы:**
- `AddWorker` (строки 26-36)
- `RemoveWorker` (строки 39-49)
- `WorkersList` (строки 52-55)

**Оставить методы:**
- `CloseConnection` (строки 58-77)
- `ListConnections` (строки 80-98)

**Результат:**
```go
type rpc struct {
    plugin *Plugin
}

// CloseConnection - остаётся
func (r *rpc) CloseConnection(req *ConnectionCloseRequest, resp *bool) error {
    // ... существующий код
}

// ListConnections - остаётся
func (r *rpc) ListConnections(_ *Empty, resp *[]*ConnectionInfo) error {
    // ... существующий код
}
```

---

### 2. `plugin.go` — Endure интеграция

**Добавить метод Collects():**
```go
// Collects returns list of dependencies
func (p *Plugin) Collects() []*dep.In {
    return []*dep.In{
        dep.Fits(func(j any) {
            rpc, ok := j.(JobsRPCer)
            if ok {
                p.jobsRPC = rpc
            }
        }, (*JobsRPCer)(nil)),
    }
}
```

**Обновить Init() для проверки конфигурации:**
```go
func (p *Plugin) Init(log Logger, cfg Configurer) error {
    // ... существующий код ...

    // Validate Jobs config
    if p.cfg.Jobs.Pipeline == "" {
        return errors.E(op, errors.Str("jobs.pipeline is required"))
    }

    return nil
}
```

**Обновить Serve() для проверки зависимости:**
```go
func (p *Plugin) Serve() chan error {
    errCh := make(chan error, 1)

    // Verify Jobs RPC is available
    if p.jobsRPC == nil {
        errCh <- errors.E(op, errors.Str("jobs plugin not available, check that jobs plugin is enabled"))
        return errCh
    }

    // ... остальной код ...
}
```

---

### 3. Очистка импортов

**plugin.go:**
```go
// Удалить если не используются:
// - "github.com/roadrunner-server/pool/..."
// - "github.com/roadrunner-server/sdk/v4/payload"
// - другие pool-related импорты

// Добавить:
// - "github.com/roadrunner-server/endure/v2/dep"
```

## Protobuf зависимости

Если Jobs использует protobuf, добавить в go.mod:
```
require (
    github.com/roadrunner-server/api/v4 vX.X.X
)
```

## Definition of Done

- [ ] Методы `AddWorker`, `RemoveWorker`, `WorkersList` удалены из `rpc.go`
- [ ] Метод `Collects()` добавлен в `plugin.go`
- [ ] `Init()` проверяет `jobs.pipeline`
- [ ] `Serve()` проверяет наличие `jobsRPC`
- [ ] Импорты очищены от pool зависимостей
- [ ] Endure корректно инжектит Jobs RPC
- [ ] RPC методы `CloseConnection` и `ListConnections` работают
- [ ] Код компилируется без ошибок

## Тестирование RPC

```bash
# Проверить что worker методы недоступны
rr workers smtp  # должно вернуть ошибку или пустой список

# Проверить что connection методы работают
rr connections smtp
```

## Зависимости

- Этап 3 завершён
- Endure container настроен правильно

## Риски

- **Средний**: Endure dependency injection может не работать — требуется проверка
- **Низкий**: Клиенты использующие worker RPC методы получат ошибки
