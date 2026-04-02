package etl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"text/template"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisSinkPayload struct {
	Type string `json:"type"`

	// 连接信息（每个任务可指向不同 Redis 实例）
	Addr     string `json:"addr"`
	Password string `json:"password"`
	DB       int    `json:"db"`

	// 写入逻辑
	Command       string `json:"command"`        // SET | HSET | ZADD | RPUSH，默认 SET
	KeyTemplate   string `json:"key_template"`   // Go template，如 "rec:score:{{.uid}}"
	FieldTemplate string `json:"field_template"` // HSET 的 field
	ValueTemplate string `json:"value_template"` // 自定义 value 模板
	ValueField    string `json:"value_field"`    // 直接取某个字段的值（与 value_template 二选一）
	ScoreField    string `json:"score_field"`    // ZADD 的 score 来源字段
	MemberField   string `json:"member_field"`   // ZADD 的 member 来源字段
	TTL           int    `json:"ttl"`            // 秒，0 = 不过期
}

type redisSink struct {
	client      *redis.Client
	command     string
	keyTmpl     *template.Template
	fieldTmpl   *template.Template
	valueTmpl   *template.Template
	valueField  string
	scoreField  string
	memberField string
	ttl         time.Duration
}

// NewRedisSinkFactory 创建 Redis SinkFactory。
// 连接信息（addr / password / db）从任务 payload 读取，每个任务可以指向不同的 Redis 实例。
func NewRedisSinkFactory() SinkFactory {
	return func(taskCfg json.RawMessage) (Sink, error) {
		var p redisSinkPayload
		if err := json.Unmarshal(taskCfg, &p); err != nil {
			return nil, fmt.Errorf("解析 Redis sink 配置失败: %w", err)
		}
		if p.Addr == "" {
			return nil, fmt.Errorf("Redis sink: addr 不能为空")
		}
		if p.KeyTemplate == "" {
			return nil, fmt.Errorf("Redis sink: key_template 不能为空")
		}

		cmd := p.Command
		if cmd == "" {
			cmd = "SET"
		}

		client := redis.NewClient(&redis.Options{
			Addr:     p.Addr,
			Password: p.Password,
			DB:       p.DB,
		})

		s := &redisSink{
			client:      client,
			command:     cmd,
			valueField:  p.ValueField,
			scoreField:  p.ScoreField,
			memberField: p.MemberField,
		}
		if p.TTL > 0 {
			s.ttl = time.Duration(p.TTL) * time.Second
		}

		var err error
		if s.keyTmpl, err = template.New("key").Parse(p.KeyTemplate); err != nil {
			client.Close()
			return nil, fmt.Errorf("key_template 无效: %w", err)
		}
		if p.FieldTemplate != "" {
			if s.fieldTmpl, err = template.New("field").Parse(p.FieldTemplate); err != nil {
				client.Close()
				return nil, fmt.Errorf("field_template 无效: %w", err)
			}
		}
		if p.ValueTemplate != "" {
			if s.valueTmpl, err = template.New("value").Parse(p.ValueTemplate); err != nil {
				client.Close()
				return nil, fmt.Errorf("value_template 无效: %w", err)
			}
		}

		return s, nil
	}
}

func (s *redisSink) Write(ctx context.Context, batch []Row) error {
	pipe := s.client.Pipeline()

	for _, row := range batch {
		key, err := renderTmpl(s.keyTmpl, row)
		if err != nil {
			return fmt.Errorf("渲染 key 失败: %w", err)
		}

		switch s.command {
		case "SET":
			val, err := s.resolveValue(row)
			if err != nil {
				return err
			}
			pipe.Set(ctx, key, val, s.ttl)

		case "HSET":
			field, err := renderTmpl(s.fieldTmpl, row)
			if err != nil {
				return fmt.Errorf("渲染 field 失败: %w", err)
			}
			val, err := s.resolveValue(row)
			if err != nil {
				return err
			}
			pipe.HSet(ctx, key, field, val)
			if s.ttl > 0 {
				pipe.Expire(ctx, key, s.ttl)
			}

		case "ZADD":
			score, err := s.resolveScore(row)
			if err != nil {
				return err
			}
			member, err := s.resolveMember(row)
			if err != nil {
				return err
			}
			pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: member})
			if s.ttl > 0 {
				pipe.Expire(ctx, key, s.ttl)
			}

		case "RPUSH":
			val, err := s.resolveValue(row)
			if err != nil {
				return err
			}
			pipe.RPush(ctx, key, val)
			if s.ttl > 0 {
				pipe.Expire(ctx, key, s.ttl)
			}

		default:
			return fmt.Errorf("不支持的 Redis 命令: %s", s.command)
		}
	}

	_, err := pipe.Exec(ctx)
	return err
}

// Close 关闭任务级 Redis 连接。
func (s *redisSink) Close() error {
	return s.client.Close()
}

// resolveValue 按优先级：value_field → value_template → 整行 JSON。
func (s *redisSink) resolveValue(row Row) (string, error) {
	if s.valueField != "" {
		v, ok := row[s.valueField]
		if !ok {
			return "", fmt.Errorf("行中不存在字段 %q", s.valueField)
		}
		return fmt.Sprint(v), nil
	}
	if s.valueTmpl != nil {
		return renderTmpl(s.valueTmpl, row)
	}
	b, err := json.Marshal(row)
	if err != nil {
		return "", fmt.Errorf("JSON 序列化行数据失败: %w", err)
	}
	return string(b), nil
}

func (s *redisSink) resolveScore(row Row) (float64, error) {
	if s.scoreField == "" {
		return 0, fmt.Errorf("ZADD 需要 score_field")
	}
	v, ok := row[s.scoreField]
	if !ok {
		return 0, fmt.Errorf("行中不存在字段 %q", s.scoreField)
	}
	switch val := v.(type) {
	case float64:
		return val, nil
	case json.Number:
		return val.Float64()
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		return 0, fmt.Errorf("score 字段 %q 类型不支持: %T", s.scoreField, v)
	}
}

func (s *redisSink) resolveMember(row Row) (string, error) {
	if s.memberField == "" {
		return "", fmt.Errorf("ZADD 需要 member_field")
	}
	v, ok := row[s.memberField]
	if !ok {
		return "", fmt.Errorf("行中不存在字段 %q", s.memberField)
	}
	return fmt.Sprint(v), nil
}

func renderTmpl(tmpl *template.Template, data any) (string, error) {
	if tmpl == nil {
		return "", fmt.Errorf("模板为 nil")
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
