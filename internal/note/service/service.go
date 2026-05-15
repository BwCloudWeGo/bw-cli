package service

import (
	"context"

	"github.com/BwCloudWeGo/bw-cli/internal/note/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/note/model"
)

// Service 只负责编排 note 用例流程，不直接处理协议对象和数据库细节。
type Service struct {
	repo model.Repository
}

// NewService 创建 note 用例服务。
func NewService(repo model.Repository) *Service {
	return &Service{repo: repo}
}

// Create 创建草稿笔记。
func (s *Service) Create(ctx context.Context, cmd dto.CreateNoteCommand) (*dto.NoteDTO, error) {
	note, err := model.NewNote(cmd.AuthorID, cmd.Title, cmd.Content)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, note); err != nil {
		return nil, err
	}
	return dto.FromNote(note), nil
}

// Get 按 ID 查询笔记。
func (s *Service) Get(ctx context.Context, id string) (*dto.NoteDTO, error) {
	note, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return dto.FromNote(note), nil
}

// PublishSubmitted 根据完整提交内容创建或发布笔记。
func (s *Service) PublishSubmitted(ctx context.Context, cmd dto.PublishNoteCommand) (*dto.NoteDTO, error) {
	// 1. 数据库中的模型和传入的参数进行校验并且赋值
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

    // 2. 已经拿到了最终要入库的数据结构 直接进行入库即可
	if err := s.repo.Save(ctx, note); err != nil {
		return nil, err
	}
	// 3. 在最终返回之前要进行数据的二次处理
	return dto.FromNote(note), nil
}
