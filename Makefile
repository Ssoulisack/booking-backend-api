# Go parameters
GOCMD=go
GORUN=$(GOCMD) run

# Docker parameters
DOCKER_IMAGE=booking-api
DOCKER_CONTAINER=booking-api-container
DOCKER_PORT=3000
# The number in the version file; override with: make build VERSION=1.0.1
VERSION ?= $(shell tr -d '[:space:]' < version)

# Docker compose parameters
DOCKER_COMPOSE=docker-compose
DOCKER_UP=$(DOCKER_COMPOSE) up -d
DOCKER_COMPOSE_BUILD=$(DOCKER_COMPOSE) up --build -d
DOCKER_DOWN=$(DOCKER_COMPOSE) down
DOCKER_RESTART=$(DOCKER_COMPOSE) restart

.PHONY: run build run-docker logs stop clean release release-dry

# Run the application
run:
	$(GORUN) ./cmd/api

# Build Docker image
build:
	docker build -t $(DOCKER_IMAGE):$(VERSION) -t $(DOCKER_IMAGE):latest .

# Build for amd64 + arm64 and push to Docker Hub, tagged with the version file and latest
# (docker login first; see scripts/release.sh)
release:
	scripts/release.sh

# The same build, but nothing is pushed
release-dry:
	scripts/release.sh --dry-run

# Run Docker container (config.yaml is mounted, never baked into the image)
run-docker:
	docker run -d --name $(DOCKER_CONTAINER) -p $(DOCKER_PORT):3000 \
		-v "$(CURDIR)/config.yaml:/app/config.yaml:ro" $(DOCKER_IMAGE):$(VERSION)

# Follow Docker container logs
logs:
	docker logs -f $(DOCKER_CONTAINER)

# Stop Docker container
stop:
	docker stop $(DOCKER_CONTAINER)

# Remove Docker container
clean:
	docker rm -f $(DOCKER_CONTAINER)

# Run Docker Compose
compose-build:
	$(DOCKER_COMPOSE_BUILD)
up:
	$(DOCKER_UP)
 # Stop Docker Compose
down:
	$(DOCKER_DOWN)
# Restart Docker Compose
restart:
	$(DOCKER_RESTART)

