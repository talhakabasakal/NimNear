package dto

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/model"
)

// NormalizeEmail trims surrounding space and lowercases the address for
// identity comparison. IAM storage still persists the submitted email; this
// helper is the single limiter/lookup normalization rule.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// RegisterRequest is the input for user registration.
type RegisterRequest struct {
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=8"`
	FirstName string `json:"first_name" validate:"required"`
	LastName  string `json:"last_name" validate:"required"`
}

// LoginRequest is the input for user login.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse is the internal session result. Browser JSON responses use
// SessionResponse and never include the JWT. API clients that need a Bearer
// token must call the dedicated token endpoint.
type LoginResponse struct {
	Token string   `json:"-"`
	User  UserInfo `json:"user"`
}

// SessionResponse is the browser cookie-auth JSON body.
type SessionResponse struct {
	User UserInfo `json:"user"`
}

// TokenResponse is the explicit Bearer issuance contract for non-browser clients.
type TokenResponse struct {
	Token string   `json:"token"`
	User  UserInfo `json:"user"`
}

// UserInfo is a public user representation.
type UserInfo struct {
	ID            uuid.UUID `json:"id"`
	Email         string    `json:"email"`
	FirstName     string    `json:"first_name"`
	LastName      string    `json:"last_name"`
	DisplayName   string    `json:"display_name"`
	WalletAddress string    `json:"wallet_address,omitempty"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

func ToUserInfo(user *model.User, walletAddress string) UserInfo {
	displayName := strings.TrimSpace(user.FirstName + " " + user.LastName)
	if displayName == "" {
		displayName = strings.TrimSpace(user.Email)
	}
	if displayName == "" {
		displayName = walletAddress
	}
	return UserInfo{
		ID:            user.ID,
		Email:         user.Email,
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		DisplayName:   displayName,
		WalletAddress: walletAddress,
		Status:        string(user.Status),
		CreatedAt:     user.CreatedAt,
	}
}

// AssignRoleRequest is the input for assigning a role to a user.
type AssignRoleRequest struct {
	UserID         uuid.UUID  `json:"user_id" validate:"required"`
	RoleID         uuid.UUID  `json:"role_id" validate:"required"`
	OrganizationID uuid.UUID  `json:"organization_id" validate:"required"`
	AppID          *uuid.UUID `json:"app_id,omitempty"`
}

// UpdateUserRequest is the input for updating a user.
type UpdateUserRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Status    string `json:"status"`
}
