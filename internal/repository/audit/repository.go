package audit

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/peizhengma/biography-v2/internal/domain/audit"
)

var (
	ErrNotFound = errors.New("audit log not found")
)

// Repository 审计日志数据访问层
type Repository struct {
	pool *pgxpool.Pool
}

// New 创建 Repository
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create 创建审计日志
func (r *Repository) Create(ctx context.Context, log *audit.Log) error {
	query := `
		INSERT INTO audit_logs (id, admin_id, action, target_type, target_id, target_label, detail, ip_address, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.pool.Exec(ctx, query,
		log.ID,
		log.AdminID,
		log.Action,
		log.TargetType,
		log.TargetID,
		log.TargetLabel,
		log.Detail,
		log.IPAddress,
		log.CreatedAt,
	)
	return err
}

// GetByID 根据 ID 获取审计日志
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*audit.Log, error) {
	query := `
		SELECT id, admin_id, action, target_type, target_id, target_label, detail, ip_address, created_at
		FROM audit_logs
		WHERE id = $1
	`

	var log audit.Log
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&log.ID,
		&log.AdminID,
		&log.Action,
		&log.TargetType,
		&log.TargetID,
		&log.TargetLabel,
		&log.Detail,
		&log.IPAddress,
		&log.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &log, nil
}

// List 获取审计日志列表
func (r *Repository) List(ctx context.Context, filter audit.ListFilter) ([]*audit.Log, int, error) {
	// 构建 WHERE 条件
	var conditions []string
	var args []interface{}
	argIndex := 1

	if filter.AdminID != nil {
		conditions = append(conditions, "admin_id = $"+string(rune('0'+argIndex)))
		args = append(args, *filter.AdminID)
		argIndex++
	}
	if filter.Action != nil {
		conditions = append(conditions, "action = $"+string(rune('0'+argIndex)))
		args = append(args, *filter.Action)
		argIndex++
	}
	if filter.TargetType != nil {
		conditions = append(conditions, "target_type = $"+string(rune('0'+argIndex)))
		args = append(args, *filter.TargetType)
		argIndex++
	}
	if filter.TargetID != nil {
		conditions = append(conditions, "target_id = $"+string(rune('0'+argIndex)))
		args = append(args, *filter.TargetID)
		argIndex++
	}
	if filter.StartTime != nil {
		conditions = append(conditions, "created_at >= $"+string(rune('0'+argIndex)))
		args = append(args, *filter.StartTime)
		argIndex++
	}
	if filter.EndTime != nil {
		conditions = append(conditions, "created_at <= $"+string(rune('0'+argIndex)))
		args = append(args, *filter.EndTime)
		argIndex++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 获取总数
	countQuery := "SELECT COUNT(*) FROM audit_logs " + whereClause
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 获取列表
	query := `
		SELECT id, admin_id, action, target_type, target_id, target_label, detail, ip_address, created_at
		FROM audit_logs
		` + whereClause + `
		ORDER BY created_at DESC
		LIMIT $` + string(rune('0'+argIndex)) + ` OFFSET $` + string(rune('0'+argIndex+1))

	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []*audit.Log
	for rows.Next() {
		var log audit.Log
		err := rows.Scan(
			&log.ID,
			&log.AdminID,
			&log.Action,
			&log.TargetType,
			&log.TargetID,
			&log.TargetLabel,
			&log.Detail,
			&log.IPAddress,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		logs = append(logs, &log)
	}

	return logs, total, nil
}

// ListSimple 简单列表查询（不带过滤）
func (r *Repository) ListSimple(ctx context.Context, limit, offset int) ([]*audit.Log, int, error) {
	// 获取总数
	countQuery := `SELECT COUNT(*) FROM audit_logs`
	var total int
	if err := r.pool.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 获取列表
	query := `
		SELECT id, admin_id, action, target_type, target_id, target_label, detail, ip_address, created_at
		FROM audit_logs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []*audit.Log
	for rows.Next() {
		var log audit.Log
		err := rows.Scan(
			&log.ID,
			&log.AdminID,
			&log.Action,
			&log.TargetType,
			&log.TargetID,
			&log.TargetLabel,
			&log.Detail,
			&log.IPAddress,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		logs = append(logs, &log)
	}

	return logs, total, nil
}

// Log 记录审计日志（便捷方法）
func (r *Repository) Log(ctx context.Context, input audit.CreateInput) error {
	log := &audit.Log{
		ID:          uuid.New(),
		AdminID:     input.AdminID,
		Action:      input.Action,
		TargetType:  input.TargetType,
		TargetID:    input.TargetID,
		TargetLabel: &input.TargetLabel,
		Detail:      input.Detail,
		IPAddress:   &input.IPAddress,
		CreatedAt:   time.Now(),
	}
	return r.Create(ctx, log)
}
