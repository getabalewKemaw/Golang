package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-manager/internal/dto"
	"task-manager/internal/models"
	"task-manager/internal/repositories"
	"task-manager/internal/services"
	"task-manager/pkg/auth"
	"task-manager/pkg/database"
)

// mockUserRepository implements repositories.UserRepository using function fields.
type mockUserRepository struct {
	createFunc     func(ctx context.Context, user *models.User) error
	getByEmailFunc func(ctx context.Context, email string) (*models.User, error)
	getByIDFunc    func(ctx context.Context, userID int64) (*models.User, error)
}

var _ repositories.UserRepository = (*mockUserRepository)(nil)

func (mock *mockUserRepository) Create(ctx context.Context, user *models.User) error {
	if mock.createFunc != nil {
		return mock.createFunc(ctx, user)
	}
	return nil
}

func (mock *mockUserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	if mock.getByEmailFunc != nil {
		return mock.getByEmailFunc(ctx, email)
	}
	return nil, nil
}

func (mock *mockUserRepository) GetByID(ctx context.Context, userID int64) (*models.User, error) {
	if mock.getByIDFunc != nil {
		return mock.getByIDFunc(ctx, userID)
	}
	return nil, nil
}

// mockTokenRepository implements repositories.TokenRepository using function fields.
type mockTokenRepository struct {
	revokeTokenFunc         func(ctx context.Context, tokenHash string, tokenType string, expiresAt time.Time) error
	isTokenRevokedFunc      func(ctx context.Context, tokenHash string) (bool, error)
	deleteExpiredTokensFunc func(ctx context.Context) (int64, error)
}

var _ repositories.TokenRepository = (*mockTokenRepository)(nil)

func (mock *mockTokenRepository) RevokeToken(ctx context.Context, tokenHash string, tokenType string, expiresAt time.Time) error {
	if mock.revokeTokenFunc != nil {
		return mock.revokeTokenFunc(ctx, tokenHash, tokenType, expiresAt)
	}
	return nil
}

func (mock *mockTokenRepository) IsTokenRevoked(ctx context.Context, tokenHash string) (bool, error) {
	if mock.isTokenRevokedFunc != nil {
		return mock.isTokenRevokedFunc(ctx, tokenHash)
	}
	return false, nil
}

func (mock *mockTokenRepository) DeleteExpiredTokens(ctx context.Context) (int64, error) {
	if mock.deleteExpiredTokensFunc != nil {
		return mock.deleteExpiredTokensFunc(ctx)
	}
	return 0, nil
}

// mockTransactor implements database.Transactor using a direct callback executor.
type mockTransactor struct {
	withinTransactionFunc func(ctx context.Context, operation func(transactionContext context.Context) error) error
}

var _ database.Transactor = (*mockTransactor)(nil)

func (mock *mockTransactor) WithinTransaction(ctx context.Context, operation func(transactionContext context.Context) error) error {
	if mock.withinTransactionFunc != nil {
		return mock.withinTransactionFunc(ctx, operation)
	}
	return operation(ctx)
}

func TestRegister_AtomicTransactionSuccess(testingT *testing.T) {
	createdTasksCount := 0

	mockUserRepo := &mockUserRepository{
		createFunc: func(ctx context.Context, user *models.User) error {
			user.ID = 7
			return nil
		},
	}

	mockTaskRepo := &mockTaskRepository{
		createFunc: func(ctx context.Context, task *models.Task) error {
			createdTasksCount++
			if task.UserID != 7 {
				testingT.Errorf("expected welcome task user ID to be 7, got %d", task.UserID)
			}
			return nil
		},
	}

	mockTokenRepo := &mockTokenRepository{}
	mockTx := &mockTransactor{}

	authService := services.NewAuthService(
		mockUserRepo,
		mockTokenRepo,
		mockTaskRepo,
		mockTx,
		"access-secret-32-bytes-minimum-length!",
		"refresh-secret-32-bytes-minimum-length!",
	)

	input := dto.RegisterRequestDTO{
		Email:    "newuser@example.com",
		Password: "SecurePassword123!",
	}

	authResponse, err := authService.Register(context.Background(), input)
	if err != nil {
		testingT.Fatalf("unexpected error registering user: %v", err)
	}

	if authResponse.User.ID != 7 {
		testingT.Errorf("expected user ID 7, got %d", authResponse.User.ID)
	}

	if createdTasksCount != 2 {
		testingT.Errorf("expected 2 welcome tasks created atomically, got %d", createdTasksCount)
	}

	if authResponse.Tokens.AccessToken == "" || authResponse.Tokens.RefreshToken == "" {
		testingT.Error("expected non-empty tokens in auth response")
	}
}

func TestRegister_RollbackOnTaskCreationFailure(testingT *testing.T) {
	mockUserRepo := &mockUserRepository{
		createFunc: func(ctx context.Context, user *models.User) error {
			user.ID = 8
			return nil
		},
	}

	// Task creation fails, triggering transaction rollback
	expectedTaskError := errors.New("database disk full: failed to insert task")
	mockTaskRepo := &mockTaskRepository{
		createFunc: func(ctx context.Context, task *models.Task) error {
			return expectedTaskError
		},
	}

	mockTokenRepo := &mockTokenRepository{}
	mockTx := &mockTransactor{}

	authService := services.NewAuthService(
		mockUserRepo,
		mockTokenRepo,
		mockTaskRepo,
		mockTx,
		"access-secret-32-bytes-minimum-length!",
		"refresh-secret-32-bytes-minimum-length!",
	)

	input := dto.RegisterRequestDTO{
		Email:    "failuser@example.com",
		Password: "SecurePassword123!",
	}

	_, err := authService.Register(context.Background(), input)
	if err == nil {
		testingT.Fatal("expected registration error due to welcome task failure, got nil")
	}

	if !errors.Is(err, expectedTaskError) {
		testingT.Fatalf("expected error '%v', got: %v", expectedTaskError, err)
	}
}

func TestLogin_InvalidCredentials(testingT *testing.T) {
	hashedPassword, err := auth.HashPassword("CorrectPassword123!")
	if err != nil {
		testingT.Fatalf("failed to hash password: %v", err)
	}

	mockUserRepo := &mockUserRepository{
		getByEmailFunc: func(ctx context.Context, email string) (*models.User, error) {
			if email == "active@example.com" {
				return &models.User{
					ID:           1,
					Email:        email,
					PasswordHash: hashedPassword,
				}, nil
			}
			return nil, repositories.ErrUserNotFound
		},
	}

	mockTaskRepo := &mockTaskRepository{}
	mockTokenRepo := &mockTokenRepository{}
	mockTx := &mockTransactor{}

	authService := services.NewAuthService(
		mockUserRepo,
		mockTokenRepo,
		mockTaskRepo,
		mockTx,
		"access-secret-32-bytes-minimum-length!",
		"refresh-secret-32-bytes-minimum-length!",
	)

	// Case 1: Non-existent email
	_, err = authService.Login(context.Background(), dto.LoginRequestDTO{
		Email:    "unknown@example.com",
		Password: "CorrectPassword123!",
	})
	if !errors.Is(err, services.ErrInvalidCredentials) {
		testingT.Errorf("expected ErrInvalidCredentials for unknown email, got: %v", err)
	}

	// Case 2: Wrong password
	_, err = authService.Login(context.Background(), dto.LoginRequestDTO{
		Email:    "active@example.com",
		Password: "WrongPassword!",
	})
	if !errors.Is(err, services.ErrInvalidCredentials) {
		testingT.Errorf("expected ErrInvalidCredentials for wrong password, got: %v", err)
	}
}
