package repo

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/BwCloudWeGo/bw-cli/internal/note/model"
)

// NoteModel is the Gorm persistence model for the notes table.
type NoteModel struct {
	ID          string     `gorm:"column:id;primaryKey;size:64"`
	AuthorID    string     `gorm:"column:author_id;index;size:64;not null"`
	Title       string     `gorm:"column:title;size:100;comment:标题"`
	Content     string     `gorm:"column:content;type:text;comment:内容"`
	Status      int32      `gorm:"column:status;comment:状态（1.草稿 2.发布）"`
	TypeID      int32      `gorm:"column:type_id;comment:笔记类型 1.文字 2.图片 3.视频"`
	Remark      string     `gorm:"column:remark;size:50;comment:备注"`
	Permission  int32      `gorm:"column:permission;comment:权限（1.公开 2.私密 3.部分 4.好友 5.密码）"`
	PublishedAt *time.Time `gorm:"column:published_at"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`
}

func (NoteModel) TableName() string {
	return "notes"
}

// GormRepository persists note aggregates with Gorm.
type GormRepository struct {
	db  *gorm.DB
	log *zap.Logger
}

// NewGormRepository constructs a note repository with optional structured logging.
func NewGormRepository(db *gorm.DB, loggers ...*zap.Logger) *GormRepository {
	log := zap.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		log = loggers[0]
	}
	return &GormRepository{db: db, log: log}
}

// AutoMigrate creates or updates the notes table schema.
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&NoteModel{})
}

// Save inserts or updates a note aggregate.
func (r *GormRepository) Save(ctx context.Context, note *model.Note) error {
	start := time.Now()
	tx := r.db.WithContext(ctx).Save(toNoteModel(note))
	r.logOperation("Save", tx.RowsAffected, start, tx.Error)
	return tx.Error
}

// FindByID loads a note aggregate by id.
func (r *GormRepository) FindByID(ctx context.Context, id string) (*model.Note, error) {
	start := time.Now()
	var record NoteModel
	tx := r.db.WithContext(ctx).Where("id = ?", id).First(&record)
	err := tx.Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = model.ErrNoteNotFound
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

func toNoteModel(note *model.Note) *NoteModel {
	return &NoteModel{
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

func toNoteDomain(record *NoteModel) *model.Note {
	return &model.Note{
		ID:          record.ID,
		AuthorID:    record.AuthorID,
		Title:       record.Title,
		Content:     record.Content,
		Status:      model.NoteStatusFromCode(record.Status),
		NoteType:    record.TypeID,
		Permission:  record.Permission,
		Remark:      record.Remark,
		PublishedAt: record.PublishedAt,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}
}
