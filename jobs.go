package coresdk

// JobService bindings — uses the generated protobuf types in
// gen/coresdk/v1 via protoc-gen-go + protoc-gen-go-grpc, marshalled with
// google.golang.org/protobuf.
//
// Migrated from a manual prost-style wire codec (commit eaf3dbe) to the
// generated client on 2026-05-12. Public types in this file (Job, JobEvent
// with the JobEventKind discriminator, LogLine, etc.) are kept as the
// Go-idiomatic surface — the generated types live in
// github.com/coresdk-dev/sdk-go/gen/coresdk/v1 and are converted at the
// boundary so callers never see raw protobuf message types.

import (
	"context"
	"encoding/json"
	"fmt"

	pb "github.com/coresdk-dev/sdk-go/gen/coresdk/v1"
)

// JobState mirrors coresdk.v1.JobState.
type JobState string

const (
	JobStatePending    JobState = "pending"
	JobStateScheduling JobState = "scheduling"
	JobStateRunning    JobState = "running"
	JobStateSucceeded  JobState = "succeeded"
	JobStateFailed     JobState = "failed"
	JobStateCancelled  JobState = "cancelled"
)

func (s JobState) IsTerminal() bool {
	return s == JobStateSucceeded || s == JobStateFailed || s == JobStateCancelled
}

// SecretRef references a single secret to inject into a job container.
type SecretRef struct {
	Name     string
	Provider string
	Path     string
	Version  string
	// Delivery: "env" | "file" | "" (server default).
	Delivery string
}

// Job is the snapshot of a job returned by Submit/Get/Cancel/List and every
// progress tick of Watch.
type Job struct {
	JobID               string
	Kind                string
	Image               string
	State               JobState
	ExitCode            int32
	Error               string
	InputS3URI          string
	OutputS3URI         string
	LogsS3URI           string
	CreatedAt           int64
	StartedAt           int64
	FinishedAt          int64
	TenantID            string
	UserID              string
	K8sNamespace        string
	K8sJobName          string
	ResolvedSecretNames []string
}

// JobEventKind discriminates JobEvent variants.
type JobEventKind string

const (
	JobEventCreated   JobEventKind = "created"
	JobEventScheduled JobEventKind = "scheduled"
	JobEventStarted   JobEventKind = "started"
	JobEventProgress  JobEventKind = "progress"
	JobEventSucceeded JobEventKind = "succeeded"
	JobEventFailed    JobEventKind = "failed"
	JobEventCancelled JobEventKind = "cancelled"
	JobEventUnknown   JobEventKind = "unknown"
)

// JobEvent is one event emitted by a server-streaming WatchJob call.
type JobEvent struct {
	JobID       string
	Kind        JobEventKind
	TS          int64
	Stage       string
	Percent     uint32
	Detail      map[string]any
	NodeName    string
	Image       string
	ExitCode    int32
	OutputS3URI string
	Error       string
	Reason      string
}

// LogLine is one stdout/stderr line tailed from a running job.
type LogLine struct {
	JobID  string
	TS     int64
	Stream string
	Line   string
}

// OutputFile is one file persisted under a job's output prefix.
type OutputFile struct {
	Key          string
	S3URI        string
	PresignedURL string
	Size         int64
	ContentType  string
}

// JobOutput lists the files persisted for a job.
type JobOutput struct {
	Files []OutputFile
}

// SubmitJobRequest contains every field of coresdk.v1.SubmitJobRequest.
type SubmitJobRequest struct {
	Kind           string
	Image          string
	Command        []string
	Args           []string
	Env            map[string]string
	InlineFiles    map[string][]byte
	InputS3URI     string
	SecretRefs     []SecretRef
	SecretBundles  []string
	TimeoutSeconds uint32
	CaptureLogs    bool
	CaptureOutput  bool
	OutputPrefix   string
	TenantID       string
	UserID         string
}

// ── conversions: SDK ↔ protobuf ─────────────────────────────────────────────

func (c *Client) tenantContext(tenantID string) *pb.TenantContext {
	if tenantID == "" {
		return nil
	}
	return &pb.TenantContext{TenantId: tenantID}
}

func deliveryFromString(s string) pb.Delivery {
	switch s {
	case "env":
		return pb.Delivery_DELIVERY_ENV
	case "file":
		return pb.Delivery_DELIVERY_FILE
	default:
		return pb.Delivery_DELIVERY_UNSPECIFIED
	}
}

func toSubmitJobRequest(req SubmitJobRequest, defaultTenant string) *pb.SubmitJobRequest {
	tenant := req.TenantID
	if tenant == "" {
		tenant = defaultTenant
	}
	pbReq := &pb.SubmitJobRequest{
		Kind:           req.Kind,
		Image:          req.Image,
		Command:        req.Command,
		Args:           req.Args,
		Env:            req.Env,
		TimeoutSeconds: req.TimeoutSeconds,
		CaptureLogs:    req.CaptureLogs,
		CaptureOutput:  req.CaptureOutput,
		OutputPrefix:   req.OutputPrefix,
		UserId:         req.UserID,
	}
	if tenant != "" {
		pbReq.Tenant = &pb.TenantContext{TenantId: tenant}
	}
	if len(req.InlineFiles) > 0 {
		pbReq.Input = &pb.Input{
			Source: &pb.Input_Inline{Inline: &pb.InlineFiles{Files: req.InlineFiles}},
		}
	} else if req.InputS3URI != "" {
		pbReq.Input = &pb.Input{Source: &pb.Input_InputS3Uri{InputS3Uri: req.InputS3URI}}
	}
	for _, r := range req.SecretRefs {
		pbReq.SecretRefs = append(pbReq.SecretRefs, &pb.SecretRef{
			Name:     r.Name,
			Provider: r.Provider,
			Path:     r.Path,
			Version:  r.Version,
			Delivery: deliveryFromString(r.Delivery),
		})
	}
	pbReq.SecretBundles = append([]string{}, req.SecretBundles...)
	return pbReq
}

func fromPbJob(j *pb.Job) *Job {
	if j == nil {
		return &Job{}
	}
	var state JobState
	switch j.State {
	case pb.JobState_JOB_STATE_SCHEDULING:
		state = JobStateScheduling
	case pb.JobState_JOB_STATE_RUNNING:
		state = JobStateRunning
	case pb.JobState_JOB_STATE_SUCCEEDED:
		state = JobStateSucceeded
	case pb.JobState_JOB_STATE_FAILED:
		state = JobStateFailed
	case pb.JobState_JOB_STATE_CANCELLED:
		state = JobStateCancelled
	default:
		state = JobStatePending
	}
	return &Job{
		JobID:               j.JobId,
		Kind:                j.Kind,
		Image:               j.Image,
		State:               state,
		ExitCode:            j.ExitCode,
		Error:               j.Error,
		InputS3URI:          j.InputS3Uri,
		OutputS3URI:         j.OutputS3Uri,
		LogsS3URI:           j.LogsS3Uri,
		CreatedAt:           j.CreatedAt,
		StartedAt:           j.StartedAt,
		FinishedAt:          j.FinishedAt,
		TenantID:            j.TenantId,
		UserID:              j.UserId,
		K8sNamespace:        j.K8SNamespace,
		K8sJobName:          j.K8SJobName,
		ResolvedSecretNames: append([]string{}, j.ResolvedSecretNames...),
	}
}

func fromPbJobEvent(ev *pb.JobEvent, jobID string) JobEvent {
	out := JobEvent{
		JobID: jobID,
		TS:    ev.Ts,
		Kind:  JobEventUnknown,
	}
	switch e := ev.Event.(type) {
	case *pb.JobEvent_Created:
		out.Kind = JobEventCreated
		out.Image = e.Created.Image
	case *pb.JobEvent_Scheduled:
		out.Kind = JobEventScheduled
		out.NodeName = e.Scheduled.NodeName
	case *pb.JobEvent_Started:
		out.Kind = JobEventStarted
	case *pb.JobEvent_Progress:
		out.Kind = JobEventProgress
		out.Stage = e.Progress.Stage
		out.Percent = e.Progress.Percent
		if raw := e.Progress.DetailJson; raw != "" {
			var detail map[string]any
			if err := json.Unmarshal([]byte(raw), &detail); err == nil {
				out.Detail = detail
			} else {
				out.Detail = map[string]any{"raw": raw}
			}
		}
	case *pb.JobEvent_Succeeded:
		out.Kind = JobEventSucceeded
		out.ExitCode = e.Succeeded.ExitCode
		out.OutputS3URI = e.Succeeded.OutputS3Uri
	case *pb.JobEvent_Failed:
		out.Kind = JobEventFailed
		out.ExitCode = e.Failed.ExitCode
		out.Error = e.Failed.Error
	case *pb.JobEvent_Cancelled:
		out.Kind = JobEventCancelled
		out.Reason = e.Cancelled.Reason
	}
	return out
}

func fromPbLogLine(line *pb.LogLine, jobID string) LogLine {
	stream := "unspecified"
	switch line.Stream {
	case pb.LogStream_LOG_STREAM_STDOUT:
		stream = "stdout"
	case pb.LogStream_LOG_STREAM_STDERR:
		stream = "stderr"
	}
	return LogLine{JobID: jobID, TS: line.Ts, Stream: stream, Line: line.Line}
}

func fromPbJobOutput(o *pb.JobOutput) *JobOutput {
	out := &JobOutput{Files: make([]OutputFile, 0, len(o.Files))}
	for _, f := range o.Files {
		out.Files = append(out.Files, OutputFile{
			Key:          f.Key,
			S3URI:        f.S3Uri,
			PresignedURL: f.PresignedUrl,
			Size:         f.Size,
			ContentType:  f.ContentType,
		})
	}
	return out
}

// ── client surface ──────────────────────────────────────────────────────────

func (c *Client) jobsClient() pb.JobServiceClient {
	return pb.NewJobServiceClient(c.conn)
}

// SubmitJob accepts a job and returns the immediate snapshot in pending state.
func (c *Client) SubmitJob(ctx context.Context, req SubmitJobRequest) (*Job, error) {
	pbReq := toSubmitJobRequest(req, c.config.TenantID)
	resp, err := c.jobsClient().SubmitJob(ctx, pbReq)
	if err != nil {
		return nil, fmt.Errorf("coresdk: SubmitJob: %w", err)
	}
	return fromPbJob(resp), nil
}

// GetJob returns the current state of a single job.
func (c *Client) GetJob(ctx context.Context, jobID string) (*Job, error) {
	resp, err := c.jobsClient().GetJob(ctx, &pb.GetJobRequest{
		JobId:  jobID,
		Tenant: c.tenantContext(c.config.TenantID),
	})
	if err != nil {
		return nil, fmt.Errorf("coresdk: GetJob: %w", err)
	}
	return fromPbJob(resp), nil
}

// CancelJob cooperatively cancels a running job. Idempotent on terminal jobs.
func (c *Client) CancelJob(ctx context.Context, jobID, reason string) (*Job, error) {
	resp, err := c.jobsClient().CancelJob(ctx, &pb.CancelJobRequest{
		JobId:  jobID,
		Reason: reason,
		Tenant: c.tenantContext(c.config.TenantID),
	})
	if err != nil {
		return nil, fmt.Errorf("coresdk: CancelJob: %w", err)
	}
	return fromPbJob(resp), nil
}

// ListJobs returns up to `limit` jobs for the caller's tenant. `state` empty = all.
func (c *Client) ListJobs(ctx context.Context, state string, limit uint32) ([]Job, error) {
	resp, err := c.jobsClient().ListJobs(ctx, &pb.ListJobsRequest{
		Tenant:      c.tenantContext(c.config.TenantID),
		StateFilter: state,
		Limit:       limit,
	})
	if err != nil {
		return nil, fmt.Errorf("coresdk: ListJobs: %w", err)
	}
	out := make([]Job, 0, len(resp.Jobs))
	for _, j := range resp.Jobs {
		out = append(out, *fromPbJob(j))
	}
	return out, nil
}

// GetJobOutput returns the output file listing with short-lived presigned URLs.
// `presignTTLSeconds` of 0 uses the server default (900s).
func (c *Client) GetJobOutput(ctx context.Context, jobID string, presignTTLSeconds uint32) (*JobOutput, error) {
	resp, err := c.jobsClient().GetJobOutput(ctx, &pb.OutputRequest{
		JobId:             jobID,
		PresignTtlSeconds: presignTTLSeconds,
		Tenant:            c.tenantContext(c.config.TenantID),
	})
	if err != nil {
		return nil, fmt.Errorf("coresdk: GetJobOutput: %w", err)
	}
	return fromPbJobOutput(resp), nil
}

// WatchJob returns a channel of JobEvents until the job reaches a terminal
// state. The channel closes on terminal events; closing `ctx` cancels the
// underlying gRPC stream.
func (c *Client) WatchJob(ctx context.Context, jobID string) (<-chan JobEvent, error) {
	stream, err := c.jobsClient().WatchJob(ctx, &pb.WatchJobRequest{
		JobId:  jobID,
		Tenant: c.tenantContext(c.config.TenantID),
	})
	if err != nil {
		return nil, fmt.Errorf("coresdk: WatchJob: %w", err)
	}
	ch := make(chan JobEvent, 8)
	go func() {
		defer close(ch)
		for {
			ev, err := stream.Recv()
			if err != nil {
				return
			}
			out := fromPbJobEvent(ev, jobID)
			select {
			case ch <- out:
			case <-ctx.Done():
				return
			}
			if out.Kind == JobEventSucceeded || out.Kind == JobEventFailed || out.Kind == JobEventCancelled {
				return
			}
		}
	}()
	return ch, nil
}

// StreamJobLogs returns a channel of LogLines tailed from the job's container.
func (c *Client) StreamJobLogs(ctx context.Context, jobID string, follow bool, tailLines uint32) (<-chan LogLine, error) {
	stream, err := c.jobsClient().GetJobLogs(ctx, &pb.LogsRequest{
		JobId:     jobID,
		Follow:    follow,
		TailLines: tailLines,
		Tenant:    c.tenantContext(c.config.TenantID),
	})
	if err != nil {
		return nil, fmt.Errorf("coresdk: StreamJobLogs: %w", err)
	}
	ch := make(chan LogLine, 32)
	go func() {
		defer close(ch)
		for {
			line, err := stream.Recv()
			if err != nil {
				return
			}
			out := fromPbLogLine(line, jobID)
			select {
			case ch <- out:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}
