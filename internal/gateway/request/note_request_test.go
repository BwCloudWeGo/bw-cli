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

func TestPublishNoteRequestBindsExtendedFields(t *testing.T) {
	payload := []byte(`{"author_id":"user-1","title":"Title","content":"Content","note_type":1,"permission":1,"topic_ids":["topic-1"],"status":2}`)

	var req request.PublishNoteRequest
	err := json.Unmarshal(payload, &req)

	require.NoError(t, err)
	require.Equal(t, "user-1", req.AuthorID)
	require.Equal(t, int32(1), req.NoteType)
	require.Equal(t, int32(1), req.Permission)
	require.Equal(t, []string{"topic-1"}, req.TopicIDs)
	require.Equal(t, int32(2), req.Status)
}
