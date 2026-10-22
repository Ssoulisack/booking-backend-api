# Go parameters
GOCMD=go
GORUN=$(GOCMD) run

# Docker parameters
DOCKER_IMAGE=backend
DOCKER_CONTAINER=backend-container
DOCKER_PORT=3000

# Docker compose parameters
DOCKER_COMPOSE=docker-compose
DOCKER_UP=$(DOCKER_COMPOSE) up -d
DOCKER_COMPOSE_BUILD=$(DOCKER_COMPOSE) up --build -d
DOCKER_DOWN=$(DOCKER_COMPOSE) down
DOCKER_RESTART=$(DOCKER_COMPOSE) restart

.PHONY: run build docker-run docker-stop docker-clean

# Run the application
run:
	$(GORUN) ./cmd/api

# Build Docker image
build:
	docker build -t $(DOCKER_IMAGE) .

# Run Docker container
run-docker:
	docker run -d --name $(DOCKER_CONTAINER) -p $(DOCKER_PORT):$(DOCKER_PORT) $(DOCKER_IMAGE)

# Stop Docker container
stop:
	docker stop $(DOCKER_CONTAINER)

# Remove Docker container
clean:
	docker rm $(DOCKER_CONTAINER)

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

