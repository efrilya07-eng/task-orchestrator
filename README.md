# Task Orchestrator

Небольшой HTTP-сервис на Go с очередью фоновых задач и сохранением задач в `tasks.json`.

## Требования

- Go 1.23 или новее.

## Запуск

```bash
go test ./...
go run .
```

Сервер слушает порт `8080`. Адрес можно изменить через переменную окружения `HTTP_ADDR`, например `HTTP_ADDR=:8081 go run .`.

## API

Создать задачу `sleep`:

```bash
curl -i -X POST http://localhost:8080/tasks \
  -H 'Content-Type: application/json' \
  -d '{"type":"sleep","duration_ms":1500}'
```

Создать задачу проверки HTTP:

```bash
curl -i -X POST http://localhost:8080/tasks \
  -H 'Content-Type: application/json' \
  -d '{"type":"http_check","url":"https://example.com"}'
```

Получить задачу по ID:

```bash
curl 'http://localhost:8080/task?id=TASK_ID'
```

Получить статистику:

```bash
curl http://localhost:8080/stats
```

При остановке сервера задачи сохраняются в `tasks.json`. При следующем запуске незавершённые задачи ставятся в очередь повторно; завершённые и упавшие задачи повторно не запускаются.
