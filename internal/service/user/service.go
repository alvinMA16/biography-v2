package user

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/peizhengma/biography-v2/internal/domain/user"
	userRepo "github.com/peizhengma/biography-v2/internal/repository/user"
)

var (
	ErrInvalidPhone       = errors.New("invalid phone number")
	ErrInvalidPassword    = errors.New("password must be at least 6 characters")
	ErrPhoneAlreadyExists = errors.New("phone number already registered")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid phone or password")
	ErrWrongPassword      = errors.New("wrong password")
)

// Service 用户服务
type Service struct {
	repo          *userRepo.Repository
	jwtSecret     string
	jwtExpireDays int
}

// Config 服务配置
type Config struct {
	JWTSecret     string
	JWTExpireDays int
}

// New 创建用户服务
func New(repo *userRepo.Repository, cfg Config) *Service {
	expireDays := cfg.JWTExpireDays
	if expireDays == 0 {
		expireDays = 30
	}

	return &Service{
		repo:          repo,
		jwtSecret:     cfg.JWTSecret,
		jwtExpireDays: expireDays,
	}
}

// Register 用户注册
func (s *Service) Register(ctx context.Context, input *user.CreateUserInput) (*user.AuthResponse, error) {
	// 验证手机号
	if !isValidPhone(input.Phone) {
		return nil, ErrInvalidPhone
	}

	// 验证密码
	if len(input.Password) < 6 {
		return nil, ErrInvalidPassword
	}

	// 检查手机号是否已存在
	exists, err := s.repo.ExistsByPhone(ctx, input.Phone)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrPhoneAlreadyExists
	}

	// 密码加密
	passwordHash, err := hashPassword(input.Password)
	if err != nil {
		return nil, err
	}

	// 创建用户
	now := time.Now()
	u := &user.User{
		ID:           uuid.New(),
		Phone:        input.Phone,
		PasswordHash: passwordHash,
		Nickname:     nilIfEmpty(input.Nickname),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.repo.Create(ctx, u); err != nil {
		if errors.Is(err, userRepo.ErrAlreadyExists) {
			return nil, ErrPhoneAlreadyExists
		}
		return nil, err
	}

	// 生成 JWT
	token, err := s.generateToken(u.ID)
	if err != nil {
		return nil, err
	}

	return &user.AuthResponse{
		Token: token,
		User:  u,
	}, nil
}

// Login 用户登录
func (s *Service) Login(ctx context.Context, input *user.LoginInput) (*user.AuthResponse, error) {
	// 根据手机号查找用户
	u, err := s.repo.GetByPhone(ctx, input.Phone)
	if err != nil {
		if errors.Is(err, userRepo.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	// 验证密码
	if !checkPassword(input.Password, u.PasswordHash) {
		return nil, ErrInvalidCredentials
	}

	// 生成 JWT
	token, err := s.generateToken(u.ID)
	if err != nil {
		return nil, err
	}

	return &user.AuthResponse{
		Token: token,
		User:  u,
	}, nil
}

// GetProfile 获取用户信息
func (s *Service) GetProfile(ctx context.Context, userID uuid.UUID) (*user.User, error) {
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, userRepo.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return u, nil
}

// UpdateProfile 更新用户信息
func (s *Service) UpdateProfile(ctx context.Context, userID uuid.UUID, input *user.UpdateProfileInput) (*user.User, error) {
	// 获取现有用户
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, userRepo.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	// 更新字段
	if input.Nickname != nil {
		u.Nickname = input.Nickname
	}
	if input.PreferredName != nil {
		u.PreferredName = input.PreferredName
	}
	if input.Gender != nil {
		u.Gender = input.Gender
	}
	if input.BirthYear != nil {
		u.BirthYear = input.BirthYear
	}
	if input.Hometown != nil {
		u.Hometown = input.Hometown
	}
	if input.MainCity != nil {
		u.MainCity = input.MainCity
	}

	// 检查是否完成资料填写
	u.ProfileCompleted = isProfileCompleted(u)

	// 保存更新
	if err := s.repo.Update(ctx, u); err != nil {
		return nil, err
	}

	return u, nil
}

// ChangePassword 修改密码
func (s *Service) ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) error {
	// 获取用户
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, userRepo.ErrNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	// 验证旧密码
	if !checkPassword(oldPassword, u.PasswordHash) {
		return ErrWrongPassword
	}

	// 验证新密码
	if len(newPassword) < 6 {
		return ErrInvalidPassword
	}

	// 加密新密码
	passwordHash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}

	// 更新密码
	return s.repo.UpdatePassword(ctx, userID, passwordHash)
}

// GetByID 根据 ID 获取用户
func (s *Service) GetByID(ctx context.Context, userID uuid.UUID) (*user.User, error) {
	return s.GetProfile(ctx, userID)
}

// UpdateEraMemories 更新时代记忆
func (s *Service) UpdateEraMemories(ctx context.Context, userID uuid.UUID, eraMemories, status string) error {
	return s.repo.UpdateEraMemories(ctx, userID, eraMemories, status)
}

// UpdateEraMemoriesStatus 更新时代记忆状态
func (s *Service) UpdateEraMemoriesStatus(ctx context.Context, userID uuid.UUID, status string) error {
	return s.repo.UpdateEraMemoriesStatus(ctx, userID, status)
}

// ============================================
// Admin 方法
// ============================================

// AdminList 获取用户列表（管理员）
func (s *Service) AdminList(ctx context.Context, limit, offset int) ([]*user.User, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.List(ctx, limit, offset)
}

// AdminCreate 管理员创建用户
func (s *Service) AdminCreate(ctx context.Context, input *user.AdminCreateInput) (*user.User, error) {
	// 验证手机号
	if !isValidPhone(input.Phone) {
		return nil, ErrInvalidPhone
	}

	// 验证密码
	if len(input.Password) < 6 {
		return nil, ErrInvalidPassword
	}

	// 检查手机号是否已存在
	exists, err := s.repo.ExistsByPhone(ctx, input.Phone)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrPhoneAlreadyExists
	}

	// 密码加密
	passwordHash, err := hashPassword(input.Password)
	if err != nil {
		return nil, err
	}

	// 创建用户
	now := time.Now()
	u := &user.User{
		ID:           uuid.New(),
		Phone:        input.Phone,
		PasswordHash: passwordHash,
		Nickname:     input.Nickname,
		Gender:       input.Gender,
		BirthYear:    input.BirthYear,
		Hometown:     input.Hometown,
		MainCity:     input.MainCity,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// 检查是否完成资料填写
	u.ProfileCompleted = isProfileCompleted(u)

	if err := s.repo.Create(ctx, u); err != nil {
		if errors.Is(err, userRepo.ErrAlreadyExists) {
			return nil, ErrPhoneAlreadyExists
		}
		return nil, err
	}

	return u, nil
}

// AdminUpdate 管理员更新用户（可更新任意字段）
func (s *Service) AdminUpdate(ctx context.Context, userID uuid.UUID, input *user.AdminUpdateInput) (*user.User, error) {
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, userRepo.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	if input.Nickname != nil {
		u.Nickname = input.Nickname
	}
	if input.PreferredName != nil {
		u.PreferredName = input.PreferredName
	}
	if input.Gender != nil {
		u.Gender = input.Gender
	}
	if input.BirthYear != nil {
		u.BirthYear = input.BirthYear
	}
	if input.Hometown != nil {
		u.Hometown = input.Hometown
	}
	if input.MainCity != nil {
		u.MainCity = input.MainCity
	}
	if input.ProfileCompleted != nil {
		u.ProfileCompleted = *input.ProfileCompleted
	}
	if input.IsActive != nil {
		u.IsActive = *input.IsActive
	}

	u.ProfileCompleted = isProfileCompleted(u)

	if err := s.repo.Update(ctx, u); err != nil {
		return nil, err
	}

	return u, nil
}

// AdminDelete 管理员删除用户（软删除）
func (s *Service) AdminDelete(ctx context.Context, userID uuid.UUID) error {
	_, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, userRepo.ErrNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	return s.repo.SoftDelete(ctx, userID)
}

// AdminResetPassword 管理员重置用户密码
func (s *Service) AdminResetPassword(ctx context.Context, userID uuid.UUID, newPassword string) error {
	// 验证用户存在
	_, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, userRepo.ErrNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	// 验证新密码
	if len(newPassword) < 6 {
		return ErrInvalidPassword
	}

	// 加密新密码
	passwordHash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}

	// 更新密码
	return s.repo.UpdatePassword(ctx, userID, passwordHash)
}

// generateToken 生成 JWT Token
func (s *Service) generateToken(userID uuid.UUID) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID.String(),
		"exp": time.Now().Add(time.Duration(s.jwtExpireDays) * 24 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

// hashPassword 密码加密
func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// checkPassword 验证密码
func checkPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// isValidPhone 验证手机号格式
func isValidPhone(phone string) bool {
	// 中国大陆手机号：11位数字，以1开头
	pattern := `^1[3-9]\d{9}$`
	matched, _ := regexp.MatchString(pattern, phone)
	return matched
}

// isProfileCompleted 检查用户资料是否完整
func isProfileCompleted(u *user.User) bool {
	return u.Nickname != nil && *u.Nickname != "" &&
		u.BirthYear != nil &&
		u.Hometown != nil && *u.Hometown != ""
}

// nilIfEmpty 空字符串转 nil
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
