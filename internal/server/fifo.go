package server

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/piranhaCodes/webpty-relay/internal/session"
)

const fifoBasePath = "/run/webpty/sessions"

// FIFOWatcher tails a FIFO file and broadcasts output to all clients attached to the session.
type FIFOWatcher struct {
	sessionID string
	session   *session.Session
	done      chan struct{}
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
func (fw *FIFOWatcher) Start() {
	fifoPath := filepath.Join(fifoBasePath, fmt.Sprintf("%s.out", fw.sessionID))

	go func() {
		defer close(fw.done)

		file, err := os.Open(fifoPath)
		if err != nil {
			log.Printf("Failed to open FIFO %s: %v", fifoPath, err)
			return
		}
		defer file.Close()

		log.Printf("Started tailing FIFO for session %s", fw.sessionID)

		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		for scanner.Scan() {
			if fw.session.IsDone() {
				log.Printf("Session %s closed, stopping FIFO watcher", fw.sessionID)
				return
			}

			data := scanner.Bytes()
			dataCopy := make([]byte, len(data))
			copy(dataCopy, data)

			fw.session.Broadcast(dataCopy)
		}

		if err := scanner.Err(); err != nil {
			log.Printf("Error reading FIFO for session %s: %v", fw.sessionID, err)
		}

		log.Printf("FIFO watcher stopped for session %s", fw.sessionID)
	}()
}

// Stop stops the FIFO watcher by closing the done channel.
func (fw *FIFOWatcher) Stop() {
	close(fw.done)
}
