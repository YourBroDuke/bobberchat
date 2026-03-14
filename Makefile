BIN_DIR := bin

.PHONY: build test lint migrate run-backend run-tui clean

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/bobberd ./cmd/bobberd
	go build -o $(BIN_DIR)/bobber ./cmd/bobber
	go build -o $(BIN_DIR)/bobber-tui ./cmd/bobber-tui

test:
	go test ./...

lint:
	go vet ./...

migrate:
	@echo "Run migrations using your migration tool against ./migrations"

run-backend:
	go run ./cmd/bobberd --config configs/backend.yaml

run-tui:
	go run ./cmd/bobber-tui

clean:
	rm -rf $(BIN_DIR)
