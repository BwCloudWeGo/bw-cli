package request_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/internal/gateway/request"
)

func TestNoteRequestsAreDeclaredOutsideHandlers(t *testing.T) {
	payload := []byte(`{"author_id":"user-1","title":"Title","content":"Content"}`)

	var req request.CreateNoteRequest
	err := json.Unmarshal(payload, &req)

	require.NoError(t, err)
	require.Equal(t, "user-1", req.AuthorID)
	require.Equal(t, "Title", req.Title)
	require.Equal(t, "Content", req.Content)
}
