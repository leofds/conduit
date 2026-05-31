XTERM_VERSION  := 5.3.0
XTERM_DIR      := internal/server/static/xterm
BINARY         := dist/conduit
BINARY_MOCKAPI := dist/mockapi
CMD            := ./cmd/conduit
CMD_MOCKAPI    := ./cmd/mockapi

VERSION       := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS       := -ldflags "-X github.com/leofds/conduit/internal/version.Version=$(VERSION)"

.PHONY: build run run-mockapi test lint clean release vendor-xterm

build:
	mkdir -p dist
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

build-mockapi:
	mkdir -p dist
	go build $(LDFLAGS) -o $(BINARY_MOCKAPI) $(CMD_MOCKAPI)

release:
	@mkdir -p dist
	@for platform in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do \
		GOOS=$$(echo $$platform | cut -d/ -f1); \
		GOARCH=$$(echo $$platform | cut -d/ -f2); \
		STAGE=dist/conduit-$(VERSION)-$$GOOS-$$GOARCH; \
		echo "Building $$GOOS/$$GOARCH..."; \
		mkdir -p $$STAGE; \
		GOOS=$$GOOS GOARCH=$$GOARCH go build $(LDFLAGS) -o $$STAGE/conduit $(CMD); \
		cp README.md cmd/conduit/defaults/conduit.yaml cmd/conduit/defaults/hosts.yaml $$STAGE/; \
		tar -czf $$STAGE.tar.gz -C dist $$(basename $$STAGE) && rm -rf $$STAGE; \
	done
	@echo "Release archives ready in dist/"

run: build
	./$(BINARY)

run-mockapi: build-mockapi
	./$(BINARY_MOCKAPI)

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
