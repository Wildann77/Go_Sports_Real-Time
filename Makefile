.PHONY: setup run dev build test vet fmt review docs-check docs-update agent-check migrate-up migrate-down docker-up docker-down tidy

ifneq (,$(wildcard ./.env))
include .env
export
endif

COMPOSE ?= docker compose
GO_BIN_DIR := $(or $(shell go env GOBIN),$(shell go env GOPATH)/bin)
MIGRATE ?= $(GO_BIN_DIR)/migrate

setup: tidy docker-up migrate-up

run:
	go run cmd/server/main.go

dev:
	# You can use CompileDaemon or air for hot-reload
	go run cmd/server/main.go

build:
	go build -o bin/server cmd/server/main.go

test:
	go test -v ./...

vet:
	go vet ./...

fmt:
	gofmt -w $$(find ./cmd ./internal -name '*.go' -type f | sort)

docs-check:
	sh ./scripts/docs_sync_check.sh

docs-update:
	sh ./scripts/docs_update_hint.sh

review:
	sh ./scripts/review_agent_standards.sh

agent-check:
	sh ./scripts/review_agent_standards.sh
	sh ./scripts/docs_update_hint.sh

migrate-up:
	$(MIGRATE) -path migrations -database "${DATABASE_URL}" -verbose up

migrate-down:
	$(MIGRATE) -path migrations -database "${DATABASE_URL}" -verbose down

docker-up:
	$(COMPOSE) up -d

docker-down:
	$(COMPOSE) down

tidy:
	go mod tidy
