package smtp

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	jobsProto "github.com/roadrunner-server/api/v4/build/jobs/v1"
)

// Pusher interface for Jobs plugin integration
// Jobs plugin must implement this interface to be collected by SMTP plugin
type Pusher interface {
	Push(ctx context.Context, job *jobsProto.Job) error
}

// ToJob converts EmailData to Jobs protobuf format
func ToJob(e *EmailData, cfg *JobsConfig) *jobsProto.Job {
	payload, _ := json.Marshal(e)

	// Generate a unique job ID
	jobID := uuid.NewString()

	return &jobsProto.Job{
		Job:     "smtp.email",
		Id:      jobID,
		Payload: payload,
		Headers: map[string]*jobsProto.HeaderValue{
			"uuid": {Value: []string{e.UUID}},
		},
		Options: &jobsProto.Options{
			Pipeline: cfg.Pipeline,
			Priority: cfg.Priority,
			Delay:    cfg.Delay,
			AutoAck:  cfg.AutoAck,
		},
	}
}
