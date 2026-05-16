package repo_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/internal/user/repo"
)

func TestSHA256HasherUsesPerPasswordSalt(t *testing.T) {
	hasher := repo.NewSHA256Hasher()

	first, err := hasher.Hash("secret123")
	require.NoError(t, err)
	second, err := hasher.Hash("secret123")
	require.NoError(t, err)

	require.NotEqual(t, first, second)
	require.True(t, hasher.Verify(first, "secret123"))
	require.True(t, hasher.Verify(second, "secret123"))
	require.False(t, hasher.Verify(first, "wrong"))
}
