package queue

import (
	"context"
	"errors"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/efrilya07-eng/task-orchestrator/internal/task"
)

var (
	ErrQueueClosed = errors.New("task queue is closed")
	ErrDuplicateID = errors.New("task with this ID already exists")
)

type Queue struct {
	// submitMu serializes submissions against channel shutdown. Add holds a
	// read lock while sending, but never holds mu during a potentially blocking
	// send, so workers can always update task state and drain the queue.
	submitMu  sync.RWMutex
	mu        sync.RWMutex
	tasks     map[string]*task.Task
	jobs      chan *task.Task
	executors map[task.Type]task.Executor
	wg        sync.WaitGroup
	closed    bool
}

func NewQueue(workers int) *Queue {
	if workers < 1 {
		workers = 1
	}

	q := &Queue{
		tasks: make(map[string]*task.Task),
		jobs:  make(chan *task.Task, 100),
		executors: map[task.Type]task.Executor{
			task.TypeSleep:     task.SleepExecutor{},
			task.TypeHTTPCheck: &task.HTTPCheckExecutor{},
		},
	}

	for i := 0; i < workers; i++ {
		q.wg.Add(1)
		go q.worker(i)
	}
	return q
}

func (q *Queue) worker(id int) {
	defer q.wg.Done()
	for t := range q.jobs {
		q.mu.Lock()
		t.Status = task.StatusRunning
		startedAt := time.Now().UTC()
		t.StartedAt = &startedAt
		exec := q.executors[t.Type]
		q.mu.Unlock()

		var err error
		if exec == nil {
			err = errors.New("unknown task type")
		} else {
			err = exec.Execute(t)
		}

		q.mu.Lock()
		if err != nil {
			t.Status = task.StatusFailed
			t.Error = err.Error()
		} else {
			t.Status = task.StatusCompleted
			t.Error = ""
		}
		finishedAt := time.Now().UTC()
		t.FinishedAt = &finishedAt
		q.mu.Unlock()

		if err != nil {
			log.Printf("worker %d: task %s failed: %v", id, t.ID, err)
		} else {
			log.Printf("worker %d: task %s completed", id, t.ID)
		}
	}
}

// Add stores a task and queues it if it has not reached a terminal state.
// A task left running when the previous process exited is safely retried.
func (q *Queue) Add(t *task.Task) error {
	if t == nil {
		return errors.New("task is nil")
	}
	if t.ID == "" {
		return errors.New("task ID is empty")
	}

	stored := t.Clone()
	if stored.Status == "" || stored.Status == task.StatusRunning {
		stored.Status = task.StatusPending
	}
	if stored.CreatedAt.IsZero() {
		stored.CreatedAt = time.Now().UTC()
	}

	// Keep Shutdown from closing jobs until this submission has finished.
	// Do not hold q.mu while sending: the send may block when the buffer is
	// full, and workers need q.mu to finish their current tasks.
	q.submitMu.RLock()
	defer q.submitMu.RUnlock()

	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return ErrQueueClosed
	}
	if _, exists := q.tasks[stored.ID]; exists {
		q.mu.Unlock()
		return ErrDuplicateID
	}
	q.tasks[stored.ID] = stored
	shouldQueue := stored.Status == task.StatusPending
	q.mu.Unlock()

	if shouldQueue {
		q.jobs <- stored
	}
	return nil
}

func (q *Queue) Get(id string) (*task.Task, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	t, ok := q.tasks[id]
	if !ok {
		return nil, false
	}
	return t.Clone(), true
}

func (q *Queue) All() []*task.Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]*task.Task, 0, len(q.tasks))
	for _, t := range q.tasks {
		result = append(result, t.Clone())
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result
}

func (q *Queue) Shutdown(ctx context.Context) {
	// Wait for in-flight Add calls before closing the channel. Once this lock
	// is acquired, no sender can still be using jobs.
	q.submitMu.Lock()
	q.mu.Lock()
	if !q.closed {
		q.closed = true
		close(q.jobs)
	}
	q.mu.Unlock()
	q.submitMu.Unlock()

	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("all workers stopped")
	case <-ctx.Done():
		log.Println("shutdown timeout")
	}
}
