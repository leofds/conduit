# TODO

## SSH

- **Passphrase-protected private keys** — `gossh.ParsePrivateKey` fails on encrypted keys. Use
  `gossh.ParsePrivateKeyWithPassphrase` (key stored in config) or prompt the passphrase
  interactively through the browser when the key is encrypted.

- **Jump host / SSH bastion (classic ProxyJump).** Support connecting to a target host through a
  bastion *inside the same SSH stack* (`jump_host: user@bastion:22` per host entry). Dial the
  bastion first (`gossh.Dial`), then open a `direct-tcpip` channel to the final destination
  `host:port` and run a second `gossh.NewClientConn` over that channel. This is self-contained
  in `internal/session/ssh/ssh.go` and needs no new transport.

- **SSH Agent integration** — Read `SSH_AUTH_SOCK` and forward the agent socket so keys
  already loaded in the OS agent are tried automatically, without storing key paths in config.

- **Configurable cipher / MAC / key-exchange algorithms** — The `gossh.ClientConfig.Config`
  field accepts allowlists for ciphers, MACs and kex algorithms. Expose these in
  `conduit.yaml` under `ssh.ciphers`, `ssh.macs`, and `ssh.kex` for security hardening.

- **Multiple private key files** — `SSHConfig.PrivateKeyFile` is a single path. Accept a list
  and try each signer in order before falling back to other auth methods.

- **Concurrent session limit** — No guard against many simultaneous connections to the same
  host. Add an optional `max_sessions` cap (global or per-host).

- **Connection reuse / multiplexing** — Each session opens a fresh `gossh.Dial`. Maintain one
  shared SSH connection per host and open additional channels over it, so the handshake cost
  is paid once. This is also the natural home for the concurrent-session limit above (the cap
  applies to channels on the shared connection).

- **Port forwarding & file transfer** — The SSH stack already has a live connection; expose
  `-L` / `-R` / `-D` port forwarding and basic file transfer (via the SFTP subsystem) so the
  browser can tunnel TCP and move files without a second tool.

- **Interactive auth prompt concurrency bug** — In `internal/session/ssh/ssh.go`, the
  keyboard-interactive / `Password:` prompts write to the same `*websocket.Conn` that the main
  loop reads from, with no synchronization. Gorilla WebSocket connections are not safe for
  concurrent writers, and the prompt readline flow can race with the main read loop. Serialize
  all writes to the connection (e.g. a mutex or a single writer goroutine fed by a channel),
  and make sure the prompt path and the main loop cannot both read from the socket at once.
  This is a correctness/safety issue, not just a feature.

## Local session

- **Concurrent session limit** — No guard against many simultaneous local sessions.
  Add an optional `max_sessions` cap in `conduit.yaml` under `local`.

## Web server / WebSocket

- **TLS / HTTPS** — `Start()` always calls `ListenAndServe` (plain HTTP). Add `tls_cert` and
  `tls_key` fields in `conduit.yaml`; when both are set, call `ListenAndServeTLS` instead.
  Without this, HTTPS requires a reverse proxy in front of conduit.

  Also support a **CA certificate (`tls_ca`)** PEM so conduit can validate *client*
  certificates (mutual TLS). When `tls_ca` is set, load it into a `x509.CertPool`, set
  `tls.Config.ClientCAs` and `ClientAuth: tls.RequireAndVerifyClientCert`, and serve the
  listener with `http.Server{ TLSConfig: ... }` (or `ListenAndServeTLS` with a preloaded
  config). This lets conduit authenticate clients by client cert instead of relying
  solely on the reverse proxy. Config sketch:

  ```yaml
  server:
    tls_cert: /etc/conduit/tls/server.crt
    tls_key:  /etc/conduit/tls/server.key
    tls_ca:   /etc/conduit/tls/ca.crt    # optional; enables mTLS (verify client certs)
  ```

  **Does mTLS apply to the WebSocket?** Yes. The WebSocket upgrade is driven by an ordinary
  HTTPS `GET ... Upgrade: websocket`, and client-cert verification happens in the TLS
  handshake *before* any HTTP request is read — so setting `ClientAuth: tls.RequireAndVerifyClientCert`
  on the `http.Server`'s `TLSConfig` protects the WebSocket connection too, with no change to
  the `websocket.Upgrader` in `internal/server/ws.go`.

  Caveat: **browsers cannot attach a client cert to an in-page `new WebSocket()`** from JS. The
  cert must be in the OS/browser keystore and the user selects it via the browser's native
  dialog during the TLS handshake. So mTLS is reliable for non-browser WebSocket clients
  (CLI/headless/other services) and complements — not replaces — the existing cookie /
  `Authorization: Bearer` token path used by browsers. Document this limitation.

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

- **Session token lifecycle** — The `conduit_session` cookie has no documented expiry or
  rotation. Define a TTL, optionally make tokens single-use, and validate signed tokens so a
  leaked token has a bounded blast radius.

- **Reverse-proxy trust for client IPs** — Per-IP rate limiting depends on a trustworthy
  client IP. Configure trusted proxies and only honor `X-Forwarded-For` / `X-Real-IP` from
  them; otherwise a client can spoof its IP and evade the per-IP cap.

- **Host key storage permissions & atomicity** — Ensure the `knownhosts` file is created
  `0600`, written atomically (temp file + rename), and guarded against concurrent writes from
  parallel sessions, so it can't be corrupted or world-readable.

## Network

- **Allow list** — Add a section in `conduit.yaml` that controls
  which hosts and IP conduit is allowed to connect to. Format:

  ```yaml
  allow_list:
    - method: ssh

## Resolver

- **Unix socket resolver** — Works like the API resolver (`resolver: api`) but instead of
  talking to a REST API over HTTP, it requests the session configuration over a Unix domain
  socket. Reuse the same request/response contract (`/ssh` and `/local` payloads) from the
  API resolver protocol, but dial a Unix socket (e.g. `unix_resolver.socket: /run/conduit/resolve.sock`)
  and speak HTTP/1.1 over it (or a simpler framed protocol). Add a new `resolver: unix` case in
  `cmd/conduit/main.go`, a new `internal/resolver/unixresolver` package, and expose
  `connect_timeout` / `response_timeout` under a `unix` config section in `conduit.yaml`.

- **PostgreSQL resolver** — Integrate with PostgreSQL so host resolution and credentials can be
  served directly from a database instead of a YAML file or an external API. Add a
  `resolver: postgres` backend that connects using a DSN/connection string
  (`postgres.dsn`, or discrete `host`/`port`/`user`/`password`/`database`/`sslmode` fields in
  `conduit.yaml`) and queries a hosts table to build the `resolver.SSHConfig` / `resolver.LocalConfig`
  for the requested host. Include connection pooling, configurable query/statement timeout, and
  optional periodic reload / row-change notifications (e.g. via `LISTEN`/`NOTIFY`).

- **Resolver caching & invalidation** — Every resolver currently hits the backing store on each
  connect (API/SSH/local). Add a short-TTL cache with explicit invalidation, reusing the
  existing SIGHUP reload path for the file resolver and invalidation hooks for the others.

- **Resolver contract tests** — There's an OpenAPI spec and a `mockapi`, but no shared
  conformance suite. Add tests that every backend (file, api, unix, postgres) must pass, so
  the resolver contract stays consistent across implementations.

## Observability

- **Health & readiness endpoints** — Add `/healthz` (liveness) and `/readyz` (readiness,
  e.g. host config loaded / resolver reachable) for container orchestration and load balancers.

- **Metrics** — Expose Prometheus metrics: active sessions, sessions per host, connection
  failures, auth failures, and WebSocket connection counts. Pairs with the rate-limiting and
  concurrent-session-limit work.

- **Structured logging** — Replace `log.Printf` with `log/slog` and add log levels plus
  request/session IDs, so sessions can be traced end-to-end and correlated with the audit log.

## Session recording / audit

- **Session recording** — Terminal I/O is streamed straight to the WebSocket and never
  persisted. Add an optional session recorder (raw bytes, asciinema cast, or at minimum
  metadata: principal, host, start/end time, exit) with a `record` config option and
  redaction rules. Often a hard requirement for compliance when handing out shells/SSH.

## Authentication / authorization

- **Pluggable auth & authorization hook** — Auth is currently delegated to the resolver and
  the reverse proxy. Add a pluggable step that validates the token *before* resolving and
  enforces per-user/per-host access, so conduit is usable without a full reverse-proxy auth
  stack. Relates to the allow list and the `conduit_session` cookie flow.

## Configuration

- **Config validation / fail-fast** — Validate `conduit.yaml` at startup (unknown resolver,
  missing required fields, unparseable durations, nonexistent cert/key files) and exit with a
  clear error instead of failing lazily at connect time.
