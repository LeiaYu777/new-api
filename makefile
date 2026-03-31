FRONTEND_DIR = ./web
BACKEND_DIR = .
SCRIPTS_DIR = ./scripts

.PHONY: all build-frontend start-backend test-go-local test-go-docker sbom

all: build-frontend start-backend

build-frontend:
	@echo "Building frontend..."
	@cd $(FRONTEND_DIR) && bun install && DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=$(cat VERSION) bun run build

start-backend:
	@echo "Starting backend dev server..."
	@cd $(BACKEND_DIR) && go run main.go &

test-go-local:
	@echo "Running Go tests with persistent local cache..."
	@$(SCRIPTS_DIR)/go-test-local.sh

test-go-docker:
	@echo "Running Go tests in Docker with persistent cache..."
	@$(SCRIPTS_DIR)/go-test-docker.sh

sbom:
	@echo "Generating SBOM..."
	@$(SCRIPTS_DIR)/generate-sbom.sh
