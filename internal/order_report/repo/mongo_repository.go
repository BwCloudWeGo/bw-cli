package repo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"

	"github.com/BwCloudWeGo/bw-cli/internal/order_report/model"
	"github.com/BwCloudWeGo/bw-cli/pkg/mongox"
)

const orderReportMongoCollectionName = "demo_orders"

// OrderReportDocument is the MongoDB document for the order_report aggregate.
type OrderReportDocument struct {
	ID           int32     `bson:"_id"`
	CustomerName string    `bson:"customer_name"`
	Status       string    `bson:"status"`
	TotalAmount  string    `bson:"total_amount"`
	CreatedAt    time.Time `bson:"created_at"`
	UpdatedAt    time.Time `bson:"updated_at"`
}

func (OrderReportDocument) MongoCollectionName() string {
	return orderReportMongoCollectionName
}

// MongoRepository persists order_report aggregates with the shared mongox DocumentStore.
type MongoRepository struct {
	documents *mongox.DocumentStore[OrderReportDocument]
	log       *zap.Logger
}

func NewMongoRepository(db *mongo.Database, loggers ...*zap.Logger) *MongoRepository {
	log := zap.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		log = loggers[0]
	}
	return &MongoRepository{documents: mongox.NewDocumentStore[OrderReportDocument](db, log), log: log}
}

func (r *MongoRepository) Save(ctx context.Context, item *model.OrderReport) error {
	start := time.Now()
	_, err := r.documents.UpsertByID(ctx, item.ID, toDocument(item))
	r.logOperation("Save", item.ID, 0, start, err)
	return err
}

func (r *MongoRepository) FindByID(ctx context.Context, id int32) (*model.OrderReport, error) {
	start := time.Now()
	document, err := r.documents.FindByID(ctx, id)
	if errors.Is(err, mongox.ErrNotFound) {
		err = model.ErrOrderReportNotFound
	}
	r.logOperation("FindByID", id, 0, start, err)
	if err != nil {
		return nil, err
	}
	return toDomainFromDocument(document), nil
}

func (r *MongoRepository) List(ctx context.Context, offset int, limit int) ([]*model.OrderReport, int64, error) {
	start := time.Now()
	filter := bson.M{}
	total, err := r.documents.Count(ctx, filter)
	if err != nil {
		r.logOperation("Count", 0, 0, start, err)
		return nil, 0, err
	}
	documents, err := r.documents.FindMany(ctx, filter, options.Find().SetSkip(int64(offset)).SetLimit(int64(limit)))
	if err != nil {
		r.logOperation("List", 0, total, start, err)
		return nil, 0, err
	}
	items := make([]*model.OrderReport, 0, len(documents))
	for i := range documents {
		items = append(items, toDomainFromDocument(&documents[i]))
	}
	r.logOperation("List", 0, total, start, nil)
	return items, total, nil
}

func (r *MongoRepository) Delete(ctx context.Context, id int32) error {
	start := time.Now()
	result, err := r.documents.DeleteByID(ctx, id)
	if err == nil && result != nil && result.DeletedCount == 0 {
		err = model.ErrOrderReportNotFound
	}
	r.logOperation("Delete", id, 0, start, err)
	return err
}

func (r *MongoRepository) logOperation(operation string, id int32, total int64, start time.Time, err error) {
	fields := []zap.Field{
		zap.String("repository", "order_report_mongo"),
		zap.String("operation", operation),
		zap.Any("aggregate_id", id),
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

func toDocument(item *model.OrderReport) *OrderReportDocument {
	return &OrderReportDocument{
		ID:           item.ID,
		CustomerName: item.CustomerName,
		Status:       item.Status,
		TotalAmount:  item.TotalAmount,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
}

func toDomainFromDocument(document *OrderReportDocument) *model.OrderReport {
	return &model.OrderReport{
		ID:           document.ID,
		CustomerName: document.CustomerName,
		Status:       document.Status,
		TotalAmount:  document.TotalAmount,
		CreatedAt:    document.CreatedAt,
		UpdatedAt:    document.UpdatedAt,
	}
}

var _ model.Repository = (*MongoRepository)(nil)
