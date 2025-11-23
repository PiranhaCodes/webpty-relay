package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/piranhaCodes/webpty-relay/internal/auth"
	"github.com/piranhaCodes/webpty-relay/internal/session"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512 * 1024 // 512KB
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins in production, can be restricted
		return true
	},
}

// WSMessage represents a WebSocket message received from the client.
type WSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
	Cols int             `json:"cols,omitempty"`
	Rows int             `json:"rows,omitempty"`
}

// WSResponse represents a WebSocket message sent to the client.
type WSResponse struct {
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
	ID   string      `json:"id,omitempty"`
}

// HandleWebSocket handles WebSocket connections for terminal sessions, including authentication, session creation, and client attachment.
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request, sessionID string) {
	token := r.URL.Query().Get("token")
	if token == "" {
		token = r.Header.Get("Authorization")
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}
	}

	if token == "" {
		http.Error(w, "missing authentication token", http.StatusUnauthorized)
		return
	}

	claims, err := auth.ValidateToken(token, s.config.JWTSecret)
	if err != nil {
		log.Printf("Invalid token: %v", err)
		http.Error(w, "invalid authentication token", http.StatusUnauthorized)
		return
	}

	if claims.SessionID != "" && claims.SessionID != sessionID {
		http.Error(w, "token session ID mismatch", http.StatusForbidden)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("WebSocket connection established for session %s (role: %s)", sessionID, claims.Role)

	client := &session.Client{
		SendCh: make(chan []byte, 256),
	}

	// Handle "new" session ID or non-existent session
	if sessionID == "new" || sessionID == "" {
		if !auth.HasPermission(claims.Role, "spawn") {
			log.Printf("Permission denied for spawn (role: %s)", claims.Role)
			sendError(conn, "permission denied")
			return
		}

		ptyID, err := s.ptyClient.Spawn()
		if err != nil {
			log.Printf("Failed to spawn PTY: %v", err)
			sendError(conn, "failed to spawn PTY session")
			return
		}

		s.watchersMu.Lock()
		sess := s.sessionManager.GetOrCreate(ptyID)
		watcher := NewFIFOWatcher(ptyID, sess)
		watcher.Start(s.config.PTYSocket)
		s.fifoWatchers[ptyID] = watcher
		s.watchersMu.Unlock()

		response := WSResponse{
			Type: "session_created",
			ID:   ptyID,
		}
		if err := conn.WriteJSON(response); err != nil {
			log.Printf("Failed to send session_created: %v", err)
		}

		sessionID = ptyID
	} else {
		_, exists := s.sessionManager.Get(sessionID)
		if !exists {
			// Session doesn't exist, try to spawn if user has permission
			if !auth.HasPermission(claims.Role, "spawn") {
				log.Printf("Permission denied for spawn (role: %s)", claims.Role)
				sendError(conn, "session not found and permission denied to create")
				return
			}

			ptyID, err := s.ptyClient.Spawn()
			if err != nil {
				log.Printf("Failed to spawn PTY: %v", err)
				sendError(conn, "failed to spawn PTY session")
				return
			}

			s.watchersMu.Lock()
			sess := s.sessionManager.GetOrCreate(ptyID)
			watcher := NewFIFOWatcher(ptyID, sess)
			watcher.Start(s.config.PTYSocket)
			s.fifoWatchers[ptyID] = watcher
			s.watchersMu.Unlock()

			response := WSResponse{
				Type: "session_created",
				ID:   ptyID,
			}
			if err := conn.WriteJSON(response); err != nil {
				log.Printf("Failed to send session_created: %v", err)
			}

			sessionID = ptyID
		} else {
			if !auth.HasPermission(claims.Role, "attach") {
				log.Printf("Permission denied for attach (role: %s)", claims.Role)
				sendError(conn, "permission denied")
				return
			}
		}
	}

	if err := s.sessionManager.AttachClient(sessionID, client); err != nil {
		log.Printf("Failed to attach client: %v", err)
		return
	}
	defer func() {
		s.sessionManager.DetachClient(sessionID, client)

		sess, exists := s.sessionManager.Get(sessionID)
		if exists && sess.ClientCount() == 0 {
			log.Printf("No clients remaining for session %s, cleaning up", sessionID)
			s.watchersMu.Lock()
			if watcher, exists := s.fifoWatchers[sessionID]; exists {
				watcher.Stop()
				delete(s.fifoWatchers, sessionID)
			}
			s.watchersMu.Unlock()

			if err := s.ptyClient.Kill(sessionID); err != nil {
				log.Printf("Failed to kill PTY session %s: %v", sessionID, err)
			}
			s.sessionManager.Cleanup(sessionID)
		} else if exists {
			// Broadcast session_closed to remaining clients
			closeMsg := map[string]interface{}{
				"type": "session_closed",
			}
			closeMsgBytes, _ := json.Marshal(closeMsg)
			sess.Broadcast(closeMsgBytes)
		}
	}()

	go s.writePump(conn, client)
	s.readPump(conn, client, sessionID, claims.Role)
}

// readPump handles reading messages from the WebSocket connection and processing them.
func (s *Server) readPump(conn *websocket.Conn, client *session.Client, sessionID string, role auth.Role) {
	defer func() {
		conn.Close()
	}()

	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetReadLimit(maxMessageSize)
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		var msg WSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle message based on type
		switch msg.Type {
		case "input":
			if !auth.HasPermission(role, "write") {
				sendError(conn, "permission denied")
				continue
			}

			// Handle both string and JSON-encoded string
			var data string
			if len(msg.Data) > 0 {
				// Try to unmarshal as JSON string first
				if err := json.Unmarshal(msg.Data, &data); err != nil {
					// If that fails, treat as raw string
					data = string(msg.Data)
				}
			}

			if err := s.ptyClient.Write(sessionID, data); err != nil {
				log.Printf("Failed to write to PTY: %v", err)
				sendError(conn, "failed to write to PTY")
			}

		case "resize":
			if !auth.HasPermission(role, "resize") {
				sendError(conn, "permission denied")
				continue
			}

			if msg.Cols <= 0 || msg.Rows <= 0 {
				sendError(conn, "invalid resize dimensions")
				continue
			}

			if err := s.ptyClient.Resize(sessionID, msg.Cols, msg.Rows); err != nil {
				log.Printf("Failed to resize PTY: %v", err)
				sendError(conn, "failed to resize PTY")
			}

		case "attach":
			response := WSResponse{
				Type: "attached",
				ID:   sessionID,
			}
			if err := conn.WriteJSON(response); err != nil {
				log.Printf("Failed to send attached: %v", err)
				return
			}

		case "heartbeat":
			response := WSResponse{
				Type: "heartbeat",
			}
			if err := conn.WriteJSON(response); err != nil {
				log.Printf("Failed to send heartbeat: %v", err)
				return
			}

		default:
			log.Printf("Unknown message type: %s", msg.Type)
		}
	}
}

// writePump handles writing messages to the WebSocket connection, including output data and ping messages.
func (s *Server) writePump(conn *websocket.Conn, client *session.Client) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.SendCh:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Send as output message
			response := WSResponse{
				Type: "output",
				Data: string(message),
			}

			if err := conn.WriteJSON(response); err != nil {
				log.Printf("Failed to write message: %v", err)
				return
			}

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func sendError(conn *websocket.Conn, message string) {
	response := map[string]interface{}{
		"type":    "error",
		"message": message,
	}
	conn.WriteJSON(response)
}
