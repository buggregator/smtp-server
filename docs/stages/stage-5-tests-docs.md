# Этап 5: Тесты и документация

## Описание
Написание тестов для новой функциональности и обновление документации с руководством по миграции.

## Файлы для создания

### 1. `jobs_integration_test.go` — unit тесты

```go
package smtp

import (
    "testing"
    "time"
)

// Mock Jobs RPC
type mockJobsRPC struct {
    pushed []*jobsproto.PushRequest
    err    error
}

func (m *mockJobsRPC) Push(req *jobsproto.PushRequest, _ *jobsproto.Empty) error {
    if m.err != nil {
        return m.err
    }
    m.pushed = append(m.pushed, req)
    return nil
}

func TestEmailJobToJobsRequest(t *testing.T) {
    job := &EmailJob{
        UUID:       "test-uuid",
        ReceivedAt: time.Now(),
        From:       "sender@test.com",
        To:         []string{"recipient@test.com"},
        Message:    ParsedMessage{Subject: "Test"},
    }

    cfg := &JobsConfig{
        Pipeline: "smtp-emails",
        Priority: 10,
    }

    req := job.ToJobsRequest(cfg)

    if req.Job.Id != "test-uuid" {
        t.Errorf("expected uuid test-uuid, got %s", req.Job.Id)
    }
    if req.Job.Options.Pipeline != "smtp-emails" {
        t.Errorf("expected pipeline smtp-emails, got %s", req.Job.Options.Pipeline)
    }
}

func TestPushToJobs(t *testing.T) {
    mock := &mockJobsRPC{}
    plugin := &Plugin{
        jobsRPC: mock,
        cfg:     &Config{Jobs: JobsConfig{Pipeline: "test"}},
    }

    job := &EmailJob{UUID: "test"}
    err := plugin.pushToJobs(job)

    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
    if len(mock.pushed) != 1 {
        t.Errorf("expected 1 push, got %d", len(mock.pushed))
    }
}

func TestPushToJobsError(t *testing.T) {
    mock := &mockJobsRPC{err: errors.New("rpc error")}
    plugin := &Plugin{
        jobsRPC: mock,
        cfg:     &Config{Jobs: JobsConfig{Pipeline: "test"}},
    }

    job := &EmailJob{UUID: "test"}
    err := plugin.pushToJobs(job)

    if err == nil {
        t.Error("expected error, got nil")
    }
}

func TestConfigValidation(t *testing.T) {
    cfg := &Config{
        Addr: "127.0.0.1:1025",
        Jobs: JobsConfig{Pipeline: ""},
    }

    err := cfg.validate()
    if err == nil {
        t.Error("expected validation error for empty pipeline")
    }
}
```

---

### 2. `docs/migration-guide.md` — руководство по миграции

```markdown
# Migration Guide: SMTP Plugin v1 to v2 (Jobs Integration)

## Breaking Changes

### Removed Features
- Worker pool configuration (`pool` section)
- RPC methods: `AddWorker`, `RemoveWorker`, `WorkersList`

### New Requirements
- Jobs plugin must be enabled
- Pipeline must be configured in Jobs

## Configuration Changes

### Before (v1)
```yaml
smtp:
  addr: "127.0.0.1:1025"
  pool:
    num_workers: 4
    max_jobs: 100
```

### After (v2)
```yaml
smtp:
  addr: "127.0.0.1:1025"
  jobs:
    pipeline: "smtp-emails"
    priority: 10

jobs:
  pipelines:
    smtp-emails:
      driver: memory
      config:
        priority: 10
```

## PHP Code Changes

### Before: Direct Worker
```php
$worker = Worker::create();
while ($payload = $worker->waitPayload()) {
    $email = json_decode($payload->body);
    processEmail($email);
    $worker->respond(new Payload('ok'));
}
```

### After: Jobs Consumer
```php
$consumer = new Consumer();
while ($task = $consumer->waitTask()) {
    $email = json_decode($task->getPayload());
    processEmail($email);
    $task->ack();
}
```

## Migration Steps

1. Update RoadRunner configuration
2. Enable Jobs plugin
3. Create pipeline for SMTP emails
4. Update PHP code to use Jobs consumer
5. Remove old pool configuration
6. Restart RoadRunner
```

---

### 3. Обновить `README.md`

Добавить секцию:
```markdown
## Jobs Integration

SMTP plugin sends emails as tasks to Jobs for processing.

### Configuration

```yaml
smtp:
  addr: "127.0.0.1:1025"
  jobs:
    pipeline: "smtp-emails"
```

### PHP Consumer

```php
// See docs/migration-guide.md
```
```

---

### 4. `docs/examples/` — примеры

**`config-example.yaml`:**
```yaml
# Полный пример конфигурации
```

**`consumer-example.php`:**
```php
// Пример PHP consumer
```

## Definition of Done

- [ ] Unit тесты написаны для:
  - [ ] `EmailJob.ToJobsRequest()`
  - [ ] `pushToJobs()` success/error
  - [ ] Config validation
- [ ] Integration тест с mock Jobs
- [ ] `migration-guide.md` создан
- [ ] `README.md` обновлён
- [ ] Примеры конфигурации добавлены
- [ ] Пример PHP consumer добавлен
- [ ] Все тесты проходят: `go test ./...`
- [ ] Документация проверена на корректность

## Тестирование

```bash
# Unit тесты
go test -v ./...

# Coverage
go test -cover ./...

# Integration (требует запущенный RR)
go test -tags=integration ./...
```

## Зависимости

- Этапы 1-4 завершены
- Код стабилен и готов к релизу

## Риски

- **Низкий**: Документация может устареть — поддерживать актуальность
