package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/model"
)

type stubUserRepo struct {
	user *model.User
}

func (s stubUserRepo) Create(context.Context, *model.User) error { return nil }
func (s stubUserRepo) GetByID(context.Context, uuid.UUID) (*model.User, error) {
	return s.user, nil
}
func (s stubUserRepo) GetByEmail(context.Context, string) (*model.User, error) { return nil, nil }
func (s stubUserRepo) Update(context.Context, *model.User) error               { return nil }
func (s stubUserRepo) Delete(context.Context, uuid.UUID) error                 { return nil }
func (s stubUserRepo) List(context.Context, int, int) ([]*model.User, int, error) {
	return nil, 0, nil
}

func TestExecuteUserAllowsAuthenticatedSessionWithoutApp(t *testing.T) {
	uc := NewValidateConnectUseCase(nil, nil)
	userID := uuid.New()
	input, err := uc.ExecuteUser(context.Background(), userID)
	if err != nil {
		t.Fatalf("ExecuteUser: %v", err)
	}
	if input.UserID != userID || input.AppID != uuid.Nil {
		t.Fatalf("input = %#v", input)
	}
}

func TestExecuteUserRejectsMissingUser(t *testing.T) {
	uc := NewValidateConnectUseCase(nil, nil)
	if _, err := uc.ExecuteUser(context.Background(), uuid.Nil); err == nil {
		t.Fatal("expected unauthorized")
	}
}

func TestExecuteUserRejectsDeletedAccount(t *testing.T) {
	userID := uuid.New()
	deleted := time.Now().UTC()
	uc := NewValidateConnectUseCase(nil, nil).WithUsers(stubUserRepo{user: &model.User{
		ID:        userID,
		Status:    model.UserStatusInactive,
		DeletedAt: &deleted,
	}})
	if _, err := uc.ExecuteUser(context.Background(), userID); err == nil {
		t.Fatal("expected deleted user to be rejected")
	}
}

func TestExecuteUserRejectsInactiveAccount(t *testing.T) {
	userID := uuid.New()
	uc := NewValidateConnectUseCase(nil, nil).WithUsers(stubUserRepo{user: &model.User{
		ID:     userID,
		Status: model.UserStatusSuspended,
	}})
	if _, err := uc.ExecuteUser(context.Background(), userID); err == nil {
		t.Fatal("expected inactive user to be rejected")
	}
}
