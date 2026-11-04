# notes-app

Приложение «Заметки» из трёх компонентов, у каждого свой Dockerfile.

```
браузер ──:80──▶ frontend (nginx) ──/api/*──▶ backend (Go) ──▶ db (PostgreSQL)
                 сеть frontend                 сеть backend (internal)
```

| Компонент  | Каталог     | Образ-основа                              | Что делает |
|------------|-------------|-------------------------------------------|------------|
| frontend   | `frontend/` | `nginxinc/nginx-unprivileged:1.28-alpine` | Отдаёт статику, проксирует `/api/` в backend |
| backend    | `backend/`  | `golang:1.24-alpine` → `distroless/static` | REST API заметок |
| db         | `db/`       | `postgres:17-alpine`                      | Хранит заметки, схема из `init.sql` |

## API

| Метод  | Путь              | Описание |
|--------|-------------------|----------|
| GET    | `/api/notes`      | Список заметок (до 100, новые первыми) |
| POST   | `/api/notes`      | Создать: `{"text": "..."}` (1–1000 символов) |
| DELETE | `/api/notes/{id}` | Удалить |
| GET    | `/healthz`        | Liveness: процесс жив |
| GET    | `/readyz`         | Readiness: есть соединение с БД |

## Локальный запуск

```bash
cp .env.example .env        # задать DB_PASSWORD
docker compose up -d --build
docker compose ps           # все три сервиса должны быть healthy
curl http://localhost/api/notes
```

Открыть http://localhost в браузере.

Остановить: `docker compose down` (данные в volume сохраняются),
с удалением данных: `docker compose down -v`.

## Решения в Dockerfile

**backend** — multi-stage сборка:
1. В stage `build` сначала копируются только `go.mod`/`go.sum` и выполняется
   `go mod download`. Этот слой кэшируется: при изменении кода зависимости
   заново не скачиваются.
2. Бинарник собирается статически (`CGO_ENABLED=0`), поэтому runtime-образу
   не нужна даже libc.
3. Runtime — `distroless/static:nonroot`: ни shell, ни пакетного менеджера,
   процесс работает не от root. Образ весит ~10 МБ вместо ~300 МБ у `golang`.
4. В distroless нет `curl`/`wget`, поэтому `HEALTHCHECK` вызывает сам бинарник:
   `/api healthcheck` делает запрос к `/readyz`.

**frontend** — `nginx-unprivileged` работает от UID 101 на порту 8080.
Статика и конфиг копируются в образ. Backend доступен по имени сервиса
`backend` через встроенный DNS Docker (`127.0.0.11`).

**db** — официальный `postgres` + `init.sql` в `/docker-entrypoint-initdb.d/`.
Скрипт выполняется один раз, на пустом volume. Пароль в образ не зашивается,
он передаётся через переменные окружения.

**compose.yaml**:
- наружу опубликован только порт frontend;
- сеть `backend` помечена `internal` — у db нет доступа в интернет и снаружи;
- `depends_on: condition: service_healthy` — сервисы стартуют по цепочке
  db → backend → frontend, только когда предыдущий прошёл healthcheck;
- данные PostgreSQL лежат в именованном volume `db-data`.

## Развёртывание на сервер

Делается Ansible-плейбуком из репозитория `notes-infra`.
