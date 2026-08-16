package common

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultSQLitePathUsesSupportedPragmas(t *testing.T) {
	parsed, err := url.Parse(SQLitePath)
	require.NoError(t, err)

	pragmas := parsed.Query()["_pragma"]
	require.Contains(t, pragmas, "busy_timeout(30000)")
	require.Contains(t, pragmas, "journal_mode(WAL)")
	require.NotContains(t, parsed.Query(), "_busy_timeout")
}

func TestNormalizeSQLitePathPreservesQueryAndAddsMissingPragmas(t *testing.T) {
	parsed, err := url.Parse(NormalizeSQLitePath("file:custom.db?cache=shared"))
	require.NoError(t, err)
	require.Equal(t, "shared", parsed.Query().Get("cache"))

	pragmas := parsed.Query()["_pragma"]
	require.Contains(t, pragmas, "busy_timeout(30000)")
	require.Contains(t, pragmas, "journal_mode(WAL)")
}

func TestNormalizeSQLitePathRespectsExplicitPragmaOverrides(t *testing.T) {
	parsed, err := url.Parse(NormalizeSQLitePath("custom.db?_pragma=busy_timeout(750)&_pragma=journal_mode(DELETE)"))
	require.NoError(t, err)

	pragmas := parsed.Query()["_pragma"]
	require.Equal(t, []string{"busy_timeout(750)", "journal_mode(DELETE)"}, pragmas)
}
