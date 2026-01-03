package database

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
    Pool *pgxpool.Pool
}

// NewCockroachDB tạo connection pool mới
func NewCockroachDB(ctx context.Context, connString string) (*DB, error) {
    // Parse cấu hình từ chuỗi kết nối (ví dụ: "host=localhost port=26257...")
    config, err := pgxpool.ParseConfig(connString)
    if err != nil {
        return nil, fmt.Errorf("không thể parse database config: %w", err)
    }

    // Tạo pool kết nối
    pool, err := pgxpool.NewWithConfig(ctx, config)
    if err != nil {
        return nil, fmt.Errorf("không thể tạo connection pool: %w", err)
    }

    return &DB{Pool: pool}, nil
}

// Close đóng kết nối khi app tắt
func (db *DB) Close() {
    db.Pool.Close()
}

// GetPool trả về pgxpool để các Service khác dùng
func (db *DB) GetPool() *pgxpool.Pool {
    return db.Pool
}

// package database

// import (
//     "context"
//     "fmt"

//     "github.com/jackc/pgx/v5/pgxpool"
// )

// type DB struct {
//     Pool *pgxpool.Pool
// }

// func NewCockroachDB(ctx context.Context, connString string) (*DB, error) {
//     config, err := pgxpool.ParseConfig(connString)
//     if err != nil {
//         return nil, fmt.Errorf("unable to parse database config: %w", err)
//     }

//     pool, err := pgxpool.NewWithConfig(ctx, config)
//     if err != nil {
//         return nil, fmt.Errorf("unable to create connection pool: %w", err)
//     }

//     return &DB{Pool: pool}, nil
// }

// func (db *DB) Close() {
//     db.Pool.Close()
// }