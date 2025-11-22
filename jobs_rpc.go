package smtp

import (
	"encoding/json"

	"github.com/google/uuid"
	jobsProto "github.com/roadrunner-server/api/v4/build/jobs/v1"
)

// JobsRPCer interface for Jobs plugin RPC methods
type JobsRPCer interface {
	Push(req *jobsProto.PushRequest, resp *jobsProto.Empty) error
	PushBatch(req *jobsProto.PushBatchRequest, resp *jobsProto.Empty) error
}

// ToJobsRequest converts EmailData to Jobs protobuf format
func ToJobsRequest(e *EmailData, cfg *JobsConfig) *jobsProto.PushRequest {
	payload, _ := json.Marshal(e)

	// Generate a unique job ID
	jobID := uuid.NewString()

	return &jobsProto.PushRequest{
		Job: &jobsProto.Job{
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
		},
	}
}
