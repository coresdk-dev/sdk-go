package coresdk

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

// ProblemDetail implements RFC 9457 Problem Details for HTTP APIs.
type ProblemDetail struct {
	Type       string         `json:"type,omitempty"`
	Title      string         `json:"title"`
	Status     int            `json:"status"`
	Detail     string         `json:"detail,omitempty"`
	Instance   string         `json:"instance,omitempty"`
	Extensions map[string]any `json:"extensions,omitempty"`
}

func (p *ProblemDetail) Error() string {
	if p.Detail != "" {
		return fmt.Sprintf("[%d] %s: %s", p.Status, p.Title, p.Detail)
	}
	return fmt.Sprintf("[%d] %s", p.Status, p.Title)
}

func (p *ProblemDetail) JSON() []byte {
	b, _ := json.Marshal(p)
	return b
}

// WriteHTTP writes the ProblemDetail as an RFC 9457 response.
// Sets Content-Type: application/problem+json and the appropriate status code.
func (p *ProblemDetail) WriteHTTP(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	if err := json.NewEncoder(w).Encode(p); err != nil {
		slog.Error("coresdk: failed to write problem detail", "error", err)
	}
}

// Unauthorized returns a 401 ProblemDetail.
func Unauthorized(detail string) *ProblemDetail {
	return &ProblemDetail{
		Type:   "https://coresdk.io/errors/unauthorized",
		Title:  "Unauthorized",
		Status: 401,
		Detail: detail,
	}
}

// Forbidden returns a 403 ProblemDetail.
func Forbidden(detail string) *ProblemDetail {
	return &ProblemDetail{
		Type:   "https://coresdk.io/errors/forbidden",
		Title:  "Forbidden",
		Status: 403,
		Detail: detail,
	}
}
