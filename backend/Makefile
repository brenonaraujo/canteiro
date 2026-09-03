SHELL := /bin/bash
.DEFAULT_GOAL := help

PKG      := ./...
COVERAGE := coverage.out
COVERPKG := ./internal/app/...,./internal/handler/...,./internal/i18n/...,./internal/platform/postgres/...,./internal/repository/...,./internal/auth/...,./internal/domain/account/...,./internal/domain/listing/...,./internal/domain/rental/...,./internal/payment/...,./internal/rental/...,./internal/repository/listing/...,./internal/repository/rental/...

.PHONY: help tidy build test lint vuln oas i18n-audit

help: ## Mostra esta ajuda.
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-12s %s\n", $$1, $$2}'

tidy: ## go mod tidy
	GOTOOLCHAIN=local go mod tidy

build: ## Compila o server em ./bin/server
	@mkdir -p bin
	CGO_ENABLED=0 GOTOOLCHAIN=local go build -trimpath -o bin/server ./cmd/server

test: ## Testes com race + coverage (falha se < 80% nos pacotes COVERPKG)
	GOTOOLCHAIN=local go test -race -coverprofile=$(COVERAGE) -covermode=atomic -coverpkg=$(COVERPKG) $(PKG)
	@go tool cover -func=$(COVERAGE) | awk '/^total:/ {p=substr($$3,1,length($$3)-1)+0; if (p < 80) {print "FAIL: coverage "p"% < 80%"; exit 1} else {print "OK: coverage "p"% >= 80%"}}'

lint: ## golangci-lint v2
	golangci-lint run --timeout=5m $(PKG)

vuln: ## govulncheck
	govulncheck ./...

oas: ## Regenera internal/api/openapi.gen.go
	oapi-codegen --config=codegen.yaml api/openapi.yaml

i18n-audit: ## Paridade en / pt-BR / es
	../scripts/check-i18n.sh ..
