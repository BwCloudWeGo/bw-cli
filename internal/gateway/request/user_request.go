// Package request stores HTTP request DTOs for the gateway layer.
package request

// RegisterUserRequest is the JSON payload used by POST /api/v1/users/register.
type RegisterUserRequest struct {
	Account     string `json:"account" binding:"required"`
	DisplayName string `json:"display_name" binding:"required"`
	Password    string `json:"password" binding:"required"`
}

// LoginUserRequest is the JSON payload used by POST /api/v1/users/login.
type LoginUserRequest struct {
	Account  string `json:"account" binding:"required"`
	Password string `json:"password" binding:"required"`
}
