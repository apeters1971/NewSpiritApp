.PHONY: run tidy build

CONTROLLER_SECRET ?= dev-secret
ADDR ?= :8080

tidy:
	go mod tidy

build:
	go build -o bin/spirit ./cmd/server

run:
	CONTROLLER_SECRET=$(CONTROLLER_SECRET) ADDR=$(ADDR) go run ./cmd/server
