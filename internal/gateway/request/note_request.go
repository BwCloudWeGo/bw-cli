package request

// CreateNoteRequest 是 POST /api/v1/notes 使用的 JSON 载荷。
type CreateNoteRequest struct {
	AuthorID string `json:"author_id" binding:"required"`
	Title    string `json:"title" binding:"required"`
	Content  string `json:"content" binding:"required"`
}

// PublishNoteRequest 是 POST /api/v1/notes/publishNote 使用的 JSON 载荷。
// 字段与 notes 表列名保持一致。
type PublishNoteRequest struct {
	AuthorID string `json:"author_id" binding:"required"`
	Title    string `json:"title"     binding:"required"`
	Content  string `json:"content"   binding:"required"`
	// 0=图文 1=视频，对应 notes.note_type
	NoteType int32 `json:"note_type"`
	// 0=公开 1=私密，对应 notes.permission
	Permission int32    `json:"permission"`
	TopicIDs   []string `json:"topic_ids"`
	Status     int32    `json:"status"`
}
