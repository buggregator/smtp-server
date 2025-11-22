# Этап 1: Конфигурация и интерфейсы

## Описание
Добавление новых структур конфигурации для Jobs интеграции и определение интерфейсов для RPC взаимодействия. Существующий функционал не изменяется — только расширяется.

## Файлы для изменения

### 1. `config.go` — добавление Jobs конфигурации

**Текущий код:** строки 11-27 (Config struct)

**Изменения:**
```go
// Добавить после строки 27
type JobsConfig struct {
    Pipeline string        `mapstructure:"pipeline"`
    Priority int64         `mapstructure:"priority"`
    Delay    int64         `mapstructure:"delay"`
    AutoAck  bool          `mapstructure:"auto_ack"`
}

// Добавить поле в Config struct (строка ~20)
Jobs JobsConfig `mapstructure:"jobs"`
```

**InitDefaults:** добавить после строки 78
```go
// Jobs defaults
if c.Jobs.Priority == 0 {
    c.Jobs.Priority = 10
}
```

**validate:** добавить валидацию pipeline (строка ~85)

---

### 2. `jobs_rpc.go` — новый файл

**Создать файл с:**
- Интерфейс `JobsRPCer` с методами Push/PushBatch
- Protobuf-совместимые структуры запросов

---

### 3. `plugin.go` — расширение Plugin struct

**Текущий код:** строки 57-71 (Plugin struct)

**Добавить поле:**
```go
jobsRPC JobsRPCer  // строка ~65
```

---

### 4. `types.go` — структура EmailJob

**Добавить после строки 75:**
```go
type EmailJob struct {
    UUID       string        `json:"uuid"`
    ReceivedAt time.Time     `json:"received_at"`
    RemoteAddr string        `json:"remote_addr"`
    From       string        `json:"from"`
    To         []string      `json:"to"`
    Message    ParsedMessage `json:"message"`
}

func (e *EmailJob) ToJobsRequest(cfg *JobsConfig) *jobsproto.PushRequest {
    // Конвертация в protobuf формат
}
```

## Файлы для создания

| Файл | Описание |
|------|----------|
| `jobs_rpc.go` | Интерфейс JobsRPCer и связанные типы |

## Definition of Done

- [ ] `JobsConfig` структура добавлена в `config.go`
- [ ] Поле `Jobs` добавлено в `Config` struct
- [ ] `InitDefaults()` устанавливает значения по умолчанию для Jobs
- [ ] `validate()` проверяет обязательность `jobs.pipeline`
- [ ] Файл `jobs_rpc.go` создан с интерфейсом `JobsRPCer`
- [ ] Поле `jobsRPC` добавлено в `Plugin` struct
- [ ] Структура `EmailJob` добавлена в `types.go`
- [ ] Метод `ToJobsRequest()` реализован
- [ ] Код компилируется без ошибок
- [ ] Существующие тесты проходят

## Зависимости

- Нет внешних зависимостей
- Требуется добавить import для Jobs protobuf (если используется)

## Риски

- **Низкий**: изменения только добавляют новый код, не затрагивают существующий функционал
