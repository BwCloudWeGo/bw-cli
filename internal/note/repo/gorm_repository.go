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
	ID          string `gorm:"primaryKey;size:64"`
	AuthorID    string `gorm:"index;size:64;not null"`
	Title       string `gorm:"size:255;not null"`
	Content     string `gorm:"type:text;not null"`
	Status      string `gorm:"size:32;not null"`
	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
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
		Status:      string(note.Status),
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
		Status:      model.NoteStatus(record.Status),
		PublishedAt: record.PublishedAt,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}
}
