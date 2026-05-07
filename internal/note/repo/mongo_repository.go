package repo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.uber.org/zap"

	"github.com/BwCloudWeGo/bw-cli/internal/note/model"
	"github.com/BwCloudWeGo/bw-cli/pkg/mongox"
)

const noteCollectionName = "notes"

// NoteDocument 是 note 聚合在 MongoDB 中的文档结构。
// 该结构只属于 repo 层，用 bson tag 描述持久化字段，避免数据库字段污染领域模型。
type NoteDocument struct {
	ID          string     `bson:"_id"`
	AuthorID    string     `bson:"author_id"`
	Title       string     `bson:"title"`
	Content     string     `bson:"content"`
	Status      int32      `bson:"status"`
	NoteType    int32      `bson:"note_type"`
	Permission  int32      `bson:"permission"`
	Remark      string     `bson:"remark"`
	TopicIDs    []string   `bson:"topic_ids"`
	PublishedAt *time.Time `bson:"published_at,omitempty"`
	CreatedAt   time.Time  `bson:"created_at"`
	UpdatedAt   time.Time  `bson:"updated_at"`
}

// MongoCollectionName 声明 NoteDocument 对应的 MongoDB 集合名称。
// 业务仓储通过 mongox.NewDocumentStore[NoteDocument] 创建公共文档操作类时，
// 会自动读取该集合名称，避免每个业务仓储重复手写 NewCollection 和集合名传参。
func (NoteDocument) MongoCollectionName() string {
	return noteCollectionName
}

// MongoRepository 通过公共 MongoDB 操作类持久化 note 聚合。
// 它实现 model.Repository，service 层只依赖接口，不关心底层使用 MongoDB 还是其他数据库。
type MongoRepository struct {
	notes mongox.DocumentSaverFinder[NoteDocument]
	log   *zap.Logger
}

// NewMongoRepository 使用配置好的 MongoDB 数据库创建 note 仓储。
// 集合名称由 NoteDocument.MongoCollectionName 提供，业务只需要传入文档结构体类型。
func NewMongoRepository(db *mongo.Database, loggers ...*zap.Logger) *MongoRepository {
	log := optionalLogger(loggers...)
	return NewMongoRepositoryWithStore(mongox.NewDocumentStore[NoteDocument](db, log), log)
}

// NewMongoRepositoryWithStore 用于测试时注入集合操作实现。
// 生产代码通常调用 NewMongoRepository 即可。
func NewMongoRepositoryWithStore(store mongox.DocumentSaverFinder[NoteDocument], loggers ...*zap.Logger) *MongoRepository {
	return &MongoRepository{notes: store, log: optionalLogger(loggers...)}
}

// Save 保留通用仓储接口方法，内部复用 MongoDB 入库操作。
func (r *MongoRepository) Save(ctx context.Context, note *model.Note) error {
	start := time.Now()
	_, err := r.notes.UpsertByID(ctx, note.ID, toNoteDocument(note))
	r.logOperation("Save", note.ID, start, err)
	return err
}

// FindByID 根据业务 ID 从 MongoDB 加载 note 聚合。
// 公共 mongox.ErrNotFound 会在这里转换成领域错误 model.ErrNoteNotFound。
func (r *MongoRepository) FindByID(ctx context.Context, id string) (*model.Note, error) {
	start := time.Now()
	document, err := r.notes.FindByID(ctx, id)
	if errors.Is(err, mongox.ErrNotFound) {
		err = model.ErrNoteNotFound
	}
	r.logOperation("FindByID", id, start, err)
	if err != nil {
		return nil, err
	}
	return toNoteFromDocument(document), nil
}

func optionalLogger(loggers ...*zap.Logger) *zap.Logger {
	if len(loggers) > 0 && loggers[0] != nil {
		return loggers[0]
	}
	return zap.NewNop()
}

func (r *MongoRepository) logOperation(operation string, noteID string, start time.Time, err error) {
	fields := []zap.Field{
		zap.String("repository", "note_mongo"),
		zap.String("operation", operation),
		zap.String("note_id", noteID),
		zap.Float64("latency_ms", float64(time.Since(start).Microseconds())/1000),
	}
	if err != nil {
		fields = append(fields, zap.Error(err))
		r.log.Warn("note mongodb repository operation completed with error", fields...)
		return
	}
	r.log.Info("note mongodb repository operation completed", fields...)
}

func toNoteDocument(note *model.Note) *NoteDocument {
	return &NoteDocument{
		ID:          note.ID,
		AuthorID:    note.AuthorID,
		Title:       note.Title,
		Content:     note.Content,
		Status:      note.Status.Code(),
		NoteType:    note.NoteType,
		Permission:  note.Permission,
		Remark:      note.Remark,
		TopicIDs:    note.TopicIDs,
		PublishedAt: note.PublishedAt,
		CreatedAt:   note.CreatedAt,
		UpdatedAt:   note.UpdatedAt,
	}
}

func toNoteFromDocument(document *NoteDocument) *model.Note {
	return &model.Note{
		ID:          document.ID,
		AuthorID:    document.AuthorID,
		Title:       document.Title,
		Content:     document.Content,
		Status:      model.NoteStatusFromCode(document.Status),
		NoteType:    document.NoteType,
		Permission:  document.Permission,
		Remark:      document.Remark,
		TopicIDs:    document.TopicIDs,
		PublishedAt: document.PublishedAt,
		CreatedAt:   document.CreatedAt,
		UpdatedAt:   document.UpdatedAt,
	}
}
