# Task Orchestrator

<p>
  <img src="https://img.shields.io/badge/Go-1.23%2B-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.23+">
  <img src="https://img.shields.io/badge/HTTP-API-2563eb?style=flat-square" alt="HTTP API">
  <img src="https://img.shields.io/badge/Background-Tasks-475569?style=flat-square" alt="Background tasks">
</p>

A small HTTP service written in **Go** for submitting and processing background tasks. Tasks are stored in a JSON file, so unfinished work can be queued again after a server restart.

## Features

- Background task queue
- Sleep tasks with a configurable duration
- HTTP check tasks for checking a URL
- Task state persistence in a JSON file
- Re-queues unfinished tasks after restart
- HTTP API for task creation, task lookup and service statistics

## Requirements

- Go 1.23 or newer

## Getting Started

Clone the repository:

~~~bash
git clone https://github.com/efrilya07-eng/task-orchestrator.git
cd task-orchestrator
~~~

Run the tests:

~~~bash
go test ./...
~~~

Start the server:

~~~bash
go run .
~~~

The server listens on port **8080** by default. Set the HTTP_ADDR environment variable to use another address:

~~~bash
HTTP_ADDR=:8081 go run .
~~~

## API

### Create a sleep task

~~~bash
curl -i -X POST http://localhost:8080/tasks \
  -H 'Content-Type: application/json' \
  -d '{"type":"sleep","duration_ms":1500}'
~~~

### Create an HTTP check task

~~~bash
curl -i -X POST http://localhost:8080/tasks \
  -H 'Content-Type: application/json' \
  -d '{"type":"http_check","url":"https://example.com"}'
~~~

### Get a task by ID

~~~bash
curl 'http://localhost:8080/task?id=TASK_ID'
~~~

### Get service statistics

~~~bash
curl http://localhost:8080/stats
~~~

## Persistence

When the server stops, tasks are saved to the JSON file. On the next start, unfinished tasks are queued again. Completed and failed tasks are not restarted.

---

<p align="center"><em>Built with Go.</em></p>
