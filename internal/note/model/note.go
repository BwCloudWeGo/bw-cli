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

// NoteStatus is the lifecycle state of a note aggregate.
type NoteStatus string

const (
	NoteStatusDraft     NoteStatus = "DRAFT"
	NoteStatusPublished NoteStatus = "PUBLISHED"
)

// Note is the note aggregate used by the note service.
type Note struct {
	ID          string
	AuthorID    string
	Title       string
	Content     string
	Status      NoteStatus
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
	if n.Status == NoteStatusPublished {
		return
	}
	now := time.Now().UTC()
	n.Status = NoteStatusPublished
	n.PublishedAt = &now
	n.UpdatedAt = now
}
