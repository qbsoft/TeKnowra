package database

import (
	"os"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Explicitly opt in with a disposable PostgreSQL; never infer the application's DB.
func TestPostgresMigrationsPersistTenantCredentials(t *testing.T) {
	dsn := os.Getenv("WEKNORA_MIGRATION_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set WEKNORA_MIGRATION_TEST_POSTGRES_DSN to a disposable database")
	}
	chdirAndRestore(t, sqliteRepoRoot(t))
	require.NoError(t, RunMigrationsWithOptions(dsn, MigrationOptions{}))
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	tx := db.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() { _ = tx.Rollback().Error })

	tenant := &types.Tenant{Name: "credentials-migration-regression"}
	// Workspace creation writes every mapped field, including nullable credentials.
	require.NoError(t, tx.Create(tenant).Error)
	var nullable bool
	require.NoError(t, tx.Raw("SELECT is_nullable = 'YES' FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'tenants' AND column_name = 'credentials' AND udt_name = 'jsonb'").Scan(&nullable).Error)
	require.True(t, nullable, "credentials must be nullable JSONB for existing workspaces")

	// The schema must retain arbitrary provider configuration without rewriting it.
	const probe = `{"migration_probe":"synthetic"}`
	require.NoError(t, tx.Exec("UPDATE tenants SET credentials = ?::jsonb WHERE id = ?", probe, tenant.ID).Error)
	var retained string
	require.NoError(t, tx.Raw("SELECT credentials->>'migration_probe' FROM tenants WHERE id = ?", tenant.ID).Scan(&retained).Error)
	require.Equal(t, "synthetic", retained)
}
