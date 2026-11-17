package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/efrilya07-eng/task-orchestrator/internal/queue"
	"github.com/efrilya07-eng/task-orchestrator/internal/task"
)

const maxRequestBody = 1 << 20 // 1 MiB

type API struct {
	queue *queue.Queue
}

func New(q *queue.Queue) *API {
	return &API{queue: q}
}

type createTaskRequest struct {
	Type       task.Type `json:"type"`
	DurationMS int64     `json:"duration_ms,omitempty"`
	URL        string    `json:"url,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type statsResponse struct {
	Total     int `json:"total"`
	Pending   int `json:"pending"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

func (a *API) CreateTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	defer r.Body.Close()

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBody))
	decoder.DisallowUnknownFields()
	var req createTaskRequest
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := ensureJSONEOF(decoder); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	id, err := task.NewID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not generate task ID")
		return
	}
	t := &task.Task{
		ID:         id,
		Type:       req.Type,
		Status:     task.StatusPending,
		DurationMS: req.DurationMS,
		URL:        strings.TrimSpace(req.URL),
		CreatedAt:  time.Now().UTC(),
	}
	if err := a.queue.Add(t); err != nil {
		if errors.Is(err, queue.ErrQueueClosed) {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	created, _ := a.queue.Get(id)
	writeJSON(w, http.StatusAccepted, created)
}

func (a *API) GetTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "query parameter 'id' is required")
		return
	}
	t, ok := a.queue.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (a *API) Stats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var stats statsResponse
	for _, t := range a.queue.All() {
		stats.Total++
		switch t.Status {
		case task.StatusPending:
			stats.Pending++
		case task.StatusRunning:
			stats.Running++
		case task.StatusCompleted:
			stats.Completed++
		case task.StatusFailed:
			stats.Failed++
		}
	}
	writeJSON(w, http.StatusOK, stats)
}

func validateRequest(req createTaskRequest) error {
	switch req.Type {
	case task.TypeSleep:
		if req.DurationMS < 0 {
			return errors.New("duration_ms cannot be negative")
		}
		if req.DurationMS > int64((24*time.Hour)/time.Millisecond) {
			return errors.New("duration_ms cannot exceed 24 hours")
		}
	case task.TypeHTTPCheck:
		parsed, err := url.ParseRequestURI(strings.TrimSpace(req.URL))
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return errors.New("url must be a valid http or https URL")
		}
	default:
		return fmt.Errorf("unsupported task type %q (use %q or %q)", req.Type, task.TypeSleep, task.TypeHTTPCheck)
	}
	return nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request body must contain one JSON object")
		}
		return fmt.Errorf("invalid trailing JSON: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}
