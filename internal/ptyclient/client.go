package ptyclient

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

const (
	connectTimeout = 5 * time.Second
	readTimeout    = 10 * time.Second
	writeTimeout   = 10 * time.Second
)

// Client handles communication with the PTY backend over a UNIX domain socket.
type Client struct {
	socketPath string
}

// NewClient creates a new PTY client for the given socket path.
func NewClient(socketPath string) *Client {
	return &Client{
		socketPath: socketPath,
	}
}

func (c *Client) connect() (net.Conn, error) {
	dialer := net.Dialer{
		Timeout: connectTimeout,
	}

	conn, err := dialer.Dial("unix", c.socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PTY socket: %w", err)
	}

	return conn, nil
}

func (c *Client) call(req *Request) (*Response, error) {
	conn, err := c.connect()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(readTimeout)); err != nil {
		return nil, fmt.Errorf("failed to set deadline: %w", err)
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(req); err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	decoder := json.NewDecoder(conn)
	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &resp, nil
}

// Spawn creates a new PTY session and returns its session ID.
func (c *Client) Spawn() (string, error) {
	req := &Request{
		Action: "spawn",
		Data:   SpawnRequest{},
	}

	resp, err := c.call(req)
	if err != nil {
		return "", err
	}

	if !resp.OK {
		return "", fmt.Errorf("spawn failed: %s", resp.Err)
	}

	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response data: %w", err)
	}

	var spawnResp SpawnResponse
	if err := json.Unmarshal(dataBytes, &spawnResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal spawn response: %w", err)
	}

	return spawnResp.ID, nil
}

// Write sends data to the stdin of a PTY session.
func (c *Client) Write(sessionID string, data string) error {
	req := &Request{
		Action: "write",
		Data: WriteRequest{
			ID:   sessionID,
			Data: data,
		},
	}

	resp, err := c.call(req)
	if err != nil {
		return err
	}

	if !resp.OK {
		return fmt.Errorf("write failed: %s", resp.Err)
	}

	return nil
}

// Resize changes the terminal size of a PTY session.
func (c *Client) Resize(sessionID string, cols, rows int) error {
	req := &Request{
		Action: "resize",
		Data: ResizeRequest{
			ID:   sessionID,
			Cols: cols,
			Rows: rows,
		},
	}

	resp, err := c.call(req)
	if err != nil {
		return err
	}

	if !resp.OK {
		return fmt.Errorf("resize failed: %s", resp.Err)
	}

	return nil
}

// Kill terminates a PTY session and cleans up all associated resources.
func (c *Client) Kill(sessionID string) error {
	req := &Request{
		Action: "kill",
		Data: KillRequest{
			ID: sessionID,
		},
	}

	resp, err := c.call(req)
	if err != nil {
		return err
	}

	if !resp.OK {
		return fmt.Errorf("kill failed: %s", resp.Err)
	}

	return nil
}

// List returns all active PTY sessions.
func (c *Client) List() (*ListResponse, error) {
	req := &Request{
		Action: "list",
		Data:   ListRequest{},
	}

	resp, err := c.call(req)
	if err != nil {
		return nil, err
	}

	if !resp.OK {
		return nil, fmt.Errorf("list failed: %s", resp.Err)
	}

	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response data: %w", err)
	}

	var listResp ListResponse
	if err := json.Unmarshal(dataBytes, &listResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal list response: %w", err)
	}

	return &listResp, nil
}
