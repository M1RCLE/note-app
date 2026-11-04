# syntax=docker/dockerfile:1

FROM postgres:17-alpine

# Все *.sql из этой папки выполняются при инициализации пустой БД.
# Схема «запекается» в образ, поэтому БД версионируется вместе с приложением.
COPY init.sql /docker-entrypoint-initdb.d/01-init.sql

# Пароль, имя БД и пользователь НЕ задаются здесь:
# они передаются через переменные окружения при запуске (см. compose.yaml).

HEALTHCHECK --interval=5s --timeout=3s --start-period=20s --retries=10 \
  CMD pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB" || exit 1
