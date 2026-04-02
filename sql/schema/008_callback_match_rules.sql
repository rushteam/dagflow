-- 将 task_ids 列整合为通用 match_rules JSONB，支持多种匹配规则而无需新增列
ALTER TABLE callbacks ADD COLUMN IF NOT EXISTS match_rules JSONB NOT NULL DEFAULT '{}';

UPDATE callbacks
SET match_rules = jsonb_build_object('task_ids', task_ids)
WHERE task_ids IS DISTINCT FROM '[]'::jsonb;

ALTER TABLE callbacks DROP COLUMN IF EXISTS task_ids;
