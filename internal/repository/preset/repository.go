package preset

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/peizhengma/biography-v2/internal/domain/preset"
)

var (
	ErrNotFound = errors.New("preset topic not found")
)

// Repository 预设话题数据访问层
type Repository struct {
	pool *pgxpool.Pool
}

// New 创建 Repository
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create 创建预设话题
func (r *Repository) Create(ctx context.Context, t *preset.Topic) error {
	query := `
		INSERT INTO preset_topics (id, topic, greeting, chat_context, age_start, age_end, is_active, sort_order, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.pool.Exec(ctx, query,
		t.ID,
		t.Topic,
		t.Greeting,
		t.ChatContext,
		t.AgeStart,
		t.AgeEnd,
		t.IsActive,
		t.SortOrder,
		t.CreatedAt,
		t.UpdatedAt,
	)

	return err
}

// GetByID 根据 ID 获取预设话题
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*preset.Topic, error) {
	query := `
		SELECT id, topic, greeting, chat_context, age_start, age_end, is_active, sort_order, created_at, updated_at
		FROM preset_topics
		WHERE id = $1
	`

	var t preset.Topic
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&t.ID,
		&t.Topic,
		&t.Greeting,
		&t.ChatContext,
		&t.AgeStart,
		&t.AgeEnd,
		&t.IsActive,
		&t.SortOrder,
		&t.CreatedAt,
		&t.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &t, nil
}

// Update 更新预设话题
func (r *Repository) Update(ctx context.Context, t *preset.Topic) error {
	query := `
		UPDATE preset_topics
		SET topic = $2, greeting = $3, chat_context = $4, age_start = $5, age_end = $6,
		    is_active = $7, sort_order = $8, updated_at = $9
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query,
		t.ID,
		t.Topic,
		t.Greeting,
		t.ChatContext,
		t.AgeStart,
		t.AgeEnd,
		t.IsActive,
		t.SortOrder,
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

// Delete 删除预设话题
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM preset_topics WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// List 获取预设话题列表
func (r *Repository) List(ctx context.Context, filter preset.ListTopicsFilter) ([]*preset.Topic, int, error) {
	whereClause := "WHERE 1=1"
	args := []interface{}{}
	argIndex := 1

	if filter.ActiveOnly {
		whereClause += " AND is_active = TRUE"
	}

	// 获取总数
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM preset_topics %s", whereClause)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 获取列表
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filter.Offset

	query := fmt.Sprintf(`
		SELECT id, topic, greeting, chat_context, age_start, age_end, is_active, sort_order, created_at, updated_at
		FROM preset_topics
		%s
		ORDER BY sort_order ASC, created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var topics []*preset.Topic
	for rows.Next() {
		var t preset.Topic
		err := rows.Scan(
			&t.ID,
			&t.Topic,
			&t.Greeting,
			&t.ChatContext,
			&t.AgeStart,
			&t.AgeEnd,
			&t.IsActive,
			&t.SortOrder,
			&t.CreatedAt,
			&t.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		topics = append(topics, &t)
	}

	return topics, total, nil
}

// GetActiveTopics 获取所有激活的预设话题
func (r *Repository) GetActiveTopics(ctx context.Context) ([]*preset.Topic, error) {
	query := `
		SELECT id, topic, greeting, chat_context, age_start, age_end, is_active, sort_order, created_at, updated_at
		FROM preset_topics
		WHERE is_active = TRUE
		ORDER BY sort_order ASC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var topics []*preset.Topic
	for rows.Next() {
		var t preset.Topic
		err := rows.Scan(
			&t.ID,
			&t.Topic,
			&t.Greeting,
			&t.ChatContext,
			&t.AgeStart,
			&t.AgeEnd,
			&t.IsActive,
			&t.SortOrder,
			&t.CreatedAt,
			&t.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		topics = append(topics, &t)
	}

	return topics, nil
}
