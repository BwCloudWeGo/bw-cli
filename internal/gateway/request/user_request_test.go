package request_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/internal/gateway/request"
)

func TestUserRequestsAreDeclaredOutsideHandlers(t *testing.T) {
	payload := []byte(`{"email":"ada@example.com","display_name":"Ada","password":"secret123"}`)

	var req request.RegisterUserRequest
	err := json.Unmarshal(payload, &req)

	require.NoError(t, err)
	require.Equal(t, "ada@example.com", req.Email)
	require.Equal(t, "Ada", req.DisplayName)
	require.Equal(t, "secret123", req.Password)
}
