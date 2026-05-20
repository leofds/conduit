# Conduit

Conduit is a lightweight web terminal server written in Go. It gives you browser-based access to a local shell session or a remote host over SSH, served through a self-contained web page powered by [xterm.js](https://xtermjs.org/).

This project is intended to be embedded into your application as an internal service, with your own authentication and host resolution logic plugged in via the resolver interface.

<img width="998" height="435" alt="image" src="https://github.com/user-attachments/assets/0b6656ee-7750-49c6-ae8b-f3279657aef9" />

<img width="998" height="237" alt="image" src="https://github.com/user-attachments/assets/2df5a849-b0ce-450f-97a6-dd3e9b299b2f" />


## Build

```bash
make build        # produces ./dist/conduit
```

## Run

```bash
make run          # copies config files to dist/, then go run
```

## How to Use

Open [http://localhost:8080/demo](http://localhost:8080/demo) in your browser. You will see a form with one field:

- **Host** — the name of a device defined in `hosts.yaml`. Conduit uses this name to look up the address and credentials server-side, so they are never sent to or exposed in the browser.

The demo page also has a hardcoded JWT token (payload `{"sub":"1234567890","name":"John Doe","admin":true}`, signed with secret `conduit`) that is automatically sent as a `conduit_session` cookie for testing against `mockapi`.

Leave the host field empty to open a local shell session on the machine running Conduit.

Once connected, the terminal opens at `/terminal` and the session runs until you close the tab or the idle timeout is reached.

> Credentials (address, username, password) are stored only in `hosts.yaml` on the server. The browser only ever sends the device name — never the actual credentials.

## Configuration

Conduit reads configs from `./conduit.yaml` or `/etc/conduit/conduit.yaml`. Both files are merged when present; the local file wins on duplicate keys.

```yaml
# Resolver to use: "file" (default) or "api"
resolver: file

# HTTP listen port
port: 8080

# Terminal type for all sessions (local and SSH)
term: xterm-256color

# Demo page
demo: true

# Local shell session
local:
  enable: true
  # command: "/bin/bash"  # defaults to /bin/login when not set

# API resolver settings (only used when resolver: api)
api:
  url: http://localhost:8080/conduit/resolve
  connect_timeout: 5s
  response_timeout: 10s
```

## Resolvers

A resolver maps the host identifier and username from the browser form to the session parameters Conduit needs to open the connection. Two resolvers are included:

### File resolver (default)

Reads hosts from `./hosts.yaml` or `/etc/conduit/hosts.yaml`. Both files are merged; the local file wins.

```yaml
hosts:
  myserver:
    address: 192.168.1.10
    port: 22
    username: admin
    password: ""
```

Set `resolver: file` in `conduit.yaml` (or omit it — file is the default).

### API resolver

On each connection attempt Conduit POSTs a JSON request to the configured URL and uses the response to open the session.

**Request**
```json
{ "type": "ssh", "host": "myserver" }
```
`type` is `"ssh"` for named hosts and `"local"` for local shell requests.  
The `Authorization: Bearer <token>` header is included when the browser sends a `conduit_session` cookie.

**Response**
```json
{ "type": "ssh", "address": "192.168.1.10", "port": "22", "username": "admin", "password": "" }
{ "type": "local", "command": "/bin/bash" }
```

Enable it in `conduit.yaml`:
```yaml
resolver: api
api:
  url: https://my-auth-backend/conduit/resolve
```

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
