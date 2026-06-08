package dto

import "github.com/BwCloudWeGo/bw-cli/internal/note/entity"

// NoteDTO 是 note 用例层返回给 handler 的数据结构。
// 它不带 gRPC、HTTP 或 MongoDB tag，避免外部协议和数据库细节泄漏到 service。
type NoteDTO struct {
	ID         string
	AuthorID   string
	Title      string
	Content    string
	Status     entity.NoteStatus
	NoteType   int32
	Permission int32
	Remark     string
	TopicIDs   []string
}

// FromNote 将领域聚合转换成对外返回的数据结构。
func FromNote(note *entity.Note) *NoteDTO {
	return &NoteDTO{
		ID:         note.ID,
		AuthorID:   note.AuthorID,
		Title:      note.Title,
		Content:    note.Content,
		Status:     note.Status,
		NoteType:   note.NoteType,
		Permission: note.Permission,
		Remark:     note.Remark,
		TopicIDs:   note.TopicIDs,
	}
}
