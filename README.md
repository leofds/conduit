# Conduit

Conduit is a lightweight web terminal server written in Go. It gives you browser-based access to a local shell session or a remote host over SSH, served through a self-contained web page powered by [xterm.js](https://xtermjs.org/).

The primary use case is providing interactive console access — to the local machine or to remote hosts via SSH — directly from a browser, with no client-side software required beyond the browser itself.

<img width="776" height="362" alt="image" src="https://github.com/user-attachments/assets/a4cd212d-92b3-462d-a48d-589078065776" />

## Build

Requirements

- Go 1.21+

```bash
make build
```

The binary is written to `./dist/conduit`.

## Run

```bash
./conduit
# or
make run
```

Open [http://localhost:8080](http://localhost:8080) in your browser, enter a host and username, and connect. The password is collected interactively in the terminal.

## Development

```bash
make test       # run tests
make lint       # run golangci-lint (requires golangci-lint installed)
make clean      # remove binary
```

To update the vendored xterm.js (requires npm):

```bash
make vendor-xterm
```

## Third-party

| Dependency | License |
|---|---|
| [xterm.js](https://github.com/xtermjs/xterm.js) | MIT |
| [gin](https://github.com/gin-gonic/gin) | MIT |
| [gorilla/websocket](https://github.com/gorilla/websocket) | BSD-2-Clause |
| [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto) | BSD-3-Clause |

## License

MIT © Leonardo Fernandes
