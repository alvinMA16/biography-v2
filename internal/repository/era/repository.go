package era

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/peizhengma/biography-v2/internal/domain/era"
)

var (
	ErrNotFound = errors.New("era memory preset not found")
)

// Repository 时代记忆预设数据访问层
type Repository struct {
	pool *pgxpool.Pool
}

// New 创建 Repository
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create 创建时代记忆预设
func (r *Repository) Create(ctx context.Context, m *era.MemoryPreset) error {
	query := `
		INSERT INTO era_memories_preset (id, start_year, end_year, category, content, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.pool.Exec(ctx, query,
		m.ID,
		m.StartYear,
		m.EndYear,
		m.Category,
		m.Content,
		m.CreatedAt,
		m.UpdatedAt,
	)

	return err
}

// GetByID 根据 ID 获取时代记忆预设
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*era.MemoryPreset, error) {
	query := `
		SELECT id, start_year, end_year, category, content, created_at, updated_at
		FROM era_memories_preset
		WHERE id = $1
	`

	var m era.MemoryPreset
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&m.ID,
		&m.StartYear,
		&m.EndYear,
		&m.Category,
		&m.Content,
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

// Update 更新时代记忆预设
func (r *Repository) Update(ctx context.Context, m *era.MemoryPreset) error {
	query := `
		UPDATE era_memories_preset
		SET start_year = $2, end_year = $3, category = $4, content = $5, updated_at = $6
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query,
		m.ID,
		m.StartYear,
		m.EndYear,
		m.Category,
		m.Content,
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

// Delete 删除时代记忆预设
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM era_memories_preset WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// List 获取时代记忆预设列表
func (r *Repository) List(ctx context.Context, filter era.ListMemoryPresetsFilter) ([]*era.MemoryPreset, int, error) {
	whereClause := "WHERE 1=1"
	args := []interface{}{}
	argIndex := 1

	if filter.Category != nil {
		whereClause += fmt.Sprintf(" AND category = $%d", argIndex)
		args = append(args, *filter.Category)
		argIndex++
	}

	// 有交集的逻辑：记忆.start_year <= 筛选.end AND 记忆.end_year >= 筛选.start
	if filter.YearEnd != nil {
		whereClause += fmt.Sprintf(" AND start_year <= $%d", argIndex)
		args = append(args, *filter.YearEnd)
		argIndex++
	}

	if filter.YearStart != nil {
		whereClause += fmt.Sprintf(" AND end_year >= $%d", argIndex)
		args = append(args, *filter.YearStart)
		argIndex++
	}

	// 获取总数
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM era_memories_preset %s", whereClause)
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
		SELECT id, start_year, end_year, category, content, created_at, updated_at
		FROM era_memories_preset
		%s
		ORDER BY start_year ASC, end_year ASC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var memories []*era.MemoryPreset
	for rows.Next() {
		var m era.MemoryPreset
		err := rows.Scan(
			&m.ID,
			&m.StartYear,
			&m.EndYear,
			&m.Category,
			&m.Content,
			&m.CreatedAt,
			&m.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		memories = append(memories, &m)
	}

	return memories, total, nil
}

// GetByYearRange 根据年份范围获取时代记忆
func (r *Repository) GetByYearRange(ctx context.Context, birthYear int) ([]*era.MemoryPreset, error) {
	// 获取用户出生后经历的重要历史事件（从出生到现在）
	query := `
		SELECT id, start_year, end_year, category, content, created_at, updated_at
		FROM era_memories_preset
		WHERE start_year >= $1
		ORDER BY start_year ASC
	`

	rows, err := r.pool.Query(ctx, query, birthYear)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []*era.MemoryPreset
	for rows.Next() {
		var m era.MemoryPreset
		err := rows.Scan(
			&m.ID,
			&m.StartYear,
			&m.EndYear,
			&m.Category,
			&m.Content,
			&m.CreatedAt,
			&m.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		memories = append(memories, &m)
	}

	return memories, nil
}
