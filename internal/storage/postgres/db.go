package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB 数据库连接池
type DB struct {
	pool *pgxpool.Pool
}

// New 创建数据库连接
func New(databaseURL string) (*DB, error) {
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		return nil, err
	}

	// 测试连接
	if err := pool.Ping(context.Background()); err != nil {
		return nil, err
	}

	return &DB{pool: pool}, nil
}

// Close 关闭连接
func (db *DB) Close() {
	if db.pool != nil {
		db.pool.Close()
	}
}

// Pool 获取连接池（供其他 repository 使用）
func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}
