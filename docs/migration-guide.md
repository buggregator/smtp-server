# Migration Guide: SMTP Plugin v1 to v2 (Jobs Integration)

## Overview

SMTP plugin v2 removes the built-in worker pool and integrates with the Jobs plugin for email processing. This provides better resource utilization, retry capabilities, and unified task management.

## Breaking Changes

### Removed Features
- Worker pool configuration (`pool` section)
- RPC methods: `AddWorker`, `RemoveWorker`, `WorkersList`
- Direct worker communication

### New Requirements
- Jobs plugin must be enabled
- Pipeline must be configured in Jobs
- PHP code must use Jobs consumer instead of direct worker

## Configuration Changes

### Before (v1)

```yaml
smtp:
  addr: "127.0.0.1:1025"
  hostname: "localhost"
  read_timeout: 60s
  write_timeout: 10s
  max_message_size: 10485760

  pool:
    num_workers: 4
    max_jobs: 100

  attachment_storage:
    mode: "memory"
```

### After (v2)

```yaml
smtp:
  addr: "127.0.0.1:1025"
  hostname: "localhost"
  read_timeout: 60s
  write_timeout: 10s
  max_message_size: 10485760

  jobs:
    pipeline: "smtp-emails"
    priority: 10
    delay: 0
    auto_ack: false

  attachment_storage:
    mode: "memory"

# Jobs plugin configuration
jobs:
  consume: ["smtp-emails"]

  pipelines:
    smtp-emails:
      driver: memory
      config:
        priority: 10
        prefetch: 100
```

## PHP Code Changes

### Before: Direct Worker

```php
<?php

use Spiral\RoadRunner\Worker;
use Spiral\RoadRunner\Payload;

$worker = Worker::create();

while ($payload = $worker->waitPayload()) {
    $email = json_decode($payload->body, true);

    // Process email
    processEmail($email);

    // Respond to worker
    $worker->respond(new Payload('CONTINUE'));
}
```

### After: Jobs Consumer

```php
<?php

use Spiral\RoadRunner\Jobs\Consumer;
use Spiral\RoadRunner\Jobs\Task\ReceivedTaskInterface;

$consumer = new Consumer();

while ($task = $consumer->waitTask()) {
    try {
        $email = json_decode($task->getPayload(), true);

        // Process email
        processEmail($email);

        // Acknowledge successful processing
        $task->ack();

    } catch (\Throwable $e) {
        // Negative acknowledge - will retry
        $task->nack($e);
    }
}
```

## Email Payload Structure

The email payload structure remains the same:

```json
{
  "event": "EMAIL_RECEIVED",
  "uuid": "connection-uuid",
  "remote_addr": "127.0.0.1:12345",
  "received_at": "2024-01-15T10:30:00Z",
  "envelope": {
    "from": "sender@example.com",
    "to": ["recipient@example.com"],
    "helo": "mail.example.com"
  },
  "authentication": {
    "attempted": true,
    "mechanism": "PLAIN",
    "username": "user",
    "password": "pass"
  },
  "message": {
    "headers": {
      "Subject": ["Test Email"]
    },
    "body": "Email body text",
    "raw": "Full RFC822 message (if include_raw: true)"
  },
  "attachments": [
    {
      "filename": "document.pdf",
      "content_type": "application/pdf",
      "size": 12345,
      "content": "base64-encoded-content"
    }
  ]
}
```

## Migration Steps

1. **Update RoadRunner configuration**
   - Remove `pool` section from `smtp`
   - Add `jobs` section with pipeline name
   - Add Jobs plugin configuration with pipeline

2. **Enable Jobs plugin**
   - Ensure Jobs plugin is in your RoadRunner build
   - Configure at least one pipeline for SMTP emails

3. **Update PHP code**
   - Replace `Worker::create()` with `new Consumer()`
   - Replace `waitPayload()` with `waitTask()`
   - Replace `respond()` with `ack()` or `nack()`

4. **Test the migration**
   - Start RoadRunner with new configuration
   - Send test emails
   - Verify emails appear in Jobs pipeline
   - Verify PHP consumer processes emails

5. **Remove old code**
   - Remove any worker pool specific error handling
   - Update monitoring/metrics if applicable

## Benefits of Migration

1. **Resource Efficiency**: Shared worker pool with other Jobs tasks
2. **Retry Support**: Automatic retries on failure with `nack()`
3. **Priority Queues**: Prioritize important emails
4. **Delayed Processing**: Schedule email processing
5. **Better Monitoring**: Jobs metrics and status
6. **Graceful Shutdown**: Proper task completion on shutdown

## Troubleshooting

### SMTP plugin fails to start

**Error**: `jobs plugin not available`

**Solution**: Ensure Jobs plugin is enabled and configured:
```yaml
jobs:
  consume: ["smtp-emails"]
  pipelines:
    smtp-emails:
      driver: memory
```

### Emails not being processed

**Check**:
1. Pipeline name matches in SMTP and Jobs config
2. Jobs consumer is running
3. Pipeline is in `consume` list

### RPC methods not available

Worker management RPC methods (`AddWorker`, `RemoveWorker`, `WorkersList`) are removed. Use Jobs plugin RPC for task management instead.

## Questions?

For issues or questions, please open an issue on the repository.
