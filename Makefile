XTERM_VERSION := 5.3.0
XTERM_DIR     := internal/server/static/xterm
BINARY        := dist/conduit
CMD           := ./cmd/conduit

VERSION       := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS       := -ldflags "-X github.com/leofds/conduit/internal/version.Version=$(VERSION)"

.PHONY: build run run-mockapi copy-configs test lint clean vendor-xterm

build:
	mkdir -p dist
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

copy-configs:
	@[ -f hosts.yaml ] && cp hosts.yaml dist/hosts.yaml || true
	@[ -f conduit.yaml ] && cp conduit.yaml dist/conduit.yaml || true

copy-mockapi-configs:
	@[ -f hosts-mockapi.yaml ] && cp hosts-mockapi.yaml dist/hosts-mockapi.yaml || true
	@[ -f conduit.yaml ] && cp conduit.yaml dist/conduit.yaml || true

run: copy-configs
	go run $(LDFLAGS) $(CMD)

run-mockapi: copy-mockapi-configs
	go run ./cmd/mockapi

test:
	go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY)

vendor-xterm:
	@echo "Fetching xterm.js $(XTERM_VERSION)..."
	cd /tmp && npm pack xterm@$(XTERM_VERSION) --quiet
	tar -xzf /tmp/xterm-$(XTERM_VERSION).tgz \
		--strip-components=1 \
		-C $(XTERM_DIR) \
		package/LICENSE \
		package/css/xterm.css \
		package/lib/xterm.js
	rm /tmp/xterm-$(XTERM_VERSION).tgz
	@echo "Done. Commit the changes in $(XTERM_DIR)/"
