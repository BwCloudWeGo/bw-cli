package handler

import (
	"context"
	stderrors "errors"

	"go.uber.org/zap"

	notev1 "github.com/BwCloudWeGo/bw-cli/api/gen/note/v1"
	"github.com/BwCloudWeGo/bw-cli/internal/note/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/note/model"
	"github.com/BwCloudWeGo/bw-cli/internal/note/service"
	apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"
)

// Server adapts note gRPC requests to note service use cases.
type Server struct {
	notev1.UnimplementedNoteServiceServer
	svc *service.Service
	log *zap.Logger
}

// NewServer constructs the note gRPC server adapter.
func NewServer(svc *service.Service, log *zap.Logger) *Server {
	return &Server{svc: svc, log: log}
}

// CreateNote handles the note creation RPC.
func (s *Server) CreateNote(ctx context.Context, req *notev1.CreateNoteRequest) (*notev1.NoteResponse, error) {
	note, err := s.svc.Create(ctx, dto.CreateNoteCommand{
		AuthorID: req.GetAuthorId(),
		Title:    req.GetTitle(),
		Content:  req.GetContent(),
	})
	if err != nil {
		return nil, mapNoteError(err)
	}
	s.log.Info("note created", zap.String("aggregate_id", note.ID), zap.String("use_case", "CreateNote"))
	return toProto(note), nil
}

// GetNote handles note lookup by id.
func (s *Server) GetNote(ctx context.Context, req *notev1.GetNoteRequest) (*notev1.NoteResponse, error) {
	note, err := s.svc.Get(ctx, req.GetId())
	if err != nil {
		return nil, mapNoteError(err)
	}
	return toProto(note), nil
}

// PublishNote handles the note publish RPC.
func (s *Server) PublishNote(ctx context.Context, req *notev1.PublishNoteRequest) (*notev1.NoteResponse, error) {
	note, err := s.svc.PublishSubmitted(ctx, dto.PublishNoteCommand{
		ID:         req.GetId(),
		AuthorID:   req.GetAuthorId(),
		Title:      req.GetTitle(),
		Content:    req.GetContent(),
		NoteType:   req.GetNoteType(),
		Permission: req.GetPermission(),
		TopicIDs:   req.GetTopicIds(),
		Status:     req.GetStatus(),
	})
	if err != nil {
		return nil, mapNoteError(err)
	}
	s.log.Info("note published", zap.String("aggregate_id", note.ID), zap.String("use_case", "PublishNote"))
	return toProto(note), nil
}

func toProto(note *dto.NoteDTO) *notev1.NoteResponse {
	return &notev1.NoteResponse{
		Id:         note.ID,
		AuthorId:   note.AuthorID,
		Title:      note.Title,
		Content:    note.Content,
		Status:     string(note.Status),
		NoteType:   int32(note.NoteType),
		Permission: int32(note.Permission),
		Remark:     note.Remark,
		TopicIds:   note.TopicIDs,
	}
}

func mapNoteError(err error) error {
	switch {
	case stderrors.Is(err, model.ErrInvalidNote):
		return apperrors.InvalidArgument("invalid_note", "invalid note input")
	case stderrors.Is(err, model.ErrNoteNotFound):
		return apperrors.NotFound("note_not_found", "note not found")
	default:
		return apperrors.Wrap(apperrors.KindInternal, "note_service_error", "note service error", err)
	}
}
