package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/efrilya07-eng/task-orchestrator/internal/task"
)

type Storage struct {
	path string
}

func New(path string) *Storage {
	return &Storage{path: path}
}

func (s *Storage) Save(tasks []*task.Task) error {
	if s == nil || s.path == "" {
		return errors.New("storage path is empty")
	}
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(s.path); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (s *Storage) Load() ([]*task.Task, error) {
	if s == nil || s.path == "" {
		return nil, errors.New("storage path is empty")
	}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var tasks []*task.Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}
