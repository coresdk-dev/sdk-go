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

	if cfg.DevMode || cfg.TLSCert == "" {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else if cfg.TLSCert != "" && cfg.TLSKey != "" && cfg.TLSCA != "" {
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
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.Dial(cfg.SidecarAddr, opts...)
	if err != nil {
		return nil, fmt.Errorf("coresdk: grpc.Dial(%s): %w", cfg.SidecarAddr, err)
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
	tenantID := decodeString(fields, 3)
	if tenantID == "" {
		tenantID = c.config.TenantID
	}

	return &Claims{
		Subject:  subject,
		TenantID: tenantID,
		Roles:    []string{},
		FailOpen: !allowed,
	}, nil
}

// EvaluatePolicy calls PolicyService.EvaluatePolicy on the sidecar.
func (c *Client) EvaluatePolicy(ctx context.Context, rule string, inputJSON string) (bool, error) {
	payload := encodeString(1, rule) + encodeString(2, inputJSON) + encodeString(3, c.config.TenantID)
	req := grpcFrame([]byte(payload))

	var respBytes []byte
	err := c.conn.Invoke(ctx, "/coresdk.v1.PolicyService/EvaluatePolicy", req, &respBytes)
	if err != nil {
		return false, fmt.Errorf("coresdk: EvaluatePolicy: %w", err)
	}

	body := grpcUnframe(respBytes)
	fields := decodeFields(body)
	return decodeBool(fields, 1), nil
}

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
	frame := make([]byte, 5+len(payload))
	frame[0] = 0 // not compressed
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(payload)))
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
		fieldNum := int(tag >> 3)
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
			fields[fieldNum] = append(fields[fieldNum], data[i:i+int(length)])
			i += int(length)
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
