# Client connection settings (override on the command line: make client PORT=9000 HOST=10.0.0.5)
PORT ?= 12341
HOST ?= 127.0.0.1

BIN_DIR := bin

.PHONY: all server client build build-server build-client tidy fmt vet test clean help

all: build

server:
	go run ./cmd/server/server.go

client:
	go run ./cmd/client/client.go -p $(PORT) -s $(HOST)

build: build-server build-client

build-server:
	go build -o $(BIN_DIR)/server ./cmd/server

build-client:
	go build -o $(BIN_DIR)/client ./cmd/client

tidy:
	go mod tidy

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./...

clean:
	rm -rf $(BIN_DIR)

help:
	@echo "Targets:"
	@echo "  make server                 Run the server"
	@echo "  make client                 Run the client (PORT=$(PORT) HOST=$(HOST))"
	@echo "  make client PORT=9000 HOST=10.0.0.5"
	@echo "  make build                  Build both binaries into ./$(BIN_DIR)"
	@echo "  make tidy | fmt | vet | test | clean"