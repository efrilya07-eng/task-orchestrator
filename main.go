package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/efrilya07-eng/task-orchestrator/internal/api"
	"github.com/efrilya07-eng/task-orchestrator/internal/queue"
	"github.com/efrilya07-eng/task-orchestrator/internal/storage"
)

func main() {
	q := queue.NewQueue(3)
	st := storage.New("tasks.json")

	// Restore saved tasks. Pending and previously running tasks are enqueued;
	// completed and failed tasks remain in storage without being rerun.
	tasks, err := st.Load()
	if err != nil {
		log.Printf("could not restore tasks: %v", err)
	} else {
		for _, t := range tasks {
			if t == nil {
				continue
			}
			if err := q.Add(t); err != nil {
				log.Printf("could not restore task %q: %v", t.ID, err)
			}
		}
		log.Printf("restored %d tasks", len(tasks))
	}

	a := api.New(q)
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks", a.CreateTask)
	mux.HandleFunc("/task", a.GetTask)
	mux.HandleFunc("/stats", a.Stats)

	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("server started on %s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stop)

	select {
	case sig := <-stop:
		log.Printf("received %s, shutting down", sig)
	case err := <-serverErr:
		log.Fatalf("HTTP server failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}
	q.Shutdown(ctx)

	if err := st.Save(q.All()); err != nil {
		log.Printf("save error: %v", err)
	}
	log.Println("server stopped")
}
