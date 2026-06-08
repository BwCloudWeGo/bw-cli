package repo

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
	dbmodel "github.com/BwCloudWeGo/bw-cli/internal/order/model"
)

// GormRepository 使用 Gorm 持久化 order 聚合。
type GormRepository struct {
	db  *gorm.DB
	log *zap.Logger
}

// NewGormRepository 创建 order 仓储，并支持可选结构化日志。
func NewGormRepository(db *gorm.DB, loggers ...*zap.Logger) *GormRepository {
	log := zap.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		log = loggers[0]
	}
	return &GormRepository{db: db, log: log}
}

// AutoMigrate 创建或更新 orders 表结构。
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&dbmodel.OrderModel{})
}

// Save 新增或更新 order 聚合。
func (r *GormRepository) Save(ctx context.Context, item *entity.Order) error {
	start := time.Now()
	tx := r.db.WithContext(ctx).Save(toRecord(item))
	r.logOperation("Save", tx.RowsAffected, start, tx.Error)
	return tx.Error
}

// FindByID 根据 ID 加载 order 聚合。
func (r *GormRepository) FindByID(ctx context.Context, id string) (*entity.Order, error) {
	start := time.Now()
	var record dbmodel.OrderModel
	tx := r.db.WithContext(ctx).Where("id = ?", id).First(&record)
	err := tx.Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = entity.ErrOrderNotFound
	}
	if err != nil {
		r.logOperation("FindByID", tx.RowsAffected, start, err)
		return nil, err
	}
	r.logOperation("FindByID", tx.RowsAffected, start, nil)
	return toDomain(&record), nil
}

// List 加载分页 order 聚合。
func (r *GormRepository) List(ctx context.Context, offset int, limit int) ([]*entity.Order, int64, error) {
	start := time.Now()
	var total int64
	countTx := r.db.WithContext(ctx).Model(&dbmodel.OrderModel{}).Count(&total)
	if countTx.Error != nil {
		r.logOperation("Count", countTx.RowsAffected, start, countTx.Error)
		return nil, 0, countTx.Error
	}
	var records []dbmodel.OrderModel
	tx := r.db.WithContext(ctx).
		Order("created_at desc").
		Offset(offset).
		Limit(limit).
		Find(&records)
	if tx.Error != nil {
		r.logOperation("List", tx.RowsAffected, start, tx.Error)
		return nil, 0, tx.Error
	}
	items := make([]*entity.Order, 0, len(records))
	for i := range records {
		items = append(items, toDomain(&records[i]))
	}
	r.logOperation("List", tx.RowsAffected, start, nil)
	return items, total, nil
}

// Delete 根据 ID 删除 order 聚合。
func (r *GormRepository) Delete(ctx context.Context, id string) error {
	start := time.Now()
	tx := r.db.WithContext(ctx).Where("id = ?", id).Delete(&dbmodel.OrderModel{})
	err := tx.Error
	if err == nil && tx.RowsAffected == 0 {
		err = entity.ErrOrderNotFound
	}
	r.logOperation("Delete", tx.RowsAffected, start, err)
	return err
}

func (r *GormRepository) logOperation(operation string, rows int64, start time.Time, err error) {
	fields := []zap.Field{
		zap.String("repository", "order"),
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

func toRecord(item *entity.Order) *dbmodel.OrderModel {
	return &dbmodel.OrderModel{
		ID:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func toDomain(record *dbmodel.OrderModel) *entity.Order {
	return &entity.Order{
		ID:          record.ID,
		Name:        record.Name,
		Description: record.Description,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}
}

var _ entity.Repository = (*GormRepository)(nil)
