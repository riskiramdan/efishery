package user

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/riskiramdan/efishery/golang/internal/constants"
	"github.com/riskiramdan/efishery/golang/internal/types"

	"github.com/dgrijalva/jwt-go"
	"golang.org/x/crypto/bcrypt"
)

// Errors
var (
	ErrWrongPassword      = errors.New("wrong password")
	ErrWrongPhone         = errors.New("wrong phone")
	ErrPhoneAlreadyExists = errors.New("Phone Already Exists")
)

// User user
type User struct {
	ID             int        `json:"id" db:"id"`
	RoleID         int        `json:"roleId" db:"roleId"`
	Name           string     `json:"name" db:"name"`
	Phone          string     `json:"phone" db:"phone"`
	Password       string     `json:"password" db:"password"`
	Token          *string    `json:"token" db:"token"`
	TokenExpiredAt *time.Time `json:"tokenExpiredAt" db:"tokenExpiredAt"`
	CreatedAt      time.Time  `json:"createdAt" db:"createdAt"`
	UpdatedAt      *time.Time `json:"updatedAt" db:"updatedAt"`
}

//FindAllUsersParams params for find all
type FindAllUsersParams struct {
	ID    int    `json:"id"`
	Page  int    `json:"page"`
	Limit int    `json:"limit"`
	Phone string `json:"phone"`
	Name  string `json:"name"`
	Token string `json:"token"`
}

// TransactionParams params for transaction
type TransactionParams struct {
	RoleID   int    `json:"roleId"`
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Password string
}

// LoginParams represent the http request data for login user
type LoginParams struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

// LoginResponse represents the response of login function
type LoginResponse struct {
	SessionID string      `json:"sessionId"`
	Claims    interface{} `json:"claims"`
}

// VerifyParams  ..
type VerifyParams struct {
	Token string `json:"token"`
}

// Storage represents the user storage interface
type Storage interface {
	FindAll(ctx context.Context, params *FindAllUsersParams) ([]*User, *types.Error)
	FindByID(ctx context.Context, userID int) (*User, *types.Error)
	FindByPhone(ctx context.Context, phone string) (*User, *types.Error)
	FindByToken(ctx context.Context, token string) (*User, *types.Error)
	Insert(ctx context.Context, user *User) (*User, *types.Error)
	Update(ctx context.Context, user *User) (*User, *types.Error)
	Delete(ctx context.Context, userID int) *types.Error
}

// ServiceInterface represents the user service interface
type ServiceInterface interface {
	ListUsers(ctx context.Context, params *FindAllUsersParams) ([]*User, int, *types.Error)
	GetUser(ctx context.Context, userID int) (*User, *types.Error)
	CreateUser(ctx context.Context, params *TransactionParams) (*User, *types.Error)
	Login(ctx context.Context, phone string, password string) (*LoginResponse, *types.Error)
	GetByToken(ctx context.Context, token string) (*User, *types.Error)
	VerifyTokenJWT(ctx context.Context, tokenString string) (interface{}, *types.Error)
}

// Service is the domain logic implementation of user Service interface
type Service struct {
	userStorage Storage
}

// ListUsers is listing users
func (s *Service) ListUsers(ctx context.Context, params *FindAllUsersParams) ([]*User, int, *types.Error) {
	users, err := s.userStorage.FindAll(ctx, params)
	if err != nil {
		err.Path = ".UserService->ListUsers()" + err.Path
		return nil, 0, err
	}
	params.Page = 0
	params.Limit = 0
	allUsers, err := s.userStorage.FindAll(ctx, params)
	if err != nil {
		err.Path = ".UserService->ListUsers()" + err.Path
		return nil, 0, err
	}

	return users, len(allUsers), nil
}

// GetUser is get user
func (s *Service) GetUser(ctx context.Context, userID int) (*User, *types.Error) {
	user, err := s.userStorage.FindByID(ctx, userID)
	if err != nil {
		err.Path = ".UserService->GetUser()" + err.Path
		return nil, err
	}

	return user, nil
}

// CreateUser create user
func (s *Service) CreateUser(ctx context.Context, params *TransactionParams) (*User, *types.Error) {
	users, _, errType := s.ListUsers(ctx, &FindAllUsersParams{
		Phone: params.Phone,
	})
	if errType != nil {
		errType.Path = ".UserService->CreateUser()" + errType.Path
		return nil, errType
	}
	if len(users) > 0 {
		return nil, &types.Error{
			Path:    ".UserService->CreateUser()",
			Message: ErrPhoneAlreadyExists.Error(),
			Error:   ErrPhoneAlreadyExists,
			Type:    "validation-error",
		}
	}

	bcryptHash, err := bcrypt.GenerateFromPassword([]byte(params.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, &types.Error{
			Path:    ".UserService->CreateUser()",
			Message: err.Error(),
			Error:   err,
			Type:    "golang-error",
		}
	}

	now := time.Now()

	user := &User{
		Name:           params.Name,
		RoleID:         params.RoleID,
		Phone:          params.Phone,
		Password:       string(bcryptHash),
		Token:          nil,
		TokenExpiredAt: nil,
		CreatedAt:      now,
		UpdatedAt:      &now,
	}

	user, errType = s.userStorage.Insert(ctx, user)
	if errType != nil {
		errType.Path = ".UserService->CreateUser()" + errType.Path
		return nil, errType
	}
	user.Password = params.Password

	return user, nil
}

// Login login
func (s *Service) Login(ctx context.Context, phone string, password string) (*LoginResponse, *types.Error) {
	users, err := s.userStorage.FindAll(ctx, &FindAllUsersParams{
		Phone: phone,
	})
	if err != nil {
		err.Path = ".UserService->Login()" + err.Path
		return nil, err
	}
	if len(users) < 1 {
		return nil, &types.Error{
			Path:    ".UserService->Login()",
			Message: ErrWrongPhone.Error(),
			Error:   ErrWrongPhone,
			Type:    "validation-error",
		}
	}

	user := users[0]
	errBcrypt := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if errBcrypt != nil {
		return nil, &types.Error{
			Path:    ".UserService->ChangePassword()",
			Message: ErrWrongPassword.Error(),
			Error:   ErrWrongPassword,
			Type:    "golang-error",
		}
	}

	now := time.Now()
	tokenExpiredAt := time.Now().Add(constants.ExpireTime)

	Token := jwt.New(constants.SigningMethod)
	tClaims := Token.Claims.(jwt.MapClaims)
	tClaims["name"] = user.Name
	tClaims["phone"] = user.Phone
	tClaims["roleId"] = user.RoleID
	tClaims["timestamp"] = tokenExpiredAt
	tClaims["iat"] = time.Now().Unix()
	tClaims["exp"] = time.Now().Add(constants.ExpireTime).Unix()
	t, errToken := Token.SignedString(constants.SignatureKey)
	if err != nil {
		err.Path = ".UserService->Login()" + err.Path
		err.Message = errToken.Error()
		return nil, err
	}

	user.Token = &t
	user.TokenExpiredAt = &tokenExpiredAt
	user.UpdatedAt = &now

	user, err = s.userStorage.Update(ctx, user)
	if err != nil {
		err.Path = ".UserService->CreateUser()" + err.Path
		return nil, err
	}

	return &LoginResponse{
		SessionID: t,
		Claims:    tClaims,
	}, nil
}

// GetByToken get user by its token
func (s *Service) GetByToken(ctx context.Context, token string) (*User, *types.Error) {
	user, err := s.userStorage.FindByToken(ctx, token)
	if err != nil {
		err.Path = ".UserService->GetByToken()" + err.Path
		return nil, err
	}

	return user, nil
}

// VerifyTokenJWT for verify token valid or not
func (s *Service) VerifyTokenJWT(ctx context.Context, tokenString string) (interface{}, *types.Error) {
	user, errType := s.GetByToken(ctx, tokenString)
	if errType != nil {
		errType.Path = ".UserService->VerifyTokenJWT()" + errType.Path
		return nil, errType
	}

	token, err := jwt.Parse(*user.Token, func(token *jwt.Token) (interface{}, error) {
		if method, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Invalid Token")
		} else if method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("Invalid Token")
		}

		return constants.SignatureKey, nil
	})
	if err != nil {
		return nil, &types.Error{
			Path:    ".UserService->VerifyTokenJWT()",
			Message: err.Error(),
			Error:   err,
			Type:    "Invalid Token",
		}
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, &types.Error{
			Path:    ".UserService->VerifyTokenJWT()",
			Message: err.Error(),
			Error:   err,
			Type:    "Invalid Token",
		}
	}
	return claims, nil
}

// NewService creates a new user AppService
func NewService(
	userStorage Storage,
) *Service {
	return &Service{
		userStorage: userStorage,
	}
}