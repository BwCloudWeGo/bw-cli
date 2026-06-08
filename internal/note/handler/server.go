package handler

import (
	"context"
	stderrors "errors"

	"go.uber.org/zap"

	notev1 "github.com/BwCloudWeGo/bw-cli/api/gen/note/v1"
	"github.com/BwCloudWeGo/bw-cli/internal/note/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/note/entity"
	"github.com/BwCloudWeGo/bw-cli/internal/note/service"
	apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"
)

// Server 将 note gRPC 请求适配到 note 用例服务。
type Server struct {
	notev1.UnimplementedNoteServiceServer
	svc *service.Service
	log *zap.Logger
}

// NewServer 创建 note gRPC 服务端适配器。
func NewServer(svc *service.Service, log *zap.Logger) *Server {
	return &Server{svc: svc, log: log}
}

// CreateNote 处理创建笔记 RPC。
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

// GetNote 处理按 ID 查询笔记 RPC。
func (s *Server) GetNote(ctx context.Context, req *notev1.GetNoteRequest) (*notev1.NoteResponse, error) {
	note, err := s.svc.Get(ctx, req.GetId())
	if err != nil {
		return nil, mapNoteError(err)
	}
	return toProto(note), nil
}

// PublishNote  发布笔记的rpc接口
func (s *Server) PublishNote(ctx context.Context, req *notev1.PublishNoteRequest) (*notev1.NoteResponse, error) {
	// 1. 不要直接把proto带过来的参数进行使用  要进行二次处理  然后拿到最终要真实操作的数据
	note, err := s.svc.PublishSubmitted(ctx, dto.PublishNoteCommand{
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
	case stderrors.Is(err, entity.ErrInvalidNote):
		return apperrors.InvalidArgument("invalid_note", "invalid note input")
	case stderrors.Is(err, entity.ErrNoteNotFound):
		return apperrors.NotFound("note_not_found", "note not found")
	default:
		return apperrors.Wrap(apperrors.KindInternal, "note_service_error", "note service error", err)
	}
}
