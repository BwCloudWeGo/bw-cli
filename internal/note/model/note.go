package model

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

// NoteStatus is the lifecycle state exposed by the note domain and API.
type NoteStatus string

const (
	NoteStatusDraft     NoteStatus = "DRAFT"
	NoteStatusPublished NoteStatus = "PUBLISHED"

	NoteStatusDraftCode     int32 = 1
	NoteStatusPublishedCode int32 = 2
)

// Note is the note aggregate used by the note service.
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

// NewNote validates input and creates a draft note.
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

// Publish moves a draft note to the published state; it is idempotent.
func (n *Note) Publish() {
	if n == nil || n.Status == NoteStatusPublished {
		return
	}
	now := time.Now().UTC()
	n.Status = NoteStatusPublished
	n.PublishedAt = &now
	n.UpdatedAt = now
}

// NoteStatusFromCode converts database/form status codes to a domain status.
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

// Code converts a domain status to the integer stored in the notes table.
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
