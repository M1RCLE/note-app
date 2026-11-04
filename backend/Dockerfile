# syntax=docker/dockerfile:1

# ---------- Stage 1: сборка ----------
FROM golang:1.24-alpine AS build
WORKDIR /src

# Сначала только go.mod/go.sum: слой с зависимостями кэшируется,
# пока они не меняются, и не пересобирается при правке кода.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO_ENABLED=0 — статический бинарник, работает без libc.
# -trimpath и -s -w убирают локальные пути и отладочную информацию.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api .

# ---------- Stage 2: runtime ----------
# distroless/static: нет shell и пакетного менеджера, только бинарник.
# Тег :nonroot — процесс запускается от UID 65532, не от root.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /api

ENV APP_PORT=8080
EXPOSE 8080
USER nonroot:nonroot

HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 \
  CMD ["/api", "healthcheck"]

ENTRYPOINT ["/api"]
