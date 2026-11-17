package task

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Type string

const (
	TypeSleep     Type = "sleep"
	TypeHTTPCheck Type = "http_check"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

type Task struct {
	ID         string     `json:"id"`
	Type       Type       `json:"type"`
	Status     Status     `json:"status"`
	DurationMS int64      `json:"duration_ms,omitempty"`
	URL        string     `json:"url,omitempty"`
	Error      string     `json:"error,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// NewID creates a UUID-shaped identifier using cryptographically secure bytes.
func NewID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate task ID: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	encoded := make([]byte, hex.EncodedLen(len(b)))
	hex.Encode(encoded, b[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[0:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:32]), nil
}

func (t *Task) Clone() *Task {
	if t == nil {
		return nil
	}
	copy := *t
	if t.StartedAt != nil {
		startedAt := *t.StartedAt
		copy.StartedAt = &startedAt
	}
	if t.FinishedAt != nil {
		finishedAt := *t.FinishedAt
		copy.FinishedAt = &finishedAt
	}
	return &copy
}

type Executor interface {
	Execute(t *Task) error
}

type SleepExecutor struct{}

func (SleepExecutor) Execute(t *Task) error {
	if t == nil {
		return errors.New("task is nil")
	}
	if t.DurationMS < 0 {
		return errors.New("duration_ms cannot be negative")
	}
	timer := time.NewTimer(time.Duration(t.DurationMS) * time.Millisecond)
	defer timer.Stop()
	<-timer.C
	return nil
}

type HTTPCheckExecutor struct {
	Client *http.Client
}

func (e *HTTPCheckExecutor) Execute(t *Task) error {
	if t == nil {
		return errors.New("task is nil")
	}
	client := e.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	resp, err := client.Get(t.URL)
	if err != nil {
		return fmt.Errorf("HTTP check failed: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("HTTP check returned status %s", resp.Status)
	}
	return nil
}
