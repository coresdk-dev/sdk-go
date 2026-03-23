package coresdk

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"crypto/tls"
	"crypto/x509"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
)

// Client wraps the gRPC channel to the sidecar.
type Client struct {
	conn   *grpc.ClientConn
	config *Config
}

// NewClient creates a gRPC client. Uses insecure in dev mode, mTLS in production.
func NewClient(cfg *Config) (*Client, error) {
	opts := []grpc.DialOption{
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	}

	// Attach x-service-token and x-service-name to every outgoing RPC
	opts = append(opts, grpc.WithUnaryInterceptor(
		func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, callOpts ...grpc.CallOption) error {
			md := metadata.Pairs("x-service-name", cfg.ServiceName)
			if cfg.ServiceToken != "" {
				md.Append("x-service-token", cfg.ServiceToken)
			}
			ctx = metadata.NewOutgoingContext(ctx, md)
			return invoker(ctx, method, req, reply, cc, callOpts...)
		},
	))

	switch {
	case cfg.DevMode || cfg.TLSCert == "":
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	case cfg.TLSCert != "" && cfg.TLSKey != "" && cfg.TLSCA != "":
		cert, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
		if err != nil {
			return nil, fmt.Errorf("coresdk: load client cert: %w", err)
		}
		caCert, err := os.ReadFile(cfg.TLSCA)
		if err != nil {
			return nil, fmt.Errorf("coresdk: load CA cert: %w", err)
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("coresdk: invalid CA cert")
		}
		tlsCfg := &tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caPool,
			MinVersion:   tls.VersionTLS13,
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	default:
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(cfg.SidecarAddr, opts...)
	if err != nil {
		return nil, fmt.Errorf("coresdk: grpc.NewClient(%s): %w", cfg.SidecarAddr, err)
	}
	return &Client{conn: conn, config: cfg}, nil
}

// ValidateToken calls AuthService.ValidateToken on the sidecar.
// Uses raw protobuf wire encoding — no protoc required.
func (c *Client) ValidateToken(ctx context.Context, token string) (*Claims, error) {
	// Encode request: token(field 1), tenant_id(field 2)
	payload := encodeString(1, token) + encodeString(2, c.config.TenantID)
	req := grpcFrame([]byte(payload))

	var respBytes []byte
	err := c.conn.Invoke(ctx, "/coresdk.v1.AuthService/ValidateToken", req, &respBytes)
	if err != nil {
		return nil, fmt.Errorf("coresdk: ValidateToken: %w", err)
	}

	body := grpcUnframe(respBytes)
	fields := decodeFields(body)

	allowed := decodeBool(fields, 1)
	subject := decodeString(fields, 2)
	roles := decodeRepeatedString(fields, 3)

	return &Claims{
		Subject:  subject,
		TenantID: c.config.TenantID,
		Roles:    roles,
		FailOpen: !allowed,
	}, nil
}

// EvaluatePolicy calls PolicyService.EvaluatePolicy on the sidecar.
func (c *Client) EvaluatePolicy(ctx context.Context, rule string, inputJSON string) (bool, error) {
	payload := encodeString(1, rule) + encodeString(2, inputJSON) + encodeString(3, c.config.TenantID)
	req := grpcFrame([]byte(payload))

	var respBytes []byte
	err := c.conn.Invoke(ctx, "/coresdk.v1.PolicyService/Evaluate", req, &respBytes)
	if err != nil {
		return false, fmt.Errorf("coresdk: EvaluatePolicy: %w", err)
	}

	body := grpcUnframe(respBytes)
	fields := decodeFields(body)
	return decodeBool(fields, 1), nil
}

// RevokeToken calls AuthService.RevokeToken on the sidecar.
func (c *Client) RevokeToken(ctx context.Context, token string) error {
	payload := encodeString(1, token) + encodeString(2, c.config.TenantID)
	req := grpcFrame([]byte(payload))

	var respBytes []byte
	err := c.conn.Invoke(ctx, "/coresdk.v1.AuthService/RevokeToken", req, &respBytes)
	if err != nil {
		return fmt.Errorf("coresdk: RevokeToken: %w", err)
	}
	return nil
}

// IsRevoked calls AuthService.IsRevoked on the sidecar.
func (c *Client) IsRevoked(ctx context.Context, token string) (bool, error) {
	payload := encodeString(1, token) + encodeString(2, c.config.TenantID)
	req := grpcFrame([]byte(payload))

	var respBytes []byte
	err := c.conn.Invoke(ctx, "/coresdk.v1.AuthService/IsRevoked", req, &respBytes)
	if err != nil {
		return false, fmt.Errorf("coresdk: IsRevoked: %w", err)
	}

	body := grpcUnframe(respBytes)
	fields := decodeFields(body)
	return decodeBool(fields, 1), nil
}

// CheckRateLimit calls RateLimitService.Check on the sidecar.
func (c *Client) CheckRateLimit(ctx context.Context, key string) (*RateLimitDecision, error) {
	payload := encodeString(1, key) + encodeString(2, c.config.TenantID)
	req := grpcFrame([]byte(payload))

	var respBytes []byte
	err := c.conn.Invoke(ctx, "/coresdk.v1.RateLimitService/Check", req, &respBytes)
	if err != nil {
		return nil, fmt.Errorf("coresdk: CheckRateLimit: %w", err)
	}

	body := grpcUnframe(respBytes)
	fields := decodeFields(body)
	return &RateLimitDecision{
		Allowed:   decodeBool(fields, 1),
		Remaining: decodeInt64(fields, 2),
		ResetAt:   decodeInt64(fields, 3),
	}, nil
}

// EmitAuditEvent calls AuditService.Emit on the sidecar.
func (c *Client) EmitAuditEvent(ctx context.Context, action, userID, outcome string, metadata map[string]string) error {
	payload := encodeString(1, action) + encodeString(2, userID) + encodeString(3, outcome) + encodeString(4, c.config.TenantID)
	// metadata as repeated key=value pairs in field 5
	for k, v := range metadata {
		payload += encodeString(5, k+"="+v)
	}
	req := grpcFrame([]byte(payload))

	var respBytes []byte
	err := c.conn.Invoke(ctx, "/coresdk.v1.AuditService/Emit", req, &respBytes)
	if err != nil {
		return fmt.Errorf("coresdk: EmitAuditEvent: %w", err)
	}
	return nil
}

// EvaluateFlag calls FlagService.Evaluate on the sidecar.
func (c *Client) EvaluateFlag(ctx context.Context, key, userID string) (*FlagDecision, error) {
	payload := encodeString(1, key) + encodeString(2, userID) + encodeString(3, c.config.TenantID)
	req := grpcFrame([]byte(payload))

	var respBytes []byte
	err := c.conn.Invoke(ctx, "/coresdk.v1.FlagService/Evaluate", req, &respBytes)
	if err != nil {
		return nil, fmt.Errorf("coresdk: EvaluateFlag: %w", err)
	}

	body := grpcUnframe(respBytes)
	fields := decodeFields(body)
	return &FlagDecision{
		Enabled: decodeBool(fields, 1),
		Key:     decodeString(fields, 2),
	}, nil
}

// CheckEntitlement calls LicenseService.CheckEntitlement on the sidecar.
func (c *Client) CheckEntitlement(ctx context.Context, key string) (*LicenseInfo, error) {
	payload := encodeString(1, key) + encodeString(2, c.config.TenantID)
	req := grpcFrame([]byte(payload))

	var respBytes []byte
	err := c.conn.Invoke(ctx, "/coresdk.v1.LicenseService/CheckEntitlement", req, &respBytes)
	if err != nil {
		return nil, fmt.Errorf("coresdk: CheckEntitlement: %w", err)
	}

	body := grpcUnframe(respBytes)
	fields := decodeFields(body)
	return &LicenseInfo{
		Allowed:  decodeBool(fields, 1),
		Plan:     decodeString(fields, 2),
		Features: decodeRepeatedString(fields, 3),
	}, nil
}

// Close tears down the underlying gRPC connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// ---------------------------------------------------------------------------
// Minimal protobuf wire codec
// ---------------------------------------------------------------------------

func encodeString(fieldNum int, value string) string {
	if value == "" {
		return ""
	}
	b := []byte(value)
	tag := varint(uint64(fieldNum<<3 | 2))
	length := varint(uint64(len(b)))
	return string(tag) + string(length) + value
}

func grpcFrame(payload []byte) []byte {
	n := len(payload)
	// gRPC framing: 4-byte big-endian message length (max ~4 GiB).
	// len(payload) fits in uint32 for any realistic message.
	frame := make([]byte, 5+n)
	frame[0] = 0 // not compressed
	binary.BigEndian.PutUint32(frame[1:5], uint32(n)) //nolint:gosec // len is non-negative and bounded by gRPC max message size
	copy(frame[5:], payload)
	return frame
}

func grpcUnframe(data []byte) []byte {
	if len(data) < 5 {
		return data
	}
	return data[5:]
}

func varint(n uint64) []byte {
	var buf []byte
	for {
		toWrite := byte(n & 0x7F)
		n >>= 7
		if n != 0 {
			buf = append(buf, toWrite|0x80)
		} else {
			buf = append(buf, toWrite)
			break
		}
	}
	return buf
}

func decodeFields(data []byte) map[int][][]byte {
	fields := map[int][][]byte{}
	i := 0
	for i < len(data) {
		tag, n := readVarint(data, i)
		if n == 0 {
			break
		}
		i += n
		fieldNum := int(tag >> 3) //nolint:gosec // tag field number fits in int
		wireType := tag & 0x7
		switch wireType {
		case 0: // varint
			val, n2 := readVarint(data, i)
			if n2 == 0 {
				goto done
			}
			i += n2
			b := make([]byte, 8)
			binary.LittleEndian.PutUint64(b, val)
			fields[fieldNum] = append(fields[fieldNum], b)
		case 2: // length-delimited
			length, n2 := readVarint(data, i)
			if n2 == 0 {
				goto done
			}
			i += n2
			msgLen := int(length) //nolint:gosec // protobuf length-delimited fields fit in int
			fields[fieldNum] = append(fields[fieldNum], data[i:i+msgLen])
			i += msgLen
		default:
			goto done
		}
	}
done:
	return fields
}

func readVarint(data []byte, pos int) (uint64, int) {
	var result uint64
	var shift uint
	for i := pos; i < len(data); i++ {
		b := data[i]
		result |= uint64(b&0x7F) << shift
		if b&0x80 == 0 {
			return result, i - pos + 1
		}
		shift += 7
	}
	return 0, 0
}

func decodeString(fields map[int][][]byte, fieldNum int) string {
	vals, ok := fields[fieldNum]
	if !ok || len(vals) == 0 {
		return ""
	}
	return string(vals[0])
}

func decodeRepeatedString(fields map[int][][]byte, fieldNum int) []string {
	vals, ok := fields[fieldNum]
	if !ok || len(vals) == 0 {
		return []string{}
	}
	result := make([]string, len(vals))
	for i, v := range vals {
		result[i] = string(v)
	}
	return result
}

func decodeInt64(fields map[int][][]byte, fieldNum int) int64 {
	vals, ok := fields[fieldNum]
	if !ok || len(vals) == 0 {
		return 0
	}
	if len(vals[0]) >= 8 {
		return int64(binary.LittleEndian.Uint64(vals[0])) //nolint:gosec // varint stored as uint64
	}
	if len(vals[0]) > 0 {
		return int64(vals[0][0])
	}
	return 0
}


func decodeBool(fields map[int][][]byte, fieldNum int) bool {
	vals, ok := fields[fieldNum]
	if !ok || len(vals) == 0 {
		return false
	}
	if len(vals[0]) >= 8 {
		return binary.LittleEndian.Uint64(vals[0]) != 0
	}
	return len(vals[0]) > 0 && vals[0][0] != 0
}
