package repo

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/BwCloudWeGo/bw-cli/internal/order_report/model"
)

// OrderReportModel is the Gorm persistence model for the demo_orders table.
type OrderReportModel struct {
	ID           int32     `gorm:"column:id;primaryKey"`
	CustomerName string    `gorm:"column:customer_name"`
	Status       string    `gorm:"column:status"`
	TotalAmount  string    `gorm:"column:total_amount"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (OrderReportModel) TableName() string {
	return "demo_orders"
}

// GormRepository persists order_report aggregates with Gorm.
type GormRepository struct {
	db  *gorm.DB
	log *zap.Logger
}

// NewGormRepository constructs a order_report repository with optional structured logging.
func NewGormRepository(db *gorm.DB, loggers ...*zap.Logger) *GormRepository {
	log := zap.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		log = loggers[0]
	}
	return &GormRepository{db: db, log: log}
}

// AutoMigrate is intentionally a no-op for table-driven services because the table already exists.
func AutoMigrate(db *gorm.DB) error {
	return nil
}

// Save inserts or updates a order_report aggregate.
func (r *GormRepository) Save(ctx context.Context, item *model.OrderReport) error {
	start := time.Now()
	tx := r.db.WithContext(ctx).Save(toRecord(item))
	r.logOperation("Save", tx.RowsAffected, start, tx.Error)
	return tx.Error
}

// FindByID loads a order_report aggregate by id.
func (r *GormRepository) FindByID(ctx context.Context, id int32) (*model.OrderReport, error) {
	start := time.Now()
	var record OrderReportModel
	tx := r.db.WithContext(ctx).Where("id = ?", id).First(&record)
	err := tx.Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = model.ErrOrderReportNotFound
	}
	if err != nil {
		r.logOperation("FindByID", tx.RowsAffected, start, err)
		return nil, err
	}
	r.logOperation("FindByID", tx.RowsAffected, start, nil)
	return toDomain(&record), nil
}

// List loads paginated order_report aggregates.
func (r *GormRepository) List(ctx context.Context, offset int, limit int) ([]*model.OrderReport, int64, error) {
	start := time.Now()
	var total int64
	countTx := r.db.WithContext(ctx).Model(&OrderReportModel{}).Count(&total)
	if countTx.Error != nil {
		r.logOperation("Count", countTx.RowsAffected, start, countTx.Error)
		return nil, 0, countTx.Error
	}
	var records []OrderReportModel
	tx := r.db.WithContext(ctx).
		Order("id desc").
		Offset(offset).
		Limit(limit).
		Find(&records)
	if tx.Error != nil {
		r.logOperation("List", tx.RowsAffected, start, tx.Error)
		return nil, 0, tx.Error
	}
	items := make([]*model.OrderReport, 0, len(records))
	for i := range records {
		items = append(items, toDomain(&records[i]))
	}
	r.logOperation("List", tx.RowsAffected, start, nil)
	return items, total, nil
}

// Delete removes a order_report aggregate by id.
func (r *GormRepository) Delete(ctx context.Context, id int32) error {
	start := time.Now()
	tx := r.db.WithContext(ctx).Where("id = ?", id).Delete(&OrderReportModel{})
	err := tx.Error
	if err == nil && tx.RowsAffected == 0 {
		err = model.ErrOrderReportNotFound
	}
	r.logOperation("Delete", tx.RowsAffected, start, err)
	return err
}

func (r *GormRepository) logOperation(operation string, rows int64, start time.Time, err error) {
	fields := []zap.Field{
		zap.String("repository", "order_report"),
		zap.String("operation", operation),
		zap.Int64("rows_affected", rows),
		zap.Float64("latency_ms", float64(time.Since(start).Microseconds())/1000),
	}
	if err != nil {
		fields = append(fields, zap.Error(err))
		r.log.Warn("repository operation completed with error", fields...)
		return
	}
	r.log.Info("repository operation completed", fields...)
}

func toRecord(item *model.OrderReport) *OrderReportModel {
	return &OrderReportModel{
		ID:           item.ID,
		CustomerName: item.CustomerName,
		Status:       item.Status,
		TotalAmount:  item.TotalAmount,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
}

func toDomain(record *OrderReportModel) *model.OrderReport {
	return &model.OrderReport{
		ID:           record.ID,
		CustomerName: record.CustomerName,
		Status:       record.Status,
		TotalAmount:  record.TotalAmount,
		CreatedAt:    record.CreatedAt,
		UpdatedAt:    record.UpdatedAt,
	}
}

var _ model.Repository = (*GormRepository)(nil)
