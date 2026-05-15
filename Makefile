SHELL := /bin/bash

BIN_DIR          := ./bin
BIN              := $(BIN_DIR)/mygrep
PKG              := ./...
MAIN             := ./cmd/mygrep

GOLANGCI_VERSION ?= v2.1.6
GOLANGCI         := $(BIN_DIR)/golangci-lint

GO_BUILD_ENV := CGO_ENABLED=0
LDFLAGS      := -s -w

PATTERN ?= TODO
FILE    ?=

.DEFAULT_GOAL := help

.PHONY: help
help: 
	@awk 'BEGIN{FS=":.*##"; printf "Цели:\n"} /^[a-zA-Z_-]+:.*##/ {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: build
build: 
	@mkdir -p $(BIN_DIR)
	$(GO_BUILD_ENV) go build -ldflags='$(LDFLAGS)' -o $(BIN) $(MAIN)

.PHONY: run
run: build 
	$(BIN) -mode=local -e='$(PATTERN)' $(FILE)

.PHONY: tidy
tidy: 
	go mod tidy

.PHONY: vet
vet: ## go vet
	go vet $(PKG)

.PHONY: test
test: 
	go test $(PKG)

.PHONY: test-race
test-race: 
	go test -race $(PKG)

.PHONY: cover
cover: 
	go test -coverprofile=coverage.out $(PKG)
	go tool cover -func=coverage.out | tail -1

.PHONY: cluster-up
cluster-up: build
	bash ./examples/start-cluster.sh

.PHONY: cluster-down
cluster-down:
	@if [ -f ./mygrep.pids ]; then \
		xargs kill < ./mygrep.pids 2>/dev/null || true; \
		rm -f ./mygrep.pids; \
		echo "кластер остановлен"; \
	else \
		echo "mygrep.pids не найден — нечего останавливать"; \
	fi

.PHONY: compare
compare: build 
	bash ./examples/compare.sh '$(FILE)' '$(PATTERN)'


$(GOLANGCI):
	@mkdir -p $(BIN_DIR)
	GOBIN=$(abspath $(BIN_DIR)) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)

.PHONY: lint
lint: $(GOLANGCI)
	$(GOLANGCI) run $(PKG)
	$(GOLANGCI) fmt --diff $(PKG)

.PHONY: lint-fix
lint-fix: $(GOLANGCI) 
	$(GOLANGCI) fmt $(PKG)

DOCKER_IMAGE ?= mygrep:local

.PHONY: docker-build
docker-build: 
	docker build -t $(DOCKER_IMAGE) .

.PHONY: docker-up
docker-up: 
	docker compose up -d --build worker1 worker2 worker3

.PHONY: docker-down
docker-down: 
	docker compose down

.PHONY: docker-logs
docker-logs: 
	docker compose logs -f worker1 worker2 worker3

.PHONY: docker-run
docker-run: 
	docker compose run --rm coordinator \
	  -mode=coordinator \
	  -workers='http://worker1:8081,http://worker2:8082,http://worker3:8083' \
	  -replication=3 -quorum=2 \
	  -e='$(PATTERN)' /data/$(FILE)

.PHONY: clean
clean: 
	rm -rf $(BIN_DIR) ./coverage.out ./mygrep.pids
