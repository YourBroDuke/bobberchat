package persistence

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	pool *pgxpool.Pool
}

func (db *DB) Pool() *pgxpool.Pool {
	if db == nil {
		return nil
	}
	return db.pool
}

func NewDB(dsn string) (*DB, error) {
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &DB{pool: pool}, nil
}

func (db *DB) Close() {
	if db == nil || db.pool == nil {
		return
	}
	db.pool.Close()
}
