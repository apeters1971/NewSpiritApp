.PHONY: run tidy build

-include .env

CONTROLLER_SECRET ?= dev-secret
CLOUDFLARE_API_TOKEN ?=
CLOUDFLARE_ACCOUNT_ID ?=
TLS_CERT ?=
TLS_KEY ?=
ifeq ($(TLS_CERT),)
ADDR ?= :8080
else
ADDR ?= :8443
endif

tidy:
	go mod tidy

build:
	go build -o bin/spirit ./cmd/server

run:
	CONTROLLER_SECRET=$(CONTROLLER_SECRET) CLOUDFLARE_API_TOKEN=$(CLOUDFLARE_API_TOKEN) CLOUDFLARE_ACCOUNT_ID=$(CLOUDFLARE_ACCOUNT_ID) ADDR=$(ADDR) TLS_CERT=$(TLS_CERT) TLS_KEY=$(TLS_KEY) go run ./cmd/server $(if $(TLS_CERT),-cert $(TLS_CERT)) $(if $(TLS_KEY),-key $(TLS_KEY))
