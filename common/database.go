package common

import (
	"net/url"
	"strings"
)

type DatabaseType string

const (
	DatabaseTypeMySQL      DatabaseType = "mysql"
	DatabaseTypeSQLite     DatabaseType = "sqlite"
	DatabaseTypePostgreSQL DatabaseType = "postgres"
	DatabaseTypeClickHouse DatabaseType = "clickhouse"
)

var mainDatabaseType = DatabaseTypeSQLite
var logDatabaseType = DatabaseTypeSQLite

func MainDatabaseType() DatabaseType {
	return mainDatabaseType
}

func LogDatabaseType() DatabaseType {
	return logDatabaseType
}

func SetMainDatabaseType(databaseType DatabaseType) {
	mainDatabaseType = databaseType
}

func SetLogDatabaseType(databaseType DatabaseType) {
	logDatabaseType = databaseType
}

func SetDatabaseTypes(mainType DatabaseType, logType DatabaseType) {
	mainDatabaseType = mainType
	logDatabaseType = logType
}

func UsingMainDatabase(databaseType DatabaseType) bool {
	return mainDatabaseType == databaseType
}

func UsingLogDatabase(databaseType DatabaseType) bool {
	return logDatabaseType == databaseType
}

const (
	sqliteBusyTimeoutPragma = "busy_timeout(30000)"
	sqliteJournalModePragma = "journal_mode(WAL)"
)

// NormalizeSQLitePath preserves the configured SQLite DSN while adding the
// concurrency defaults understood by go-sqlite. Explicit _pragma values are
// never replaced.
func NormalizeSQLitePath(path string) string {
	parsed, err := url.Parse(path)
	if err != nil {
		return path
	}

	query := parsed.Query()
	hasBusyTimeout := false
	hasJournalMode := false
	for _, pragma := range query["_pragma"] {
		switch sqlitePragmaName(pragma) {
		case "busy_timeout":
			hasBusyTimeout = true
		case "journal_mode":
			hasJournalMode = true
		}
	}
	if !hasBusyTimeout {
		query.Add("_pragma", sqliteBusyTimeoutPragma)
	}
	if !hasJournalMode {
		query.Add("_pragma", sqliteJournalModePragma)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func sqlitePragmaName(pragma string) string {
	name := strings.TrimSpace(pragma)
	if separator := strings.IndexAny(name, "(= \t"); separator >= 0 {
		name = name[:separator]
	}
	return strings.ToLower(strings.TrimSpace(name))
}

// go-sqlite recognizes PRAGMAs through the _pragma query parameter. Keeping
// these in the default DSN applies them to every connection in the pool.
var SQLitePath = NormalizeSQLitePath("one-api.db")
