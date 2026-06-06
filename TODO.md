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

- **Configurable max auth retries** — `RetryableAuthMethod` retry count is hardcoded to `3`.
  Expose it as `ssh.max_auth_retries` in `conduit.yaml`.

- **Concurrent session limit** — No guard against many simultaneous connections to the same
  host. Add an optional `max_sessions` cap (global or per-host).

## Local session

- **Run as a specific user** — Set `cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid, Gid}}`
  to run the shell as a configured user/group without requiring full `sudo`.

- **Graceful shutdown** — `exec.CommandContext` sends `SIGKILL` immediately when the context
  is cancelled (idle timeout, WebSocket close). Send `SIGTERM` first, wait briefly, then
  `SIGKILL` to allow the shell to clean up.

- **Concurrent session limit** — No guard against many simultaneous local sessions.
  Add an optional `max_sessions` cap in `conduit.yaml` under `local`.

## Web server / WebSocket

- **HTTP server timeouts** — `http.Server` in `Start()` has no `ReadTimeout`, `WriteTimeout`,
  `ReadHeaderTimeout`, or `IdleTimeout`. A client that opens a TCP connection and never
  finishes the HTTP request holds a goroutine indefinitely. Expose these under `server.timeouts`
  in `conduit.yaml` with sensible defaults (e.g. `read: 10s`, `write: 0` for streaming,
  `idle: 120s`).

- **TLS / HTTPS** — `Start()` always calls `ListenAndServe` (plain HTTP). Add `tls_cert` and
  `tls_key` fields in `conduit.yaml`; when both are set, call `ListenAndServeTLS` instead.
  Without this, HTTPS requires a reverse proxy in front of conduit.

- **WebSocket handshake timeout** — `websocket.Upgrader` has no `HandshakeTimeout`. A client
  that initiates the upgrade and stalls mid-handshake holds the connection open forever.
  Set a sensible default (e.g. 10 s) and expose it as `server.ws_handshake_timeout`.

- **Rate limiting** — No limit on WebSocket connections per IP. A single client can open
  unlimited concurrent sessions. Add a per-IP connection cap (e.g. using `golang.org/x/time/rate`)
  configurable via `server.max_connections_per_ip`.

- **WebSocket buffer sizes** — `websocket.Upgrader` uses gorilla's default 4096-byte read/write
  buffers. For terminal sessions with dense output, larger buffers (e.g. 32 KB) reduce syscall
  overhead. Expose as `server.ws_read_buffer` and `server.ws_write_buffer`.

- **WebSocket per-message compression** — `EnableCompression` is false by default. Enabling
  `permessage-deflate` can significantly reduce bandwidth on verbose terminal sessions.
  Expose as `server.ws_compression: true`.

- **Token via Authorization header** — `tokenFromCookie` is the only auth token source.
  Add a fallback to the `Authorization: Bearer <token>` header for integrations where
  cookies are not practical.

- **`cols`/`rows` bounds clamping** — `parseUint16` accepts values up to 65535 with no upper
  bound check. Clamp to a reasonable maximum (e.g. 500 cols / 200 rows) to avoid allocating
  oversized PTY buffers.

- **Graceful shutdown timeout** — `Shutdown()` uses a hardcoded 10-second context timeout.
  Expose as `server.shutdown_timeout` in `conduit.yaml`.

## Security

- **Panic recovery leaks stack trace** — `gin.Default()` includes Gin's built-in Recovery
  middleware, which writes the full goroutine stack to the HTTP response body on panic.
  Replace with a custom recovery handler that returns a generic `500 Internal Server Error`
  and logs the stack server-side only.

- **Log injection via `host` parameter** — `host` from the URL path parameter is written
  directly to log output (e.g. `log.Printf("resolver error host=%s …", host, …)`). A crafted
  host value containing `\n`, ANSI escape codes, or log-format characters can forge log
  entries. Sanitize or quote the value before logging (e.g. `%q` instead of `%s`).

- **Plaintext credentials in `hosts.yaml`** — SSH passwords are stored in plaintext. Support
  environment variable interpolation (`password: "${MY_SSH_PASS}"`) so secrets can be
  injected at runtime via environment or a secret manager without being written to disk.

- **`private_key_file` path not validated** — When using the API resolver, the returned
  `private_key_file` path is opened with `os.ReadFile` without checking whether it falls
  within an expected directory. A compromised or misconfigured API endpoint could cause
  conduit to read arbitrary files from the filesystem. Validate that the path is absolute
  and within a configured `ssh.key_dir` before opening it.

