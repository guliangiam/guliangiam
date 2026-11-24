-- scripts/dev/reset_schema.sql — 仅用于本地开发/CI 的库重置脚本
--
-- 用途：清空并重建 guliangiam schema，使 db/postgresql/V1__guliangiam_schema_init.sql
--       可以幂等重放。
-- 严禁在生产环境执行：本脚本会无差别删除 guliangiam schema 下的全部对象与数据。
--
-- 使用方式：
--   psql "$POSTGRES_DSN" -v ON_ERROR_STOP=1 -f scripts/dev/reset_schema.sql
--   psql "$POSTGRES_DSN" -v ON_ERROR_STOP=1 -f db/postgresql/V1__guliangiam_schema_init.sql
--
-- 说明：按《GuliangIAM 开发规范手册》第 3 章约定，V1 脚本在开发阶段保持
-- “单文件 + 幂等重放”，因此破坏性语句集中在本文件，正式 schema 脚本中不得出现。


-- 用途：本地开发/CI 的库重置脚本 — DROP + CREATE guliangiam schema
-- 何时使用：需要幂等重放 V1 DDL 前先清库；严禁指向生产
-- ============================================================
DROP SCHEMA IF EXISTS guliangiam CASCADE;

-- 重建 schema，使后续 V1 脚本无需处理 schema 不存在的情况。
-- 其余对象（扩展、函数 guliangiam.uuidv7()、表、索引）全部由 V1 脚本负责创建。
CREATE SCHEMA IF NOT EXISTS guliangiam;
