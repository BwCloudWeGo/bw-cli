package dto

// CreateCommand 是创建 order 记录的业务入参。
// handler 层只负责从协议请求中组装该结构，service 层基于它编排用例流程。
type CreateCommand struct {
	Name        string
	Description string
}

// UpdateCommand 是更新 order 记录的业务入参。
// ID 由路径或 RPC request 明确传入，避免 service 依赖具体协议对象。
type UpdateCommand struct {
	ID          string
	Name        string
	Description string
}

// ListCommand 是查询 order 列表的分页入参。
// 分页默认值和最大值由 service 层统一归一化。
type ListCommand struct {
	Page     int32
	PageSize int32
}
