package memoir

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/peizhengma/biography-v2/internal/domain/memoir"
)

var (
	ErrNotFound = errors.New("memoir not found")
)

// Repository 回忆录数据访问层
type Repository struct {
	pool *pgxpool.Pool
}

// New 创建 Repository
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create 创建回忆录
func (r *Repository) Create(ctx context.Context, m *memoir.Memoir) error {
	query := `
		INSERT INTO memoirs (id, user_id, conversation_id, title, content, time_period, start_year, end_year, sort_order, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := r.pool.Exec(ctx, query,
		m.ID,
		m.UserID,
		m.ConversationID,
		m.Title,
		m.Content,
		m.TimePeriod,
		m.StartYear,
		m.EndYear,
		m.SortOrder,
		m.CreatedAt,
		m.UpdatedAt,
	)

	return err
}

// GetByID 根据 ID 获取回忆录
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*memoir.Memoir, error) {
	query := `
		SELECT id, user_id, conversation_id, title, content, time_period, start_year, end_year, sort_order, created_at, updated_at, deleted_at
		FROM memoirs
		WHERE id = $1 AND deleted_at IS NULL
	`

	var m memoir.Memoir
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&m.ID,
		&m.UserID,
		&m.ConversationID,
		&m.Title,
		&m.Content,
		&m.TimePeriod,
		&m.StartYear,
		&m.EndYear,
		&m.SortOrder,
		&m.CreatedAt,
		&m.UpdatedAt,
		&m.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &m, nil
}

// Update 更新回忆录
func (r *Repository) Update(ctx context.Context, m *memoir.Memoir) error {
	query := `
		UPDATE memoirs
		SET title = $2, content = $3, time_period = $4, start_year = $5, end_year = $6, sort_order = $7, updated_at = $8
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.pool.Exec(ctx, query,
		m.ID,
		m.Title,
		m.Content,
		m.TimePeriod,
		m.StartYear,
		m.EndYear,
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

// SoftDelete 软删除回忆录
func (r *Repository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE memoirs SET deleted_at = $2 WHERE id = $1 AND deleted_at IS NULL`

	result, err := r.pool.Exec(ctx, query, id, time.Now())
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// List 获取回忆录列表
func (r *Repository) List(ctx context.Context, filter memoir.ListMemoirsFilter) ([]*memoir.Memoir, int, error) {
	// 获取总数
	countQuery := `SELECT COUNT(*) FROM memoirs WHERE user_id = $1 AND deleted_at IS NULL`
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, filter.UserID).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 获取列表
	query := `
		SELECT id, user_id, conversation_id, title, content, time_period, start_year, end_year, sort_order, created_at, updated_at, deleted_at
		FROM memoirs
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY sort_order ASC, start_year ASC NULLS LAST, created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, filter.UserID, filter.Limit, filter.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var memoirs []*memoir.Memoir
	for rows.Next() {
		var m memoir.Memoir
		err := rows.Scan(
			&m.ID,
			&m.UserID,
			&m.ConversationID,
			&m.Title,
			&m.Content,
			&m.TimePeriod,
			&m.StartYear,
			&m.EndYear,
			&m.SortOrder,
			&m.CreatedAt,
			&m.UpdatedAt,
			&m.DeletedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		memoirs = append(memoirs, &m)
	}

	return memoirs, total, nil
}

// ListByUserID 获取用户的所有回忆录（按时间线排序）
func (r *Repository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*memoir.Memoir, error) {
	query := `
		SELECT id, user_id, conversation_id, title, content, time_period, start_year, end_year, sort_order, created_at, updated_at, deleted_at
		FROM memoirs
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY sort_order ASC, start_year ASC NULLS LAST, created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memoirs []*memoir.Memoir
	for rows.Next() {
		var m memoir.Memoir
		err := rows.Scan(
			&m.ID,
			&m.UserID,
			&m.ConversationID,
			&m.Title,
			&m.Content,
			&m.TimePeriod,
			&m.StartYear,
			&m.EndYear,
			&m.SortOrder,
			&m.CreatedAt,
			&m.UpdatedAt,
			&m.DeletedAt,
		)
		if err != nil {
			return nil, err
		}
		memoirs = append(memoirs, &m)
	}

	return memoirs, nil
}

// GetByConversationID 根据对话 ID 获取回忆录
func (r *Repository) GetByConversationID(ctx context.Context, conversationID uuid.UUID) (*memoir.Memoir, error) {
	query := `
		SELECT id, user_id, conversation_id, title, content, time_period, start_year, end_year, sort_order, created_at, updated_at, deleted_at
		FROM memoirs
		WHERE conversation_id = $1 AND deleted_at IS NULL
	`

	var m memoir.Memoir
	err := r.pool.QueryRow(ctx, query, conversationID).Scan(
		&m.ID,
		&m.UserID,
		&m.ConversationID,
		&m.Title,
		&m.Content,
		&m.TimePeriod,
		&m.StartYear,
		&m.EndYear,
		&m.SortOrder,
		&m.CreatedAt,
		&m.UpdatedAt,
		&m.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &m, nil
}

// GetMaxSortOrder 获取用户回忆录的最大排序值
func (r *Repository) GetMaxSortOrder(ctx context.Context, userID uuid.UUID) (int, error) {
	query := `SELECT COALESCE(MAX(sort_order), 0) FROM memoirs WHERE user_id = $1 AND deleted_at IS NULL`
	var maxOrder int
	err := r.pool.QueryRow(ctx, query, userID).Scan(&maxOrder)
	return maxOrder, err
}

// UpdateSortOrders 批量更新排序
func (r *Repository) UpdateSortOrders(ctx context.Context, orders map[uuid.UUID]int) error {
	if len(orders) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for id, order := range orders {
		batch.Queue(`UPDATE memoirs SET sort_order = $2, updated_at = $3 WHERE id = $1`, id, order, time.Now())
	}

	results := r.pool.SendBatch(ctx, batch)
	defer results.Close()

	for range orders {
		if _, err := results.Exec(); err != nil {
			return err
		}
	}

	return nil
}

// Count 获取用户回忆录数量
func (r *Repository) Count(ctx context.Context, userID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM memoirs WHERE user_id = $1 AND deleted_at IS NULL`
	var count int
	err := r.pool.QueryRow(ctx, query, userID).Scan(&count)
	return count, err
}

// ListAll 获取所有回忆录（管理端）
func (r *Repository) ListAll(ctx context.Context, limit, offset int, includeDeleted bool) ([]*memoir.Memoir, int, error) {
	whereClause := "WHERE deleted_at IS NULL"
	if includeDeleted {
		whereClause = ""
	}

	// 获取总数
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM memoirs %s", whereClause)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 获取列表
	query := fmt.Sprintf(`
		SELECT id, user_id, conversation_id, title, content, time_period, start_year, end_year, sort_order, created_at, updated_at, deleted_at
		FROM memoirs
		%s
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, whereClause)

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var memoirs []*memoir.Memoir
	for rows.Next() {
		var m memoir.Memoir
		err := rows.Scan(
			&m.ID,
			&m.UserID,
			&m.ConversationID,
			&m.Title,
			&m.Content,
			&m.TimePeriod,
			&m.StartYear,
			&m.EndYear,
			&m.SortOrder,
			&m.CreatedAt,
			&m.UpdatedAt,
			&m.DeletedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		memoirs = append(memoirs, &m)
	}

	return memoirs, total, nil
}
