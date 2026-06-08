package repo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.uber.org/zap"

	"github.com/BwCloudWeGo/bw-cli/internal/note/entity"
	dbmodel "github.com/BwCloudWeGo/bw-cli/internal/note/model"
	"github.com/BwCloudWeGo/bw-cli/pkg/mongox"
)

// MongoRepository 通过公共 MongoDB 操作类持久化 note 聚合。
// 它实现 entity.Repository，service 层只依赖接口，不关心底层使用 MongoDB 还是其他数据库。
type MongoRepository struct {
	notes mongox.DocumentSaverFinder[dbmodel.NoteDocument]
	log   *zap.Logger
}

// NewMongoRepository 使用配置好的 MongoDB 数据库创建 note 仓储。
// 集合名称由 NoteDocument.MongoCollectionName 提供，业务只需要传入文档结构体类型。
func NewMongoRepository(db *mongo.Database, loggers ...*zap.Logger) *MongoRepository {
	log := optionalLogger(loggers...)
	return NewMongoRepositoryWithStore(mongox.NewDocumentStore[dbmodel.NoteDocument](db, log), log)
}

// NewMongoRepositoryWithStore 用于测试时注入集合操作实现。
// 生产代码通常调用 NewMongoRepository 即可。
func NewMongoRepositoryWithStore(store mongox.DocumentSaverFinder[dbmodel.NoteDocument], loggers ...*zap.Logger) *MongoRepository {
	return &MongoRepository{notes: store, log: optionalLogger(loggers...)}
}

// Save 保留通用仓储接口方法，内部复用 MongoDB 入库操作。
func (r *MongoRepository) Save(ctx context.Context, note *entity.Note) error {
	start := time.Now()
	_, err := r.notes.UpsertByID(ctx, note.ID, toNoteDocument(note))
	r.logOperation("Save", note.ID, start, err)
	return err
}

// FindByID 根据业务 ID 从 MongoDB 加载 note 聚合。
// 公共 mongox.ErrNotFound 会在这里转换成领域错误 entity.ErrNoteNotFound。
func (r *MongoRepository) FindByID(ctx context.Context, id string) (*entity.Note, error) {
	start := time.Now()
	document, err := r.notes.FindByID(ctx, id)
	if errors.Is(err, mongox.ErrNotFound) {
		err = entity.ErrNoteNotFound
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

func toNoteDocument(note *entity.Note) *dbmodel.NoteDocument {
	return &dbmodel.NoteDocument{
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

func toNoteFromDocument(document *dbmodel.NoteDocument) *entity.Note {
	return &entity.Note{
		ID:          document.ID,
		AuthorID:    document.AuthorID,
		Title:       document.Title,
		Content:     document.Content,
		Status:      entity.NoteStatusFromCode(document.Status),
		NoteType:    document.NoteType,
		Permission:  document.Permission,
		Remark:      document.Remark,
		TopicIDs:    document.TopicIDs,
		PublishedAt: document.PublishedAt,
		CreatedAt:   document.CreatedAt,
		UpdatedAt:   document.UpdatedAt,
	}
}
