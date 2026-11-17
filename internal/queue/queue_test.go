package queue

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/efrilya07-eng/task-orchestrator/internal/task"
)

func TestAddExecutesTaskAndReturnsSnapshot(t *testing.T) {
	q := NewQueue(1)
	defer q.Shutdown(context.Background())

	input := &task.Task{
		ID:         "test-sleep",
		Type:       task.TypeSleep,
		Status:     task.StatusPending,
		DurationMS: 1,
	}
	if err := q.Add(input); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		got, ok := q.Get(input.ID)
		if !ok {
			t.Fatal("task was not stored")
		}
		if got.Status == task.StatusCompleted {
			got.Status = task.StatusFailed // A returned snapshot must not mutate queue state.
			stored, _ := q.Get(input.ID)
			if stored.Status != task.StatusCompleted {
				t.Fatalf("modifying returned snapshot changed stored status: %s", stored.Status)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("task did not complete before timeout")
}

func TestAddRejectsDuplicateID(t *testing.T) {
	q := NewQueue(1)
	defer q.Shutdown(context.Background())

	tsk := &task.Task{ID: "same-id", Type: task.TypeSleep, Status: task.StatusCompleted}
	if err := q.Add(tsk); err != nil {
		t.Fatalf("first Add() error = %v", err)
	}
	if err := q.Add(tsk); err != ErrDuplicateID {
		t.Fatalf("second Add() error = %v, want %v", err, ErrDuplicateID)
	}
}

type firstTaskGateExecutor struct {
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (e *firstTaskGateExecutor) Execute(_ *task.Task) error {
	if e.calls.Add(1) == 1 {
		close(e.started)
		<-e.release
	}
	return nil
}

func TestAddWhileQueueIsFullDoesNotBlockWorkers(t *testing.T) {
	q := NewQueue(1)
	exec := &firstTaskGateExecutor{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	q.mu.Lock()
	q.executors[task.TypeSleep] = exec
	q.mu.Unlock()

	first := &task.Task{ID: "blocking-task", Type: task.TypeSleep, Status: task.StatusPending}
	if err := q.Add(first); err != nil {
		t.Fatalf("Add(first) error = %v", err)
	}
	select {
	case <-exec.started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start the first task")
	}

	// Fill the buffered jobs channel while the only worker is occupied.
	for i := 0; i < cap(q.jobs); i++ {
		item := &task.Task{ID: fmt.Sprintf("queued-%d", i), Type: task.TypeSleep, Status: task.StatusPending}
		if err := q.Add(item); err != nil {
			t.Fatalf("Add(%s) error = %v", item.ID, err)
		}
	}

	lastAdd := make(chan error, 1)
	go func() {
		lastAdd <- q.Add(&task.Task{ID: "after-full", Type: task.TypeSleep, Status: task.StatusPending})
	}()

	// The final Add is waiting for queue capacity. Releasing the worker must
	// let it acquire q.mu, complete its task, and receive another queued job.
	close(exec.release)
	select {
	case err := <-lastAdd:
		if err != nil {
			t.Fatalf("Add(after-full) error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Add blocked because a worker could not update task state")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	q.Shutdown(ctx)
}
