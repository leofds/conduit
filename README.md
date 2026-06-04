# Conduit

Conduit is a lightweight web terminal server written in Go. It gives you browser-based access to a local shell session or a remote host over SSH, served through a self-contained web page powered by [xterm.js](https://xtermjs.org/).

This project is intended to be embedded into your application as an internal service, with your own authentication and host resolution logic plugged in via the resolver interface. It is designed to run behind a reverse proxy (e.g. nginx) that handles TLS termination.

<img width="998" height="435" alt="image" src="https://github.com/user-attachments/assets/0b6656ee-7750-49c6-ae8b-f3279657aef9" />

<img width="998" height="237" alt="image" src="https://github.com/user-attachments/assets/2df5a849-b0ce-450f-97a6-dd3e9b299b2f" />

## Getting Started

1\. Build and Run the project by running:

```bash
make run
```
It produces `./dist/conduit`, `./dist/conduit.yaml` and `./dist/hosts.yaml`.

2\. Open [http://localhost:8080/demo](http://localhost:8080/demo) in your browser. You will see a form with one field:

  - **Host** the name of a device defined in `hosts.yaml`. Conduit uses this name to look up the address and credentials server-side, so they are never sent to or exposed in the browser.
    Leave this field empty to open a local shell session on the machine running Conduit. See [Local shell](#local-shell) for setup options.

3\. Click on `Connect` to open the terminal.

Once connected, the terminal opens at `/terminal` and the session runs until you close the tab or the idle timeout is reached.

The demo page also has a hardcoded JWT token (payload `{"sub":"1234567890","name":"John Doe","admin":true}`, signed with secret `conduit`) that is automatically sent as a `conduit_session` cookie for testing with the `api` resolver.

### Command-line flags

- **`-R <host>`** — remove the stored SSH host key fingerprint for the given host from the TOFU known-hosts file and exit.
  Useful after a server's host key has changed (e.g. after a reinstall). The next connection to that host will prompt to trust the new fingerprint.

  ```bash
  conduit -R myserver
  ```

## Features

- **Browser-based terminal** - full xterm.js terminal served over WebSocket, no client software required
- **SSH sessions** - connect to remote hosts with password, private key, or interactive keyboard-interactive auth
- **Local shell sessions** - spawn a local shell or login prompt directly in the browser
- **Pluggable resolver** - map host identifiers to credentials via a YAML file or your own REST API backend
- **TOFU host key verification** - Trust on First Use fingerprint checking with an interactive confirmation prompt; persisted to a local YAML store
- **Per-host settings** - security and timing options can be overridden per host in `hosts.yaml` or returned by the API resolver: whether to verify the host key, whether to auto-accept unknown keys on first use, the inactivity timeout, and the SSH keepalive interval
- **Debug banner** - optional banner with session details displayed in the terminal before the session starts
- **Idle timeout & keepalive** - configurable inactivity timeout and SSH keepalive probes
- **Origin allowlist** - restrict which pages may open a WebSocket terminal (CSWSH protection)
- **OpenAPI spec** - machine-readable contract for the API resolver at [`api/openapi.yaml`](api/openapi.yaml)

## Configuration

Conduit reads configs from `./conduit.yaml` or `/etc/conduit/conduit.yaml`. Both files are merged when present; the local file wins on duplicate keys.

```yaml
# Show a debug banner with session details before the session starts
debug_banner: false

# Demo page
demo: true

# Allow local shell sessions
allow_local_shell: true

# HTTP listen port
port: 8080

# WebSocket origin allowlist (CSWSH protection).
# List exact Origins (scheme + host + port) allowed to open a terminal.
# Empty or omitted = allow all origins (default).
# allowed_origins:
#   - http://localhost:8080
#   - https://myapp.example.com

# Local shell session
local:
  # Direct shell (no login prompt):
  command: "/bin/bash"
  # Login on Linux — requires passwordless sudo for /bin/login:
  #command: "sudo -n /bin/login"
  # Login on macOS — requires passwordless sudo for /usr/bin/login:
  #command: "sudo /usr/bin/login -p"
  term: xterm-256color  # terminal type reported to the local shell
  idle_timeout: 10m     # close session after inactivity, 0 disables
  working_dir: ""       # empty = inherit conduit's working directory
  env:
    LANG: en_US.UTF-8
    #TZ: UTC

# SSH session
ssh:
  port: "22"               # SSH port
  term: xterm-256color     # terminal type requested in the SSH PTY
  idle_timeout: 10m        # close session after this period of inactivity, 0 disables the timeout
  keepalive_interval: 30s  # how often to send a keepalive probe to the server, 0 disables keepalives
  dial_timeout: 10s        # TCP+SSH handshake timeout, 0 means no timeout
  verify_host_key: true    # enable TOFU host key verification for all SSH connections
  tofu_auto_accept: false  # skip the interactive fingerprint prompt and auto-accept unknown host keys
  known_hosts_file: ./known_hosts.yaml  # TOFU host-key fingerprint store
  # env defines environment variables forwarded to the SSH server via Setenv.
  # The SSH server must allow them with AcceptEnv.
  env:
    LANG: en_US.UTF-8

# Resolver to use: "file" (default) or "api"
resolver: file

# API resolver settings (only used when resolver: api)
api:
  url: http://localhost:8040/conduit/resolve
  connect_timeout: 5s
  response_timeout: 10s
```

### Local shell

The `command` field controls what program is launched for local sessions. The value is split on whitespace, so arguments are supported (e.g. `"sudo -n /bin/login"` becomes `["sudo", "-n", "/bin/login"]`).

- **`/bin/bash`** (default): opens a shell directly as the conduit process user, no login prompt. Simplest option for single-user or testing setups.

- **`sudo -n /bin/login`** (Linux): shows a login prompt. Requires the conduit process user to have passwordless sudo access to `/bin/login`. As root, run:
  ```bash
  echo "conduit ALL=(root) NOPASSWD: /bin/login" | sudo tee /etc/sudoers.d/conduit
  ```
  Replace `conduit` with the user running the conduit process.

- **`sudo /usr/bin/login -p`** (macOS): shows a login prompt. Requires passwordless sudo access to `/usr/bin/login`. As root, run:
  ```bash
  echo "conduit ALL=(root) NOPASSWD: /usr/bin/login" | sudo tee /etc/sudoers.d/conduit
  ```

- **`env`**: additional environment variables injected into every local shell session, such as `LANG`, `TZ`, or a custom `PATH`.

## Resolvers

A resolver maps the host identifier from the browser form to the session parameters Conduit needs to open the connection. Two resolvers are included:

### File resolver (default)

Reads hosts from `./hosts.yaml` or `/etc/conduit/hosts.yaml`. Both files are merged; the local file wins.

```yaml
hosts:
  myserver:
    address: 192.168.1.10
    username: admin
    # Optional fields below (omit when not needed):
    password: ""              # used when private_key_file is not set
    port: 22                  # defaults to 22 when omitted
    private_key_file: ""      # path to PEM private key (e.g. /home/user/.ssh/id_rsa)
    term: xterm-256color      # override ssh.term for this host only
    verify_host_key: true     # override the global verify_host_key for this host only
    tofu_auto_accept: false   # override the global tofu_auto_accept for this host only
    idle_timeout: 10m         # override the global idle_timeout for this host only
    keepalive_interval: 30s   # override the global keepalive_interval for this host only
    env:                      # environment variables; forwarded to the SSH server via Setenv
      LANG: en_US.UTF-8
      TZ: UTC
```

Set `resolver: file` in `conduit.yaml` (or omit it — file is the default).

### API resolver

On each connection attempt Conduit POSTs a JSON request to one of two endpoints derived from the configured base URL and uses the response to open the session.

**SSH endpoint** — `POST <url>/ssh`

Request:
```json
{ "host": "myserver" }
```
Response:
```json
{
  "address": "192.168.1.10",
  "port": "22",
  "username": "admin",
  "password": "",
  "private_key_file": "",
  "term": "xterm-256color",
  "verify_host_key": true,
  "tofu_auto_accept": false,
  "idle_timeout": "10m",
  "keepalive_interval": "30s",
  "env": {
    "LANG": "en_US.UTF-8"
  }
}
```

All fields except `address` and `username` are optional — omit any to fall back to the global setting in `conduit.yaml`.

`env` values are forwarded to the SSH server via `Setenv` before the shell starts. The SSH server must have `AcceptEnv` configured for the variables to be accepted.

**Local endpoint** — `POST <url>/local`

Request: *(no body)*
Response:
```json
{
  "command": "/bin/bash",
  "term": "xterm-256color",
  "working_dir": "/home/user",
  "idle_timeout": "10m",
  "env": {
    "LANG": "en_US.UTF-8"
  }
}
```

`term`, `working_dir`, `idle_timeout`, and `env` are optional — omit any to fall back to the corresponding `local.term` / `local.working_dir` / `local.idle_timeout` / `local.env` setting in `conduit.yaml`.

Local shell sessions also inherit any `local.env` values configured in `conduit.yaml`.

The `Authorization: Bearer <token>` header is included on both requests when the browser sends a `conduit_session` cookie.

Enable it in `conduit.yaml`:
```yaml
resolver: api
api:
  url: https://my-auth-backend/conduit/resolve
```

The full contract is described in the OpenAPI specification at [`api/openapi.yaml`](api/openapi.yaml).

## TOFU host key verification

When `verify_host_key: true` is set in `conduit.yaml`, Conduit verifies the server's SSH host key using Trust on First Use (TOFU) semantics for all SSH connections:

- **First connection** — the user is prompted in the terminal to confirm the server's SHA-256 fingerprint (similar to OpenSSH's `Are you sure you want to continue connecting?`). On confirmation, the fingerprint is saved to the file defined by `ssh.known_hosts_file` (default `./known_hosts.yaml`).
- **Subsequent connections** — the fingerprint is compared against the saved value. A mismatch aborts the connection and prints an error in the terminal.

The known-hosts file is created automatically and uses a simple YAML format:

```yaml
hosts:
  myserver: SHA256:abc123...
```

### Auto-accept

The interactive prompt can be skipped by setting `tofu_auto_accept: true`. Conduit will then silently trust and persist any unknown fingerprint without asking the user:

```yaml
ssh:
  verify_host_key: true
  tofu_auto_accept: false  # default — user must confirm on first connection
```

This can also be overridden per host in `hosts.yaml` (file resolver) or returned in the API resolver response, independently of the global setting:

```yaml
hosts:
  my-dev-vm:
    address: 192.168.1.10
    username: admin
    tofu_auto_accept: true  # skip the prompt for this host only
```

> **Note:** enabling `tofu_auto_accept` is convenient for trusted internal hosts but removes the protection against a changed or spoofed host key on first use.

When `verify_host_key` is omitted or `false`, the host key is not checked at all (equivalent to `StrictHostKeyChecking no`).

### Resetting a stored fingerprint

If a server's host key changes (e.g. after a reinstall), Conduit will refuse the connection due to a fingerprint mismatch. Remove the stored entry with:

```bash
conduit -R <host>
```

The next connection to that host will prompt to trust the new fingerprint.

## mockapi — API resolver test server

`cmd/mockapi` is a standalone HTTP server that implements the API resolver protocol using `hosts.yaml` as its data source. Use it to test the `api` resolver without a real backend.

```bash
make run-mockapi
```

It reads `conduit.yaml` to match the configured port and endpoint. Edit `hosts.yaml` to control what the mock returns — it follows the same schema as `hosts.yaml`.

## Development

```bash
make test          # run tests
make lint          # run golangci-lint (requires golangci-lint installed)
make clean         # remove binary
make vendor-xterm  # update vendored xterm.js (requires npm)
```
