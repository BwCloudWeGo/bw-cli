package service

// CreateNoteCommand contains validated input for creating a note.
type CreateNoteCommand struct {
	AuthorID string
	Title    string
	Content  string
}

// PublishNoteCommand contains the full payload for publishing a note.
type PublishNoteCommand struct {
	ID         string
	AuthorID   string
	Title      string
	Content    string
	NoteType   int32
	Permission int32
	TopicIDs   []string
	Status     int32
}
