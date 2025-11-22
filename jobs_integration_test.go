package smtp

import (
	"testing"
	"time"

	jobsProto "github.com/roadrunner-server/api/v4/build/jobs/v1"
	"github.com/roadrunner-server/errors"
	"go.uber.org/zap"
)

// mockJobsRPC implements JobsRPCer for testing
type mockJobsRPC struct {
	pushed []*jobsProto.PushRequest
	err    error
}

func (m *mockJobsRPC) Push(req *jobsProto.PushRequest, _ *jobsProto.Empty) error {
	if m.err != nil {
		return m.err
	}
	m.pushed = append(m.pushed, req)
	return nil
}

func (m *mockJobsRPC) PushBatch(req *jobsProto.PushBatchRequest, _ *jobsProto.Empty) error {
	return nil
}

func TestToJobsRequest(t *testing.T) {
	email := &EmailData{
		Event:      "EMAIL_RECEIVED",
		UUID:       "test-uuid-123",
		RemoteAddr: "127.0.0.1:12345",
		ReceivedAt: time.Now(),
		Envelope: EnvelopeData{
			From: "sender@test.com",
			To:   []string{"recipient@test.com"},
			Helo: "localhost",
		},
		Message: MessageData{
			Headers: map[string][]string{
				"Subject": {"Test Subject"},
			},
			Body: "Test body",
		},
	}

	cfg := &JobsConfig{
		Pipeline: "smtp-emails",
		Priority: 10,
		Delay:    0,
		AutoAck:  false,
	}

	req := ToJobsRequest(email, cfg)

	if req.Job == nil {
		t.Fatal("expected job to be non-nil")
	}

	if req.Job.Job != "smtp.email" {
		t.Errorf("expected job name 'smtp.email', got '%s'", req.Job.Job)
	}

	if req.Job.Options.Pipeline != "smtp-emails" {
		t.Errorf("expected pipeline 'smtp-emails', got '%s'", req.Job.Options.Pipeline)
	}

	if req.Job.Options.Priority != 10 {
		t.Errorf("expected priority 10, got %d", req.Job.Options.Priority)
	}

	if len(req.Job.Payload) == 0 {
		t.Error("expected non-empty payload")
	}

	// Check headers
	if uuidHeader, ok := req.Job.Headers["uuid"]; !ok || len(uuidHeader.Value) == 0 || uuidHeader.Value[0] != "test-uuid-123" {
		t.Error("expected uuid header with value 'test-uuid-123'")
	}
}

func TestPushToJobs(t *testing.T) {
	mock := &mockJobsRPC{}
	logger, _ := zap.NewDevelopment()
	plugin := &Plugin{
		jobsRPC: mock,
		log:     logger,
		cfg: &Config{
			Jobs: JobsConfig{
				Pipeline: "test-pipeline",
				Priority: 5,
			},
		},
	}

	email := &EmailData{
		UUID:       "test-uuid",
		ReceivedAt: time.Now(),
		Envelope: EnvelopeData{
			From: "test@test.com",
			To:   []string{"recipient@test.com"},
		},
	}

	err := plugin.pushToJobs(email)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(mock.pushed) != 1 {
		t.Errorf("expected 1 push, got %d", len(mock.pushed))
	}
}

func TestPushToJobsError(t *testing.T) {
	mock := &mockJobsRPC{err: errors.Str("rpc error")}
	logger, _ := zap.NewDevelopment()
	plugin := &Plugin{
		jobsRPC: mock,
		log:     logger,
		cfg: &Config{
			Jobs: JobsConfig{
				Pipeline: "test-pipeline",
			},
		},
	}

	email := &EmailData{
		UUID:       "test-uuid",
		ReceivedAt: time.Now(),
	}

	err := plugin.pushToJobs(email)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestPushToJobsNoRPC(t *testing.T) {
	plugin := &Plugin{
		jobsRPC: nil,
		cfg: &Config{
			Jobs: JobsConfig{
				Pipeline: "test-pipeline",
			},
		},
	}

	email := &EmailData{
		UUID: "test-uuid",
	}

	err := plugin.pushToJobs(email)
	if err == nil {
		t.Error("expected error when jobsRPC is nil")
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				Addr: "127.0.0.1:1025",
				Jobs: JobsConfig{Pipeline: "smtp-emails"},
				AttachmentStorage: AttachmentConfig{
					Mode: "memory",
				},
			},
			wantErr: false,
		},
		{
			name: "missing pipeline",
			cfg: Config{
				Addr: "127.0.0.1:1025",
				Jobs: JobsConfig{Pipeline: ""},
				AttachmentStorage: AttachmentConfig{
					Mode: "memory",
				},
			},
			wantErr: true,
		},
		{
			name: "missing addr",
			cfg: Config{
				Addr: "",
				Jobs: JobsConfig{Pipeline: "smtp-emails"},
				AttachmentStorage: AttachmentConfig{
					Mode: "memory",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid attachment mode",
			cfg: Config{
				Addr: "127.0.0.1:1025",
				Jobs: JobsConfig{Pipeline: "smtp-emails"},
				AttachmentStorage: AttachmentConfig{
					Mode: "invalid",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestJobsConfigDefaults(t *testing.T) {
	cfg := &Config{
		Addr: "127.0.0.1:1025",
		Jobs: JobsConfig{
			Pipeline: "test",
		},
	}

	err := cfg.InitDefaults()
	if err != nil {
		t.Fatalf("InitDefaults() error = %v", err)
	}

	if cfg.Jobs.Priority != 10 {
		t.Errorf("expected default priority 10, got %d", cfg.Jobs.Priority)
	}
}
