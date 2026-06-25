# Conduit Doc

This document covers all settings for `conduit.yaml`, the hosts file format, and the API resolver.

# 1. Configuration Reference

---

## 1.1. Global settings

```yaml
debug_banner: true
demo: true
port: 8080
allow_local_shell: true
resolver: file
```

### debug_banner

```yaml
debug_banner: true
```

Default: `true`

When `true`, displays a banner in the terminal before the session starts with session details (method, host, configuration, HTTP headers, environment variables, etc.). Banner content is written over the WebSocket and shown in the xterm.js terminal.

### demo

```yaml
demo: true
```

Default: `true`

When `true`, enables the `/demo` page and redirects `/` to `/demo`. The demo page shows a host selector form for testing. Set to `false` in production if you serve your own UI.

### allow_local_shell

```yaml
allow_local_shell: true
```

Default: `true`

When `true`, local shell sessions (session method `local`) are permitted. When `false`, the resolver must return an SSH configuration or the connection is rejected.

### port

```yaml
port: 8080
```

Default: `8080`

HTTP listen port.

### resolver

```yaml
resolver: file
```

Default: `"file"`

The resolver backend that maps host identifiers to session configurations.

- **`"file"`** — reads from `hosts.yaml`.
- **`"api"`** — calls a REST API backend.

---

## 1.2 WebSocket Allowed Origins

```yaml
allowed_origins:
  - http://localhost:8080
  - https://myapp.example.com
```

Default: not set (allow all origins)

WebSocket Origin allowlist for CSWSH protection. Restricts which web pages may open a WebSocket terminal connection. List the exact Origin URIs (scheme + host + port).

- **Not set** (`allowed_origins` omitted) — all origins are allowed. Suitable behind a reverse proxy that already enforces access control.
- **Empty list** (`allowed_origins: []`) — all origins are blocked. Use this to enforce that connections only come from the same page Conduit serves.
- **List with values** — only origins in the list are allowed.

---

## 1.3 Headers

```yaml
headers:
  X-Content-Type-Options: nosniff
  X-Frame-Options: DENY
  Content-Security-Policy: "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; script-src 'self' 'unsafe-inline'; connect-src 'self' ws: wss:; base-uri 'none'; frame-ancestors 'none'"
  Referrer-Policy: no-referrer
```

Default: empty (no custom headers). Omit `headers` or set to `{}`.

The `headers` map sets arbitrary HTTP response headers on every response. Useful for security headers and CORS.

### Common security headers

**`X-Content-Type-Options: nosniff`**
Prevents the browser from MIME-sniffing the response content type. Without this, an attacker could disguise an HTML file as a plain-text upload and have it rendered as a web page, enabling XSS.

**`X-Frame-Options: DENY`**
Prevents the page from being rendered inside an `<iframe>`. Protects against clickjacking.

**`Content-Security-Policy`**
The most powerful XSS defense. Restricts which resources (scripts, styles, images, fonts, etc.) the browser is allowed to load.

The recommended policy for Conduit:

```
default-src 'self';
style-src 'self' 'unsafe-inline';
img-src 'self' data:;
font-src 'self';
script-src 'self' 'unsafe-inline';
connect-src 'self' ws: wss:;
base-uri 'none';
frame-ancestors 'none'
```

- `'unsafe-inline'` on `script-src` and `style-src` is required by xterm.js (inline `<script>` and `<style>` tags).
- `connect-src` must include `ws:` and `wss:` for the WebSocket connection.
- `frame-ancestors 'none'` prevents embedding in an iframe (clickjacking).

**`Referrer-Policy: no-referrer`**
Controls what information is sent in the `Referer` header when navigating to another site. `no-referrer` sends nothing at all.

---

## 1.4 Terminal

```yaml
terminal_options:
  scrollback: 5000
  theme:
    background: '#1e1e1e'
    foreground: '#d4d4d4'
```

Default:
```json
{ "scrollback": 5000, "theme": { "background": "#1e1e1e", "foreground": "#d4d4d4" } }
```

The `terminal_options` map is passed as JSON to the `new Terminal()` constructor in xterm.js. Any option from the [ITerminalOptions interface](https://xtermjs.org/docs/api/terminal/interfaces/iterminaloptions/) is supported.

### Common options

- **`scrollback`** (number) — maximum lines in the scrollback buffer. Default `5000`.
- **`cursorBlink`** (bool) — whether the cursor blinks. Default `false`.
- **`cursorStyle`** (string) — `'block'`, `'underline'`, or `'bar'`. Default `'block'`.
- **`fontSize`** (number) — font size in pixels.
- **`fontFamily`** (string) — CSS `font-family`.
- **`theme.background`** (string) — terminal background color. Default `'#1e1e1e'`.
- **`theme.foreground`** (string) — terminal text color. Default `'#d4d4d4'`.

Any other [xterm.js theme key](https://xtermjs.org/docs/api/terminal/interfaces/iterminaloptions/#theme) is supported.

---

## 1.5 Server Config

```yaml
server:
  timeouts:
    read: 10s
    write: 0s
    read_header: 10s
    idle: 120s
    ws_handshake: 10s
```

### timeouts

Default values:
- `read: 10s`
- `write: 0s` (disabled / no write timeout)
- `read_header: 10s`
- `idle: 120s`
- `ws_handshake: 10s`

These values are applied to the embedded HTTP server created by Conduit. They control how long the server waits for request reads, response writes, header reads, and idle connections before closing them.

---

## 1.6 Local session

```yaml
local:
  command: "/bin/bash"
  term: xterm-256color
  idle_timeout: 10m
  working_dir: ""
  env:
    LANG: en_US.UTF-8
```

Settings for local shell sessions.

### `command`

Default: `"/bin/bash"`

The program launched for local shell sessions. The value is split on whitespace so arguments are supported.

**Direct shell** (simplest, no login prompt):
```yaml
local:
  command: "/bin/bash"
```

**Login shell on Linux** (shows a login prompt):
```yaml
local:
  command: "sudo -n /bin/login"
```
Requires the conduit process user to have passwordless sudo access to `/bin/login`:
```bash
echo "conduit ALL=(root) NOPASSWD: /bin/login" | sudo tee /etc/sudoers.d/conduit
```

**Login shell on macOS** (shows a login prompt):
```yaml
local:
  command: "sudo -n /usr/bin/login -p"
```
Requires passwordless sudo access to `/usr/bin/login`:
```bash
echo "conduit ALL=(root) NOPASSWD: /usr/bin/login" | sudo tee /etc/sudoers.d/conduit
```

### `term`

Default: `"xterm-256color"`

Terminal type (TERM environment variable) reported to the local shell.

### `idle_timeout`

Default: `"10m"`

Close the session after this period of inactivity (no keyboard input). Set to `0` to disable.

### `working_dir`

Default: `""` (inherit conduit's working directory)

Working directory for local shell sessions. Empty means the shell starts in conduit's working directory (the binary's directory).

### `env`

Default: empty

Additional environment variables injected into every local shell session.

```yaml
local:
  env:
    LANG: en_US.UTF-8
    TZ: UTC
    PATH: /usr/local/bin:/usr/bin:/bin
```

---

## 1.7 SSH session

```yaml
ssh:
  port: "22"
  term: xterm-256color
  idle_timeout: 10m
  keepalive_interval: 30s
  dial_timeout: 10s
  verify_host_key: true
  auto_accept_host_key: false
  known_hosts_file: ./known_hosts.yaml
  env:
    LANG: en_US.UTF-8
```

Settings for SSH sessions. Each setting can be overridden per host in `hosts.yaml` or returned by the API resolver, unless noted otherwise.

### `port`

Default: `"22"`

Default TCP port for SSH connections.

### `term`

Default: `"xterm-256color"`

Terminal type requested in the SSH PTY.

### `idle_timeout`

Default: `"10m"`

Close the SSH session after this period of inactivity (no keyboard input). Set to `0` to disable.

### `keepalive_interval`

Default: `"30s"`

How often to send a keepalive probe to the SSH server. Set to `0` to disable keepalives.

### `dial_timeout`

Default: `"10s"`

Timeout for the TCP connection + SSH handshake. Set to `0` for no timeout. *This setting is global only, not overridable per host.*

### `verify_host_key`

Default: `true`

Enable TOFU (Trust On First Use) host key verification for all SSH connections. When `true`, the server's host key fingerprint is verified on every connection:
- **First connection** — the user is prompted in the terminal to confirm the fingerprint.
- **Subsequent connections** — the fingerprint is compared against the saved value. A mismatch aborts the connection.

When `false` or omitted, the host key is not checked (equivalent to `StrictHostKeyChecking no`).

**Resetting a stored fingerprint**

If a server's host key changes (e.g. after a reinstall), Conduit will refuse the connection due to a fingerprint mismatch. Remove the stored entry with:

```bash
conduit -R <host>
```

The next connection to that host will prompt to trust the new fingerprint.

### `auto_accept_host_key`

Default: `false`

Skip the interactive fingerprint prompt and auto-accept unknown host keys. When `true`, the first connection to a host silently trusts and persists the fingerprint without asking the user.

### `known_hosts_file`

Default: `"./known_hosts.yaml"`

Path to the YAML file where TOFU host key fingerprints are persisted. The file is created automatically. *This setting is global only.*

### `env`

Default: empty

Environment variables forwarded to the SSH server via `Setenv` before the shell starts. The SSH server must have `AcceptEnv` configured for these variables to be accepted. Rejected variables are logged and (when `debug_banner: true`) shown in red in the terminal.

---

## 1.8 API resolver

```yaml
api:
  url: http://localhost:8040/conduit/resolve
  connect_timeout: 5s
  response_timeout: 10s
```

Settings for the API resolver. Only used when `resolver: api`.

### `url`

Default: none (required)

Base URL for the resolver API. Conduit appends `/ssh` and `/local` to this URL.

### `connect_timeout`

Default: `"5s"`

Timeout for establishing the TCP connection to the resolver API.

### `response_timeout`

Default: `"10s"`

Timeout for the full HTTP request/response cycle with the resolver API.

---

# 2. Hosts file (`hosts.yaml`)

Used by the file resolver (`resolver: file`). Read from `./hosts.yaml` or `/etc/conduit/hosts.yaml`. Both files are merged; the local file wins on duplicate keys.

### Format

```yaml
hosts:
  myserver:
    address: 192.168.1.10
    username: admin
    password: ""
    port: 22
    term: xterm-256color
    verify_host_key: true
    auto_accept_host_key: false
    idle_timeout: 10m
    keepalive_interval: 30s
    env:
      LANG: en_US.UTF-8
      TZ: UTC
```

### Fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `address` | string | yes | — | SSH hostname or IP address |
| `username` | string | yes | — | SSH login username |
| `password` | string | no | `""` | SSH password. Leave empty for interactive/keyboard-interactive auth |
| `port` | string | no | `ssh.port` (22) | TCP port |
| `private_key_file` | string | no | `""` | Path to PEM private key (e.g. `~/.ssh/id_rsa`) |
| `term` | string | no | `ssh.term` | Terminal type override |
| `verify_host_key` | bool | no | `ssh.verify_host_key` | Host key verification override |
| `auto_accept_host_key` | bool | no | `ssh.auto_accept_host_key` | Auto-accept fingerprint override |
| `idle_timeout` | duration | no | `ssh.idle_timeout` | Idle timeout override |
| `keepalive_interval` | duration | no | `ssh.keepalive_interval` | Keepalive interval override |
| `env` | map\[string\]string | no | `ssh.env` | Env vars merged with `ssh.env`; host values override |

---

# 3. API resolver protocol

On each connection attempt Conduit POSTs a JSON request to one of two endpoints derived from the configured `api.url`. The `Authorization: Bearer <token>` header is included when the browser sends a `conduit_session` cookie.

### SSH endpoint — `POST <url>/ssh`

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
  "auto_accept_host_key": false,
  "idle_timeout": "10m",
  "keepalive_interval": "30s",
  "env": {
    "LANG": "en_US.UTF-8"
  }
}
```

All fields except `address` and `username` are optional — omit any to fall back to the global setting in `conduit.yaml`.

### Local endpoint — `POST <url>/local`

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

`term`, `working_dir`, `idle_timeout`, and `env` are optional — omit any to fall back to the corresponding `local.*` setting in `conduit.yaml`. Local shell sessions also inherit any `local.env` values.

The full contract is described in the OpenAPI specification at [`api/openapi.yaml`](../api/openapi.yaml).
