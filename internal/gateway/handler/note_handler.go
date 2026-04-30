package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	notev1 "github.com/BwCloudWeGo/bw-cli/api/gen/note/v1"
	"github.com/BwCloudWeGo/bw-cli/internal/gateway/request"
	apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"
	"github.com/BwCloudWeGo/bw-cli/pkg/httpx"
)

// NoteHandler adapts note HTTP endpoints to the internal note gRPC client.
type NoteHandler struct {
	client notev1.NoteServiceClient
	log    *zap.Logger
}

// NewNoteHandler wires the note gRPC client into HTTP handler methods.
func NewNoteHandler(client notev1.NoteServiceClient, log *zap.Logger) *NoteHandler {
	return &NoteHandler{client: client, log: log}
}

// Create handles note creation requests.
func (h *NoteHandler) Create(c *gin.Context) {
	var req request.CreateNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_request", err.Error()))
		return
	}
	resp, err := h.client.CreateNote(outgoingContext(c), &notev1.CreateNoteRequest{
		AuthorId: req.AuthorID,
		Title:    req.Title,
		Content:  req.Content,
	})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	h.log.Info("gateway note create proxied", zap.String("request_id", httpx.RequestID(c)), zap.String("note_id", resp.GetId()), zap.String("user_id", resp.GetAuthorId()))
	httpx.Created(c, resp)
}

// Get proxies note lookup by id.
func (h *NoteHandler) Get(c *gin.Context) {
	resp, err := h.client.GetNote(outgoingContext(c), &notev1.GetNoteRequest{Id: c.Param("id")})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}

// Publish proxies the note publish command.
func (h *NoteHandler) Publish(c *gin.Context) {
	resp, err := h.client.PublishNote(outgoingContext(c), &notev1.PublishNoteRequest{Id: c.Param("id")})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	h.log.Info("gateway note publish proxied", zap.String("request_id", httpx.RequestID(c)), zap.String("note_id", resp.GetId()), zap.String("user_id", resp.GetAuthorId()))
	httpx.OK(c, resp)
}

// PublishNote handles note publish requests submitted via JSON body.
func (h *NoteHandler) PublishNote(c *gin.Context) {
	var req request.PublishNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_request", err.Error()))
		return
	}
	resp, err := h.client.PublishNote(outgoingContext(c), &notev1.PublishNoteRequest{
		AuthorId:   req.AuthorID,
		Title:      req.Title,
		Content:    req.Content,
		NoteType:   req.NoteType,
		Permission: req.Permission,
		TopicIds:   req.TopicIDs,
		Status:     req.Status,
	})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	h.log.Info("gateway note publish proxied", zap.String("request_id", httpx.RequestID(c)), zap.String("note_id", resp.GetId()), zap.String("user_id", resp.GetAuthorId()))
	httpx.OK(c, resp)
}
