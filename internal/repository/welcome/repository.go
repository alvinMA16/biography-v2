package welcome

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/peizhengma/biography-v2/internal/domain/welcome"
)

var (
	ErrNotFound = errors.New("welcome message not found")
)

// Repository 欢迎语数据访问层
type Repository struct {
	pool *pgxpool.Pool
}

// New 创建 Repository
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create 创建欢迎语
func (r *Repository) Create(ctx context.Context, m *welcome.Message) error {
	query := `
		INSERT INTO welcome_messages (id, content, show_greeting, is_active, sort_order, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.pool.Exec(ctx, query,
		m.ID,
		m.Content,
		m.ShowGreeting,
		m.IsActive,
		m.SortOrder,
		m.CreatedAt,
		m.UpdatedAt,
	)
	return err
}

// GetByID 根据 ID 获取欢迎语
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*welcome.Message, error) {
	query := `
		SELECT id, content, show_greeting, is_active, sort_order, created_at, updated_at
		FROM welcome_messages
		WHERE id = $1
	`

	var m welcome.Message
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&m.ID,
		&m.Content,
		&m.ShowGreeting,
		&m.IsActive,
		&m.SortOrder,
		&m.CreatedAt,
		&m.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &m, nil
}

// Update 更新欢迎语
func (r *Repository) Update(ctx context.Context, m *welcome.Message) error {
	query := `
		UPDATE welcome_messages
		SET content = $2, show_greeting = $3, is_active = $4, sort_order = $5, updated_at = $6
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query,
		m.ID,
		m.Content,
		m.ShowGreeting,
		m.IsActive,
		m.SortOrder,
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

// Delete 删除欢迎语
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM welcome_messages WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// List 获取欢迎语列表
func (r *Repository) List(ctx context.Context, limit, offset int) ([]*welcome.Message, int, error) {
	// 获取总数
	countQuery := `SELECT COUNT(*) FROM welcome_messages`
	var total int
	if err := r.pool.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 获取列表
	query := `
		SELECT id, content, show_greeting, is_active, sort_order, created_at, updated_at
		FROM welcome_messages
		ORDER BY sort_order ASC, created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var messages []*welcome.Message
	for rows.Next() {
		var m welcome.Message
		err := rows.Scan(
			&m.ID,
			&m.Content,
			&m.ShowGreeting,
			&m.IsActive,
			&m.SortOrder,
			&m.CreatedAt,
			&m.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		messages = append(messages, &m)
	}

	return messages, total, nil
}

// GetActive 获取启用的欢迎语
func (r *Repository) GetActive(ctx context.Context) ([]*welcome.Message, error) {
	query := `
		SELECT id, content, show_greeting, is_active, sort_order, created_at, updated_at
		FROM welcome_messages
		WHERE is_active = TRUE
		ORDER BY sort_order ASC, created_at DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*welcome.Message
	for rows.Next() {
		var m welcome.Message
		err := rows.Scan(
			&m.ID,
			&m.Content,
			&m.ShowGreeting,
			&m.IsActive,
			&m.SortOrder,
			&m.CreatedAt,
			&m.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		messages = append(messages, &m)
	}

	return messages, nil
}
