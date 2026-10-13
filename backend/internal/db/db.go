// Package db abre la conexión a PostgreSQL y aplica las migraciones.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"sort"
	"time"

	_ "github.com/lib/pq" // registra el driver "postgres"
)

// Las migraciones se compilan DENTRO del binario (go:embed). Así el contenedor
// final no necesita copiar archivos .sql aparte.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open abre el pool de conexiones y espera a que la BD responda. En Docker
// Compose la BD puede tardar unos segundos en estar lista, por eso reintentamos.
func Open(ctx context.Context, url string, log *slog.Logger) (*sql.DB, error) {
	conn, err := sql.Open("postgres", url)
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(30 * time.Minute)

	var lastErr error
	for attempt := 1; attempt <= 15; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		lastErr = conn.PingContext(pingCtx)
		cancel()
		if lastErr == nil {
			return conn, nil
		}
		log.Info("esperando a la base de datos", "intento", attempt)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	_ = conn.Close()
	return nil, fmt.Errorf("no se pudo conectar a la base de datos: %w", lastErr)
}

// Migrate aplica, en orden, los .sql que todavía no se han ejecutado.
// Cada migración corre en una transacción: o se aplica completa o no se aplica.
func Migrate(ctx context.Context, conn *sql.DB, log *slog.Logger) error {
	// Usamos UNA conexión dedicada para poder tomar un "advisory lock":
	// si arrancan dos instancias a la vez, la segunda espera a que la primera termine.
	c, err := conn.Conn(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	if _, err := c.ExecContext(ctx, `SELECT pg_advisory_lock(727274)`); err != nil {
		return err
	}
	defer c.ExecContext(context.Background(), `SELECT pg_advisory_unlock(727274)`) //nolint:errcheck

	if _, err := c.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return err
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names) // 0001_..., 0002_...: el orden alfabético es el orden de ejecución

	for _, name := range names {
		var exists bool
		if err := c.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, name).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}

		body, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		tx, err := c.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migración %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		log.Info("migración aplicada", "version", name)
	}
	return nil
}
