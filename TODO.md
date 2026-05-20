# TODO

## Security

- **`internal/session/ssh/ssh.go`** — Replace `InsecureIgnoreHostKey()` with a stored host key fingerprint check to prevent MITM attacks on SSH connections.

## Server

- **`internal/server/ws.go`** — Validate the WebSocket `Origin` header against an allowlist of permitted hosts.
