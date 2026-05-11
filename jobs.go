package coresdk

// JobService bindings — mirrors sdk-python/coresdk/_jobs.py. Manual prost-style
// wire encoding so we don't need protoc in the build.
//
// See proto/coresdk/v1/jobs.proto for the canonical message layout.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"google.golang.org/grpc"
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
	JobID                string
	Kind                 string
	Image                string
	State                JobState
	ExitCode             int32
	Error                string
	InputS3URI           string
	OutputS3URI          string
	LogsS3URI            string
	CreatedAt            int64
	StartedAt            int64
	FinishedAt           int64
	TenantID             string
	UserID               string
	K8sNamespace         string
	K8sJobName           string
	ResolvedSecretNames  []string
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
	Kind            string
	Image           string
	Command         []string
	Args            []string
	Env             map[string]string
	InlineFiles     map[string][]byte
	InputS3URI      string
	SecretRefs      []SecretRef
	SecretBundles   []string
	TimeoutSeconds  uint32
	CaptureLogs     bool
	CaptureOutput   bool
	OutputPrefix    string
	TenantID        string
	UserID          string
}

// ── wire helpers ────────────────────────────────────────────────────────────

func encodeBytes(fieldNum int, value []byte) []byte {
	tag := varint(uint64(fieldNum<<3 | 2)) //nolint:gosec
	length := varint(uint64(len(value)))
	out := make([]byte, 0, len(tag)+len(length)+len(value))
	out = append(out, tag...)
	out = append(out, length...)
	out = append(out, value...)
	return out
}

func encodeMessageBytes(fieldNum int, payload []byte) []byte {
	return encodeBytes(fieldNum, payload)
}

func encodeStringMapEntry(key, value string) []byte {
	return []byte(encodeString(1, key) + encodeString(2, value))
}

func encodeStringMap(fieldNum int, m map[string]string) []byte {
	var out []byte
	for k, v := range m {
		out = append(out, encodeMessageBytes(fieldNum, encodeStringMapEntry(k, v))...)
	}
	return out
}

func encodeBytesMap(fieldNum int, m map[string][]byte) []byte {
	var out []byte
	for k, v := range m {
		entry := append([]byte(encodeString(1, k)), encodeBytes(2, v)...)
		out = append(out, encodeMessageBytes(fieldNum, entry)...)
	}
	return out
}

func encodeTenant(fieldNum int, tenantID string) []byte {
	if tenantID == "" {
		return nil
	}
	return encodeMessageBytes(fieldNum, []byte(encodeString(1, tenantID)))
}

func encodeSecretRef(r SecretRef) []byte {
	payload := []byte(encodeString(1, r.Name) +
		encodeString(2, r.Provider) +
		encodeString(3, r.Path) +
		encodeString(4, r.Version))
	switch r.Delivery {
	case "env":
		payload = append(payload, encodeVarintField(5, 1)...)
	case "file":
		payload = append(payload, encodeVarintField(5, 2)...)
	}
	return encodeMessageBytes(7, payload)
}

func encodeInput(inline map[string][]byte, inputS3URI string) []byte {
	if len(inline) > 0 {
		inner := encodeMessageBytes(1, encodeBytesMap(1, inline))
		return encodeMessageBytes(6, inner)
	}
	if inputS3URI != "" {
		return encodeMessageBytes(6, []byte(encodeString(2, inputS3URI)))
	}
	return nil
}

func encodeSubmitJobRequest(req SubmitJobRequest) []byte {
	var p []byte
	p = append(p, []byte(encodeString(1, req.Kind))...)
	p = append(p, []byte(encodeString(2, req.Image))...)
	for _, c := range req.Command {
		p = append(p, []byte(encodeString(3, c))...)
	}
	for _, a := range req.Args {
		p = append(p, []byte(encodeString(4, a))...)
	}
	p = append(p, encodeStringMap(5, req.Env)...)
	p = append(p, encodeInput(req.InlineFiles, req.InputS3URI)...)
	for _, r := range req.SecretRefs {
		p = append(p, encodeSecretRef(r)...)
	}
	for _, b := range req.SecretBundles {
		p = append(p, []byte(encodeString(8, b))...)
	}
	if req.TimeoutSeconds != 0 {
		p = append(p, encodeVarintField(10, uint64(req.TimeoutSeconds))...)
	}
	if req.CaptureLogs {
		p = append(p, encodeVarintField(11, 1)...)
	}
	if req.CaptureOutput {
		p = append(p, encodeVarintField(12, 1)...)
	}
	if req.OutputPrefix != "" {
		p = append(p, []byte(encodeString(13, req.OutputPrefix))...)
	}
	p = append(p, encodeTenant(14, req.TenantID)...)
	p = append(p, []byte(encodeString(15, req.UserID))...)
	return p
}

func decodeJob(body []byte) Job {
	f := decodeFields(body)
	stateInt := decodeInt64(f, 4)
	var state JobState
	switch stateInt {
	case 2:
		state = JobStateScheduling
	case 3:
		state = JobStateRunning
	case 4:
		state = JobStateSucceeded
	case 5:
		state = JobStateFailed
	case 6:
		state = JobStateCancelled
	default:
		state = JobStatePending
	}
	return Job{
		JobID:               decodeString(f, 1),
		Kind:                decodeString(f, 2),
		Image:               decodeString(f, 3),
		State:               state,
		ExitCode:            int32(decodeInt64(f, 5)), //nolint:gosec
		Error:               decodeString(f, 6),
		InputS3URI:          decodeString(f, 7),
		OutputS3URI:         decodeString(f, 8),
		LogsS3URI:           decodeString(f, 9),
		CreatedAt:           decodeInt64(f, 10),
		StartedAt:           decodeInt64(f, 11),
		FinishedAt:          decodeInt64(f, 12),
		TenantID:            decodeString(f, 13),
		UserID:              decodeString(f, 14),
		K8sNamespace:        decodeString(f, 15),
		K8sJobName:          decodeString(f, 16),
		ResolvedSecretNames: decodeRepeatedString(f, 17),
	}
}

func decodeJobEvent(body []byte) JobEvent {
	f := decodeFields(body)
	ev := JobEvent{
		JobID: decodeString(f, 1),
		TS:    decodeInt64(f, 2),
		Kind:  JobEventUnknown,
	}
	if subs, ok := f[10]; ok && len(subs) > 0 {
		sf := decodeFields(subs[0])
		ev.Kind = JobEventCreated
		ev.Image = decodeString(sf, 1)
	} else if subs, ok := f[11]; ok && len(subs) > 0 {
		sf := decodeFields(subs[0])
		ev.Kind = JobEventScheduled
		ev.NodeName = decodeString(sf, 1)
	} else if _, ok := f[12]; ok {
		ev.Kind = JobEventStarted
	} else if subs, ok := f[13]; ok && len(subs) > 0 {
		sf := decodeFields(subs[0])
		ev.Kind = JobEventProgress
		ev.Stage = decodeString(sf, 1)
		ev.Percent = uint32(decodeInt64(sf, 2)) //nolint:gosec
		raw := decodeString(sf, 3)
		if raw != "" {
			var detail map[string]any
			if err := json.Unmarshal([]byte(raw), &detail); err == nil {
				ev.Detail = detail
			} else {
				ev.Detail = map[string]any{"raw": raw}
			}
		}
	} else if subs, ok := f[14]; ok && len(subs) > 0 {
		sf := decodeFields(subs[0])
		ev.Kind = JobEventSucceeded
		ev.ExitCode = int32(decodeInt64(sf, 1)) //nolint:gosec
		ev.OutputS3URI = decodeString(sf, 2)
	} else if subs, ok := f[15]; ok && len(subs) > 0 {
		sf := decodeFields(subs[0])
		ev.Kind = JobEventFailed
		ev.ExitCode = int32(decodeInt64(sf, 1)) //nolint:gosec
		ev.Error = decodeString(sf, 2)
	} else if subs, ok := f[16]; ok && len(subs) > 0 {
		sf := decodeFields(subs[0])
		ev.Kind = JobEventCancelled
		ev.Reason = decodeString(sf, 1)
	}
	return ev
}

func decodeLogLine(body []byte) LogLine {
	f := decodeFields(body)
	stream := "unspecified"
	switch decodeInt64(f, 2) {
	case 1:
		stream = "stdout"
	case 2:
		stream = "stderr"
	}
	return LogLine{TS: decodeInt64(f, 1), Stream: stream, Line: decodeString(f, 3)}
}

func decodeJobOutput(body []byte) JobOutput {
	f := decodeFields(body)
	var files []OutputFile
	for _, raw := range f[1] {
		sf := decodeFields(raw)
		files = append(files, OutputFile{
			Key:          decodeString(sf, 1),
			S3URI:        decodeString(sf, 2),
			PresignedURL: decodeString(sf, 3),
			Size:         decodeInt64(sf, 4),
			ContentType:  decodeString(sf, 5),
		})
	}
	return JobOutput{Files: files}
}

// ── client surface ──────────────────────────────────────────────────────────

// SubmitJob accepts a job, returns the immediate snapshot in pending state.
// Subscribe to progress via WatchJob.
func (c *Client) SubmitJob(ctx context.Context, req SubmitJobRequest) (*Job, error) {
	if req.TenantID == "" {
		req.TenantID = c.config.TenantID
	}
	payload := encodeSubmitJobRequest(req)
	frame := grpcFrame(payload)
	var resp []byte
	if err := c.conn.Invoke(ctx, "/coresdk.v1.JobService/SubmitJob", frame, &resp); err != nil {
		return nil, fmt.Errorf("coresdk: SubmitJob: %w", err)
	}
	j := decodeJob(grpcUnframe(resp))
	return &j, nil
}

// GetJob returns the current state of a single job.
func (c *Client) GetJob(ctx context.Context, jobID string) (*Job, error) {
	payload := []byte(encodeString(1, jobID))
	payload = append(payload, encodeTenant(2, c.config.TenantID)...)
	var resp []byte
	if err := c.conn.Invoke(ctx, "/coresdk.v1.JobService/GetJob", grpcFrame(payload), &resp); err != nil {
		return nil, fmt.Errorf("coresdk: GetJob: %w", err)
	}
	j := decodeJob(grpcUnframe(resp))
	return &j, nil
}

// CancelJob cooperatively cancels a running job. Idempotent on terminal jobs.
func (c *Client) CancelJob(ctx context.Context, jobID, reason string) (*Job, error) {
	payload := []byte(encodeString(1, jobID) + encodeString(2, reason))
	payload = append(payload, encodeTenant(3, c.config.TenantID)...)
	var resp []byte
	if err := c.conn.Invoke(ctx, "/coresdk.v1.JobService/CancelJob", grpcFrame(payload), &resp); err != nil {
		return nil, fmt.Errorf("coresdk: CancelJob: %w", err)
	}
	j := decodeJob(grpcUnframe(resp))
	return &j, nil
}

// ListJobs returns up to `limit` jobs for the caller's tenant. `state` empty = all.
func (c *Client) ListJobs(ctx context.Context, state string, limit uint32) ([]Job, error) {
	payload := encodeTenant(1, c.config.TenantID)
	payload = append(payload, []byte(encodeString(2, state))...)
	if limit != 0 {
		payload = append(payload, encodeVarintField(3, uint64(limit))...)
	}
	var resp []byte
	if err := c.conn.Invoke(ctx, "/coresdk.v1.JobService/ListJobs", grpcFrame(payload), &resp); err != nil {
		return nil, fmt.Errorf("coresdk: ListJobs: %w", err)
	}
	body := grpcUnframe(resp)
	f := decodeFields(body)
	out := make([]Job, 0, len(f[1]))
	for _, raw := range f[1] {
		out = append(out, decodeJob(raw))
	}
	return out, nil
}

// GetJobOutput returns the output file listing with short-lived presigned URLs.
// `presignTTLSeconds` of 0 uses the server default (900s).
func (c *Client) GetJobOutput(ctx context.Context, jobID string, presignTTLSeconds uint32) (*JobOutput, error) {
	payload := []byte(encodeString(1, jobID))
	if presignTTLSeconds != 0 {
		payload = append(payload, encodeVarintField(2, uint64(presignTTLSeconds))...)
	}
	payload = append(payload, encodeTenant(3, c.config.TenantID)...)
	var resp []byte
	if err := c.conn.Invoke(ctx, "/coresdk.v1.JobService/GetJobOutput", grpcFrame(payload), &resp); err != nil {
		return nil, fmt.Errorf("coresdk: GetJobOutput: %w", err)
	}
	out := decodeJobOutput(grpcUnframe(resp))
	return &out, nil
}

// WatchJob returns a channel of JobEvents until the job reaches a terminal
// state. The channel closes on terminal events; closing `ctx` cancels the
// underlying gRPC stream.
func (c *Client) WatchJob(ctx context.Context, jobID string) (<-chan JobEvent, error) {
	payload := []byte(encodeString(1, jobID))
	payload = append(payload, encodeTenant(3, c.config.TenantID)...)
	desc := &grpc.StreamDesc{
		StreamName:    "WatchJob",
		ServerStreams: true,
	}
	stream, err := c.conn.NewStream(ctx, desc, "/coresdk.v1.JobService/WatchJob")
	if err != nil {
		return nil, fmt.Errorf("coresdk: WatchJob: %w", err)
	}
	if err := stream.SendMsg(grpcFrame(payload)); err != nil {
		return nil, fmt.Errorf("coresdk: WatchJob send: %w", err)
	}
	if err := stream.CloseSend(); err != nil {
		return nil, fmt.Errorf("coresdk: WatchJob close: %w", err)
	}
	ch := make(chan JobEvent, 8)
	go func() {
		defer close(ch)
		for {
			var msg []byte
			if err := stream.RecvMsg(&msg); err != nil {
				if err == io.EOF {
					return
				}
				return
			}
			ev := decodeJobEvent(grpcUnframe(msg))
			ev.JobID = jobID
			select {
			case ch <- ev:
			case <-ctx.Done():
				return
			}
			if ev.Kind == JobEventSucceeded || ev.Kind == JobEventFailed || ev.Kind == JobEventCancelled {
				return
			}
		}
	}()
	return ch, nil
}

// StreamJobLogs returns a channel of LogLines tailed from the job's container.
func (c *Client) StreamJobLogs(ctx context.Context, jobID string, follow bool, tailLines uint32) (<-chan LogLine, error) {
	payload := []byte(encodeString(1, jobID))
	if follow {
		payload = append(payload, encodeVarintField(2, 1)...)
	}
	if tailLines != 0 {
		payload = append(payload, encodeVarintField(3, uint64(tailLines))...)
	}
	payload = append(payload, encodeTenant(4, c.config.TenantID)...)
	desc := &grpc.StreamDesc{
		StreamName:    "GetJobLogs",
		ServerStreams: true,
	}
	stream, err := c.conn.NewStream(ctx, desc, "/coresdk.v1.JobService/GetJobLogs")
	if err != nil {
		return nil, fmt.Errorf("coresdk: GetJobLogs: %w", err)
	}
	if err := stream.SendMsg(grpcFrame(payload)); err != nil {
		return nil, fmt.Errorf("coresdk: GetJobLogs send: %w", err)
	}
	if err := stream.CloseSend(); err != nil {
		return nil, fmt.Errorf("coresdk: GetJobLogs close: %w", err)
	}
	ch := make(chan LogLine, 32)
	go func() {
		defer close(ch)
		for {
			var msg []byte
			if err := stream.RecvMsg(&msg); err != nil {
				return
			}
			line := decodeLogLine(grpcUnframe(msg))
			line.JobID = jobID
			select {
			case ch <- line:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}
