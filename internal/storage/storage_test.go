package storage

import (
	"path/filepath"
	"testing"

	"github.com/efrilya07-eng/task-orchestrator/internal/task"
)

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	st := New(path)
	want := []*task.Task{{ID: "stored-task", Type: task.TypeSleep, Status: task.StatusCompleted, DurationMS: 42}}

	if err := st.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := st.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != want[0].ID || got[0].DurationMS != want[0].DurationMS {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}
