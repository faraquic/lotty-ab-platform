package auth

import "time"

type LoginRequest struct {
	Email        string `json:"email" binding:"required,email"`
	HashPassword string `json:"hash_password" binding:"required"`
}

type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}
