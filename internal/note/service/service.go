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

// Publish changes an existing note to the published state and persists it.
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

// PublishSubmitted creates or updates a note from a full publish payload.
func (s *Service) PublishSubmitted(ctx context.Context, cmd PublishNoteCommand) (*NoteDTO, error) {
	if cmd.ID != "" {
		return s.Publish(ctx, cmd.ID)
	}
	note, err := model.NewNote(cmd.AuthorID, cmd.Title, cmd.Content)
	if err != nil {
		return nil, err
	}
	note.NoteType = cmd.NoteType
	note.Permission = cmd.Permission
	note.TopicIDs = cmd.TopicIDs
	if cmd.Status == model.NoteStatusDraftCode {
		note.Status = model.NoteStatusDraft
	} else {
		note.Publish()
	}
	if err := s.repo.Save(ctx, note); err != nil {
		return nil, err
	}
	return toDTO(note), nil
}
