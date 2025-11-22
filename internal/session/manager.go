package session

import (
	"fmt"
	"log"
	"sync"
)

// Client represents a WebSocket client connection attached to a session.
type Client struct {
	SendCh chan []byte
}

// Manager manages all PTY sessions in a thread-safe manner.
type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewManager creates a new session manager.
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

// GetOrCreate returns an existing session or creates a new one if it doesn't exist.
func (m *Manager) GetOrCreate(sessionID string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists := m.sessions[sessionID]; exists {
		return session
	}

	session := NewSession(sessionID)
	m.sessions[sessionID] = session
	log.Printf("Created new session: %s", sessionID)
	return session
}

// Get retrieves a session by ID, returning the session and whether it exists.
func (m *Manager) Get(sessionID string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	return session, exists
}

// Remove removes a session from the manager and closes it.
func (m *Manager) Remove(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists := m.sessions[sessionID]; exists {
		session.Close()
		delete(m.sessions, sessionID)
		log.Printf("Removed session: %s", sessionID)
	}
}

// List returns all active sessions.
func (m *Manager) List() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// AttachClient attaches a client to a session, creating the session if it doesn't exist.
func (m *Manager) AttachClient(sessionID string, client *Client) error {
	session := m.GetOrCreate(sessionID)
	session.AddClient(client)
	log.Printf("Client attached to session %s (total clients: %d)", sessionID, session.ClientCount())
	return nil
}

// DetachClient detaches a client from a session.
func (m *Manager) DetachClient(sessionID string, client *Client) error {
	session, exists := m.Get(sessionID)
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.RemoveClient(client)
	log.Printf("Client detached from session %s (remaining clients: %d)", sessionID, session.ClientCount())

	if session.ClientCount() == 0 {
		log.Printf("No clients remaining for session %s, marking for cleanup", sessionID)
	}

	return nil
}

// Cleanup removes a session and closes it.
func (m *Manager) Cleanup(sessionID string) {
	m.Remove(sessionID)
}
