## 1. Problem Definition

### Purpose

Create an SMTP server plugin for RoadRunner that acts as a **profiling/debugging tool** for Buggregator. The plugin captures all incoming SMTP traffic (emails, authentication attempts, headers, attachments) and forwards complete request information to PHP workers for inspection and debugging.

### Key Characteristics

- **Development-focused**: Not a production mail server
- **Passive authentication**: Accepts and logs auth credentials without actual verification
- **Full transparency**: Captures and exposes ALL protocol details to PHP
- **Attachment handling**: Supports email attachments with flexible delivery options

### Use Case

Developers send emails from their applications to this SMTP server. Buggregator receives complete SMTP transaction details (envelope, headers, body, attachments, auth attempts) for debugging and testing email functionality.

---

## 2. Technical Requirements

### 2.1 SMTP Protocol Implementation

**Supported Commands** (minimal RFC 5321 subset):

```
HELO/EHLO - connection greeting
AUTH LOGIN/PLAIN - authentication (captured but not verified)
MAIL FROM - envelope sender
RCPT TO - envelope recipients
DATA - message content
RSET - reset transaction
QUIT - close connection
```

**Authentication Behavior**:

- Accept `AUTH LOGIN` and `AUTH PLAIN` mechanisms
- Always return success (250 OK) regardless of credentials
- Capture username/password and forward to PHP
- No actual verification against any database

**Response Strategy**:

- Always accept emails (250 OK) unless protocol error
- Log protocol violations but remain permissive
- No SPF/DKIM/DMARC checks (profiling mode)

### 2.2 Configuration Structure

```yaml
smtp:
  addr: "127.0.0.1:1025"              # Listen address (single SMTP server)
  hostname: "buggregator.local"        # Server hostname (EHLO response)
  read_timeout: "60s"                  # Connection read timeout
  write_timeout: "10s"                 # Connection write timeout
  max_message_size: 10485760           # 10MB default (0 = unlimited)
      
  attachment_storage:
    mode: "memory"                     # "memory" or "tempfile"
    temp_dir: "/tmp/smtp-attachments" # Used when mode=tempfile
    cleanup_after: "1h"                # Auto-cleanup temp files
    
  pool:
    num_workers: 4
    max_jobs: 0
    allocate_timeout: 60s
    destroy_timeout: 60s
```

**Note**: Only one SMTP server instance per plugin (simplified configuration).

### 2.3 Data Structures

**RPC Method Signature** (similar to TCP plugin pattern):

PHP receives JSON in `payload.Context`:

```json
{
  "event": "EMAIL_RECEIVED",
  "uuid": "550e8400-e29b-41d4-a716-446655440000",
  "remote_addr": "192.168.1.100:54321",
  "received_at": "2025-11-03T10:30:45Z",
  
  "envelope": {
    "from": "sender@example.com",
    "to": ["recipient@example.com", "cc@example.com"],
    "helo": "client.example.com"
  },
  
  "authentication": {
    "attempted": true,
    "mechanism": "LOGIN",
    "username": "testuser",
    "password": "testpass123"
  },
  
  "message": {
    "headers": {
      "From": "sender@example.com",
      "To": "recipient@example.com",
      "Subject": "Test Email",
      "Content-Type": "multipart/mixed; boundary=\"boundary123\""
    },
    "body": "Plain text body or HTML",
    "raw": "Full RFC822 message (optional)"
  },
  
  "attachments": [
    {
      "filename": "document.pdf",
      "content_type": "application/pdf",
      "size": 152340,
      "content": "base64encodedcontent...",  // if mode=memory
      "path": "/tmp/smtp-att-uuid.pdf"      // if mode=tempfile
    }
  ]
}
```

**PHP Response** (in `payload.Context`):

```
CONTINUE - Accept email and keep SMTP connection alive (ready for next email in session)
CLOSE    - Accept email and close SMTP connection (send 221 Goodbye)
```

**Important**: PHP Worker is released immediately after responding. The SMTP connection remains in Go goroutine memory, not blocking workers.

### 2.4 External Dependencies

**SMTP Library Selection**: Option 1: Use `github.com/emersion/go-smtp` (recommended)

- Mature, RFC-compliant SMTP server library
- Handles protocol details, parsing, authentication flows
- Extensible backend interface

Option 2: Custom implementation using `net/textproto`

- More control but higher complexity
- Need to handle all RFC 5321 edge cases

**Email Parsing**:

- `net/mail` (stdlib) - for basic header parsing
- `github.com/emersion/go-message` - for MIME multipart parsing
- Extract attachments, parse boundaries, decode base64/quoted-printable

---

## 3. Architecture Design

### 3.1 Concurrency Model

**Goroutine Strategy**:

```
Main Server Goroutine
  ↓
Accept Loop (blocking on single listener)
  ↓
Connection Handler (1 goroutine per SMTP connection)
  ↓
SMTP Session Loop:
    - Read commands sequentially
    - On DATA command: parse email
    - Send to PHP Worker (blocks until worker responds)
    - Release worker immediately
    - SMTP connection stays in Go
    - Wait for next MAIL FROM or QUIT
```

**Critical Pattern - Worker Release**:

```go
// Pseudo-code showing worker lifecycle
for {
    email := smtpConn.ReadEmail()  // SMTP connection in Go goroutine
    
    // Acquire worker, send data, wait response
    response := phpWorkerPool.Exec(email)  // BLOCKS here
    
    // Worker is RELEASED immediately after Exec returns
    // SMTP connection still alive in this goroutine
    
    if response == "CLOSE" {
        smtpConn.SendBye()
        break
    }
    
    // CONTINUE - ready for next email on same connection
}
```

**Key Characteristics**:

- **SMTP connections live in Go goroutines** (lightweight, can have thousands)
- **PHP Workers are borrowed only during email processing** (expensive, limited pool)
- **No queuing between SMTP and workers** - SMTP connection blocks if all workers busy (backpressure)
- **Each email = one worker round-trip** (acquire → send → receive → release)

**Comparison with TCP Plugin**:

- TCP: Holds worker during entire connection lifecycle
- SMTP: Borrows worker per email, releases immediately
- SMTP is more efficient for bursty email traffic

### 3.2 Channel Usage

```go
type Plugin struct {
    // Error channel for Serve() lifecycle
    errCh chan error
    
    // No internal job queues (direct PHP worker execution)
    // Context cancellation for graceful shutdown
}
```

**No internal queuing** - rely on RoadRunner's worker pool backpressure. If all workers busy, SMTP connection blocks (acceptable for dev tool).

### 3.3 Resource Pooling

**sync.Pool Usage**:

```go
// Reuse buffers for reading SMTP commands
cmdBufferPool sync.Pool  // *bytes.Buffer for command parsing

// Reuse email data structures
emailDataPool sync.Pool  // *EmailData before JSON marshaling

// Reuse payload structures (same as TCP plugin)
pldPool sync.Pool       // *payload.Payload
```

**Connection Pool**:

```go
// Track active connections for graceful shutdown
connections sync.Map    // uuid -> *smtp.Conn
```

### 3.4 Error Handling Strategy

**SMTP Protocol Errors**:

- Invalid commands: Return `500 Syntax error` but continue session
- Oversized messages: Return `552 Message too large` and reject
- Timeout: Close connection silently, log warning

**PHP Worker Errors**:

- Worker crash: Return `451 Temporary failure` to SMTP client
- Worker timeout: Same as above
- Worker returns error: Log but send `250 OK` (profiler always accepts)

**Logging Levels**:

- `DEBUG`: All SMTP commands and responses
- `INFO`: New connections, completed emails
- `WARN`: Protocol violations, timeouts
- `ERROR`: Worker pool failures, critical errors

---

## 4. Performance Considerations

### 4.1 Expected Load

- **Target**: 50-100 concurrent SMTP connections (dev environment)
- **Email size**: Typically < 5MB, max 10MB
- **Throughput**: ~10-50 emails/second

### 4.2 Optimization Strategies

**Memory Management**:

- Use `sync.Pool` for frequently allocated objects
- Stream large attachments instead of loading into memory (if mode=tempfile)
- Limit buffered data per connection

**CPU Optimization**:

- MIME parsing is CPU-intensive → consider goroutine per email
- Base64 decoding for attachments → use stdlib `encoding/base64` (optimized)

**I/O Optimization**:

- Use `bufio.Reader/Writer` for SMTP protocol I/O
- Batch writes when possible (reduce syscalls)

### 4.3 Resource Limits

**Per-Connection Limits**:

- Max message size: Configurable (default 10MB)
- Read timeout: 60s (prevent slow-loris attacks)
- Max recipients: 100 (prevent abuse)

**Global Limits**:

- Max concurrent connections: Limited by file descriptors (ulimit)
- PHP worker pool: Controlled by RoadRunner pool config

---

## 5. Integration Points

### 5.1 PHP Client Interface

**Expected PHP Code** (Buggregator):

```php
// Worker receives SMTP event
$payload = $worker->waitPayload();

$data = json_decode($payload->context, true);
// $data['event'] === 'EMAIL_RECEIVED'
// $data['envelope']['from'] === 'sender@example.com'
// $data['attachments'][0]['content'] === 'base64...'

// Process email quickly (store in DB, display in UI, etc.)
$inspector->captureEmail($data);

// Respond immediately to release worker
// SMTP connection stays alive in Go
$worker->respond(new Payload('', 'CONTINUE'));

// Worker is now free to handle other requests
// SMTP goroutine waits for next email on same connection
```

**Important Notes**:

- PHP should process email as fast as possible
- Worker is released immediately after respond()
- Long-running processing should be queued elsewhere
- SMTP connection blocking in Go is acceptable (cheap goroutine)

### 5.2 Configuration Integration

Plugin reads from `.rr.yaml`:

```yaml
version: "3"

server:
  command: "php worker.php"
  relay: pipes

smtp:
  addr: "127.0.0.1:1025"
  hostname: "buggregator.local"
  max_message_size: 10485760
  
  attachment_storage:
    mode: "memory"
  
  pool:
    num_workers: 4
```

**Single SMTP server** - no nested server configurations needed.

### 5.3 RoadRunner Container Integration

**Dependencies**:

- `server.Server` - for creating PHP worker pool
- `config.Configurer` - for reading configuration
- `log.Logger` - for structured logging

**Interfaces to Implement**:

- `service.Named` - plugin name "smtp"
- `service.Service` - Serve() and Stop() lifecycle
- `pool.Pool` - RPC interface for PHP commands (optional - for stats/management)

---

## 6. Acceptance Criteria

### 6.1 Functional Requirements

✅ **Must Have**:

- [ ] Accept SMTP connections on configured address
- [ ] Parse SMTP commands (HELO, MAIL FROM, RCPT TO, DATA, QUIT)
- [ ] Capture AUTH LOGIN/PLAIN credentials without verification
- [ ] Parse MIME multipart messages and extract attachments
- [ ] Forward complete email data to PHP worker as JSON
- [ ] Support both base64 (memory) and tempfile attachment modes
- [ ] Handle multiple concurrent connections (50+)
- [ ] Graceful shutdown (close active connections)

✅ **Should Have**:

- [ ] Configurable message size limits
- [ ] Connection timeouts (read/write)
- [ ] Multiple server configurations (different ports)
- [ ] RPC interface for connection management (close by UUID)

✅ **Nice to Have**:

- [ ] Metrics (emails received, connections active)
- [ ] SMTP command logging (debug mode)
- [ ] Worker pool statistics via RPC

### 6.2 Performance Targets

- **Latency**: < 100ms per email (excluding PHP processing)
- **Throughput**: 50 emails/second on modest hardware
- **Concurrency**: 100 simultaneous connections without degradation
- **Memory**: < 50MB baseline, < 500MB under load

### 6.3 Testing Scenarios

**Unit Tests**:

- [ ] SMTP command parsing
- [ ] MIME multipart parsing with attachments
- [ ] Base64 encoding/decoding
- [ ] Configuration validation

**Integration Tests**:

- [ ] Send email via standard SMTP client (Go `net/smtp`)
- [ ] Test with real email clients (Thunderbird, Outlook)
- [ ] Verify attachments arrive correctly in PHP
- [ ] Test authentication capture (LOGIN/PLAIN)
- [ ] Test connection limits and timeouts

**Load Tests**:

- [ ] 100 concurrent connections sending 1KB emails
- [ ] 10 connections sending 10MB emails
- [ ] 1000 emails/minute burst test

### 6.4 Deployment Considerations

**Docker Compatibility**:

- Bind to `0.0.0.0` for container networking
- Configurable port mapping

**Security Notes**:

- **WARNING**: This is a development tool - DO NOT expose to public internet
- No authentication verification by design
- Accepts all emails (spam/abuse risk if exposed)

**Documentation Required**:

- Configuration examples for Buggregator
- PHP worker example code
- Attachment handling modes comparison
- Troubleshooting common issues (port conflicts, worker timeouts)

---

## 7. Open Questions & Decisions

### 7.1 Library Choice

**Decision Needed**: Use `github.com/emersion/go-smtp` or custom implementation?

**Recommendation**: Use `emersion/go-smtp`

- ✅ Handles RFC 5321 edge cases
- ✅ Built-in authentication framework
- ✅ Well-maintained, used in production
- ❌ Additional dependency
- ❌ Less control over low-level protocol

### 7.2 Attachment Storage Default

**Decision Needed**: Default `attachment_storage.mode` - "memory" or "tempfile"?

**Recommendation**: "memory" for simplicity

- Most development emails are < 5MB
- Simpler PHP integration (no file cleanup logic)
- Switch to "tempfile" for large attachments (user can configure)

### 7.3 Raw Message Storage

**Decision Needed**: Include full raw RFC822 message in JSON?

**Recommendation**: Optional via config flag `include_raw: false` (default)

- Raw message can be very large (duplicates parsed data)
- Useful for debugging parsing issues
- Let user enable if needed

### 7.4 SMTP Response Customization

**Decision Needed**: Should PHP control SMTP responses (250 OK vs 550 Reject)?

**Recommendation**: No - always accept (profiler behavior)

- Simplifies PHP interface
- Matches "capture everything" philosophy
- User can add filtering in PHP if needed

---

## Next Steps

1. **Review & Approve** this FeatureRequest
2. **Confirm library choice** (emersion/go-smtp vs custom)
3. **Proceed to Phase 2**: Detailed code structure with interfaces and implementation blueprint

**Estimated Complexity**: Medium

- SMTP protocol: Well-defined, library available
- MIME parsing: Stdlib + library support
- Integration: Follows TCP plugin pattern closely
- Timeline: ~3-5 days for core implementation + testing