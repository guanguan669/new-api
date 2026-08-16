package model

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestConnectionPoolConfigForSQLiteUsesBoundedHighConcurrencyDefaults(t *testing.T) {
	t.Setenv("SQLITE_MAX_OPEN_CONNS", "")
	t.Setenv("SQLITE_MAX_IDLE_CONNS", "")
	t.Setenv("SQLITE_MAX_LIFETIME", "")

	config := connectionPoolConfigFor(common.DatabaseTypeSQLite)
	require.Equal(t, 32, config.maxOpenConns)
	require.Equal(t, 16, config.maxIdleConns)
	require.Zero(t, config.maxLifetimeSecs)
}

func TestConnectionPoolConfigForSQLiteHonorsAndNormalizesOverrides(t *testing.T) {
	t.Setenv("SQLITE_MAX_OPEN_CONNS", "48")
	t.Setenv("SQLITE_MAX_IDLE_CONNS", "96")
	t.Setenv("SQLITE_MAX_LIFETIME", "-1")

	config := connectionPoolConfigFor(common.DatabaseTypeSQLite)
	require.Equal(t, 48, config.maxOpenConns)
	require.Equal(t, 48, config.maxIdleConns)
	require.Zero(t, config.maxLifetimeSecs)
}

func TestConnectionPoolConfigForNonSQLiteKeepsExistingSettings(t *testing.T) {
	t.Setenv("SQL_MAX_OPEN_CONNS", "123")
	t.Setenv("SQL_MAX_IDLE_CONNS", "45")
	t.Setenv("SQL_MAX_LIFETIME", "67")

	config := connectionPoolConfigFor(common.DatabaseTypePostgreSQL)
	require.Equal(t, 123, config.maxOpenConns)
	require.Equal(t, 45, config.maxIdleConns)
	require.Equal(t, 67, config.maxLifetimeSecs)
}

func TestSQLitePoolUsesWALAndHandlesConcurrentReadsAndWrites(t *testing.T) {
	t.Setenv("SQLITE_MAX_OPEN_CONNS", "")
	t.Setenv("SQLITE_MAX_IDLE_CONNS", "")
	t.Setenv("SQLITE_MAX_LIFETIME", "")

	databasePath := common.NormalizeSQLitePath(filepath.Join(t.TempDir(), "concurrency.db"))
	db, err := gorm.Open(sqlite.Open(databasePath), newGormConfig(true))
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	configureConnectionPool(sqlDB, common.DatabaseTypeSQLite)
	require.Equal(t, 32, sqlDB.Stats().MaxOpenConnections)

	var journalMode string
	require.NoError(t, db.Raw("PRAGMA journal_mode").Scan(&journalMode).Error)
	require.Equal(t, "wal", strings.ToLower(journalMode))
	var busyTimeout int
	require.NoError(t, db.Raw("PRAGMA busy_timeout").Scan(&busyTimeout).Error)
	require.Equal(t, 30000, busyTimeout)
	require.NoError(t, db.Exec("CREATE TABLE concurrent_sqlite_writes (id INTEGER PRIMARY KEY, value INTEGER NOT NULL)").Error)

	const writers = 16
	const readers = 16
	const operationsPerWorker = 20
	start := make(chan struct{})
	errors := make(chan error, writers+readers)
	var workers sync.WaitGroup
	for worker := 0; worker < writers; worker++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			<-start
			for operation := 0; operation < operationsPerWorker; operation++ {
				if err := db.Exec("INSERT INTO concurrent_sqlite_writes (id, value) VALUES (?, ?)", worker*operationsPerWorker+operation, operation).Error; err != nil {
					errors <- fmt.Errorf("writer %d operation %d: %w", worker, operation, err)
					return
				}
			}
		}(worker)
	}
	for worker := 0; worker < readers; worker++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			<-start
			for operation := 0; operation < operationsPerWorker; operation++ {
				var count int64
				if err := db.Raw("SELECT COUNT(*) FROM concurrent_sqlite_writes").Scan(&count).Error; err != nil {
					errors <- fmt.Errorf("reader %d operation %d: %w", worker, operation, err)
					return
				}
			}
		}(worker)
	}
	close(start)
	workers.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}

	var count int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM concurrent_sqlite_writes").Scan(&count).Error)
	require.Equal(t, int64(writers*operationsPerWorker), count)
}
