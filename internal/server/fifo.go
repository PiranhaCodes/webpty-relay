// Package server provides the HTTP and WebSocket server for the relay service.
package server

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/piranhaCodes/webpty-relay/internal/session"
)

// getFIFOBasePath returns the base path for FIFO files.
// It derives the path from the PTY socket path by using the same directory with "sessions" subdirectory.
func getFIFOBasePath(ptySocketPath string) string {
	// Derive sessions directory from socket directory
	// If socket is ~/.webpty/pty.sock, sessions will be ~/.webpty/sessions
	return filepath.Join(filepath.Dir(ptySocketPath), "sessions")
}

// FIFOWatcher tails a FIFO file and broadcasts output to all clients attached to the session.
type FIFOWatcher struct {
	sessionID string
	session   *session.Session
	done      chan struct{}
	once      sync.Once
}

// NewFIFOWatcher creates a new FIFO watcher for the given session.
func NewFIFOWatcher(sessionID string, sess *session.Session) *FIFOWatcher {
	return &FIFOWatcher{
		sessionID: sessionID,
		session:   sess,
		done:      make(chan struct{}),
	}
}

// Start begins tailing the FIFO file in a goroutine, broadcasting all output to session clients.
// It requires the PTY socket path to determine the correct FIFO base path.
func (fw *FIFOWatcher) Start(ptySocketPath string) {
	fifoBasePath := getFIFOBasePath(ptySocketPath)
	fifoPath := filepath.Join(fifoBasePath, fmt.Sprintf("%s.out", fw.sessionID))

	go func() {
		defer fw.stop()

		// Wait for FIFO to be created (with timeout)
		maxWait := 10
		waited := 0
		for waited < maxWait {
			if _, err := os.Stat(fifoPath); err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
			waited++
		}

		file, err := os.Open(fifoPath)
		if err != nil {
			log.Printf("Failed to open FIFO %s: %v", fifoPath, err)
			return
		}
		defer file.Close()

		log.Printf("Started tailing FIFO for session %s", fw.sessionID)

		// Use buffered reader for raw bytes (not just lines)
		reader := bufio.NewReader(file)
		buf := make([]byte, 4096)

		for {
			if fw.session.IsDone() {
				log.Printf("Session %s closed, stopping FIFO watcher", fw.sessionID)
				return
			}

			n, err := reader.Read(buf)
			if err != nil {
				if err == io.EOF {
					// EOF is normal for FIFOs, continue reading
					time.Sleep(10 * time.Millisecond)
					continue
				}
				log.Printf("Error reading FIFO for session %s: %v", fw.sessionID, err)
				return
			}

			if n > 0 {
				dataCopy := make([]byte, n)
				copy(dataCopy, buf[:n])
				fw.session.Broadcast(dataCopy)
			}
		}
	}()
}

// stop closes the done channel exactly once using sync.Once.
func (fw *FIFOWatcher) stop() {
	fw.once.Do(func() {
		close(fw.done)
	})
}

// Stop stops the FIFO watcher by closing the done channel.
func (fw *FIFOWatcher) Stop() {
	fw.stop()
}
