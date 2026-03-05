package topic

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/peizhengma/biography-v2/internal/domain/topic"
)

var (
	ErrNotFound = errors.New("topic not found")
)

// Repository 话题数据访问层
type Repository struct {
	pool *pgxpool.Pool
}

// New 创建 Repository
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create 创建话题
func (r *Repository) Create(ctx context.Context, t *topic.TopicCandidate) error {
	query := `
		INSERT INTO topic_candidates (id, user_id, title, greeting, context, status, source, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.pool.Exec(ctx, query,
		t.ID,
		t.UserID,
		t.Title,
		t.Greeting,
		t.Context,
		t.Status,
		t.Source,
		t.CreatedAt,
		t.UpdatedAt,
	)

	return err
}

// CreateBatch 批量创建话题
func (r *Repository) CreateBatch(ctx context.Context, topics []*topic.TopicCandidate) error {
	if len(topics) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, t := range topics {
		batch.Queue(
			`INSERT INTO topic_candidates (id, user_id, title, greeting, context, status, source, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			t.ID, t.UserID, t.Title, t.Greeting, t.Context, t.Status, t.Source, t.CreatedAt, t.UpdatedAt,
		)
	}

	results := r.pool.SendBatch(ctx, batch)
	defer results.Close()

	for range topics {
		if _, err := results.Exec(); err != nil {
			return err
		}
	}

	return nil
}

// GetByID 根据 ID 获取话题
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*topic.TopicCandidate, error) {
	query := `
		SELECT id, user_id, title, greeting, context, status, source, created_at, updated_at
		FROM topic_candidates
		WHERE id = $1
	`

	var t topic.TopicCandidate
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&t.ID,
		&t.UserID,
		&t.Title,
		&t.Greeting,
		&t.Context,
		&t.Status,
		&t.Source,
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

// Update 更新话题
func (r *Repository) Update(ctx context.Context, t *topic.TopicCandidate) error {
	query := `
		UPDATE topic_candidates
		SET title = $2, greeting = $3, context = $4, status = $5, updated_at = $6
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query,
		t.ID,
		t.Title,
		t.Greeting,
		t.Context,
		t.Status,
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

// UpdateStatus 更新话题状态
func (r *Repository) UpdateStatus(ctx context.Context, id uuid.UUID, status topic.Status) error {
	query := `UPDATE topic_candidates SET status = $2, updated_at = $3 WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id, status, time.Now())
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// Delete 删除话题
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM topic_candidates WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// List 获取话题列表
func (r *Repository) List(ctx context.Context, userID uuid.UUID, status *topic.Status, limit, offset int) ([]*topic.TopicCandidate, int, error) {
	whereClause := "WHERE user_id = $1"
	args := []interface{}{userID}
	argIndex := 2

	if status != nil {
		whereClause += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, *status)
		argIndex++
	}

	// 获取总数
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM topic_candidates %s", whereClause)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 获取列表
	query := fmt.Sprintf(`
		SELECT id, user_id, title, greeting, context, status, source, created_at, updated_at
		FROM topic_candidates
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

	var topics []*topic.TopicCandidate
	for rows.Next() {
		var t topic.TopicCandidate
		err := rows.Scan(
			&t.ID,
			&t.UserID,
			&t.Title,
			&t.Greeting,
			&t.Context,
			&t.Status,
			&t.Source,
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

// GetAvailableTopics 获取可用话题（状态为 approved 的话题）
func (r *Repository) GetAvailableTopics(ctx context.Context, userID uuid.UUID, limit int) ([]*topic.TopicCandidate, error) {
	query := `
		SELECT id, user_id, title, greeting, context, status, source, created_at, updated_at
		FROM topic_candidates
		WHERE user_id = $1 AND status = 'approved'
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var topics []*topic.TopicCandidate
	for rows.Next() {
		var t topic.TopicCandidate
		err := rows.Scan(
			&t.ID,
			&t.UserID,
			&t.Title,
			&t.Greeting,
			&t.Context,
			&t.Status,
			&t.Source,
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

// GetPendingTopics 获取待审核话题
func (r *Repository) GetPendingTopics(ctx context.Context, userID uuid.UUID) ([]*topic.TopicCandidate, error) {
	query := `
		SELECT id, user_id, title, greeting, context, status, source, created_at, updated_at
		FROM topic_candidates
		WHERE user_id = $1 AND status = 'pending'
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var topics []*topic.TopicCandidate
	for rows.Next() {
		var t topic.TopicCandidate
		err := rows.Scan(
			&t.ID,
			&t.UserID,
			&t.Title,
			&t.Greeting,
			&t.Context,
			&t.Status,
			&t.Source,
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

// CountByStatus 按状态统计话题数量
func (r *Repository) CountByStatus(ctx context.Context, userID uuid.UUID, status topic.Status) (int, error) {
	query := `SELECT COUNT(*) FROM topic_candidates WHERE user_id = $1 AND status = $2`
	var count int
	err := r.pool.QueryRow(ctx, query, userID, status).Scan(&count)
	return count, err
}

// GetTopicTitles 获取用户话题标题列表（用于避免重复生成）
func (r *Repository) GetTopicTitles(ctx context.Context, userID uuid.UUID) ([]string, error) {
	query := `SELECT title FROM topic_candidates WHERE user_id = $1`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var titles []string
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			return nil, err
		}
		titles = append(titles, title)
	}

	return titles, nil
}

// DeleteByUserID 删除用户的所有话题
func (r *Repository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	query := `DELETE FROM topic_candidates WHERE user_id = $1`
	_, err := r.pool.Exec(ctx, query, userID)
	return err
}
