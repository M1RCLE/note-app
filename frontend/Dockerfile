# syntax=docker/dockerfile:1

# nginx-unprivileged: тот же официальный nginx, но работает от UID 101
# и слушает непривилегированный порт 8080, поэтому root не нужен.
FROM nginxinc/nginx-unprivileged:1.28-alpine

# Конфиг сайта заменяет дефолтный.
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Статика фронтенда.
COPY index.html app.js style.css /usr/share/nginx/html/

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/nginx-health || exit 1
