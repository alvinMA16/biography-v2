package user

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/peizhengma/biography-v2/internal/domain/user"
)

var (
	ErrNotFound      = errors.New("user not found")
	ErrAlreadyExists = errors.New("user already exists")
)

// Repository 用户数据访问层
type Repository struct {
	pool *pgxpool.Pool
}

// New 创建 Repository
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create 创建用户
func (r *Repository) Create(ctx context.Context, u *user.User) error {
	query := `
		INSERT INTO users (id, phone, password_hash, nickname, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.pool.Exec(ctx, query,
		u.ID,
		u.Phone,
		u.PasswordHash,
		u.Nickname,
		u.CreatedAt,
		u.UpdatedAt,
	)

	if err != nil {
		// 检查是否是唯一约束冲突
		if isDuplicateKeyError(err) {
			return ErrAlreadyExists
		}
		return err
	}

	return nil
}

// GetByID 根据 ID 获取用户
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	query := `
		SELECT id, phone, password_hash, nickname, preferred_name, gender,
		       birth_year, hometown, main_city, profile_completed, era_memories,
		       era_memories_status, is_admin, is_active, settings,
		       created_at, updated_at, deleted_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`

	var u user.User
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&u.ID,
		&u.Phone,
		&u.PasswordHash,
		&u.Nickname,
		&u.PreferredName,
		&u.Gender,
		&u.BirthYear,
		&u.Hometown,
		&u.MainCity,
		&u.ProfileCompleted,
		&u.EraMemories,
		&u.EraMemoriesStatus,
		&u.IsAdmin,
		&u.IsActive,
		&u.Settings,
		&u.CreatedAt,
		&u.UpdatedAt,
		&u.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &u, nil
}

// GetByPhone 根据手机号获取用户
func (r *Repository) GetByPhone(ctx context.Context, phone string) (*user.User, error) {
	query := `
		SELECT id, phone, password_hash, nickname, preferred_name, gender,
		       birth_year, hometown, main_city, profile_completed, era_memories,
		       era_memories_status, is_admin, is_active, settings,
		       created_at, updated_at, deleted_at
		FROM users
		WHERE phone = $1 AND deleted_at IS NULL
	`

	var u user.User
	err := r.pool.QueryRow(ctx, query, phone).Scan(
		&u.ID,
		&u.Phone,
		&u.PasswordHash,
		&u.Nickname,
		&u.PreferredName,
		&u.Gender,
		&u.BirthYear,
		&u.Hometown,
		&u.MainCity,
		&u.ProfileCompleted,
		&u.EraMemories,
		&u.EraMemoriesStatus,
		&u.IsAdmin,
		&u.IsActive,
		&u.Settings,
		&u.CreatedAt,
		&u.UpdatedAt,
		&u.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &u, nil
}

// Update 更新用户
func (r *Repository) Update(ctx context.Context, u *user.User) error {
	query := `
		UPDATE users
		SET nickname = $2, preferred_name = $3, gender = $4, birth_year = $5,
		    hometown = $6, main_city = $7, profile_completed = $8, era_memories = $9,
		    era_memories_status = $10, is_active = $11, settings = $12, updated_at = $13
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.pool.Exec(ctx, query,
		u.ID,
		u.Nickname,
		u.PreferredName,
		u.Gender,
		u.BirthYear,
		u.Hometown,
		u.MainCity,
		u.ProfileCompleted,
		u.EraMemories,
		u.EraMemoriesStatus,
		u.IsActive,
		u.Settings,
		time.Now(),
	)

	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// UpdateSettings 更新用户设置
func (r *Repository) UpdateSettings(ctx context.Context, id uuid.UUID, settings *user.UserSettings) error {
	query := `UPDATE users SET settings = $2, updated_at = $3 WHERE id = $1 AND deleted_at IS NULL`

	result, err := r.pool.Exec(ctx, query, id, settings, time.Now())
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// UpdateEraMemories 更新时代记忆
func (r *Repository) UpdateEraMemories(ctx context.Context, id uuid.UUID, eraMemories string, status string) error {
	query := `UPDATE users SET era_memories = $2, era_memories_status = $3, updated_at = $4 WHERE id = $1 AND deleted_at IS NULL`

	result, err := r.pool.Exec(ctx, query, id, eraMemories, status, time.Now())
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// UpdateEraMemoriesStatus 更新时代记忆状态
func (r *Repository) UpdateEraMemoriesStatus(ctx context.Context, id uuid.UUID, status string) error {
	query := `UPDATE users SET era_memories_status = $2, updated_at = $3 WHERE id = $1 AND deleted_at IS NULL`

	result, err := r.pool.Exec(ctx, query, id, status, time.Now())
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// UpdatePassword 更新密码
func (r *Repository) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	query := `
		UPDATE users
		SET password_hash = $2, updated_at = $3
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.pool.Exec(ctx, query, id, passwordHash, time.Now())
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// SoftDelete 软删除用户
func (r *Repository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE users
		SET deleted_at = $2
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.pool.Exec(ctx, query, id, time.Now())
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// List 获取用户列表（管理端使用）
func (r *Repository) List(ctx context.Context, limit, offset int) ([]*user.User, int, error) {
	// 获取总数
	countQuery := `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`
	var total int
	if err := r.pool.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 获取列表
	query := `
		SELECT id, phone, password_hash, nickname, preferred_name, gender,
		       birth_year, hometown, main_city, profile_completed, era_memories,
		       era_memories_status, is_admin, is_active, settings,
		       created_at, updated_at, deleted_at
		FROM users
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []*user.User
	for rows.Next() {
		var u user.User
		err := rows.Scan(
			&u.ID,
			&u.Phone,
			&u.PasswordHash,
			&u.Nickname,
			&u.PreferredName,
			&u.Gender,
			&u.BirthYear,
			&u.Hometown,
			&u.MainCity,
			&u.ProfileCompleted,
			&u.EraMemories,
			&u.EraMemoriesStatus,
			&u.IsAdmin,
			&u.IsActive,
			&u.Settings,
			&u.CreatedAt,
			&u.UpdatedAt,
			&u.DeletedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, &u)
	}

	return users, total, nil
}

// ExistsByPhone 检查手机号是否已存在
func (r *Repository) ExistsByPhone(ctx context.Context, phone string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE phone = $1 AND deleted_at IS NULL)`
	var exists bool
	err := r.pool.QueryRow(ctx, query, phone).Scan(&exists)
	return exists, err
}

// isDuplicateKeyError 检查是否是唯一约束冲突
func isDuplicateKeyError(err error) bool {
	// pgx 的错误处理
	if err == nil {
		return false
	}
	// 检查 PostgreSQL 错误码 23505 (unique_violation)
	return err.Error() == "ERROR: duplicate key value violates unique constraint \"users_phone_key\" (SQLSTATE 23505)" ||
		// 更通用的检查
		contains(err.Error(), "duplicate key") ||
		contains(err.Error(), "23505")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
