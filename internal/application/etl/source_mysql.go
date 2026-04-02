package etl

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type mysqlSourcePayload struct {
	Type    string `json:"type"`
	DSN     string `json:"dsn"`     // user:password@tcp(host:port)/dbname?charset=utf8mb4
	SQL     string `json:"sql"`
	Timeout int    `json:"timeout"` // 秒，默认 300
}

type mysqlSource struct {
	dsn     string
	sql     string
	timeout time.Duration
}

// NewMySQLSourceFactory 创建 MySQL SourceFactory。
// 连接信息从任务 payload 读取，每个任务可指向不同的 MySQL 实例。
func NewMySQLSourceFactory() SourceFactory {
	return func(taskCfg json.RawMessage) (Source, error) {
		var p mysqlSourcePayload
		if err := json.Unmarshal(taskCfg, &p); err != nil {
			return nil, fmt.Errorf("解析 MySQL source 配置失败: %w", err)
		}
		if p.DSN == "" {
			return nil, fmt.Errorf("MySQL source: dsn 不能为空")
		}
		if p.SQL == "" {
			return nil, fmt.Errorf("MySQL source: sql 不能为空")
		}

		timeout := time.Duration(p.Timeout) * time.Second
		if timeout <= 0 {
			timeout = 5 * time.Minute
		}

		return &mysqlSource{dsn: p.DSN, sql: p.SQL, timeout: timeout}, nil
	}
}

func (s *mysqlSource) Query(ctx context.Context, batchSize int, fn func(batch []Row) error) error {
	db, err := sql.Open("mysql", s.dsn)
	if err != nil {
		return fmt.Errorf("MySQL: 连接失败: %w", err)
	}
	defer db.Close()

	db.SetConnMaxLifetime(s.timeout)
	db.SetMaxOpenConns(1)

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	rows, err := db.QueryContext(ctx, s.sql)
	if err != nil {
		return fmt.Errorf("MySQL: 查询失败: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("MySQL: 获取列信息失败: %w", err)
	}

	batch := make([]Row, 0, batchSize)

	for rows.Next() {
		values := make([]any, len(columns))
		ptrs := make([]any, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}

		if err := rows.Scan(ptrs...); err != nil {
			return fmt.Errorf("MySQL: 扫描行失败: %w", err)
		}

		row := make(Row, len(columns))
		for i, col := range columns {
			row[col] = normalizeValue(values[i])
		}
		batch = append(batch, row)

		if len(batch) >= batchSize {
			if err := fn(batch); err != nil {
				return err
			}
			batch = make([]Row, 0, batchSize)
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("MySQL: 遍历结果失败: %w", err)
	}

	if len(batch) > 0 {
		return fn(batch)
	}
	return nil
}

// normalizeValue 将 database/sql 返回的值转为可 JSON 序列化的类型。
func normalizeValue(v any) any {
	switch val := v.(type) {
	case []byte:
		return string(val)
	case nil:
		return nil
	default:
		return val
	}
}
