package mongox

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"
)

// CollectionNamed 由业务 MongoDB 文档结构体实现，用来声明该文档对应的集合名称。
// 业务侧只需要在文档结构体上实现这个方法，就可以通过 NewDocumentStore 自动创建集合操作类。
type CollectionNamed interface {
	MongoCollectionName() string
}

// DocumentSaverFinder 是业务仓储最常用的保存和按 ID 查询能力。
// 具体业务 repo 可以依赖这个公共小接口做单元测试，避免每个服务重复定义自己的 Store 接口。
type DocumentSaverFinder[T any] interface {
	UpsertByID(ctx context.Context, id any, document *T, opts ...options.Lister[options.ReplaceOptions]) (*mongo.UpdateResult, error)
	FindByID(ctx context.Context, id any, opts ...options.Lister[options.FindOneOptions]) (*T, error)
}

// DocumentStore 是面向业务 repo 层的通用 MongoDB 文档仓储。
//
// T 是业务在 repo 包中定义的 MongoDB 文档结构体。DocumentStore 内部复用 Collection，
// 并把 Collection 已封装好的所有 CRUD 方法再次收口到公共仓储类型中，业务仓储不需要
// 为每个服务重复编写 NewCollection、Store 接口或基础 CRUD 委托代码。
type DocumentStore[T any] struct {
	collection *Collection[T]
}

// NewDocumentStore 根据文档结构体声明的集合名称创建通用文档仓储。
//
// 使用方式：
//
//	type NoteDocument struct { ... }
//	func (NoteDocument) MongoCollectionName() string { return "notes" }
//	notes := mongox.NewDocumentStore[NoteDocument](db, log)
func NewDocumentStore[T CollectionNamed](db *mongo.Database, loggers ...*zap.Logger) *DocumentStore[T] {
	var document T
	return NewNamedDocumentStore[T](db, document.MongoCollectionName(), loggers...)
}

// NewNamedDocumentStore 根据显式集合名称创建通用文档仓储。
// 当业务暂时不方便让文档结构体实现 MongoCollectionName 时，可以使用这个构造函数。
func NewNamedDocumentStore[T any](db *mongo.Database, collectionName string, loggers ...*zap.Logger) *DocumentStore[T] {
	return &DocumentStore[T]{collection: NewCollection[T](db, collectionName, loggers...)}
}

func newDocumentStoreWithCollection[T any](collection *Collection[T]) *DocumentStore[T] {
	return &DocumentStore[T]{collection: collection}
}

// Collection 返回底层公共集合操作类，供少量需要高级能力的 repo 层扩展使用。
// 常规 CRUD 优先直接调用 DocumentStore 的方法。
func (s *DocumentStore[T]) Collection() *Collection[T] {
	return s.collection
}

// Insert 插入单条文档。
func (s *DocumentStore[T]) Insert(ctx context.Context, document *T, opts ...options.Lister[options.InsertOneOptions]) (*mongo.InsertOneResult, error) {
	return s.collection.Insert(ctx, document, opts...)
}

// UpsertByID 按 _id 保存文档，存在则替换，不存在则新增。
func (s *DocumentStore[T]) UpsertByID(ctx context.Context, id any, document *T, opts ...options.Lister[options.ReplaceOptions]) (*mongo.UpdateResult, error) {
	return s.collection.UpsertByID(ctx, id, document, opts...)
}

// ReplaceByID 按 _id 整体替换一条已有文档。
func (s *DocumentStore[T]) ReplaceByID(ctx context.Context, id any, document *T, opts ...options.Lister[options.ReplaceOptions]) (*mongo.UpdateResult, error) {
	return s.collection.ReplaceByID(ctx, id, document, opts...)
}

// ReplaceOne 按自定义 filter 整体替换一条文档。
func (s *DocumentStore[T]) ReplaceOne(ctx context.Context, filter any, document *T, opts ...options.Lister[options.ReplaceOptions]) (*mongo.UpdateResult, error) {
	return s.collection.ReplaceOne(ctx, filter, document, opts...)
}

// FindByID 按 MongoDB _id 查询单条文档。
func (s *DocumentStore[T]) FindByID(ctx context.Context, id any, opts ...options.Lister[options.FindOneOptions]) (*T, error) {
	return s.collection.FindByID(ctx, id, opts...)
}

// FindOne 按自定义 filter 查询单条文档。
func (s *DocumentStore[T]) FindOne(ctx context.Context, filter any, opts ...options.Lister[options.FindOneOptions]) (*T, error) {
	return s.collection.FindOne(ctx, filter, opts...)
}

// FindMany 按 filter 查询多条文档。
func (s *DocumentStore[T]) FindMany(ctx context.Context, filter any, opts ...options.Lister[options.FindOptions]) ([]T, error) {
	return s.collection.FindMany(ctx, filter, opts...)
}

// UpdateOne 按 filter 局部更新一条文档。
func (s *DocumentStore[T]) UpdateOne(ctx context.Context, filter any, update any, opts ...options.Lister[options.UpdateOneOptions]) (*mongo.UpdateResult, error) {
	return s.collection.UpdateOne(ctx, filter, update, opts...)
}

// DeleteByID 按 _id 删除一条文档。
func (s *DocumentStore[T]) DeleteByID(ctx context.Context, id any, opts ...options.Lister[options.DeleteOneOptions]) (*mongo.DeleteResult, error) {
	return s.collection.DeleteByID(ctx, id, opts...)
}

// DeleteOne 按自定义 filter 删除一条文档。
func (s *DocumentStore[T]) DeleteOne(ctx context.Context, filter any, opts ...options.Lister[options.DeleteOneOptions]) (*mongo.DeleteResult, error) {
	return s.collection.DeleteOne(ctx, filter, opts...)
}

// Count 统计匹配 filter 的文档数量。
func (s *DocumentStore[T]) Count(ctx context.Context, filter any, opts ...options.Lister[options.CountOptions]) (int64, error) {
	return s.collection.Count(ctx, filter, opts...)
}
