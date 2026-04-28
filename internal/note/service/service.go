package service

import (
	"context"

	"github.com/BwCloudWeGo/bw-cli/internal/note/model"
)

// Service orchestrates note use cases.
type Service struct {
	repo model.Repository
}

// NewService constructs the note use-case service.
func NewService(repo model.Repository) *Service {
	return &Service{repo: repo}
}

// CreateNoteCommand contains validated input for creating a note.
type CreateNoteCommand struct {
	AuthorID string
	Title    string
	Content  string
}

// NoteDTO is the public note data returned by use cases.
type NoteDTO struct {
	ID       string
	AuthorID string
	Title    string
	Content  string
	Status   model.NoteStatus
}

// Create stores a new draft note.
func (s *Service) Create(ctx context.Context, cmd CreateNoteCommand) (*NoteDTO, error) {
	note, err := model.NewNote(cmd.AuthorID, cmd.Title, cmd.Content)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, note); err != nil {
		return nil, err
	}
	return toDTO(note), nil
}

// Get returns one note by id.
func (s *Service) Get(ctx context.Context, id string) (*NoteDTO, error) {
	note, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toDTO(note), nil
}

// Publish changes a note to the published state and persists it.
func (s *Service) Publish(ctx context.Context, id string) (*NoteDTO, error) {
	note, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	note.Publish()
	if err := s.repo.Save(ctx, note); err != nil {
		return nil, err
	}
	return toDTO(note), nil
}

func toDTO(note *model.Note) *NoteDTO {
	return &NoteDTO{
		ID:       note.ID,
		AuthorID: note.AuthorID,
		Title:    note.Title,
		Content:  note.Content,
		Status:   note.Status,
	}
}
