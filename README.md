# Conduit

Conduit is a lightweight web terminal server written in Go. It gives you browser-based access to a local shell session or a remote host over SSH, served through a self-contained web page powered by [xterm.js](https://xtermjs.org/).

This project is intended to be embedded into your application as an internal service, with your own authentication and host resolution logic plugged in via the resolver interface. It is designed to run behind a reverse proxy (e.g. nginx) that handles TLS termination.

<img width="998" height="435" alt="image" src="https://github.com/user-attachments/assets/0b6656ee-7750-49c6-ae8b-f3279657aef9" />

<img width="998" height="237" alt="image" src="https://github.com/user-attachments/assets/2df5a849-b0ce-450f-97a6-dd3e9b299b2f" />

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

- **`-R <host>`** / **`--reset-known-host <host>`** — remove the stored SSH host key fingerprint for the given host from the TOFU known-hosts file and exit.
  Useful after a server's host key has changed (e.g. after a reinstall). The next connection to that host will prompt to trust the new fingerprint.

  ```bash
  conduit -R myserver
  conduit --reset-known-host myserver
  ```

- **`-W`** / **`--write-defaults`** — create `conduit.yaml` and `hosts.yaml` from the embedded defaults when no standard config files are present.
  This is useful for bootstrapping a new installation or for generating example config files in the current working directory.

  ```bash
  conduit -W
  conduit --write-defaults
  ```

## Configuration

Conduit reads configs from `./conduit.yaml` or `/etc/conduit/conduit.yaml`. Both files are merged when present; the local file wins on duplicate keys.

## Resolver

A resolver maps the host identifier from the browser form to session parameters. It is selected in `conduit.yaml` via the `resolver` field. Two are included:

- **File resolver** (`resolver: file`) — reads hosts from `./hosts.yaml` or `/etc/conduit/hosts.yaml`.
- **API resolver** (`resolver: api`) — calls a REST API backend. See [DOCS.md](DOCS.md) and [`api/openapi.yaml`](api/openapi.yaml) for the contract.

## API resolver test server (mockapi)

`cmd/mockapi` is a standalone HTTP server that implements the API resolver protocol using `hosts.yaml` as its data source.

```bash
make run-mockapi
```

## Development

```bash
make test          # run tests
make lint          # run golangci-lint (requires golangci-lint installed)
make clean         # remove binary
make vendor-xterm  # update vendored xterm.js (requires npm)
```
