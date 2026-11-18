package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const SchemaVersion = 1
const migrationLock int64 = 0x534849505241 // SHIPRA

func Open(ctx context.Context, dsnFile string) (*sql.DB, error) {
	data, err := os.ReadFile(dsnFile)
	if err != nil {
		return nil, fmt.Errorf("read database connection file: %w", err)
	}
	dsn := strings.TrimSpace(string(data))
	if dsn == "" || len(dsn) > 4096 {
		return nil, errors.New("database connection file must contain one bounded DSN")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, errors.New("invalid database connection configuration")
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, errors.New("database unavailable")
	}
	return db, nil
}

func CurrentVersion(ctx context.Context, db *sql.DB) (int, error) {
	var version int
	err := db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		return 0, errors.New("schema unavailable; run migrate")
	}
	return version, nil
}

func RequireVersion(ctx context.Context, db *sql.DB) error {
	v, err := CurrentVersion(ctx, db)
	if err != nil {
		return err
	}
	if v != SchemaVersion {
		return fmt.Errorf("schema version %d is incompatible with server version %d", v, SchemaVersion)
	}
	return nil
}

func Migrate(ctx context.Context, db *sql.DB) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", migrationLock); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLock)
	if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version integer PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("create migration registry: %w", err)
	}
	var current int
	if err := conn.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&current); err != nil {
		return err
	}
	if current > SchemaVersion {
		return fmt.Errorf("database schema %d is newer than binary %d", current, SchemaVersion)
	}
	if current == SchemaVersion {
		return nil
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range schemaV1 {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply schema version 1: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations(version) VALUES (1)"); err != nil {
		return err
	}
	return tx.Commit()
}

var schemaV1 = []string{
	`CREATE TABLE workspaces (id uuid PRIMARY KEY, name text NOT NULL, singleton boolean NOT NULL DEFAULT true UNIQUE CHECK (singleton))`,
	`CREATE TABLE users (id uuid PRIMARY KEY, username text NOT NULL UNIQUE CHECK (username = lower(username)), password_hash text NOT NULL, active boolean NOT NULL DEFAULT true, created_at timestamptz NOT NULL DEFAULT now())`,
	`CREATE TABLE memberships (workspace_id uuid NOT NULL REFERENCES workspaces(id), user_id uuid NOT NULL REFERENCES users(id), role text NOT NULL CHECK (role IN ('viewer','operator','admin')), PRIMARY KEY (workspace_id,user_id))`,
	`CREATE TABLE sessions (id uuid PRIMARY KEY, user_id uuid NOT NULL REFERENCES users(id), token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash)=32), csrf_hash bytea NOT NULL CHECK (octet_length(csrf_hash)=32), created_at timestamptz NOT NULL DEFAULT now(), last_seen_at timestamptz NOT NULL DEFAULT now(), expires_at timestamptz NOT NULL, revoked_at timestamptz)`,
	`CREATE INDEX sessions_user_active ON sessions(user_id,expires_at) WHERE revoked_at IS NULL`,
	`CREATE TABLE api_tokens (id uuid PRIMARY KEY, user_id uuid NOT NULL REFERENCES users(id), token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash)=32), scope text NOT NULL CHECK (scope IN ('read','write','admin')), expires_at timestamptz NOT NULL, revoked_at timestamptz, created_at timestamptz NOT NULL DEFAULT now())`,
	`CREATE TABLE projects (id uuid PRIMARY KEY, workspace_id uuid NOT NULL REFERENCES workspaces(id), name text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE (workspace_id,name), UNIQUE (id,workspace_id))`,
	`CREATE TABLE repositories (id uuid PRIMARY KEY, project_id uuid NOT NULL, workspace_id uuid NOT NULL, provider text NOT NULL CHECK (provider='github'), installation_id text NOT NULL, repository_id text NOT NULL, default_ref text NOT NULL, FOREIGN KEY (project_id,workspace_id) REFERENCES projects(id,workspace_id), UNIQUE (workspace_id,provider,installation_id,repository_id))`,
	`CREATE TABLE audit_events (id uuid PRIMARY KEY, workspace_id uuid REFERENCES workspaces(id), actor_id uuid REFERENCES users(id), action text NOT NULL, target text NOT NULL, outcome text NOT NULL CHECK (outcome IN ('success','denied','failure')), request_id text, metadata jsonb NOT NULL DEFAULT '{}'::jsonb, created_at timestamptz NOT NULL DEFAULT now())`,
	`CREATE INDEX audit_events_workspace_time ON audit_events(workspace_id,created_at DESC,id DESC)`,
}
