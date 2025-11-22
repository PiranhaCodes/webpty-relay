# WebPTY Relay Service

A production-grade relay service that connects web UIs to PTY backend services via WebSocket and HTTP APIs.

## Features

- **WebSocket Support**: Real-time terminal streaming via WebSocket connections
- **Multi-Client Sessions**: Multiple clients can attach to the same terminal session
- **RBAC Authentication**: Role-based access control (admin, write, read)
- **JWT Tokens**: Secure invite token generation for session access
- **Admin API**: Manage sessions, create invites, and terminate sessions
- **FIFO Tailing**: Automatic tailing of PTY output and broadcasting to clients
- **Graceful Shutdown**: Clean resource cleanup on shutdown

## Architecture

```
UI (Browser) <--WebSocket--> Relay Service <--UNIX Socket--> PTY Backend
                              |
                              +--FIFO Tailer--> /run/webpty/sessions/<id>.out
```

## Building

```bash
go build ./cmd/webpty-relay
```

This will create a `webpty-relay` binary in the current directory.

## Configuration

Create `/etc/webpty/config.yml`:

```yaml
relay_port: 7000
pty_socket: /run/webpty/pty.sock
admin_password: "your-secure-password"
jwt_secret: "your-jwt-secret-key"
```

**Defaults:**
- `relay_port`: 7000
- `pty_socket`: /run/webpty/pty.sock
- `admin_password`: "" (empty, admin endpoints disabled)
- `jwt_secret`: "change-me-in-production"

If the config file doesn't exist, defaults will be used.

## Running

```bash
./webpty-relay
```

The service will start on port 7000 (or configured port) and connect to the PTY backend at `/run/webpty/pty.sock`.

## API Endpoints

### Health Check

```bash
curl http://localhost:7000/health
```

### Create Invite Token

```bash
curl -X POST http://localhost:7000/api/admin/invite \
  -H "X-Admin-Password: your-password" \
  -H "Content-Type: application/json" \
  -d '{
    "role": "write",
    "expires_in": 3600
  }'
```

### List Sessions

```bash
curl -X GET http://localhost:7000/api/admin/sessions \
  -H "X-Admin-Password: your-password"
```

### Terminate Session

```bash
curl -X DELETE http://localhost:7000/api/admin/session/<session-id> \
  -H "X-Admin-Password: your-password"
```

## WebSocket Connection

Connect to a terminal session:

```javascript
const ws = new WebSocket('ws://localhost:7000/ws/session/<session-id>?token=<jwt-token>');

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  if (msg.type === 'output') {
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

## Roles & Permissions

### admin
- All permissions
- Can create invite tokens
- Can view all sessions
- Can terminate any session

### write
- Can spawn new sessions
- Can send input to terminal
- Can resize terminal
- Can attach to sessions

### read
- Can attach to sessions (view-only)
- Cannot send input or resize

## Project Structure

```
webpty-relay/
├── cmd/
│   └── webpty-relay/
│       └── main.go              # Entry point
├── internal/
│   ├── auth/                    # Authentication & authorization
│   │   ├── jwt.go               # JWT token generation/validation
│   │   ├── admin.go             # Admin password verification
│   │   └── permissions.go       # RBAC permissions
│   ├── config/                  # Configuration management
│   │   └── config.go            # Config loading from YAML
│   ├── ptyclient/               # PTY backend client
│   │   ├── client.go            # UNIX socket client
│   │   └── types.go             # Request/response types
│   ├── server/                  # HTTP/WebSocket server
│   │   ├── server.go            # Main server logic
│   │   ├── routes.go            # HTTP route handlers
│   │   ├── websocket.go         # WebSocket handlers
│   │   └── fifo.go              # FIFO tailing
│   └── session/                 # Session management
│       ├── manager.go           # Session manager
│       └── session.go           # Session type
└── pkg/
    └── protocol/
        └── relay-protocol.md    # Protocol documentation
```

## Protocol Documentation

See [pkg/protocol/relay-protocol.md](pkg/protocol/relay-protocol.md) for detailed protocol specification.

## Dependencies

- `github.com/golang-jwt/jwt/v5` - JWT token handling
- `github.com/gorilla/websocket` - WebSocket support
- `gopkg.in/yaml.v3` - YAML configuration parsing

## Security Considerations

1. **Change JWT Secret**: Must be changed in production
2. **Strong Admin Password**: Use a strong password
3. **Token Expiry**: Configure appropriate expiry times
4. **TLS/HTTPS**: Use HTTPS/WSS in production
5. **Rate Limiting**: Consider adding rate limiting for production

## Logging

The service logs:
- Server startup/shutdown
- WebSocket connections/disconnections
- Session creation/cleanup
- PTY client errors
- Authentication failures
- Admin actions

## License

See LICENSE file for details.

