package quote

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/peizhengma/biography-v2/internal/domain/quote"
)

var (
	ErrNotFound = errors.New("quote not found")
)

// Repository 激励语数据访问层
type Repository struct {
	pool *pgxpool.Pool
}

// New 创建 Repository
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create 创建激励语
func (r *Repository) Create(ctx context.Context, q *quote.Quote) error {
	query := `
		INSERT INTO quotes (id, content, type, is_active, show_greeting, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.pool.Exec(ctx, query,
		q.ID,
		q.Content,
		q.Type,
		q.IsActive,
		q.ShowGreeting,
		q.CreatedAt,
		q.UpdatedAt,
	)

	return err
}

// GetByID 根据 ID 获取激励语
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*quote.Quote, error) {
	query := `
		SELECT id, content, type, is_active, show_greeting, created_at, updated_at
		FROM quotes
		WHERE id = $1
	`

	var q quote.Quote
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&q.ID,
		&q.Content,
		&q.Type,
		&q.IsActive,
		&q.ShowGreeting,
		&q.CreatedAt,
		&q.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &q, nil
}

// Update 更新激励语
func (r *Repository) Update(ctx context.Context, q *quote.Quote) error {
	query := `
		UPDATE quotes
		SET content = $2, type = $3, is_active = $4, show_greeting = $5, updated_at = $6
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query,
		q.ID,
		q.Content,
		q.Type,
		q.IsActive,
		q.ShowGreeting,
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

// Delete 删除激励语
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM quotes WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// List 获取激励语列表
func (r *Repository) List(ctx context.Context, quoteType *quote.Type, activeOnly bool, limit, offset int) ([]*quote.Quote, int, error) {
	whereClause := "WHERE 1=1"
	args := []interface{}{}
	argIndex := 1

	if quoteType != nil {
		whereClause += fmt.Sprintf(" AND type = $%d", argIndex)
		args = append(args, *quoteType)
		argIndex++
	}

	if activeOnly {
		whereClause += " AND is_active = TRUE"
	}

	// 获取总数
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM quotes %s", whereClause)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 获取列表
	query := fmt.Sprintf(`
		SELECT id, content, type, is_active, show_greeting, created_at, updated_at
		FROM quotes
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var quotes []*quote.Quote
	for rows.Next() {
		var q quote.Quote
		err := rows.Scan(
			&q.ID,
			&q.Content,
			&q.Type,
			&q.IsActive,
			&q.ShowGreeting,
			&q.CreatedAt,
			&q.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		quotes = append(quotes, &q)
	}

	return quotes, total, nil
}

// GetRandom 随机获取一条激励语
func (r *Repository) GetRandom(ctx context.Context, quoteType quote.Type) (*quote.Quote, error) {
	query := `
		SELECT id, content, type, is_active, show_greeting, created_at, updated_at
		FROM quotes
		WHERE type = $1 AND is_active = TRUE
		ORDER BY RANDOM()
		LIMIT 1
	`

	var q quote.Quote
	err := r.pool.QueryRow(ctx, query, quoteType).Scan(
		&q.ID,
		&q.Content,
		&q.Type,
		&q.IsActive,
		&q.ShowGreeting,
		&q.CreatedAt,
		&q.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &q, nil
}

// GetGreetingQuotes 获取问候语
func (r *Repository) GetGreetingQuotes(ctx context.Context) ([]*quote.Quote, error) {
	query := `
		SELECT id, content, type, is_active, show_greeting, created_at, updated_at
		FROM quotes
		WHERE show_greeting = TRUE AND is_active = TRUE
		ORDER BY RANDOM()
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var quotes []*quote.Quote
	for rows.Next() {
		var q quote.Quote
		err := rows.Scan(
			&q.ID,
			&q.Content,
			&q.Type,
			&q.IsActive,
			&q.ShowGreeting,
			&q.CreatedAt,
			&q.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		quotes = append(quotes, &q)
	}

	return quotes, nil
}
