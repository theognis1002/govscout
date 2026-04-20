package web

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	stdsync "sync"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/securecookie"
)

type Server struct {
	db       *sql.DB
	tmpls    map[string]*template.Template
	cookie   *securecookie.SecureCookie
	router   chi.Router
	syncing  atomic.Bool
	devMode  bool
	bgTasks  stdsync.WaitGroup
	bgCtx    context.Context
	bgCancel context.CancelFunc
}

func NewServer(db *sql.DB, opts ...ServerOption) *Server {
	secret := os.Getenv("AUTH_SECRET")
	if secret == "" {
		log.Println("WARNING: AUTH_SECRET not set, using insecure default. Set AUTH_SECRET in production!")
		secret = "dev-secret-change-me-in-production!!"
	}

	bgCtx, cancel := context.WithCancel(context.Background())
	s := &Server{
		db:       db,
		tmpls:    loadTemplates(),
		cookie:   newSecureCookie(secret),
		bgCtx:    bgCtx,
		bgCancel: cancel,
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.devMode {
		log.Println("dev mode: templates and CSS will reload from disk on each request")
	}
	s.router = s.routes()
	return s
}

type ServerOption func(*Server)

func WithDevMode(dev bool) ServerOption {
	return func(s *Server) { s.devMode = dev }
}

func (s *Server) routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// Static
	r.Get("/static/style.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		if s.devMode {
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFile(w, r, "internal/web/static/style.css")
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Write(staticCSS)
	})

	// Public
	r.Get("/login", s.handleLoginPage)
	r.Post("/login", s.handleLogin)
	r.Post("/logout", s.handleLogout)

	// Health
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	// Auth required
	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth)

		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/opportunities", http.StatusFound)
		})
		r.Get("/opportunities", s.handleOpportunities)
		r.Get("/opportunities/partial", s.handleOpportunitiesPartial)
		r.Get("/opportunities/export.csv", s.handleOpportunitiesExport)
		r.Get("/opportunities/{id}", s.handleOpportunityDetail)

		r.Get("/filters", s.handleFilters)
		r.Post("/filters", s.handleFilterCreate)
		r.Get("/filters/{id}", s.handleFilterEdit)
		r.Post("/filters/{id}", s.handleFilterUpdate)
		r.Post("/filters/{id}/delete", s.handleFilterDelete)

		r.Get("/alerts", s.handleAlertsList)
		r.Get("/alerts/new", s.handleAlertForm)
		r.Post("/alerts", s.handleAlertCreate)
		r.Get("/alerts/{id}", s.handleAlertDetail)
		r.Post("/alerts/{id}", s.handleAlertUpdate)
		r.Post("/alerts/{id}/toggle", s.handleAlertToggle)
		r.Get("/alerts/{id}/preview", s.handleAlertPreview)

		// Admin
		r.Group(func(r chi.Router) {
			r.Use(s.requireAdmin)
			r.Post("/admin/sync", s.handleAdminSync)
			r.Get("/admin/sync-runs", s.handleAdminSyncRuns)
			r.Get("/admin/users", s.handleAdminUsers)
			r.Post("/admin/users", s.handleAdminCreateUser)
			r.Post("/admin/users/{id}/delete", s.handleAdminDeleteUser)
		})
	})

	return r
}

func (s *Server) templates() map[string]*template.Template {
	if s.devMode {
		tmpls, err := loadTemplatesFromDisk()
		if err != nil {
			log.Printf("dev reload error: %v (using cached templates)", err)
			return s.tmpls
		}
		return tmpls
	}
	return s.tmpls
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// ListenAndServe starts the HTTP server. It blocks until the server stops.
// Deprecated: prefer Run with an explicit context.
func (s *Server) ListenAndServe(addr string) error {
	return s.Run(context.Background(), addr)
}

// Run starts the HTTP server and shuts down cleanly when ctx is cancelled.
// It waits (bounded) for in-flight requests and background goroutines.
func (s *Server) Run(ctx context.Context, addr string) error {
	if addr == "" {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		addr = fmt.Sprintf(":%s", port)
	}
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           s,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", addr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		log.Println("shutdown signal received, draining...")
	case err := <-errCh:
		if err != nil {
			return err
		}
		return nil
	}

	// Cancel background goroutines (admin-triggered sync) so they stop promptly.
	s.bgCancel()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	shutdownErr := httpSrv.Shutdown(shutdownCtx)

	// Wait (bounded) for background tasks.
	done := make(chan struct{})
	go func() { s.bgTasks.Wait(); close(done) }()
	select {
	case <-done:
	case <-shutdownCtx.Done():
		log.Println("background tasks still running at shutdown deadline")
	}

	if shutdownErr != nil {
		return fmt.Errorf("shutdown: %w", shutdownErr)
	}
	return nil
}
