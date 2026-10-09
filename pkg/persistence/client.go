// Package persistence is the shared ClickHouse write/read layer for
// agentjetson/core services (clickhouse-consumer, object-storage,
// query-service). Schema is owned exclusively by seed/sql — this package
// never issues CREATE TABLE.
package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Config holds native-protocol connection parameters.
// Env names match core compose and Makefile:
//
//	CLICKHOUSE_HOST, CLICKHOUSE_PORT, CLICKHOUSE_USER,
//	CLICKHOUSE_PASSWORD, CLICKHOUSE_DB
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

// DefaultConfig returns localhost defaults used by local demos.
func DefaultConfig() Config {
	return Config{
		Host:     "localhost",
		Port:     9000,
		User:     "default",
		Password: "pass",
		Database: "default",
	}
}

// Client wraps a clickhouse-go native connection.
type Client struct {
	conn driver.Conn
}

// Open dials ClickHouse native TCP and pings. It does not apply schema.
func Open(cfg Config) (*Client, error) {
	if cfg.Host == "" {
		cfg.Host = "localhost"
	}
	if cfg.Port == 0 {
		cfg.Port = 9000
	}
	if cfg.User == "" {
		cfg.User = "default"
	}
	if cfg.Database == "" {
		cfg.Database = "default"
	}

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)},
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.User,
			Password: cfg.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout:      5 * time.Second,
		MaxOpenConns:     10,
		MaxIdleConns:     5,
		ConnMaxLifetime:  time.Hour,
		ConnOpenStrategy: clickhouse.ConnOpenInOrder,
	})
	if err != nil {
		return nil, fmt.Errorf("clickhouse open: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Ping(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("clickhouse ping: %w", err)
	}
	return &Client{conn: conn}, nil
}

// Conn exposes the underlying driver connection for advanced use
// (e.g. query-service SELECTs against query_* views).
func (c *Client) Conn() driver.Conn { return c.conn }

// Ping checks connectivity.
func (c *Client) Ping(ctx context.Context) error {
	return c.conn.Ping(ctx)
}

// Close releases the connection pool.
func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}
