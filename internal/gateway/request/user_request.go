// request 包存放 gateway 层的 HTTP 请求 DTO。
package request

// RegisterUserRequest 是 POST /api/v1/users/register 使用的 JSON 载荷。
type RegisterUserRequest struct {
	Account     string `json:"account" binding:"required"`
	DisplayName string `json:"display_name" binding:"required"`
	Password    string `json:"password" binding:"required"`
}

// LoginUserRequest 是 POST /api/v1/users/login 使用的 JSON 载荷。
type LoginUserRequest struct {
	Account  string `json:"account" binding:"required"`
	Password string `json:"password" binding:"required"`
}
