# TODO

## SSH

- **Passphrase-protected private keys** — `gossh.ParsePrivateKey` fails on encrypted keys. Use
  `gossh.ParsePrivateKeyWithPassphrase` (key stored in config) or prompt the passphrase
  interactively through the browser when the key is encrypted.

- **Jump host / ProxyJump** — Support connecting through a bastion host
  (`jump_host: user@bastion:22` per host entry). Dial the jump host first, then open a
  channel to the final destination over that connection.

- **SSH Agent integration** — Read `SSH_AUTH_SOCK` and forward the agent socket so keys
  already loaded in the OS agent are tried automatically, without storing key paths in config.

- **Configurable cipher / MAC / key-exchange algorithms** — The `gossh.ClientConfig.Config`
  field accepts allowlists for ciphers, MACs and kex algorithms. Expose these in
  `conduit.yaml` under `ssh.ciphers`, `ssh.macs`, and `ssh.kex` for security hardening.

- **Multiple private key files** — `SSHConfig.PrivateKeyFile` is a single path. Accept a list
  and try each signer in order before falling back to other auth methods.

- **Concurrent session limit** — No guard against many simultaneous connections to the same
  host. Add an optional `max_sessions` cap (global or per-host).

## Local session

- **Concurrent session limit** — No guard against many simultaneous local sessions.
  Add an optional `max_sessions` cap in `conduit.yaml` under `local`.

## Web server / WebSocket

- **TLS / HTTPS** — `Start()` always calls `ListenAndServe` (plain HTTP). Add `tls_cert` and
  `tls_key` fields in `conduit.yaml`; when both are set, call `ListenAndServeTLS` instead.
  Without this, HTTPS requires a reverse proxy in front of conduit.

- **Rate limiting** — No limit on WebSocket connections per IP. A single client can open
  unlimited concurrent sessions. Add a per-IP connection cap (e.g. using `golang.org/x/time/rate`)
  configurable via `server.max_connections_per_ip`.

- **WebSocket buffer sizes** — `websocket.Upgrader` uses gorilla's default 4096-byte read/write
  buffers. For terminal sessions with dense output, larger buffers (e.g. 32 KB) reduce syscall
  overhead. Expose as `server.ws_read_buffer` and `server.ws_write_buffer`.

- **WebSocket per-message compression** — `EnableCompression` is false by default. Enabling
  `permessage-deflate` can significantly reduce bandwidth on verbose terminal sessions.
  Expose as `server.ws_compression: true`.

- **Graceful shutdown timeout** — `Shutdown()` uses a hardcoded 10-second context timeout.
  Expose as `server.shutdown_timeout` in `conduit.yaml`.

## Security

- **Panic recovery leaks stack trace** — `gin.Default()` includes Gin's built-in Recovery
  middleware, which writes the full goroutine stack to the HTTP response body on panic.
  Replace with a custom recovery handler that returns a generic `500 Internal Server Error`
  and logs the stack server-side only.

- **Plaintext credentials in `hosts.yaml`** — SSH passwords are stored in plaintext. Support
  environment variable interpolation (`password: "${MY_SSH_PASS}"`) so secrets can be
  injected at runtime via environment or a secret manager without being written to disk.
