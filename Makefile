XTERM_VERSION  := 5.3.0
XTERM_DIR      := internal/server/static/xterm
BINARY         := dist/conduit
BINARY_CTL	   := dist/conduitctl
BINARY_MOCKAPI := dist/mockapi
CMD            := ./cmd/conduit
CMD_CTL		   := ./cmd/conduitctl
CMD_MOCKAPI    := ./cmd/mockapi

SSH_TEST_COMPOSE := test/sshserver/docker-compose.test.yml
SSH_TEST_KEYS    := test/sshserver/keys
SSH_TEST_KEY     := $(SSH_TEST_KEYS)/authorized_keys.test

VERSION       := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS       := -ldflags "-X github.com/leofds/conduit/internal/version.Version=$(VERSION)"

.PHONY: build-ctl build run run-mockapi test test-integration lint clean release vendor-xterm sshserver-keys sshserver-up sshserver-down sshserver-shell

build-ctl:
	mkdir -p dist
	go build $(LDFLAGS) -o $(BINARY_CTL) ${CMD_CTL}

build: build-ctl
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
		GOOS=$$GOOS GOARCH=$$GOARCH go build $(LDFLAGS) -o $$STAGE/conduitctl $(CMD_CTL); \
		cp README.md DOCS.md cmd/conduit/defaults/conduit.yaml cmd/conduit/defaults/hosts.yaml $$STAGE/; \
		tar -czf $$STAGE.tar.gz -C dist $$(basename $$STAGE) && rm -rf $$STAGE; \
	done
	@echo "Release archives ready in dist/"

run: build
	./$(BINARY)

run-mockapi: build-mockapi
	./$(BINARY_MOCKAPI)

test:
	go test ./...

# Generate the test SSH keypair on the host (git-ignored). The public key is
# mounted into the container and installed as authorized_keys; the private key
# stays on the host so integration tests can authenticate with it.
sshserver-keys:
	@mkdir -p $(SSH_TEST_KEYS)
	@if [ -f "$(SSH_TEST_KEY)" ]; then \
		echo "Key already exists: $(SSH_TEST_KEY) (delete it to regenerate)"; \
	else \
		ssh-keygen -t ed25519 -N '' -C conduit-sshtest -f "$(SSH_TEST_KEY)" -q; \
		echo "Generated $(SSH_TEST_KEY) and $(SSH_TEST_KEY).pub"; \
	fi

# Start/stop the containerized test SSH server (see test/sshserver/README.md).
sshserver-up: sshserver-keys
	docker compose -f $(SSH_TEST_COMPOSE) up -d --build

sshserver-down:
	docker compose -f $(SSH_TEST_COMPOSE) down

# Open a shell inside the running test SSH server container, as the master user.
# -it allocates a TTY so the interactive shell stays attached; -l makes it a
# login shell so /etc/profile.d/prompt.sh (which sets the PS1) is sourced.
# The command changes into $HOME so the prompt shows ~ instead of /.
sshserver-shell:
	@docker compose -f $(SSH_TEST_COMPOSE) exec -it -u master sshserver /bin/sh -lc 'cd "$$HOME"; exec /bin/sh'

# Run integration tests against the containerized SSH server.
# Requires Docker; tests tagged "integration" are skipped when it is unavailable.
test-integration: sshserver-up
	CONDUIT_SSH_TEST_ADDR=127.0.0.1:2222 \
	CONDUIT_SSH_TEST_KEY=$(CURDIR)/$(SSH_TEST_KEY) \
	go test -tags integration ./...; \
	status=$$?; \
	docker compose -f $(SSH_TEST_COMPOSE) down; \
	exit $$status

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
