package mongox

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientOptionsIncludeConfiguredCredentials(t *testing.T) {
	opts := clientOptions(Config{
		URI:      "mongodb://127.0.0.1:27017",
		Username: "app",
		Password: "secret",
	})

	require.NotNil(t, opts.Auth)
	require.Equal(t, "app", opts.Auth.Username)
	require.Equal(t, "secret", opts.Auth.Password)
	require.True(t, opts.Auth.PasswordSet)
}
