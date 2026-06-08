package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/internal/note/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/note/entity"
	"github.com/BwCloudWeGo/bw-cli/internal/note/service"
)

type memoryNoteRepo struct {
	byID map[string]*entity.Note
}

func newMemoryNoteRepo() *memoryNoteRepo {
	return &memoryNoteRepo{byID: map[string]*entity.Note{}}
}

func (r *memoryNoteRepo) Save(_ context.Context, note *entity.Note) error {
	r.byID[note.ID] = note
	return nil
}

func (r *memoryNoteRepo) FindByID(_ context.Context, id string) (*entity.Note, error) {
	note, ok := r.byID[id]
	if !ok {
		return nil, entity.ErrNoteNotFound
	}
	return note, nil
}

func TestCreateNote(t *testing.T) {
	svc := service.NewService(newMemoryNoteRepo())

	note, err := svc.Create(context.Background(), dto.CreateNoteCommand{
		AuthorID: "user-1",
		Title:    "DDD scaffold",
		Content:  "Gin plus gRPC demo",
	})
	require.NoError(t, err)
	require.Equal(t, entity.NoteStatusDraft, note.Status)

	found, err := svc.Get(context.Background(), note.ID)
	require.NoError(t, err)
	require.Equal(t, note.ID, found.ID)
	require.Equal(t, entity.NoteStatusDraft, found.Status)
}

func TestPublishSubmittedCreatesPublishedNote(t *testing.T) {
	repo := newMemoryNoteRepo()
	svc := service.NewService(repo)

	published, err := svc.PublishSubmitted(context.Background(), dto.PublishNoteCommand{
		AuthorID:   "user-1",
		Title:      "Published",
		Content:    "Created from publish payload",
		NoteType:   1,
		Permission: 1,
		TopicIDs:   []string{"topic-1"},
	})

	require.NoError(t, err)
	require.NotEmpty(t, published.ID)
	require.Equal(t, "user-1", published.AuthorID)
	require.Equal(t, entity.NoteStatusPublished, published.Status)
	require.Equal(t, int32(1), published.NoteType)
	require.Equal(t, []string{"topic-1"}, published.TopicIDs)

	found, err := repo.FindByID(context.Background(), published.ID)
	require.NoError(t, err)
	require.Equal(t, published.ID, found.ID)
}

func TestCreateNoteRequiresAuthorTitleAndContent(t *testing.T) {
	svc := service.NewService(newMemoryNoteRepo())

	_, err := svc.Create(context.Background(), dto.CreateNoteCommand{
		AuthorID: "",
		Title:    "DDD scaffold",
		Content:  "Gin plus gRPC demo",
	})
	require.ErrorIs(t, err, entity.ErrInvalidNote)
}
