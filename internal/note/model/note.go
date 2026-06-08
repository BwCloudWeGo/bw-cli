package model

import "time"

// NoteModel 是 notes 表的 Gorm 持久化模型。
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

const noteCollectionName = "notes"

// NoteDocument 是 notes 集合的 MongoDB 文档模型。
type NoteDocument struct {
	ID          string     `bson:"_id"`
	AuthorID    string     `bson:"author_id"`
	Title       string     `bson:"title"`
	Content     string     `bson:"content"`
	Status      int32      `bson:"status"`
	NoteType    int32      `bson:"note_type"`
	Permission  int32      `bson:"permission"`
	Remark      string     `bson:"remark"`
	TopicIDs    []string   `bson:"topic_ids"`
	PublishedAt *time.Time `bson:"published_at,omitempty"`
	CreatedAt   time.Time  `bson:"created_at"`
	UpdatedAt   time.Time  `bson:"updated_at"`
}

func (NoteDocument) MongoCollectionName() string {
	return noteCollectionName
}
