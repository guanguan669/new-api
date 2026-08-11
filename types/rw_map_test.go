package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRWMapReplaceReplacesAndCopiesInput(t *testing.T) {
	m := NewRWMap[string, int]()
	m.Set("old", 1)
	replacement := map[string]int{"new": 2}

	m.Replace(replacement)
	replacement["new"] = 3
	replacement["later"] = 4

	_, oldExists := m.Get("old")
	require.False(t, oldExists)
	value, newExists := m.Get("new")
	require.True(t, newExists)
	require.Equal(t, 2, value)
	_, laterExists := m.Get("later")
	require.False(t, laterExists)
}
