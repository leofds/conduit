# Conduit

Conduit is a lightweight web terminal server written in Go. It gives you browser-based access to a local shell session or a remote host over SSH, served through a self-contained web page powered by [xterm.js](https://xtermjs.org/).

The primary use case is providing interactive console access — to the local machine or to remote hosts via SSH — directly from a browser, with no client-side software required beyond the browser itself.

<img width="776" height="362" alt="image" src="https://github.com/user-attachments/assets/a4cd212d-92b3-462d-a48d-589078065776" />

## Build

```bash
make build        # produces ./dist/conduit
```

## Run

```bash
make run          # copies config files to dist/, then go run
```

Open [http://localhost:8080](http://localhost:8080), enter a host and username, and connect. The password is collected interactively in the terminal.

## Configuration

Conduit reads configs from `./conduit.yaml` or `/etc/conduit/conduit.yaml`. Both files are merged when present; the local file wins on duplicate keys.

```yaml
# Resolver to use: "file" (default) or "api"
resolver: file

# Set to false to prevent local shell sessions
enable_local_shell: true

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
local:
  shell: /bin/bash
  username: ""

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
{ "type": "ssh", "host": "myserver", "user": "admin" }
```
`type` is `"ssh"` for named hosts and `"local"` for local shell requests.  
The `Authorization: Bearer <token>` header is included when the browser sends a `conduit_auth` cookie.

**Response**
```json
{ "type": "ssh", "address": "192.168.1.10", "port": "22", "username": "admin", "password": "" }
{ "type": "local", "shell": "/bin/bash", "username": "admin" }
```

Enable it in `conduit.yaml`:
```yaml
resolver: api
api:
  url: https://my-auth-backend/conduit/resolve
```

## mockapi — API resolver test server

`cmd/mockapi` is a standalone HTTP server that implements the API resolver protocol using `hosts-mockapi.yaml` as its data source. Use it to test the `api` resolver without a real backend.

```bash
make run-mockapi
```

It reads `conduit.yaml` to match the configured port and endpoint. Edit `hosts-mockapi.yaml` to control what the mock returns — it follows the same schema as `hosts.yaml`.

## Development

```bash
make test          # run tests
make lint          # run golangci-lint (requires golangci-lint installed)
make clean         # remove binary
make vendor-xterm  # update vendored xterm.js (requires npm)
```
