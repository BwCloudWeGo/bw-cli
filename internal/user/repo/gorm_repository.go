package repo

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/BwCloudWeGo/bw-cli/internal/user/model"
)

// UserModel is the Gorm persistence model for the users table.
type UserModel struct {
	ID           string `gorm:"primaryKey;size:64"`
	Email        string `gorm:"uniqueIndex;size:255;not null"`
	DisplayName  string `gorm:"size:128;not null"`
	PasswordHash string `gorm:"size:255;not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (UserModel) TableName() string {
	return "users"
}

// GormRepository persists user aggregates with Gorm.
type GormRepository struct {
	db  *gorm.DB
	log *zap.Logger
}

// NewGormRepository constructs a user repository with optional structured logging.
func NewGormRepository(db *gorm.DB, loggers ...*zap.Logger) *GormRepository {
	log := zap.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		log = loggers[0]
	}
	return &GormRepository{db: db, log: log}
}

// AutoMigrate creates or updates the users table schema.
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&UserModel{})
}

// Save inserts or updates a user aggregate.
func (r *GormRepository) Save(ctx context.Context, user *model.User) error {
	start := time.Now()
	tx := r.db.WithContext(ctx).Save(toUserModel(user))
	err := tx.Error
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "unique") {
		err = model.ErrEmailAlreadyExists
	}
	r.logOperation("Save", tx.RowsAffected, start, err)
	return err
}

// FindByID loads a user aggregate by id.
func (r *GormRepository) FindByID(ctx context.Context, id string) (*model.User, error) {
	start := time.Now()
	var record UserModel
	tx := r.db.WithContext(ctx).Where("id = ?", id).First(&record)
	err := tx.Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = model.ErrUserNotFound
	}
	if err != nil {
		r.logOperation("FindByID", tx.RowsAffected, start, err)
		return nil, err
	}
	r.logOperation("FindByID", tx.RowsAffected, start, nil)
	return toUserDomain(&record), nil
}

// FindByEmail loads a user aggregate by normalized email.
func (r *GormRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	start := time.Now()
	var record UserModel
	tx := r.db.WithContext(ctx).Where("email = ?", strings.TrimSpace(strings.ToLower(email))).First(&record)
	err := tx.Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = model.ErrUserNotFound
	}
	if err != nil {
		r.logOperation("FindByEmail", tx.RowsAffected, start, err)
		return nil, err
	}
	r.logOperation("FindByEmail", tx.RowsAffected, start, nil)
	return toUserDomain(&record), nil
}

func (r *GormRepository) logOperation(operation string, rows int64, start time.Time, err error) {
	fields := []zap.Field{
		zap.String("repository", "user"),
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

func toUserModel(user *model.User) *UserModel {
	return &UserModel{
		ID:           user.ID,
		Email:        user.Email,
		DisplayName:  user.DisplayName,
		PasswordHash: user.PasswordHash,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}
}

func toUserDomain(record *UserModel) *model.User {
	return &model.User{
		ID:           record.ID,
		Email:        record.Email,
		DisplayName:  record.DisplayName,
		PasswordHash: record.PasswordHash,
		CreatedAt:    record.CreatedAt,
		UpdatedAt:    record.UpdatedAt,
	}
}
