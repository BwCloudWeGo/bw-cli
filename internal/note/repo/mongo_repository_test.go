package repo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/BwCloudWeGo/bw-cli/internal/note/entity"
	dbmodel "github.com/BwCloudWeGo/bw-cli/internal/note/model"
	"github.com/BwCloudWeGo/bw-cli/pkg/mongox"
)

type fakeNoteDocumentStore struct {
	upsertID       any
	upsertDocument *dbmodel.NoteDocument
	upsertErr      error

	findID       any
	findDocument *dbmodel.NoteDocument
	findErr      error
}

var _ mongox.CollectionNamed = dbmodel.NoteDocument{}

func (s *fakeNoteDocumentStore) UpsertByID(ctx context.Context, id any, document *dbmodel.NoteDocument, opts ...options.Lister[options.ReplaceOptions]) (*mongo.UpdateResult, error) {
	s.upsertID = id
	s.upsertDocument = document
	if s.upsertErr != nil {
		return nil, s.upsertErr
	}
	return &mongo.UpdateResult{MatchedCount: 1, ModifiedCount: 1}, nil
}

func (s *fakeNoteDocumentStore) FindByID(ctx context.Context, id any, opts ...options.Lister[options.FindOneOptions]) (*dbmodel.NoteDocument, error) {
	s.findID = id
	if s.findErr != nil {
		return nil, s.findErr
	}
	return s.findDocument, nil
}

func TestMongoRepositorySaveUsesSharedMongoxCollection(t *testing.T) {
	store := &fakeNoteDocumentStore{}
	repository := NewMongoRepositoryWithStore(store)
	note, err := entity.NewNote("user-1", "Mongo note", "stored by shared collection")
	require.NoError(t, err)
	note.NoteType = 2
	note.Permission = 1
	note.TopicIDs = []string{"topic-1"}
	note.Publish()

	require.NoError(t, repository.Save(context.Background(), note))

	require.Equal(t, note.ID, store.upsertID)
	require.NotNil(t, store.upsertDocument)
	require.Equal(t, note.ID, store.upsertDocument.ID)
	require.Equal(t, note.AuthorID, store.upsertDocument.AuthorID)
	require.Equal(t, note.Title, store.upsertDocument.Title)
	require.Equal(t, note.Content, store.upsertDocument.Content)
	require.Equal(t, note.Status.Code(), store.upsertDocument.Status)
	require.Equal(t, note.NoteType, store.upsertDocument.NoteType)
	require.Equal(t, note.Permission, store.upsertDocument.Permission)
	require.Equal(t, note.TopicIDs, store.upsertDocument.TopicIDs)
}

func TestMongoRepositoryFindByIDMapsDocumentToDomain(t *testing.T) {
	store := &fakeNoteDocumentStore{
		findDocument: &dbmodel.NoteDocument{
			ID:         "note-1",
			AuthorID:   "user-1",
			Title:      "Mongo note",
			Content:    "stored by shared collection",
			Status:     entity.NoteStatusPublishedCode,
			NoteType:   2,
			Permission: 1,
			TopicIDs:   []string{"topic-1"},
		},
	}
	repository := NewMongoRepositoryWithStore(store)

	note, err := repository.FindByID(context.Background(), "note-1")

	require.NoError(t, err)
	require.Equal(t, "note-1", store.findID)
	require.Equal(t, "note-1", note.ID)
	require.Equal(t, entity.NoteStatusPublished, note.Status)
	require.Equal(t, int32(2), note.NoteType)
	require.Equal(t, []string{"topic-1"}, note.TopicIDs)
}

func TestMongoRepositoryFindByIDMapsMongoNotFound(t *testing.T) {
	repository := NewMongoRepositoryWithStore(&fakeNoteDocumentStore{findErr: mongox.ErrNotFound})

	_, err := repository.FindByID(context.Background(), "missing")

	require.ErrorIs(t, err, entity.ErrNoteNotFound)
}
