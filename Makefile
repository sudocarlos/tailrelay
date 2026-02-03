.PHONY: frontend-build dev-build dev-docker-build clean help

# Build metadata
VERSION ?= $(git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(date -u +%Y-%m-%dT%H:%M:%SZ)
BRANCH ?= $(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
BUILDER ?= $(whoami)

# Go build flags with metadata
LDFLAGS = -w -s \
	-X github.com/sudocarlos/tailrelay-webui/cmd/webui.version=$(VERSION) \
	-X github.com/sudocarlos/tailrelay-webui/cmd/webui.commit=$(COMMIT) \
	-X github.com/sudocarlos/tailrelay-webui/cmd/webui.date=$(DATE) \
	-X github.com/sudocarlos/tailrelay-webui/cmd/webui.branch=$(BRANCH) \
	-X github.com/sudocarlos/tailrelay-webui/cmd/webui.builtBy=$(BUILDER)

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

frontend-build: ## Build SPA assets (requires Node.js/npm)
	@echo "Building frontend assets..."
	cd webui/frontend && npm install
	cd webui/frontend && npm run build

dev-build: frontend-build ## Build webui binary locally for development
	@echo "Building tailrelay-webui with metadata:"
	@echo "  VERSION: $(VERSION)"
	@echo "  COMMIT:  $(COMMIT)"
	@echo "  DATE:    $(DATE)"
	@echo "  BRANCH:  $(BRANCH)"
	@echo "  BUILDER: $(BUILDER)"
	@mkdir -p data
	cd webui && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
		-ldflags="$(LDFLAGS)" \
		-o ../data/tailrelay-webui ./cmd/webui
	@if [ -f data/tailrelay-webui ]; then \
		echo "✅ Build successful: data/tailrelay-webui"; \
		ls -lh data/tailrelay-webui; \
	else \
		echo "❌ Build failed: data/tailrelay-webui not found"; \
		exit 1; \
	fi

dev-docker-build: dev-build ## Build development Docker image using local binary
	@echo "Building development Docker image..."
	docker buildx build --load -f Dockerfile.dev -t sudocarlos/tailrelay:dev-local .
	@echo "✅ Development image built and loaded: sudocarlos/tailrelay:dev-local"

clean: ## Remove build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf data/tailrelay-webui
	@echo "✅ Clean complete"
