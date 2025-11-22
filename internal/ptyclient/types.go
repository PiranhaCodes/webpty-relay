// Package ptyclient provides a client for communicating with the PTY backend service.
package ptyclient

// Request represents a request to the PTY backend.
type Request struct {
	Action string      `json:"action"`
	Data   interface{} `json:"data"`
}

// Response represents a response from the PTY backend.
type Response struct {
	OK   bool        `json:"ok"`
	Err  string      `json:"err,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

// SpawnRequest contains data for a spawn action.
type SpawnRequest struct{}

// SpawnResponse contains the session ID from a successful spawn response.
type SpawnResponse struct {
	ID string `json:"id"`
}

// WriteRequest contains data for a write action.
type WriteRequest struct {
	ID   string `json:"id"`
	Data string `json:"data"`
}

// ResizeRequest contains data for a resize action.
type ResizeRequest struct {
	ID   string `json:"id"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

// KillRequest contains data for a kill action.
type KillRequest struct {
	ID string `json:"id"`
}

// ListRequest contains data for a list action.
type ListRequest struct{}

// SessionInfo represents a session in the list response.
type SessionInfo struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// ListResponse contains the list of sessions from a successful list response.
type ListResponse struct {
	Sessions []SessionInfo `json:"sessions"`
	Count    int           `json:"count"`
}
