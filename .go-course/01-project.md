# Проект: Spindle

## 0. Имя и как оно используется

Имя выбрано 2026-09-28 после проверки GitHub Search и Go module proxy: короткое, свободное,
произносимое, без дефисов. Проверено — опубликованного Go-модуля `spindle` нет, заметных
репозиториев с таким именем нет.

```
module path           github.com/твой-логин/spindle
каталоги              cmd/api, internal/{auth,projects,tasks,notifications,platform}
переменные окружения  SPINDLE_DB_DSN, SPINDLE_HTTP_ADDR, SPINDLE_LOG_LEVEL
база данных           spindle
docker compose        сервис api, зависит от postgres
префикс метрик        spindle_
поля логов            {"service":"spindle","version":...}
```

Альтернативы, рассмотренные и отклонённые: `TaskFlow` (32 706 репозиториев; в Go уже есть
библиотека `taskflow/taskflow` и `go-taskflow` — путаница при чтении кода), `Sprintline`
(занято, но оставалось запасным), `Relay`/`Baton` (общие имена), `Ledger`/`Slate` (занятые пути
модулей), `Tandem`/`Gantry` (тысячи репозиториев в ML/Python-нише).

## 1. Идея в одном абзаце

**Spindle** — REST-сервис управления задачами в проектах (упрощённый трекер): проекты, задачи,
комментарии, история переходов, роли участников. Всё, что меняет состояние задачи (уведомления,
проверка дедлайнов) обрабатывается **асинхронно** фоновыми воркерами: сервис создаёт задания
в очереди на PostgreSQL, воркеры забирают их, обрабатывают с повторами и дедлоками. Это позволяет
показать весь требуемый набор production-практик на одном связном домене: транзакции, конкурентность,
context и отмена, graceful shutdown, структурированные логи, метрики, health checks, миграции,
Docker Compose и CI.

**Почему не «URL-shortener» и не «органайзер документов»:** трекер закрывает все обязательные темы
естественно и без посторонних рисков (OCR/cgo, форматы файлов). Органайзер документов отложен
как возможный второй проект после курса.

## 2. Домен

### 2.1. Статусы задачи

`backlog` → `todo` → `in_progress` → `done`

Разрешённые переходы задаются таблицей в домене (на L07), неразрешённый переход — доменная ошибка
`ErrInvalidTransition`, не HTTP-ошибка «500».

### 2.2. Сущности

```
User        id, email (unique), password_hash, display_name, created_at
Project     id, name, description, owner_id, created_at, archived_at?
Membership  project_id, user_id, role (owner | editor | viewer), created_at
Task        id, project_id, title, description, status, priority (low|normal|high|urgent),
            assignee_id?, due_at?, created_by, created_at, updated_at, completed_at?
Comment     id, task_id, author_id, body, created_at
Transition  id, task_id, from_status, to_status, actor_id, created_at   (audit trail, append-only)
Notification id, user_id, task_id, kind, payload(jsonb), read_at?, created_at
Job         id, kind, payload(jsonb), status (pending|running|done|failed|dead),
            attempts, max_attempts, run_at, lease_until?, last_error?, created_at, updated_at
```

`Transition` пишется в той же транзакции, что и смена статуса. Это первое осознанное требование
к консистентности в проекте.

### 2.3. Роли и доступ

- `owner` — всё в проекте, включая удаление проекта и участников.
- `editor` — создавать/менять задачи, комментировать, менять статусы.
- `viewer` — только чтение.
- Проверка прав — в слое usecase/service, транспорт только извлекает identity из контекста.
- Никакого «admin bypass» из коробки; при необходимости — явная проверка.

## 3. API (v1)

Все ответы — JSON. Ошибки — единый конверт (см. `02-requirements.md`, N7).

### Аутентификация
```
POST   /api/v1/auth/register        → 201 {user}
POST   /api/v1/auth/login           → 200 {access_token, refresh_token, expires_in}
POST   /api/v1/auth/refresh         → 200 {access_token, ...}
POST   /api/v1/auth/logout          → 204 (отзыв refresh-токена)
GET    /api/v1/me                   → 200 {user}
```

### Проекты и участники
```
POST   /api/v1/projects
GET    /api/v1/projects                 (только проекты, где пользователь участник; пагинация)
GET    /api/v1/projects/{projectID}
PATCH  /api/v1/projects/{projectID}
POST   /api/v1/projects/{projectID}/archive
POST   /api/v1/projects/{projectID}/members
GET    /api/v1/projects/{projectID}/members
DELETE /api/v1/projects/{projectID}/members/{userID}
```

### Задачи
```
POST   /api/v1/projects/{projectID}/tasks
GET    /api/v1/projects/{projectID}/tasks
       ?status=&assignee_id=&due_before=&q=&limit=&offset=&sort=
GET    /api/v1/projects/{projectID}/tasks/{taskID}
PATCH  /api/v1/projects/{projectID}/tasks/{taskID}
DELETE /api/v1/projects/{projectID}/tasks/{taskID}
POST   /api/v1/projects/{projectID}/tasks/{taskID}/comments
GET    /api/v1/projects/{projectID}/tasks/{taskID}/comments
POST   /api/v1/projects/{projectID}/tasks/{taskID}/transition   {to_status}
GET    /api/v1/projects/{projectID}/tasks/{taskID}/transitions
```

### Уведомления и операционные ручки
```
GET    /api/v1/notifications?unread=true
POST   /api/v1/notifications/{id}/read
GET    /healthz      liveness
GET    /readyz       readiness (БД доступна, воркеры запущены)
GET    /metrics      prometheus
GET    /debug/pprof/*   только с localhost и только если явно включено флагом
```

## 4. Фоновая обработка

Три вида заданий (`Job.kind`):

1. `notify_task_assigned` — уведомление назначенному.
2. `notify_task_commented` — уведомления участникам, кроме автора.
3. `notify_due_soon` — создаётся периодической задачей (ticker), находит задачи с `due_at`
   в ближайшем окне, для которых ещё нет уведомления; **идемпотентна** по ключу
   `(task_id, kind, due_at)`.

Свойства конвейера (проверяемые в заданиях):

- Забор заданий: `SELECT ... FOR UPDATE SKIP LOCKED` + lease.
- Повторы: экспоненциальный backoff + jitter, `max_attempts`, затем `dead` (DLQ).
- Доставка результата — **at-least-once**, поэтому обработчики идемпотентны.
- Остановка: `context` отменяется, воркеры дорабатывают текущее задание и выходят;
  `Shutdown` с таймаутом, затем `http.Server.Shutdown`.

## 5. Сценарии для демонстрации

1. Регистрация → логин → создание проекта → приглашение участника.
2. Создание задач, фильтрация/поиск/сортировка/пагинация.
3. Смена статуса по разрешённой таблице, попытка неразрешённого перехода → 422 с кодом ошибки.
4. Назначение задачи → появляется job → воркер создаёт уведомление → пользователь его читает.
5. Задача с дедлайном → периодический скан → уведомление «срок скоро».
6. Искусственный сбой обработчика → задание уходит в повтор → после `max_attempts` в `dead`,
   это видно в `/metrics` и в логах.
7. `docker compose down` / SIGTERM → graceful shutdown: HTTP-сервер перестаёт принимать соединения,
   воркеры дорабатывают, процесс завершается без ошибок в логах.
8. `/healthz` и `/readyz` дают 200; при остановленном Postgres `/readyz` даёт 503.

## 6. Что сознательно НЕ входит в v1 (защита от разрастания)

- Файлы и вложения, загрузка изображений, OCR, поиск по тексту.
- Email/SMS/пуш-уведомления (только интерфейс- заглушка `Notifier`, реализация — in-app).
- WebSocket/SSE-реалтайм, дашборды, отчёты и аналитика.
- Календарь/Gantt, Kanban-доска, зависимости между задачами, подзадачи.
- Soft delete везде, версионирование сущностей, аудит всего подряд.
- Kubernetes, Kafka, gRPC, микросервисы, CQRS, event sourcing.
- Мультиарендность уровня организации (только проекты и участники).
- i18n, rate limiting на уровне edge (базовый лимит запросов — да, в v2).
