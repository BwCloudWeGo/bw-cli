package repo

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/BwCloudWeGo/bw-cli/internal/note/entity"
	dbmodel "github.com/BwCloudWeGo/bw-cli/internal/note/model"
)

// GormRepository 使用 Gorm 持久化 note 聚合。
type GormRepository struct {
	db  *gorm.DB
	log *zap.Logger
}

// NewGormRepository 创建 note 仓储，并支持可选结构化日志。
func NewGormRepository(db *gorm.DB, loggers ...*zap.Logger) *GormRepository {
	log := zap.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		log = loggers[0]
	}
	return &GormRepository{db: db, log: log}
}

// AutoMigrate 创建或更新 notes 表结构。
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&dbmodel.NoteModel{})
}

// Save 新增或更新 note 聚合。
func (r *GormRepository) Save(ctx context.Context, note *entity.Note) error {
	start := time.Now()
	tx := r.db.WithContext(ctx).Save(toNoteModel(note))
	r.logOperation("Save", tx.RowsAffected, start, tx.Error)
	return tx.Error
}

// FindByID 根据 ID 加载 note 聚合。
func (r *GormRepository) FindByID(ctx context.Context, id string) (*entity.Note, error) {
	start := time.Now()
	var record dbmodel.NoteModel
	tx := r.db.WithContext(ctx).Where("id = ?", id).First(&record)
	err := tx.Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = entity.ErrNoteNotFound
	}
	if err != nil {
		r.logOperation("FindByID", tx.RowsAffected, start, err)
		return nil, err
	}
	r.logOperation("FindByID", tx.RowsAffected, start, nil)
	return toNoteDomain(&record), nil
}

func (r *GormRepository) logOperation(operation string, rows int64, start time.Time, err error) {
	fields := []zap.Field{
		zap.String("repository", "note"),
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

func toNoteModel(note *entity.Note) *dbmodel.NoteModel {
	return &dbmodel.NoteModel{
		ID:          note.ID,
		AuthorID:    note.AuthorID,
		Title:       note.Title,
		Content:     note.Content,
		Status:      note.Status.Code(),
		TypeID:      note.NoteType,
		Permission:  note.Permission,
		Remark:      note.Remark,
		PublishedAt: note.PublishedAt,
		CreatedAt:   note.CreatedAt,
		UpdatedAt:   note.UpdatedAt,
	}
}

func toNoteDomain(record *dbmodel.NoteModel) *entity.Note {
	return &entity.Note{
		ID:          record.ID,
		AuthorID:    record.AuthorID,
		Title:       record.Title,
		Content:     record.Content,
		Status:      entity.NoteStatusFromCode(record.Status),
		NoteType:    record.TypeID,
		Permission:  record.Permission,
		Remark:      record.Remark,
		PublishedAt: record.PublishedAt,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}
}
