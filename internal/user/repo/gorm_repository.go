package repo

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/BwCloudWeGo/bw-cli/internal/user/entity"
	dbmodel "github.com/BwCloudWeGo/bw-cli/internal/user/model"
)

// GormRepository 使用 Gorm 持久化 user 聚合。
type GormRepository struct {
	db  *gorm.DB
	log *zap.Logger
}

// NewGormRepository 创建 user 仓储，并支持可选结构化日志。
func NewGormRepository(db *gorm.DB, loggers ...*zap.Logger) *GormRepository {
	log := zap.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		log = loggers[0]
	}
	return &GormRepository{db: db, log: log}
}

// AutoMigrate 创建或更新 users 表结构。
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&dbmodel.UserModel{})
}

// Save 新增或更新 user 聚合。
func (r *GormRepository) Save(ctx context.Context, user *entity.User) error {
	start := time.Now()
	record := toUserModel(user)
	tx := r.db.WithContext(ctx).Save(record)
	err := tx.Error
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "unique") {
		err = entity.ErrAccountAlreadyExists
	}
	if err == nil && user.ID == "" {
		user.ID = strconv.FormatInt(record.ID, 10)
	}
	r.logOperation("Save", tx.RowsAffected, start, err)
	return err
}

// FindByID 根据 ID 加载 user 聚合。
func (r *GormRepository) FindByID(ctx context.Context, id string) (*entity.User, error) {
	start := time.Now()
	var record dbmodel.UserModel
	tx := r.db.WithContext(ctx).Where("id = ?", id).First(&record)
	err := tx.Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = entity.ErrUserNotFound
	}
	if err != nil {
		r.logOperation("FindByID", tx.RowsAffected, start, err)
		return nil, err
	}
	r.logOperation("FindByID", tx.RowsAffected, start, nil)
	return toUserDomain(&record), nil
}

// FindByAccount 根据规范化账号加载 user 聚合。
func (r *GormRepository) FindByAccount(ctx context.Context, account string) (*entity.User, error) {
	start := time.Now()
	var record dbmodel.UserModel
	tx := r.db.WithContext(ctx).Where("account = ?", entity.NormalizeAccount(account)).First(&record)
	err := tx.Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = entity.ErrUserNotFound
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

func toUserModel(user *entity.User) *dbmodel.UserModel {
	id, _ := strconv.ParseInt(user.ID, 10, 64)
	salt, _, _ := strings.Cut(user.PasswordHash, ":")
	return &dbmodel.UserModel{
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

func toUserDomain(record *dbmodel.UserModel) *entity.User {
	return &entity.User{
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
