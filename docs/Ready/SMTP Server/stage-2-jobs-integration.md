# Этап 2: Jobs интеграция

## Описание
Реализация отправки email в Jobs через RPC. На этом этапе добавляется параллельный путь обработки — и старый (WorkerPool), и новый (Jobs) работают одновременно.

## Файлы для изменения

### 1. `plugin.go` — метод pushToJobs

**Добавить после строки 220:**
```go
func (p *Plugin) pushToJobs(email *EmailJob) error {
    const op = errors.Op("smtp_push_to_jobs")

    if p.jobsRPC == nil {
        return errors.E(op, errors.Str("jobs RPC not available"))
    }

    req := email.ToJobsRequest(&p.cfg.Jobs)

    var empty jobsproto.Empty
    err := p.jobsRPC.Push(req, &empty)
    if err != nil {
        return errors.E(op, err)
    }

    p.log.Debug("email pushed to jobs",
        zap.String("uuid", email.UUID),
        zap.String("pipeline", p.cfg.Jobs.Pipeline),
    )

    return nil
}
```

---

### 2. `session.go` — модификация Data()

**Текущий код:** строки 57-119

**Изменить строки 81-114:**

Заменить вызов `sendToWorker()` на:
```go
// После parseEmail (строка 81)

// Create job from email
job := &EmailJob{
    UUID:       s.uuid,
    ReceivedAt: time.Now(),
    RemoteAddr: s.remoteAddr,
    From:       s.from,
    To:         s.to,
    Message:    parsedMessage,
}

// Push to Jobs via RPC
err = s.backend.plugin.pushToJobs(job)
if err != nil {
    s.backend.log.Error("failed to push email to jobs",
        zap.Error(err),
        zap.String("uuid", s.uuid),
    )
    return &smtp.SMTPError{
        Code:         451,
        EnhancedCode: smtp.EnhancedCode{4, 3, 0},
        Message:      "Temporary failure, try again later",
    }
}

return nil
```

---

### 3. `backend.go` — доступ к plugin

**Текущий код:** строки 10-13 (Backend struct)

Уже имеет доступ к plugin — изменения не требуются.

## Логика переключения

Для тестирования можно добавить feature flag:

```go
// В Config
UseJobs bool `mapstructure:"use_jobs"` // временный флаг

// В Session.Data()
if s.backend.plugin.cfg.UseJobs {
    // новый путь через Jobs
} else {
    // старый путь через sendToWorker
}
```

## Definition of Done

- [ ] Метод `pushToJobs()` реализован в `plugin.go`
- [ ] `Session.Data()` модифицирован для вызова Jobs
- [ ] Логирование добавлено для отслеживания push операций
- [ ] Ошибки RPC корректно обрабатываются
- [ ] SMTP сервер возвращает 451 при ошибке Jobs
- [ ] Email успешно появляется в Jobs pipeline
- [ ] Код компилируется без ошибок

## Тестирование

```bash
# 1. Запустить RoadRunner с Jobs и SMTP плагинами

# 2. Отправить тестовый email
swaks --to test@localhost --server localhost:1025

# 3. Проверить логи на "email pushed to jobs"

# 4. Проверить Jobs на наличие задачи
```

## Зависимости

- Этап 1 завершён
- Jobs plugin запущен и настроен
- Pipeline создан в конфигурации Jobs

## Риски

- **Средний**: Jobs может быть недоступен — требуется обработка ошибок
- **Средний**: Сериализация может быть медленной для больших attachments
