package conversation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/peizhengma/biography-v2/internal/domain/conversation"
)

var (
	ErrNotFound = errors.New("conversation not found")
)

// Repository 对话数据访问层
type Repository struct {
	pool *pgxpool.Pool
}

// New 创建 Repository
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create 创建对话
func (r *Repository) Create(ctx context.Context, c *conversation.Conversation) error {
	query := `
		INSERT INTO conversations (id, user_id, title, topic, topics, greeting, context, summary, mode, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err := r.pool.Exec(ctx, query,
		c.ID,
		c.UserID,
		c.Title,
		c.Topic,
		c.Topics,
		c.Greeting,
		c.Context,
		c.Summary,
		c.Mode,
		c.Status,
		c.CreatedAt,
		c.UpdatedAt,
	)

	return err
}

// GetByID 根据 ID 获取对话
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*conversation.Conversation, error) {
	query := `
		SELECT id, user_id, title, topic, topics, greeting, context, summary, mode, status, created_at, updated_at, deleted_at
		FROM conversations
		WHERE id = $1 AND deleted_at IS NULL
	`

	var c conversation.Conversation
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&c.ID,
		&c.UserID,
		&c.Title,
		&c.Topic,
		&c.Topics,
		&c.Greeting,
		&c.Context,
		&c.Summary,
		&c.Mode,
		&c.Status,
		&c.CreatedAt,
		&c.UpdatedAt,
		&c.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &c, nil
}

// GetByIDWithMessages 获取对话及其消息
func (r *Repository) GetByIDWithMessages(ctx context.Context, id uuid.UUID) (*conversation.Conversation, error) {
	c, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	messages, err := r.GetMessages(ctx, id, 0, 1000)
	if err != nil {
		return nil, err
	}

	c.Messages = messages
	c.MessageCount = len(messages)

	return c, nil
}

// Update 更新对话
func (r *Repository) Update(ctx context.Context, c *conversation.Conversation) error {
	query := `
		UPDATE conversations
		SET title = $2, topic = $3, topics = $4, greeting = $5, context = $6, summary = $7, mode = $8, status = $9, updated_at = $10
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.pool.Exec(ctx, query,
		c.ID,
		c.Title,
		c.Topic,
		c.Topics,
		c.Greeting,
		c.Context,
		c.Summary,
		c.Mode,
		c.Status,
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

// UpdateStatus 更新对话状态
func (r *Repository) UpdateStatus(ctx context.Context, id uuid.UUID, status conversation.Status) error {
	query := `UPDATE conversations SET status = $2, updated_at = $3 WHERE id = $1 AND deleted_at IS NULL`

	result, err := r.pool.Exec(ctx, query, id, status, time.Now())
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// UpdateSummary 更新对话摘要
func (r *Repository) UpdateSummary(ctx context.Context, id uuid.UUID, summary string) error {
	query := `UPDATE conversations SET summary = $2, updated_at = $3 WHERE id = $1 AND deleted_at IS NULL`

	result, err := r.pool.Exec(ctx, query, id, summary, time.Now())
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// SoftDelete 软删除对话
func (r *Repository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE conversations SET deleted_at = $2 WHERE id = $1 AND deleted_at IS NULL`

	result, err := r.pool.Exec(ctx, query, id, time.Now())
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// List 获取对话列表
func (r *Repository) List(ctx context.Context, filter conversation.ListConversationsFilter) ([]*conversation.Conversation, int, error) {
	// 构建查询条件
	whereClause := "WHERE user_id = $1 AND deleted_at IS NULL"
	args := []interface{}{filter.UserID}
	argIndex := 2

	if filter.Status != nil {
		whereClause += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, *filter.Status)
		argIndex++
	}

	// 获取总数
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM conversations %s", whereClause)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 获取列表（带消息数量）
	query := fmt.Sprintf(`
		SELECT c.id, c.user_id, c.title, c.topic, c.topics, c.greeting, c.context, c.summary, c.mode, c.status,
		       c.created_at, c.updated_at,
		       (SELECT COUNT(*) FROM messages m WHERE m.conversation_id = c.id) as message_count
		FROM conversations c
		%s
		ORDER BY c.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var conversations []*conversation.Conversation
	for rows.Next() {
		var c conversation.Conversation
		err := rows.Scan(
			&c.ID,
			&c.UserID,
			&c.Title,
			&c.Topic,
			&c.Topics,
			&c.Greeting,
			&c.Context,
			&c.Summary,
			&c.Mode,
			&c.Status,
			&c.CreatedAt,
			&c.UpdatedAt,
			&c.MessageCount,
		)
		if err != nil {
			return nil, 0, err
		}
		conversations = append(conversations, &c)
	}

	return conversations, total, nil
}

// ListAll 获取所有对话（管理端使用）
func (r *Repository) ListAll(ctx context.Context, userID *uuid.UUID, status *conversation.Status, limit, offset int) ([]*conversation.Conversation, int, error) {
	whereClause := "WHERE deleted_at IS NULL"
	args := []interface{}{}
	argIndex := 1

	if userID != nil {
		whereClause += fmt.Sprintf(" AND user_id = $%d", argIndex)
		args = append(args, *userID)
		argIndex++
	}

	if status != nil {
		whereClause += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, *status)
		argIndex++
	}

	// 获取总数
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM conversations %s", whereClause)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 获取列表
	query := fmt.Sprintf(`
		SELECT c.id, c.user_id, c.title, c.topic, c.topics, c.greeting, c.context, c.summary, c.mode, c.status,
		       c.created_at, c.updated_at,
		       (SELECT COUNT(*) FROM messages m WHERE m.conversation_id = c.id) as message_count
		FROM conversations c
		%s
		ORDER BY c.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var conversations []*conversation.Conversation
	for rows.Next() {
		var c conversation.Conversation
		err := rows.Scan(
			&c.ID,
			&c.UserID,
			&c.Title,
			&c.Topic,
			&c.Topics,
			&c.Greeting,
			&c.Context,
			&c.Summary,
			&c.Mode,
			&c.Status,
			&c.CreatedAt,
			&c.UpdatedAt,
			&c.MessageCount,
		)
		if err != nil {
			return nil, 0, err
		}
		conversations = append(conversations, &c)
	}

	return conversations, total, nil
}

// GetActiveByUserID 获取用户的活跃对话
func (r *Repository) GetActiveByUserID(ctx context.Context, userID uuid.UUID) (*conversation.Conversation, error) {
	query := `
		SELECT id, user_id, title, topic, topics, greeting, context, summary, mode, status, created_at, updated_at, deleted_at
		FROM conversations
		WHERE user_id = $1 AND status = 'active' AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1
	`

	var c conversation.Conversation
	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&c.ID,
		&c.UserID,
		&c.Title,
		&c.Topic,
		&c.Topics,
		&c.Greeting,
		&c.Context,
		&c.Summary,
		&c.Mode,
		&c.Status,
		&c.CreatedAt,
		&c.UpdatedAt,
		&c.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &c, nil
}

// Delete 删除对话（级联删除消息）
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM conversations WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// ============================================
// 消息相关方法
// ============================================

// AddMessage 添加消息
func (r *Repository) AddMessage(ctx context.Context, msg *conversation.Message) error {
	query := `
		INSERT INTO messages (id, conversation_id, role, content, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := r.pool.Exec(ctx, query,
		msg.ID,
		msg.ConversationID,
		msg.Role,
		msg.Content,
		msg.CreatedAt,
	)

	return err
}

// AddMessages 批量添加消息
func (r *Repository) AddMessages(ctx context.Context, messages []conversation.Message) error {
	if len(messages) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, msg := range messages {
		batch.Queue(
			`INSERT INTO messages (id, conversation_id, role, content, created_at) VALUES ($1, $2, $3, $4, $5)`,
			msg.ID, msg.ConversationID, msg.Role, msg.Content, msg.CreatedAt,
		)
	}

	results := r.pool.SendBatch(ctx, batch)
	defer results.Close()

	for range messages {
		if _, err := results.Exec(); err != nil {
			return err
		}
	}

	return nil
}

// GetMessages 获取对话消息
func (r *Repository) GetMessages(ctx context.Context, conversationID uuid.UUID, offset, limit int) ([]conversation.Message, error) {
	query := `
		SELECT id, conversation_id, role, content, created_at
		FROM messages
		WHERE conversation_id = $1
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, conversationID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []conversation.Message
	for rows.Next() {
		var m conversation.Message
		err := rows.Scan(
			&m.ID,
			&m.ConversationID,
			&m.Role,
			&m.Content,
			&m.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}

	return messages, nil
}

// GetMessageCount 获取消息数量
func (r *Repository) GetMessageCount(ctx context.Context, conversationID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM messages WHERE conversation_id = $1`
	var count int
	err := r.pool.QueryRow(ctx, query, conversationID).Scan(&count)
	return count, err
}

// GetLastMessage 获取最后一条消息
func (r *Repository) GetLastMessage(ctx context.Context, conversationID uuid.UUID) (*conversation.Message, error) {
	query := `
		SELECT id, conversation_id, role, content, created_at
		FROM messages
		WHERE conversation_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var m conversation.Message
	err := r.pool.QueryRow(ctx, query, conversationID).Scan(
		&m.ID,
		&m.ConversationID,
		&m.Role,
		&m.Content,
		&m.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &m, nil
}
