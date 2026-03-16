BIN_DIR := bin

.PHONY: build test test-integration test-api test-e2e lint migrate run-backend run-tui clean

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/bobberd ./backend/cmd/bobberd
	go build -o $(BIN_DIR)/bobber ./cli/cmd/bobber
	go build -o $(BIN_DIR)/bobber-tui ./tui/cmd/bobber-tui

test:
	go test ./backend/... ./cli/... ./tui/...

test-integration:
	go test -tags=integration -race ./backend/test/integration/ -v

test-api:
	go test -tags=integration -race ./backend/test/api/ -v -count=1

test-e2e:
	./scripts/e2e-test.sh

lint:
	go vet ./backend/... ./cli/... ./tui/...

migrate:
	PGPASSWORD=$${PGPASSWORD:-bobberchat} psql -h $${PGHOST:-localhost} -U $${PGUSER:-bobberchat} -d $${PGDB:-bobberchat} -f migrations/001_initial_schema.sql

run-backend:
	go run ./backend/cmd/bobberd --config configs/backend.yaml

run-tui:
	go run ./tui/cmd/bobber-tui

clean:
	rm -rf $(BIN_DIR)
