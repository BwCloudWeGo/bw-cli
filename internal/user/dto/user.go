package dto

import "github.com/BwCloudWeGo/bw-cli/internal/user/model"

// UserDTO 是 user 用例层返回给 handler 的数据结构。
// 它不携带 gRPC、HTTP 或数据库 tag，避免外部协议和持久化细节泄漏到 service 层。
type UserDTO struct {
	ID          string
	Account     string
	DisplayName string
	Sex         bool
}

// FromUser 将 user 领域聚合转换成对外返回的数据结构。
func FromUser(user *model.User) *UserDTO {
	return &UserDTO{
		ID:          user.ID,
		Account:     user.Account,
		DisplayName: user.DisplayName,
		Sex:         user.Sex,
	}
}
