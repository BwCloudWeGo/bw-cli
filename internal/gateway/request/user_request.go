// Package request stores HTTP request DTOs for the gateway layer.
package request

// RegisterUserRequest is the JSON payload used by POST /api/v1/users/register.
type RegisterUserRequest struct {
	Email       string `json:"email" binding:"required,email"`
	DisplayName string `json:"display_name" binding:"required"`
	Password    string `json:"password" binding:"required"`
}

// LoginUserRequest is the JSON payload used by POST /api/v1/users/login.
type LoginUserRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}
