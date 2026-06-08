package repo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"

	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
	dbmodel "github.com/BwCloudWeGo/bw-cli/internal/order/model"
	"github.com/BwCloudWeGo/bw-cli/pkg/mongox"
)

// MongoRepository 使用共享 mongox 文档存储持久化 order 聚合。
// 它实现 entity.Repository，可在不修改 service 代码的情况下替换 GormRepository。
type MongoRepository struct {
	documents *mongox.DocumentStore[dbmodel.OrderDocument]
	log       *zap.Logger
}

// NewMongoRepository 使用配置好的数据库创建 MongoDB 仓储。
func NewMongoRepository(db *mongo.Database, loggers ...*zap.Logger) *MongoRepository {
	log := zap.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		log = loggers[0]
	}
	return &MongoRepository{
		documents: mongox.NewDocumentStore[dbmodel.OrderDocument](db, log),
		log:       log,
	}
}

// Save 按 MongoDB _id 新增或更新 order 聚合。
func (r *MongoRepository) Save(ctx context.Context, item *entity.Order) error {
	start := time.Now()
	_, err := r.documents.UpsertByID(ctx, item.ID, toDocument(item))
	r.logOperation("Save", item.ID, 0, start, err)
	return err
}

// FindByID 按 MongoDB _id 加载 order 聚合。
func (r *MongoRepository) FindByID(ctx context.Context, id string) (*entity.Order, error) {
	start := time.Now()
	document, err := r.documents.FindByID(ctx, id)
	if errors.Is(err, mongox.ErrNotFound) {
		err = entity.ErrOrderNotFound
	}
	r.logOperation("FindByID", id, 0, start, err)
	if err != nil {
		return nil, err
	}
	return toDomainFromDocument(document), nil
}

// List 按创建时间排序加载分页 order 聚合。
func (r *MongoRepository) List(ctx context.Context, offset int, limit int) ([]*entity.Order, int64, error) {
	start := time.Now()
	filter := bson.M{}
	total, err := r.documents.Count(ctx, filter)
	if err != nil {
		r.logOperation("Count", "", 0, start, err)
		return nil, 0, err
	}

	documents, err := r.documents.FindMany(ctx, filter,
		options.Find().
			SetSort(bson.D{{Key: "created_at", Value: -1}}).
			SetSkip(int64(offset)).
			SetLimit(int64(limit)),
	)
	if err != nil {
		r.logOperation("List", "", total, start, err)
		return nil, 0, err
	}

	items := make([]*entity.Order, 0, len(documents))
	for i := range documents {
		items = append(items, toDomainFromDocument(&documents[i]))
	}
	r.logOperation("List", "", total, start, nil)
	return items, total, nil
}

// Delete 按 MongoDB _id 删除 order 聚合。
func (r *MongoRepository) Delete(ctx context.Context, id string) error {
	start := time.Now()
	result, err := r.documents.DeleteByID(ctx, id)
	if err == nil && result != nil && result.DeletedCount == 0 {
		err = entity.ErrOrderNotFound
	}
	r.logOperation("Delete", id, 0, start, err)
	return err
}

func (r *MongoRepository) logOperation(operation string, id string, total int64, start time.Time, err error) {
	fields := []zap.Field{
		zap.String("repository", "order_mongo"),
		zap.String("operation", operation),
		zap.String("aggregate_id", id),
		zap.Int64("total", total),
		zap.Float64("latency_ms", float64(time.Since(start).Microseconds())/1000),
	}
	if err != nil {
		fields = append(fields, zap.Error(err))
		r.log.Warn("mongodb repository operation completed with error", fields...)
		return
	}
	r.log.Info("mongodb repository operation completed", fields...)
}

func toDocument(item *entity.Order) *dbmodel.OrderDocument {
	return &dbmodel.OrderDocument{
		ID:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func toDomainFromDocument(document *dbmodel.OrderDocument) *entity.Order {
	return &entity.Order{
		ID:          document.ID,
		Name:        document.Name,
		Description: document.Description,
		CreatedAt:   document.CreatedAt,
		UpdatedAt:   document.UpdatedAt,
	}
}

var _ entity.Repository = (*MongoRepository)(nil)
