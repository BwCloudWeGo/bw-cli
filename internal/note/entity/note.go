package entity

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNoteNotFound = errors.New("note not found")
	ErrInvalidNote  = errors.New("invalid note")
)

// NoteStatus 是 note 业务和 API 暴露的生命周期状态。
type NoteStatus string

const (
	NoteStatusDraft     NoteStatus = "DRAFT"
	NoteStatusPublished NoteStatus = "PUBLISHED"

	NoteStatusDraftCode     int32 = 1
	NoteStatusPublishedCode int32 = 2
)

// Note 是 note 服务使用的笔记聚合。
type Note struct {
	ID          string
	AuthorID    string
	Title       string
	Content     string
	Status      NoteStatus
	NoteType    int32
	Permission  int32
	Remark      string
	TopicIDs    []string
	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewNote 校验输入并创建草稿笔记。
func NewNote(authorID string, title string, content string) (*Note, error) {
	authorID = strings.TrimSpace(authorID)
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if authorID == "" || title == "" || content == "" {
		return nil, ErrInvalidNote
	}
	now := time.Now().UTC()
	return &Note{
		ID:        uuid.NewString(),
		AuthorID:  authorID,
		Title:     title,
		Content:   content,
		Status:    NoteStatusDraft,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Publish 将草稿笔记切换到已发布状态；该操作幂等。
func (n *Note) Publish() {
	if n == nil || n.Status == NoteStatusPublished {
		return
	}
	now := time.Now().UTC()
	n.Status = NoteStatusPublished
	n.PublishedAt = &now
	n.UpdatedAt = now
}

// NoteStatusFromCode 将数据库或表单状态码转换为业务状态。
func NoteStatusFromCode(code int32) NoteStatus {
	switch code {
	case NoteStatusDraftCode:
		return NoteStatusDraft
	case NoteStatusPublishedCode:
		return NoteStatusPublished
	default:
		return NoteStatusDraft
	}
}

// Code 将业务状态转换为 notes 表中存储的整数。
func (s NoteStatus) Code() int32 {
	switch s {
	case NoteStatusDraft:
		return NoteStatusDraftCode
	case NoteStatusPublished:
		return NoteStatusPublishedCode
	default:
		return 0
	}
}
