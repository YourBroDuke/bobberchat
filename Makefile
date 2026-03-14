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
	PGPASSWORD=$${PGPASSWORD:-bobberchat} psql -h $${PGHOST:-localhost} -U $${PGUSER:-bobberchat} -d $${PGDB:-bobberchat} -f migrations/001_initial_schema.sql

run-backend:
	go run ./cmd/bobberd --config configs/backend.yaml

run-tui:
	go run ./cmd/bobber-tui

clean:
	rm -rf $(BIN_DIR)
