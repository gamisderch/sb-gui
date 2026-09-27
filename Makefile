BIN_DIR := bin
BIN_NAME := sb-cli

.PHONY: all build run clean test

all: build

build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags="-s -w" -o $(BIN_DIR)/$(BIN_NAME) ./cmd/sb-cli

run: build
	./$(BIN_DIR)/$(BIN_NAME)

clean:
	rm -rf $(BIN_DIR)

test:
	go test -v ./...
