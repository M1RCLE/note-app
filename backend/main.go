// Backend: REST API заметок поверх PostgreSQL.
//
//	GET    /api/notes      — список заметок
//	POST   /api/notes      — создать заметку {"text": "..."}
//	DELETE /api/notes/{id} — удалить заметку
//	GET    /healthz        — liveness (процесс жив)
//	GET    /readyz         — readiness (есть соединение с БД)
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"
)

type Note struct {
	ID        int64     `json:"id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

type server struct {
	db  *sql.DB
	log *slog.Logger
}

func main() {
	// В distroless-образе нет curl/wget, поэтому Docker HEALTHCHECK
	// вызывает сам бинарник: `/api healthcheck`.
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	db, err := sql.Open("postgres", dsnFromEnv())
	if err != nil {
		log.Error("open db", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)

	s := &server{db: db, log: log}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /readyz", s.readyz)
	mux.HandleFunc("GET /api/notes", s.listNotes)
	mux.HandleFunc("POST /api/notes", s.createNote)
	mux.HandleFunc("DELETE /api/notes/{id}", s.deleteNote)

	srv := &http.Server{
		Addr:              ":" + getenv("APP_PORT", "8080"),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
	}

	// Graceful shutdown по SIGTERM (его шлёт docker stop).
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("listen", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown", "err", err)
	}
}

// healthcheck возвращает 0, если /readyz отвечает 200 (приложение и БД доступны).
func healthcheck() int {
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + getenv("APP_PORT", "8080") + "/readyz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func dsnFromEnv() string {
	return fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
		getenv("DB_HOST", "db"),
		getenv("DB_PORT", "5432"),
		getenv("DB_NAME", "notes"),
		getenv("DB_USER", "notes"),
		os.Getenv("DB_PASSWORD"),
	)
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func (s *server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.db.PingContext(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "db unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) listNotes(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(),
		`SELECT id, text, created_at FROM notes ORDER BY id DESC LIMIT 100`)
	if err != nil {
		s.internalError(w, "list notes", err)
		return
	}
	defer rows.Close()

	notes := make([]Note, 0)
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.Text, &n.CreatedAt); err != nil {
			s.internalError(w, "scan note", err)
			return
		}
		notes = append(notes, n)
	}
	if err := rows.Err(); err != nil {
		s.internalError(w, "iterate notes", err)
		return
	}
	writeJSON(w, http.StatusOK, notes)
}

func (s *server) createNote(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	req.Text = strings.TrimSpace(req.Text)
	if req.Text == "" || len(req.Text) > 1000 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "text must be 1..1000 chars"})
		return
	}

	var n Note
	err := s.db.QueryRowContext(r.Context(),
		`INSERT INTO notes (text) VALUES ($1) RETURNING id, text, created_at`, req.Text,
	).Scan(&n.ID, &n.Text, &n.CreatedAt)
	if err != nil {
		s.internalError(w, "insert note", err)
		return
	}
	writeJSON(w, http.StatusCreated, n)
}

func (s *server) deleteNote(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	res, err := s.db.ExecContext(r.Context(), `DELETE FROM notes WHERE id = $1`, id)
	if err != nil {
		s.internalError(w, "delete note", err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) internalError(w http.ResponseWriter, op string, err error) {
	s.log.Error(op, "err", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
