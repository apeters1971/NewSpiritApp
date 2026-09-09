.PHONY: run tidy build

CONTROLLER_SECRET ?= dev-secret
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
	CONTROLLER_SECRET=$(CONTROLLER_SECRET) ADDR=$(ADDR) TLS_CERT=$(TLS_CERT) TLS_KEY=$(TLS_KEY) go run ./cmd/server $(if $(TLS_CERT),-cert $(TLS_CERT)) $(if $(TLS_KEY),-key $(TLS_KEY))
