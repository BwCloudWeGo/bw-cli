package service

import "github.com/BwCloudWeGo/bw-cli/internal/note/model"

// NoteDTO is the public note data returned by use cases.
type NoteDTO struct {
	ID         string
	AuthorID   string
	Title      string
	Content    string
	Status     model.NoteStatus
	NoteType   int32
	Permission int32
	Remark     string
	TopicIDs   []string
}

// toDTO converts a note aggregate into the service response DTO.
func toDTO(note *model.Note) *NoteDTO {
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
