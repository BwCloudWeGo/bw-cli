package request

// CreateNoteRequest is the JSON payload used by POST /api/v1/notes.
type CreateNoteRequest struct {
	AuthorID string `json:"author_id" binding:"required"`
	Title    string `json:"title" binding:"required"`
	Content  string `json:"content" binding:"required"`
}
