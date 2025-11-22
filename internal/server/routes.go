package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/piranhaCodes/webpty-relay/internal/auth"
)

// ErrorResponse represents an error response
type ErrorResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

// SuccessResponse represents a success response
type SuccessResponse struct {
	OK   bool        `json:"ok"`
	Data interface{} `json:"data,omitempty"`
}

// InviteRequest represents a request to create an invite
type InviteRequest struct {
	Role      string `json:"role"`
	SessionID string `json:"session_id,omitempty"`
	ExpiresIn int    `json:"expires_in,omitempty"` // seconds, default 24h
}

// InviteResponse represents an invite response
type InviteResponse struct {
	Token string `json:"token"`
	URL   string `json:"url"`
}

// SessionInfo represents session information for admin API
type SessionInfo struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	ClientCount int    `json:"client_count"`
}

// handleHealth handles the health check endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SuccessResponse{
		OK:   true,
		Data: map[string]string{"status": "healthy"},
	})
}

// handleAdminInvite handles POST /api/admin/invite to create JWT invite tokens.
func (s *Server) handleAdminInvite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	password := r.Header.Get("X-Admin-Password")
	if password == "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			claims, err := auth.ValidateToken(strings.TrimPrefix(authHeader, "Bearer "), s.config.JWTSecret)
			if err != nil || claims.Role != auth.RoleAdmin {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(ErrorResponse{
					OK:    false,
					Error: "unauthorized",
				})
				return
			}
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(ErrorResponse{
				OK:    false,
				Error: "missing admin password or token",
			})
			return
		}
	} else {
		if !auth.VerifyAdminPassword(password, s.config.AdminPassword) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(ErrorResponse{
				OK:    false,
				Error: "invalid admin password",
			})
			return
		}
	}

	// Parse request
	var req InviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			OK:    false,
			Error: "invalid request body",
		})
		return
	}

	role := auth.Role(req.Role)
	if role != auth.RoleAdmin && role != auth.RoleWrite && role != auth.RoleRead {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			OK:    false,
			Error: "invalid role (must be admin, write, or read)",
		})
		return
	}

	expiresIn := time.Hour * 24
	if req.ExpiresIn > 0 {
		expiresIn = time.Duration(req.ExpiresIn) * time.Second
	}

	token, err := auth.GenerateInviteToken(s.config.JWTSecret, role, req.SessionID, expiresIn)
	if err != nil {
		log.Printf("Failed to generate invite token: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{
			OK:    false,
			Error: "failed to generate token",
		})
		return
	}

	url := "/ws/session/"
	if req.SessionID != "" {
		url += req.SessionID
	} else {
		url += "<session-id>"
	}
	url += "?token=" + token

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SuccessResponse{
		OK: true,
		Data: InviteResponse{
			Token: token,
			URL:   url,
		},
	})
}

// handleAdminSessions handles GET /api/admin/sessions to list all active terminal sessions.
func (s *Server) handleAdminSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check admin password
	password := r.Header.Get("X-Admin-Password")
	if password == "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			claims, err := auth.ValidateToken(strings.TrimPrefix(authHeader, "Bearer "), s.config.JWTSecret)
			if err != nil || claims.Role != auth.RoleAdmin {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(ErrorResponse{
					OK:    false,
					Error: "unauthorized",
				})
				return
			}
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(ErrorResponse{
				OK:    false,
				Error: "missing admin password or token",
			})
			return
		}
	} else {
		if !auth.VerifyAdminPassword(password, s.config.AdminPassword) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(ErrorResponse{
				OK:    false,
				Error: "invalid admin password",
			})
			return
		}
	}

	// Get sessions from manager
	sessions := s.sessionManager.List()
	sessionInfos := make([]SessionInfo, 0, len(sessions))

	for _, sess := range sessions {
		sessionInfos = append(sessionInfos, SessionInfo{
			ID:          sess.ID,
			Status:      string(sess.Status),
			ClientCount: sess.ClientCount(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SuccessResponse{
		OK: true,
		Data: map[string]interface{}{
			"sessions": sessionInfos,
			"count":    len(sessionInfos),
		},
	})
}

// handleAdminDeleteSession handles DELETE /api/admin/session/:id to terminate a terminal session.
func (s *Server) handleAdminDeleteSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check admin password
	password := r.Header.Get("X-Admin-Password")
	if password == "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			claims, err := auth.ValidateToken(strings.TrimPrefix(authHeader, "Bearer "), s.config.JWTSecret)
			if err != nil || claims.Role != auth.RoleAdmin {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(ErrorResponse{
					OK:    false,
					Error: "unauthorized",
				})
				return
			}
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(ErrorResponse{
				OK:    false,
				Error: "missing admin password or token",
			})
			return
		}
	} else {
		if !auth.VerifyAdminPassword(password, s.config.AdminPassword) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(ErrorResponse{
				OK:    false,
				Error: "invalid admin password",
			})
			return
		}
	}

	// Extract session ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/session/")
	if path == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			OK:    false,
			Error: "session ID required",
		})
		return
	}

	if err := s.ptyClient.Kill(path); err != nil {
		log.Printf("Failed to kill PTY session %s: %v", path, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{
			OK:    false,
			Error: "failed to kill session",
		})
		return
	}

	s.watchersMu.Lock()
	if watcher, exists := s.fifoWatchers[path]; exists {
		watcher.Stop()
		delete(s.fifoWatchers, path)
	}
	s.watchersMu.Unlock()

	s.sessionManager.Cleanup(path)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SuccessResponse{
		OK: true,
	})
}
