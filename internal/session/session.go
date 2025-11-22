// Package session provides session management for PTY sessions and their attached clients.
package session

import (
	"sync"
)

// Status represents the status of a session.
type Status string

const (
	StatusActive  Status = "active"
	StatusExiting Status = "exiting"
)

// Session represents a PTY session with attached WebSocket clients.
type Session struct {
	ID       string
	Status   Status
	clients  map[*Client]bool
	mu       sync.RWMutex
	outputCh chan []byte
	done     chan struct{}
}

// NewSession creates a new session with the given ID.
func NewSession(id string) *Session {
	return &Session{
		ID:       id,
		Status:   StatusActive,
		clients:  make(map[*Client]bool),
		outputCh: make(chan []byte, 100),
		done:     make(chan struct{}),
	}
}

// AddClient adds a WebSocket client to the session.
func (s *Session) AddClient(client *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[client] = true
}

// RemoveClient removes a WebSocket client from the session.
func (s *Session) RemoveClient(client *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, client)
}

// ClientCount returns the number of attached clients.
func (s *Session) ClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

// Broadcast sends data to all attached clients. If a client's channel is full, that client is skipped.
func (s *Session) Broadcast(data []byte) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for client := range s.clients {
		select {
		case client.SendCh <- data:
		default:
		}
	}
}

// Close closes the session and notifies all clients by closing their send channels.
func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	close(s.done)

	for client := range s.clients {
		close(client.SendCh)
	}
	s.clients = make(map[*Client]bool)
}

// IsDone returns true if the session has been closed.
func (s *Session) IsDone() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

// GetClients returns a copy of all clients for safe iteration.
func (s *Session) GetClients() []*Client {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clients := make([]*Client, 0, len(s.clients))
	for client := range s.clients {
		clients = append(clients, client)
	}
	return clients
}
