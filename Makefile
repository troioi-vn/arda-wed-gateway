SHELL := /bin/bash

.PHONY: help run-backend test-backend fmt-backend dev-frontend api-generate docker-up docker-down bootstrap

help:
	@echo "Available targets:"
	@echo "  bootstrap      - Print setup checklist"
	@echo "  run-backend    - Run Go backend locally"
	@echo "  test-backend   - Run backend unit tests"
	@echo "  fmt-backend    - Format backend Go files"
	@echo "  dev-frontend   - Run frontend dev server"
	@echo "  api-generate   - Generate frontend API client from OpenAPI"
	@echo "  docker-up      - Start local stack with Docker Compose"
	@echo "  docker-down    - Stop local stack"

bootstrap:
	@./scripts/bootstrap.sh

run-backend:
	cd backend && go run ./cmd/gateway

test-backend:
	cd backend && go test ./...

fmt-backend:
	cd backend && gofmt -w $$(find . -name '*.go' -type f)

dev-frontend:
	cd frontend && npm run dev

api-generate:
	cd frontend && npm run api:generate

docker-up:
	docker compose up --build

docker-down:
	docker compose down --remove-orphans
