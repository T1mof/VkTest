# Настройка переменных
APP_NAME = vktest
MAIN_PATH = ./cmd/server
CONFIG_PATH = ./configs/config.yaml
PROTO_DIR = ./proto
PROTO_FILES = $(PROTO_DIR)/*.proto
SERVER_OUT = ./bin/server
GO = go
PROTOC = protoc

# Установка зависимостей
.PHONY: deps
deps:
	$(GO) mod download
	$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Синхронизация зависимостей
.PHONY: tidy
tidy:
	$(GO) mod tidy

# Генерация protobuf файлов
.PHONY: proto
proto:
	@echo "Генерация кода из proto-файлов..."
	$(PROTOC) --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		$(PROTO_FILES)

# Компиляция
.PHONY: build
build: tidy proto
	@echo "Сборка сервера..."
	@if not exist bin mkdir bin
	$(GO) build -o $(SERVER_OUT) $(MAIN_PATH)

# Запуск сервера
.PHONY: run
run: build
	@echo "Запуск сервера..."
	$(SERVER_OUT) --config=$(CONFIG_PATH)

# Запуск тестов
.PHONY: test
test:
	@echo "Запуск тестов..."
	$(GO) test -v ./...

# Проверка кода (линтинг)
.PHONY: lint
lint:
	@echo "Запуск линтера..."
	golangci-lint run ./...