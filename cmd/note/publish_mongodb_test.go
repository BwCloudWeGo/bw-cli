package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	notev1 "github.com/BwCloudWeGo/bw-cli/api/gen/note/v1"
	notehandler "github.com/BwCloudWeGo/bw-cli/internal/note/handler"
	"github.com/BwCloudWeGo/bw-cli/internal/note/model"
	noterepo "github.com/BwCloudWeGo/bw-cli/internal/note/repo"
	noteservice "github.com/BwCloudWeGo/bw-cli/internal/note/service"
	"github.com/BwCloudWeGo/bw-cli/pkg/config"
	"github.com/BwCloudWeGo/bw-cli/pkg/mongox"
)

func TestPublishNoteWritesSubmittedNoteToConfiguredMongoDB(t *testing.T) {
	if os.Getenv("APP_RUN_NOTE_PUBLISH_MONGODB") != "true" {
		t.Skip("set APP_RUN_NOTE_PUBLISH_MONGODB=true to write a published note into MongoDB using configs/config.yaml")
	}

	previous := config.GlobalConfig
	defer func() { config.GlobalConfig = previous }()

	configPath := filepath.Join("..", "..", "configs", "config.yaml")
	require.NoError(t, config.InitGlobal(configPath))
	cfg := config.MustGlobal()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	log := zap.NewNop()
	mongoClient, err := mongox.NewClient(cfg.MongoDB.MongoxConfig())
	require.NoError(t, err)
	defer disconnectMongo(mongoClient, log)
	require.NoError(t, mongox.Ping(ctx, mongoClient))

	mongoDB := mongox.Database(mongoClient, cfg.MongoDB.Database)
	repository := noterepo.NewMongoRepository(mongoDB, log)
	server := notehandler.NewServer(noteservice.NewService(repository), log)

	unique := time.Now().UTC().UnixNano()
	resp, err := server.PublishNote(ctx, &notev1.PublishNoteRequest{
		AuthorId:   fmt.Sprintf("integration-user-%d", unique),
		Title:      fmt.Sprintf("publish mongodb integration %d", unique),
		Content:    "this note is written by the real publish flow",
		NoteType:   1,
		Permission: 1,
		TopicIds:   []string{"integration", "mongodb"},
		Status:     model.NoteStatusPublishedCode,
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.GetId())
	require.Equal(t, "PUBLISHED", resp.GetStatus())

	stored, err := repository.FindByID(ctx, resp.GetId())
	require.NoError(t, err)
	require.Equal(t, resp.GetId(), stored.ID)
	require.Equal(t, resp.GetAuthorId(), stored.AuthorID)
	require.Equal(t, resp.GetTitle(), stored.Title)
	require.Equal(t, resp.GetContent(), stored.Content)
	require.Equal(t, model.NoteStatusPublished, stored.Status)
	require.Equal(t, int32(1), stored.NoteType)
	require.Equal(t, int32(1), stored.Permission)
	require.Equal(t, []string{"integration", "mongodb"}, stored.TopicIDs)
	require.NotNil(t, stored.PublishedAt)
	t.Logf("published note stored in mongodb: database=%s collection=notes note_id=%s", cfg.MongoDB.Database, stored.ID)
}
