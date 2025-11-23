// Package server provides the HTTP and WebSocket server for the relay service.
package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/piranhaCodes/webpty-relay/internal/config"
	"github.com/piranhaCodes/webpty-relay/internal/ptyclient"
	"github.com/piranhaCodes/webpty-relay/internal/session"
)

// Server represents the HTTP/WebSocket server.
type Server struct {
	config         *config.Config
	httpServer     *http.Server
	sessionManager *session.Manager
	ptyClient      *ptyclient.Client
	fifoWatchers   map[string]*FIFOWatcher
	watchersMu     sync.RWMutex
}

// NewServer creates a new server instance with the given configuration.
func NewServer(cfg *config.Config) *Server {
	return &Server{
		config:         cfg,
		sessionManager: session.NewManager(),
		ptyClient:      ptyclient.NewClient(cfg.PTYSocket),
		fifoWatchers:   make(map[string]*FIFOWatcher),
	}
}

// corsMiddleware adds CORS headers to allow requests from the UI
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Allow requests from localhost:3000 (UI) or any origin in development
		allowedOrigins := []string{
			"http://localhost:3000",
			"http://127.0.0.1:3000",
		}

		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				allowed = true
				break
			}
		}

		// If no origin header, allow (for same-origin requests)
		if origin == "" {
			allowed = true
		}

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Admin-Password")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) setupRoutes() {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ws/session/", s.handleWebSocketRoute)
	mux.HandleFunc("/api/admin/invite", s.handleAdminInvite)
	mux.HandleFunc("/api/admin/sessions", s.handleAdminSessions)
	mux.HandleFunc("/api/admin/session/", s.handleAdminDeleteSession)

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.config.RelayPort),
		Handler: corsMiddleware(mux),
	}
}

func (s *Server) handleWebSocketRoute(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/ws/session/")
	if path == "" || path == r.URL.Path {
		http.Error(w, "session ID required", http.StatusBadRequest)
		return
	}

	s.HandleWebSocket(w, r, path)
}

// Start starts the HTTP server and begins accepting connections.
func (s *Server) Start() error {
	s.setupRoutes()

	log.Printf("Starting WebPTY Relay server on port %d", s.config.RelayPort)
	log.Printf("PTY socket: %s", s.config.PTYSocket)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

// Shutdown gracefully shuts down the server, stopping all FIFO watchers, killing PTY sessions, and closing HTTP connections.
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down server...")

	s.watchersMu.Lock()
	for sessionID, watcher := range s.fifoWatchers {
		log.Printf("Stopping FIFO watcher for session %s", sessionID)
		watcher.Stop()
	}
	s.fifoWatchers = make(map[string]*FIFOWatcher)
	s.watchersMu.Unlock()

	sessions := s.sessionManager.List()
	for _, sess := range sessions {
		log.Printf("Cleaning up session %s", sess.ID)
		if err := s.ptyClient.Kill(sess.ID); err != nil {
			log.Printf("Failed to kill PTY session %s: %v", sess.ID, err)
		}
		s.sessionManager.Cleanup(sess.ID)
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return s.httpServer.Shutdown(shutdownCtx)
}

// Run starts the server and handles graceful shutdown on SIGTERM or SIGINT signals.
func (s *Server) Run() error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		if err := s.Start(); err != nil {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return err
	case sig := <-sigChan:
		log.Printf("Received signal: %v", sig)
		ctx := context.Background()
		return s.Shutdown(ctx)
	}
}
