# Этап 3: Удаление WorkerPool

## Описание
Полное удаление WorkerPool и связанного кода после успешного тестирования Jobs интеграции. Это критический этап — после него откат будет сложным.

## Файлы для удаления

| Файл | Причина |
|------|---------|
| `handler.go` | Содержит `sendToWorker()` — больше не нужен |
| `workers_manager.go` | Содержит `AddWorker()`, `RemoveWorker()` — управление пулом |

## Файлы для изменения

### 1. `plugin.go` — удаление Pool

**Удалить интерфейсы (строки 24-54):**
```go
// DELETE: Pool interface (строки 24-37)
// DELETE: Server interface (строки 39-54)
```

**Удалить из Plugin struct (строки 57-71):**
```go
// DELETE: server Server (строка ~63)
// DELETE: wPool Pool (строка ~64)
// DELETE: pldPool sync.Pool (строка ~67)
```

**Изменить Init() (строки 73-111):**
- Удалить инициализацию `pldPool` (строки 101-109)
- Удалить зависимость от `Server`

**Изменить Serve() (строки 113-171):**
- Удалить создание пула (строки 121-129)
- Добавить проверку `jobsRPC`:
```go
if p.jobsRPC == nil {
    errCh <- errors.E(op, errors.Str("jobs plugin not available"))
    return errCh
}
```

**Изменить Stop() (строки 173-220):**
- Удалить `p.wPool.Destroy()` (строки 215-217)

**Удалить методы:**
- `Reset()` (строки 224-245)
- `Workers()` (строки 247-268)

---

### 2. `session.go` — очистка

**Удалить из Data() (строки 57-119):**
- Весь код связанный с `sendToWorker` (уже заменён на этапе 2)
- Проверить отсутствие ссылок на pool

---

### 3. `backend.go` — без изменений

Структура Backend не содержит ссылок на pool.

## Код для удаления из plugin.go

```go
// Строки 24-37: DELETE
type Pool interface {
    Workers() []*process.State
    RemoveWorker(ctx context.Context) error
    AddWorker() error
    Exec(ctx context.Context, p *payload.Payload, stopCh chan struct{}) (chan *staticPool.PExec, error)
    Reset(ctx context.Context) error
    Destroy(ctx context.Context)
}

// Строки 39-54: DELETE
type Server interface {
    NewPool(ctx context.Context, cfg *pool.Config, env map[string]string, ...) (*staticPool.Pool, error)
}

// Из Plugin struct: DELETE
server Server
wPool  Pool
pldPool sync.Pool

// Строки 121-129 в Serve(): DELETE
p.wPool, err = p.server.NewPool(...)

// Строки 215-217 в Stop(): DELETE
if p.wPool != nil {
    p.wPool.Destroy(ctx)
}

// Строки 224-268: DELETE полностью
func (p *Plugin) Reset() error { ... }
func (p *Plugin) Workers() []*process.State { ... }
```

## Definition of Done

- [ ] Файл `handler.go` удалён
- [ ] Файл `workers_manager.go` удалён
- [ ] Интерфейсы `Pool` и `Server` удалены из `plugin.go`
- [ ] Поля `server`, `wPool`, `pldPool` удалены из `Plugin`
- [ ] Методы `Reset()` и `Workers()` удалены
- [ ] `Serve()` проверяет наличие `jobsRPC`
- [ ] `Stop()` не вызывает pool методы
- [ ] Импорты очищены (удалить неиспользуемые)
- [ ] Код компилируется без ошибок
- [ ] SMTP плагин запускается и обрабатывает email через Jobs

## Тестирование

```bash
# 1. Убедиться что плагин запускается без pool конфигурации

# 2. Отправить email и проверить обработку
swaks --to test@localhost --server localhost:1025

# 3. Проверить что нет panic при остановке
```

## Риски

- **Высокий**: После удаления откат сложен — требуется тщательное тестирование перед этапом
- **Средний**: Возможны забытые ссылки на pool в других местах

## Точка невозврата

⚠️ **Важно**: Создайте git tag перед этим этапом для возможности отката:
```bash
git tag -a pre-jobs-migration -m "Before WorkerPool removal"
```
