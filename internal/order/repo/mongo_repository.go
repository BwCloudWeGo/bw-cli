package repo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"

	"github.com/BwCloudWeGo/bw-cli/internal/order/model"
	"github.com/BwCloudWeGo/bw-cli/pkg/mongox"
)

const orderMongoCollectionName = "orders"

// OrderDocument is the MongoDB document for the order aggregate.
// Keep BSON tags in repo layer only, so the domain model stays storage-agnostic.
type OrderDocument struct {
	ID          string    `bson:"_id"`
	Name        string    `bson:"name"`
	Description string    `bson:"description"`
	CreatedAt   time.Time `bson:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at"`
}

// MongoCollectionName declares the MongoDB collection for OrderDocument.
// mongox.NewDocumentStore reads this value, so business repositories do not need
// to repeat NewCollection boilerplate for every service.
func (OrderDocument) MongoCollectionName() string {
	return orderMongoCollectionName
}

// MongoRepository persists order aggregates with the shared mongox DocumentStore.
// It implements model.Repository and can replace GormRepository without changing service code.
type MongoRepository struct {
	documents *mongox.DocumentStore[OrderDocument]
	log       *zap.Logger
}

// NewMongoRepository constructs a MongoDB repository using the configured database.
func NewMongoRepository(db *mongo.Database, loggers ...*zap.Logger) *MongoRepository {
	log := zap.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		log = loggers[0]
	}
	return &MongoRepository{
		documents: mongox.NewDocumentStore[OrderDocument](db, log),
		log:       log,
	}
}

// Save inserts or updates an order aggregate by MongoDB _id.
func (r *MongoRepository) Save(ctx context.Context, item *model.Order) error {
	start := time.Now()
	_, err := r.documents.UpsertByID(ctx, item.ID, toDocument(item))
	r.logOperation("Save", item.ID, 0, start, err)
	return err
}

// FindByID loads an order aggregate by MongoDB _id.
func (r *MongoRepository) FindByID(ctx context.Context, id string) (*model.Order, error) {
	start := time.Now()
	document, err := r.documents.FindByID(ctx, id)
	if errors.Is(err, mongox.ErrNotFound) {
		err = model.ErrOrderNotFound
	}
	r.logOperation("FindByID", id, 0, start, err)
	if err != nil {
		return nil, err
	}
	return toDomainFromDocument(document), nil
}

// List loads paginated order aggregates ordered by creation time.
func (r *MongoRepository) List(ctx context.Context, offset int, limit int) ([]*model.Order, int64, error) {
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

	items := make([]*model.Order, 0, len(documents))
	for i := range documents {
		items = append(items, toDomainFromDocument(&documents[i]))
	}
	r.logOperation("List", "", total, start, nil)
	return items, total, nil
}

// Delete removes an order aggregate by MongoDB _id.
func (r *MongoRepository) Delete(ctx context.Context, id string) error {
	start := time.Now()
	result, err := r.documents.DeleteByID(ctx, id)
	if err == nil && result != nil && result.DeletedCount == 0 {
		err = model.ErrOrderNotFound
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

func toDocument(item *model.Order) *OrderDocument {
	return &OrderDocument{
		ID:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func toDomainFromDocument(document *OrderDocument) *model.Order {
	return &model.Order{
		ID:          document.ID,
		Name:        document.Name,
		Description: document.Description,
		CreatedAt:   document.CreatedAt,
		UpdatedAt:   document.UpdatedAt,
	}
}

var _ model.Repository = (*MongoRepository)(nil)
