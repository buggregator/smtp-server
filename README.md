# RoadRunner SMTP Plugin

SMTP server plugin for profiling and debugging email traffic in development environments.

## Features

- Accepts SMTP connections on configurable port
- Captures authentication attempts without verification
- Parses emails with attachments
- Forwards complete email data to PHP workers
- Designed for Buggregator integration

## Configuration

```yaml
smtp:
  addr: "127.0.0.1:1025"
  hostname: "buggregator.local"
  read_timeout: "60s"
  write_timeout: "10s"
  max_message_size: 10485760

  attachment_storage:
    mode: "memory"
    temp_dir: "/tmp/smtp-attachments"
    cleanup_after: "1h"

  pool:
    num_workers: 4
    max_jobs: 0
    allocate_timeout: 60s
    destroy_timeout: 60s
```

## Status

Work in progress - Step 1 complete (configuration & skeleton)
