GO       ?= go
BIN      := bin
CMD      := ./cmd/api
COVERAGE := coverage.out

.DEFAULT_GOAL := help

.PHONY: help run build test race cover fmt fmt-check vet check tidy clean

help: ## Показать список целей
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

run: ## Запустить сервис
	$(GO) run $(CMD)

build: ## Собрать бинарник в $(BIN)
	@mkdir -p $(BIN)
	$(GO) build -o $(BIN)/ $(CMD)

test: ## Прогнать тесты
	$(GO) test ./...

race: ## Прогнать тесты с детектором гонок
	CGO_ENABLED=1 $(GO) test -race ./...

cover: ## Покрытие + отчёт по функциям
	$(GO) test ./... -coverprofile=$(COVERAGE)
	$(GO) tool cover -func=$(COVERAGE)

fmt: ## Отформатировать код (пишет файлы)
	gofmt -w .

fmt-check: ## Проверить форматирование (для CI, ничего не пишет)
	@test -z "$$(gofmt -l . | grep -v '^\.')" || (gofmt -l . | grep -v '^\.' && echo "код не отформатирован" && exit 1)

vet: ## Статический анализ
	$(GO) vet ./...

check: fmt-check vet test ## Всё зелёное перед коммитом
	@echo "ok"

tidy: ## Привести go.mod в порядок
	$(GO) mod tidy

clean: ## Убрать артефакты
	rm -rf $(BIN) $(COVERAGE)