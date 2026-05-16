package repo

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/BwCloudWeGo/bw-cli/internal/user/model"
)

// UserModel is the Gorm persistence model for the users table.
type UserModel struct {
	ID           int64  `gorm:"primaryKey;column:id;autoIncrement"`
	Account      string `gorm:"uniqueIndex;column:account;size:64;not null"`
	DisplayName  string `gorm:"column:name;size:20"`
	Sex          bool   `gorm:"column:sex"`
	PasswordSalt string `gorm:"column:password_salt;size:64;not null"`
	PasswordHash string `gorm:"column:password_hash;size:255;not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (UserModel) TableName() string {
	return "xls_user"
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
	record := toUserModel(user)
	tx := r.db.WithContext(ctx).Save(record)
	err := tx.Error
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "unique") {
		err = model.ErrAccountAlreadyExists
	}
	if err == nil && user.ID == "" {
		user.ID = strconv.FormatInt(record.ID, 10)
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

// FindByAccount loads a user aggregate by normalized account.
func (r *GormRepository) FindByAccount(ctx context.Context, account string) (*model.User, error) {
	start := time.Now()
	var record UserModel
	tx := r.db.WithContext(ctx).Where("account = ?", model.NormalizeAccount(account)).First(&record)
	err := tx.Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = model.ErrUserNotFound
	}
	if err != nil {
		r.logOperation("FindByAccount", tx.RowsAffected, start, err)
		return nil, err
	}
	r.logOperation("FindByAccount", tx.RowsAffected, start, nil)
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
	id, _ := strconv.ParseInt(user.ID, 10, 64)
	salt, _, _ := strings.Cut(user.PasswordHash, ":")
	return &UserModel{
		ID:           id,
		Account:      user.Account,
		DisplayName:  user.DisplayName,
		Sex:          user.Sex,
		PasswordSalt: salt,
		PasswordHash: user.PasswordHash,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}
}

func toUserDomain(record *UserModel) *model.User {
	return &model.User{
		ID:           strconv.FormatInt(record.ID, 10),
		Account:      record.Account,
		DisplayName:  record.DisplayName,
		Sex:          record.Sex,
		PasswordSalt: record.PasswordSalt,
		PasswordHash: record.PasswordHash,
		CreatedAt:    record.CreatedAt,
		UpdatedAt:    record.UpdatedAt,
	}
}
