package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/efrilya07-eng/task-orchestrator/internal/queue"
	"github.com/efrilya07-eng/task-orchestrator/internal/task"
)

func TestCreateTaskAndGetTask(t *testing.T) {
	q := queue.NewQueue(1)
	defer q.Shutdown(context.Background())
	a := New(q)

	req := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(`{"type":"sleep","duration_ms":1}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	a.CreateTask(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST /tasks status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var created task.Task
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" {
		t.Fatal("created task has empty ID")
	}
	if created.StartedAt != nil || created.FinishedAt != nil {
		t.Fatal("new task should not have start or finish timestamps")
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		getReq := httptest.NewRequest(http.MethodGet, "/task?id="+created.ID, nil)
		getRec := httptest.NewRecorder()
		a.GetTask(getRec, getReq)
		if getRec.Code != http.StatusOK {
			t.Fatalf("GET /task status = %d, body = %s", getRec.Code, getRec.Body.String())
		}
		var got task.Task
		if err := json.Unmarshal(getRec.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode get response: %v", err)
		}
		if got.Status == task.StatusCompleted {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("created task did not complete")
}

func TestCreateTaskRejectsUnknownType(t *testing.T) {
	q := queue.NewQueue(1)
	defer q.Shutdown(context.Background())
	a := New(q)

	req := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(`{"type":"unknown"}`))
	rec := httptest.NewRecorder()
	a.CreateTask(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}
