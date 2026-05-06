package mongox

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"
)

// ErrNotFound 表示 MongoDB 查询没有命中任何文档。
// 公共封装会把官方 driver 的 mongo.ErrNoDocuments 统一转换成这个错误，
// 业务仓储层只需要判断 mongox.ErrNotFound，再映射成自己的领域错误即可。
var ErrNotFound = errors.New("mongodb document not found")

type collectionOperator interface {
	InsertOne(context.Context, any, ...options.Lister[options.InsertOneOptions]) (*mongo.InsertOneResult, error)
	ReplaceOne(context.Context, any, any, ...options.Lister[options.ReplaceOptions]) (*mongo.UpdateResult, error)
	FindOne(context.Context, any, ...options.Lister[options.FindOneOptions]) *mongo.SingleResult
	Find(context.Context, any, ...options.Lister[options.FindOptions]) (*mongo.Cursor, error)
	UpdateOne(context.Context, any, any, ...options.Lister[options.UpdateOneOptions]) (*mongo.UpdateResult, error)
	DeleteOne(context.Context, any, ...options.Lister[options.DeleteOneOptions]) (*mongo.DeleteResult, error)
	CountDocuments(context.Context, any, ...options.Lister[options.CountOptions]) (int64, error)
}

// Collection 是面向业务仓储层的 MongoDB 集合操作类。
//
// T 是集合文档结构体类型，通常定义在具体业务的 repo 包中，并通过 bson tag
// 声明 MongoDB 字段名称。业务代码应优先依赖这个类提供的通用 CRUD 方法，
// 避免在 handler 或 service 中散落调用官方 driver，从而保持分层清晰。
type Collection[T any] struct {
	name      string
	operation collectionOperator
	log       *zap.Logger
}

// NewCollection 根据 mongo.Database 和集合名称创建一个强类型集合操作类。
//
// loggers 是可选参数；传入 zap.Logger 后，每次 MongoDB 操作都会记录数据源、
// collection、operation、耗时、影响数量和错误信息，方便排查线上慢查询或失败调用。
func NewCollection[T any](db *mongo.Database, name string, loggers ...*zap.Logger) *Collection[T] {
	return newCollectionWithOperator[T](name, db.Collection(name), loggers...)
}

func newCollectionWithOperator[T any](name string, operation collectionOperator, loggers ...*zap.Logger) *Collection[T] {
	log := zap.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		log = loggers[0]
	}
	return &Collection[T]{name: name, operation: operation, log: log}
}

// Insert 插入单条文档，并返回 MongoDB driver 的 InsertOneResult。
//
// 适用于明确只做新增的场景。如果业务希望“有则更新、无则新增”，
// 应使用 UpsertByID 或 UpdateOne 搭配 upsert 选项。
func (c *Collection[T]) Insert(ctx context.Context, document *T, opts ...options.Lister[options.InsertOneOptions]) (*mongo.InsertOneResult, error) {
	start := time.Now()
	result, err := c.operation.InsertOne(ctx, document, opts...)
	fields := []zap.Field{}
	if result != nil {
		fields = append(fields, zap.Any("inserted_id", result.InsertedID))
	}
	c.logOperation("Insert", start, err, fields...)
	return result, err
}

// UpsertByID 按 _id 保存文档。
//
// 当 _id 已存在时，会整体替换旧文档；当 _id 不存在时，会插入新文档。
// 这个方法适合用业务主键作为 MongoDB _id 的场景，例如 note.ID、order.ID。
func (c *Collection[T]) UpsertByID(ctx context.Context, id any, document *T, opts ...options.Lister[options.ReplaceOptions]) (*mongo.UpdateResult, error) {
	replaceOpts := append([]options.Lister[options.ReplaceOptions]{options.Replace().SetUpsert(true)}, opts...)
	return c.ReplaceByID(ctx, id, document, replaceOpts...)
}

// ReplaceByID 按 _id 整体替换一条已有文档。
//
// 它不会默认开启 upsert。如果需要不存在时自动新增，请使用 UpsertByID。
func (c *Collection[T]) ReplaceByID(ctx context.Context, id any, document *T, opts ...options.Lister[options.ReplaceOptions]) (*mongo.UpdateResult, error) {
	return c.ReplaceOne(ctx, bson.M{"_id": id}, document, opts...)
}

// ReplaceOne 按自定义 filter 整体替换一条文档。
//
// filter 可以是 bson.M、bson.D 或其他 driver 支持的 BSON 结构。
// replacement 会作为完整文档写入，因此调用方要确保字段完整，避免误删旧字段。
func (c *Collection[T]) ReplaceOne(ctx context.Context, filter any, document *T, opts ...options.Lister[options.ReplaceOptions]) (*mongo.UpdateResult, error) {
	start := time.Now()
	result, err := c.operation.ReplaceOne(ctx, filter, document, opts...)
	fields := updateResultFields(result)
	c.logOperation("ReplaceOne", start, err, fields...)
	return result, err
}

// FindByID 按 MongoDB _id 查询单条文档。
//
// 查询不到时返回 ErrNotFound；查询成功时返回解码后的 T 指针。
func (c *Collection[T]) FindByID(ctx context.Context, id any, opts ...options.Lister[options.FindOneOptions]) (*T, error) {
	return c.FindOne(ctx, bson.M{"_id": id}, opts...)
}

// FindOne 按自定义 filter 查询单条文档。
//
// 查询不到时会把 mongo.ErrNoDocuments 转换为 ErrNotFound，
// 这样业务层不需要直接依赖 MongoDB driver 的错误类型。
func (c *Collection[T]) FindOne(ctx context.Context, filter any, opts ...options.Lister[options.FindOneOptions]) (*T, error) {
	start := time.Now()
	var document T
	err := c.operation.FindOne(ctx, filter, opts...).Decode(&document)
	err = normalizeFindErr(err)
	c.logOperation("FindOne", start, err)
	if err != nil {
		return nil, err
	}
	return &document, nil
}

// FindMany 按 filter 查询多条文档。
//
// opts 可传入 options.Find().SetSort(...).SetLimit(...) 等分页和排序参数。
// 方法内部会负责关闭 cursor，并把查询结果解码成 []T 返回。
func (c *Collection[T]) FindMany(ctx context.Context, filter any, opts ...options.Lister[options.FindOptions]) ([]T, error) {
	start := time.Now()
	cursor, err := c.operation.Find(ctx, filter, opts...)
	if err != nil {
		c.logOperation("FindMany", start, err)
		return nil, err
	}
	defer cursor.Close(ctx)

	var documents []T
	err = cursor.All(ctx, &documents)
	c.logOperation("FindMany", start, err, zap.Int("documents", len(documents)))
	if err != nil {
		return nil, err
	}
	return documents, nil
}

// UpdateOne 按 filter 局部更新一条文档。
//
// update 通常传入 bson.M{"$set": ...}、bson.M{"$inc": ...} 等更新表达式。
// 如果需要 upsert，可以通过 options.UpdateOne().SetUpsert(true) 传入。
func (c *Collection[T]) UpdateOne(ctx context.Context, filter any, update any, opts ...options.Lister[options.UpdateOneOptions]) (*mongo.UpdateResult, error) {
	start := time.Now()
	result, err := c.operation.UpdateOne(ctx, filter, update, opts...)
	fields := updateResultFields(result)
	c.logOperation("UpdateOne", start, err, fields...)
	return result, err
}

// DeleteByID 按 _id 删除一条文档。
//
// 适用于真正物理删除。若业务要求保留历史记录，应使用 UpdateOne 做软删除。
func (c *Collection[T]) DeleteByID(ctx context.Context, id any, opts ...options.Lister[options.DeleteOneOptions]) (*mongo.DeleteResult, error) {
	return c.DeleteOne(ctx, bson.M{"_id": id}, opts...)
}

// DeleteOne 按自定义 filter 删除一条文档。
//
// 调用方应保证 filter 足够精确，避免误删非目标数据。
func (c *Collection[T]) DeleteOne(ctx context.Context, filter any, opts ...options.Lister[options.DeleteOneOptions]) (*mongo.DeleteResult, error) {
	start := time.Now()
	result, err := c.operation.DeleteOne(ctx, filter, opts...)
	fields := []zap.Field{}
	if result != nil {
		fields = append(fields, zap.Int64("deleted_count", result.DeletedCount))
	}
	c.logOperation("DeleteOne", start, err, fields...)
	return result, err
}

// Count 统计匹配 filter 的文档数量。
//
// 大集合上统计可能产生较高成本，生产环境应配合索引和合理的查询条件使用。
func (c *Collection[T]) Count(ctx context.Context, filter any, opts ...options.Lister[options.CountOptions]) (int64, error) {
	start := time.Now()
	count, err := c.operation.CountDocuments(ctx, filter, opts...)
	c.logOperation("Count", start, err, zap.Int64("count", count))
	return count, err
}

func normalizeFindErr(err error) error {
	if errors.Is(err, mongo.ErrNoDocuments) {
		return ErrNotFound
	}
	return err
}

func updateResultFields(result *mongo.UpdateResult) []zap.Field {
	if result == nil {
		return nil
	}
	return []zap.Field{
		zap.Int64("matched_count", result.MatchedCount),
		zap.Int64("modified_count", result.ModifiedCount),
		zap.Int64("upserted_count", result.UpsertedCount),
		zap.Any("upserted_id", result.UpsertedID),
	}
}

func (c *Collection[T]) logOperation(operation string, start time.Time, err error, fields ...zap.Field) {
	fields = append([]zap.Field{
		zap.String("datasource", "mongodb"),
		zap.String("collection", c.name),
		zap.String("operation", operation),
		zap.Float64("latency_ms", float64(time.Since(start).Microseconds())/1000),
	}, fields...)
	if err != nil {
		fields = append(fields, zap.Error(err))
		c.log.Warn("mongodb operation completed with error", fields...)
		return
	}
	c.log.Info("mongodb operation completed", fields...)
}
