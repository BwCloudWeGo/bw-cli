package main

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.uber.org/zap"

	"github.com/BwCloudWeGo/bw-cli/pkg/config"
	"github.com/BwCloudWeGo/bw-cli/pkg/mongox"
)

const noteMongoExampleCollection = "note_mongodb_examples"

// noteMongoExampleDocument 是 note 服务的 MongoDB 示例文档结构。
// 这个结构只用于演示公共 mongox.DocumentStore 的调用方式，不参与正式笔记业务表结构。
type noteMongoExampleDocument struct {
	ID        string    `bson:"_id"`
	Service   string    `bson:"service"`
	Title     string    `bson:"title"`
	Content   string    `bson:"content"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

// MongoCollectionName 声明示例文档对应的 MongoDB 集合名称。
// 业务侧只要让文档结构实现该方法，就可以直接使用 mongox.NewDocumentStore 创建通用操作类。
func (noteMongoExampleDocument) MongoCollectionName() string {
	return noteMongoExampleCollection
}

// runMongoDocumentStoreExample 演示 note 服务如何读取当前配置中的 mongodb.*，
// 并通过公共 mongox.DocumentStore 类完成一次完整的数据操作。
//
// 这个函数不会在 main 中自动执行，避免服务每次启动都写入示例数据。
// 本地验证时可以执行：
//
//	APP_RUN_NOTE_MONGODB_EXAMPLE=true go test ./cmd/note -run TestRunMongoDocumentStoreExampleUsesCurrentConfig -v
func runMongoDocumentStoreExample(ctx context.Context, cfg *config.Config, log *zap.Logger) (*noteMongoExampleDocument, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if log == nil {
		log = zap.NewNop()
	}

	// 从当前配置文件解析出的 MongoDB 配置创建客户端；账号、密码、数据库名都来自配置。
	mongoClient, err := mongox.NewClient(cfg.MongoDB.MongoxConfig())
	if err != nil {
		return nil, fmt.Errorf("create mongodb client: %w", err)
	}
	defer disconnectMongo(mongoClient, log)

	if err := mongox.Ping(ctx, mongoClient); err != nil {
		return nil, fmt.Errorf("ping mongodb: %w", err)
	}

	db := mongox.Database(mongoClient, cfg.MongoDB.Database)
	examples := mongox.NewDocumentStore[noteMongoExampleDocument](db, log)
	now := time.Now().UTC()
	documentID := fmt.Sprintf("%s:mongox-example", cfg.App.NoteServiceName)
	document := &noteMongoExampleDocument{
		ID:        documentID,
		Service:   cfg.App.NoteServiceName,
		Title:     "note mongodb example",
		Content:   "created by mongox.DocumentStore UpsertByID",
		CreatedAt: now,
		UpdatedAt: now,
	}

	// UpsertByID：按 _id 保存文档。存在则整体替换，不存在则新增。
	if _, err := examples.UpsertByID(ctx, document.ID, document); err != nil {
		return nil, fmt.Errorf("upsert example document: %w", err)
	}

	// FindByID：按 _id 查询单条文档。未找到时会返回 mongox.ErrNotFound。
	if _, err := examples.FindByID(ctx, document.ID); err != nil {
		return nil, fmt.Errorf("find example document: %w", err)
	}

	// UpdateOne：演示局部更新，适合只更新少量字段的业务场景。
	if _, err := examples.UpdateOne(ctx, bson.M{"_id": document.ID}, bson.M{
		"$set": bson.M{
			"content":    "updated by mongox.DocumentStore UpdateOne",
			"updated_at": time.Now().UTC(),
		},
	}); err != nil {
		return nil, fmt.Errorf("update example document: %w", err)
	}

	// Count：演示统计当前 note 服务写入的示例文档数量。
	count, err := examples.Count(ctx, bson.M{"service": cfg.App.NoteServiceName})
	if err != nil {
		return nil, fmt.Errorf("count example documents: %w", err)
	}
	log.Info("note mongodb example completed",
		zap.String("collection", noteMongoExampleCollection),
		zap.String("document_id", document.ID),
		zap.Int64("service_document_count", count),
	)

	return examples.FindByID(ctx, document.ID)
}
