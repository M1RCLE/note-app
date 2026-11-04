-- Выполняется entrypoint-скриптом postgres ОДИН раз,
-- при первом запуске на пустом volume.
CREATE TABLE IF NOT EXISTS notes (
    id         BIGSERIAL PRIMARY KEY,
    text       TEXT        NOT NULL CHECK (char_length(text) BETWEEN 1 AND 1000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO notes (text) VALUES ('Первая заметка: приложение развёрнуто');
