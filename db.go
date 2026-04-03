package main

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed db/migrations/001_create_tasks.sql
var migration001 string

//go:embed db/migrations/002_create_time_entries.sql
var migration002 string

// NewPool creates a new pgx connection pool.
func NewPool(ctx context.Context, databaseURL string, maxConns int32) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	config.MaxConns = maxConns

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}

	return pool, nil
}

// RunMigrations executes all embedded SQL migrations in order.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrations := []string{migration001, migration002}
	for i, sql := range migrations {
		if _, err := pool.Exec(ctx, sql); err != nil {
			return fmt.Errorf("exec migration %03d: %w", i+1, err)
		}
	}
	return nil
}
