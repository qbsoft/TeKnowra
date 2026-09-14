package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/Tencent/WeKnora/internal/logger"
)

// RunTeKnowraMigrations applies this fork's own migrations after the upstream
// set has run.
//
// # Why a second stream instead of numbers in migrations/versioned
//
// golang-migrate orders migrations by number and remembers a single
// high-water mark per table. Upstream hands out numbers sequentially, so any
// number this fork picks is a number upstream will eventually hand out too:
// pick low and the files collide on the next sync; pick high (the original
// 000900) and the water mark jumps past upstream's later numbers so they
// silently never run. Sharing one sequence with a repository we do not
// control cannot be made collision-free — three renumber-and-replay rounds
// (85-88, 89-92, 93-95) proved it.
//
// So the fork's migrations live in migrations/teknowra with their own
// numbering from 000001 and their own water mark in
// teknowra_schema_migrations (the x-migrations-table URL option). The two
// sequences never meet; an upstream sync never renumbers anything again.
//
// The rules this stream must follow to stay mergeable:
//   - Runs strictly after upstream's migrations, so it may reference upstream
//     tables, but upstream can never reference ours (it does not know they
//     exist).
//   - SQLite mode is skipped: upstream keeps a parallel migrations/sqlite
//     tree, and this fork's features never had SQLite variants — the tables
//     would be missing in that mode with or without this runner.
func RunTeKnowraMigrations(dsn string) error {
	ctx := context.Background()

	if strings.HasPrefix(dsn, "sqlite3://") {
		logger.Warnf(ctx, "[teknowra] fork migrations are postgres-only; sqlite mode runs without them")
		return nil
	}

	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	m, err := migrate.New(
		"file://migrations/teknowra",
		dsn+sep+"x-migrations-table=teknowra_schema_migrations",
	)
	if err != nil {
		return fmt.Errorf("teknowra migrations: create instance: %w", err)
	}
	defer m.Close()

	before, dirty, err := m.Version()
	switch {
	case errors.Is(err, migrate.ErrNilVersion):
		logger.Infof(ctx, "[teknowra] no fork migration history yet, starting from 0")
	case err != nil:
		return fmt.Errorf("teknowra migrations: read version: %w", err)
	case dirty:
		// Same posture as upstream's runner without AutoRecoverDirty: a dirty
		// mark means a migration died halfway and a human must look. These
		// files are few and tiny, so the mark is almost certainly stale, but
		// guessing here can drop a table.
		return fmt.Errorf("teknowra migrations: version %d is dirty, resolve manually", before)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("teknowra migrations: apply: %w", err)
	}
	after, _, _ := m.Version()
	logger.Infof(ctx, "[teknowra] fork migrations at version %d", after)
	return nil
}
