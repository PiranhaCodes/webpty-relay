# WebPTY Relay Protocol Specification

## Overview

The WebPTY Relay service acts as a bridge between web clients (UI) and the PTY backend service. It exposes an HTTP/WebSocket server that manages terminal sessions and handles authentication, authorization, and session management.

## Architecture

```
UI (Browser) <--WebSocket--> Relay Service <--UNIX Socket--> PTY Backend
                              |
                              +--FIFO Tailer--> /run/webpty/sessions/<id>.out
```

## Endpoints

### HTTP Endpoints

#### GET /health

Health check endpoint.

**Response:**
```json
{
  "ok": true,
  "data": {
    "status": "healthy"
  }
}
```

#### POST /api/admin/invite

Creates an invite token for accessing terminal sessions.

**Headers:**
- `X-Admin-Password: <password>` OR
- `Authorization: Bearer <admin-token>`

**Request:**
```json
{
  "role": "read" | "write" | "admin",
  "session_id": "<optional-session-id>",
  "expires_in": 86400
}
```

**Response:**
```json
{
  "ok": true,
  "data": {
    "token": "<jwt-token>",
    "url": "/ws/session/<session-id>?token=<jwt-token>"
  }
}
```

**Error Response:**
```json
{
  "ok": false,
  "error": "error message"
}
```

#### GET /api/admin/sessions

Lists all active terminal sessions.

**Headers:**
- `X-Admin-Password: <password>` OR
- `Authorization: Bearer <admin-token>`

**Response:**
```json
{
  "ok": true,
  "data": {
    "sessions": [
      {
        "id": "<session-id>",
        "status": "active" | "exiting",
        "client_count": 2
      }
    ],
    "count": 1
  }
}
```

#### DELETE /api/admin/session/:id

Terminates a terminal session.

**Headers:**
- `X-Admin-Password: <password>` OR
- `Authorization: Bearer <admin-token>`

**Response:**
```json
{
  "ok": true
}
```

### WebSocket Endpoints

#### WS /ws/session/:session-id

WebSocket connection for terminal streaming.

**Query Parameters:**
- `token`: JWT authentication token (required)

**Alternative:** Token can be provided in `Authorization: Bearer <token>` header.

## WebSocket Protocol

### Client → Server Messages

#### Input

Sends input to the PTY terminal.

```json
{
  "type": "input",
  "data": "ls -la\n"
}
```

**Permissions:** `write`, `admin`

#### Resize

Resizes the terminal.

```json
{
  "type": "resize",
  "cols": 80,
  "rows": 24
}
```

**Permissions:** `write`, `admin`

#### Attach

Attaches to an existing session (or creates new if session ID doesn't exist).

```json
{
  "type": "attach"
}
```

**Permissions:** `read`, `write`, `admin`

#### Heartbeat

Keep-alive message.

```json
{
  "type": "heartbeat"
}
```

**Permissions:** All roles

### Server → Client Messages

#### Output

Terminal output data.

```json
{
  "type": "output",
  "data": "raw terminal output bytes"
}
```

#### Session Created

Sent when a new PTY session is created.

```json
{
  "type": "session_created",
  "id": "<session-id>"
}
```

#### Attached

Sent in response to attach message.

```json
{
  "type": "attached",
  "id": "<session-id>"
}
```

#### Heartbeat

Response to client heartbeat.

```json
{
  "type": "heartbeat"
}
```

#### Error

Error message.

```json
{
  "type": "error",
  "data": "error message"
}
```

## Authentication & Authorization

### JWT Token Structure

```json
{
  "role": "admin" | "write" | "read",
  "session_id": "<optional-session-id>",
  "iat": 1234567890,
  "exp": 1234654290
}
```

### Roles & Permissions

#### admin
- All permissions
- Can create invite tokens
- Can view all sessions
- Can terminate any session

#### write
- Can spawn new sessions
- Can send input to terminal
- Can resize terminal
- Can attach to sessions

#### read
- Can attach to sessions (view-only)
- Cannot send input or resize

### Token Generation

Tokens are generated using HS256 algorithm with a server secret key.

Default expiry: 24 hours (configurable via `expires_in` in invite request).

## Session Management

### Session Lifecycle

1. **Creation**: When first client connects to a non-existent session ID, a new PTY session is spawned.

2. **Active**: Multiple clients can attach to the same session. All output is broadcast to all attached clients.

3. **Termination**: 
   - When admin explicitly kills session
   - When last client disconnects (optional, configurable)
   - When PTY process exits

4. **Cleanup**: 
   - FIFO watcher stopped
   - PTY session killed
   - Session removed from manager

### Multi-Client Support

- Multiple WebSocket clients can attach to the same session
- All terminal output is broadcast to all attached clients
- Input from any client with write permissions is forwarded to PTY
- Last client disconnect triggers cleanup (if configured)

## FIFO Tailing

The relay service tails PTY output from FIFO pipes:

- Location: `/run/webpty/sessions/<session-id>.out`
- Each session has a dedicated goroutine that reads from the FIFO
- Output is broadcast to all WebSocket clients attached to that session
- Watcher stops when session is closed

## Configuration

Configuration file: `/etc/webpty/config.yml`

```yaml
relay_port: 7000
pty_socket: /run/webpty/pty.sock
admin_password: "secure-password"
jwt_secret: "change-me-in-production"
```

**Defaults:**
- `relay_port`: 7000
- `pty_socket`: /run/webpty/pty.sock
- `admin_password`: "" (empty, admin endpoints disabled)
- `jwt_secret`: "change-me-in-production"

## Error Handling

All errors follow a consistent format:

```json
{
  "ok": false,
  "error": "error message"
}
```

Common errors:
- `"missing authentication token"`: No token provided
- `"invalid authentication token"`: Token validation failed
- `"permission denied"`: Role doesn't have required permission
- `"session not found"`: Session ID doesn't exist
- `"failed to spawn PTY session"`: PTY backend error
- `"invalid request body"`: Malformed request

## Example Flow

### 1. Admin Creates Invite

```bash
curl -X POST http://localhost:7000/api/admin/invite \
  -H "X-Admin-Password: mypassword" \
  -H "Content-Type: application/json" \
  -d '{"role": "write", "expires_in": 3600}'
```

Response:
```json
{
  "ok": true,
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "url": "/ws/session/<session-id>?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }
}
```

### 2. Client Connects via WebSocket

```javascript
const ws = new WebSocket('ws://localhost:7000/ws/session/new-session?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...');

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  if (msg.type === 'session_created') {
    console.log('Session created:', msg.id);
  } else if (msg.type === 'output') {
    // Display terminal output
    terminal.write(msg.data);
  }
};

// Send input
ws.send(JSON.stringify({
  type: 'input',
  data: 'ls -la\n'
}));

// Resize terminal
ws.send(JSON.stringify({
  type: 'resize',
  cols: 120,
  rows: 40
}));
```

### 3. Admin Views Sessions

```bash
curl -X GET http://localhost:7000/api/admin/sessions \
  -H "X-Admin-Password: mypassword"
```

### 4. Admin Terminates Session

```bash
curl -X DELETE http://localhost:7000/api/admin/session/abc-123-def \
  -H "X-Admin-Password: mypassword"
```

## Implementation Details

### WebSocket Connection Management

- Each WebSocket connection runs in its own goroutine
- Read/write pumps handle message processing
- Ping/pong keep-alive mechanism (60s timeout)
- Graceful connection closure

### FIFO Watcher

- One goroutine per active session
- Uses buffered scanner for efficient reading
- Handles FIFO file creation delays
- Stops automatically when session closes

### Session Manager

- Thread-safe session storage
- Automatic cleanup on last client disconnect
- Supports concurrent client attach/detach

### PTY Client

- JSON-RPC over UNIX domain socket
- Request/response pattern
- Timeout handling (5s connect, 10s read/write)
- Error propagation

## Security Considerations

1. **JWT Secret**: Must be changed in production
2. **Admin Password**: Should be strong and stored securely
3. **Token Expiry**: Configure appropriate expiry times
4. **Origin Checking**: WebSocket origin validation can be configured
5. **Rate Limiting**: Consider adding rate limiting for production
6. **TLS**: Use HTTPS/WSS in production environments

## Logging

The relay service logs:
- Server startup/shutdown
- WebSocket connections/disconnections
- Session creation/cleanup
- PTY client errors
- Authentication failures
- Admin actions

Log format: Standard Go `log` package format with timestamps.

