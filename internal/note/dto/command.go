package dto

// CreateNoteCommand 是创建笔记用例的入参。
// handler 层负责把 gRPC/HTTP 请求转换成该结构，service 层只读取已经整理好的业务字段。
type CreateNoteCommand struct {
	AuthorID string
	Title    string
	Content  string
}

// PublishNoteCommand 是发布笔记用例的完整入参。
// 它承接外部请求字段，但不包含任何协议对象或数据库对象。
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
