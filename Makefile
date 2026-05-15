# Makefile для mygrep.
#
# Все цели описаны в .PHONY, чтобы make не путал их с одноимёнными файлами.
# golangci-lint ставится ЛОКАЛЬНО в ./bin фиксированной версии — так у любого
# разработчика и CI результат `make lint` идентичен.

SHELL := /bin/bash

# ---------- параметры ----------
BIN_DIR          := ./bin
BIN              := $(BIN_DIR)/mygrep
PKG              := ./...
MAIN             := ./cmd/mygrep

GOLANGCI_VERSION ?= v2.1.6
GOLANGCI         := $(BIN_DIR)/golangci-lint

# CGO выключаем — бинарь полностью статический, удобно в Docker scratch.
GO_BUILD_ENV := CGO_ENABLED=0
LDFLAGS      := -s -w

# Параметры запуска по умолчанию для `make run` (переопределяются из CLI:
#   make run PATTERN='^abc' FILE=/usr/share/dict/words)
PATTERN ?= TODO
FILE    ?=

.DEFAULT_GOAL := help

# ---------- основные цели ----------
.PHONY: help
help: ## показать список целей
	@awk 'BEGIN{FS=":.*##"; printf "Цели:\n"} /^[a-zA-Z_-]+:.*##/ {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: build
build: ## собрать бинарь в ./bin/mygrep
	@mkdir -p $(BIN_DIR)
	$(GO_BUILD_ENV) go build -ldflags='$(LDFLAGS)' -o $(BIN) $(MAIN)

.PHONY: run
run: build ## локальный прогон: make run PATTERN='^abc' FILE=path/to/file
	$(BIN) -mode=local -e='$(PATTERN)' $(FILE)

.PHONY: tidy
tidy: ## go mod tidy
	go mod tidy

.PHONY: vet
vet: ## go vet
	go vet $(PKG)

.PHONY: test
test: ## юнит и интеграционные тесты
	go test $(PKG)

.PHONY: test-race
test-race: ## тесты с детектором гонок (медленнее, но обязателен для concurrency-кода)
	go test -race $(PKG)

.PHONY: cover
cover: ## покрытие
	go test -coverprofile=coverage.out $(PKG)
	go tool cover -func=coverage.out | tail -1

# ---------- кластер ----------
.PHONY: cluster-up
cluster-up: build ## поднять 3 локальных воркера (8081..8083)
	bash ./examples/start-cluster.sh

.PHONY: cluster-down
cluster-down: ## остановить воркеров, поднятых через cluster-up
	@if [ -f ./mygrep.pids ]; then \
		xargs kill < ./mygrep.pids 2>/dev/null || true; \
		rm -f ./mygrep.pids; \
		echo "кластер остановлен"; \
	else \
		echo "mygrep.pids не найден — нечего останавливать"; \
	fi

.PHONY: compare
compare: build ## сравнить с системным grep: make compare FILE=... PATTERN=...
	bash ./examples/compare.sh '$(FILE)' '$(PATTERN)'

# ---------- линтер ----------
# Устанавливаем golangci-lint v2 локально, чтобы версия была общая у всех.
# GOBIN форсит установку именно в $(BIN_DIR), а не в $GOPATH/bin.
$(GOLANGCI):
	@mkdir -p $(BIN_DIR)
	GOBIN=$(abspath $(BIN_DIR)) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)

.PHONY: lint
lint: $(GOLANGCI) ## статический анализ (без правок)
	$(GOLANGCI) run $(PKG)
	$(GOLANGCI) fmt --diff $(PKG)

.PHONY: lint-fix
lint-fix: $(GOLANGCI) ## автофикс (только форматирование импортов через gci)
	$(GOLANGCI) fmt $(PKG)

# ---------- docker ----------
DOCKER_IMAGE ?= mygrep:local

.PHONY: docker-build
docker-build: ## собрать образ $(DOCKER_IMAGE)
	docker build -t $(DOCKER_IMAGE) .

.PHONY: docker-up
docker-up: ## поднять кластер через docker compose (3 воркера)
	docker compose up -d --build worker1 worker2 worker3

.PHONY: docker-down
docker-down: ## остановить кластер и удалить контейнеры
	docker compose down

.PHONY: docker-logs
docker-logs: ## хвост логов воркеров
	docker compose logs -f worker1 worker2 worker3

# Пример: make docker-run PATTERN='^abra' FILE=examples/start-cluster.sh
.PHONY: docker-run
docker-run: ## запустить координатор в кластере: PATTERN=... FILE=...
	docker compose run --rm coordinator \
	  -mode=coordinator \
	  -workers='http://worker1:8081,http://worker2:8082,http://worker3:8083' \
	  -replication=3 -quorum=2 \
	  -e='$(PATTERN)' /data/$(FILE)

# ---------- утилиты ----------
.PHONY: clean
clean: ## удалить артефакты сборки
	rm -rf $(BIN_DIR) ./coverage.out ./mygrep.pids
