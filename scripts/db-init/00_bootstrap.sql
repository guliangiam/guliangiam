-- ============================================================
-- GuliangIAM Production Database Bootstrap
-- ============================================================
-- Version: 1.0.0
-- Scope:   Cluster + Database + Schema + Role 层准备
--          （不创建业务表/函数/触发器——这些由 V1 migration 负责）
-- Who runs: PostgreSQL superuser 或具备 CREATE ROLE / CREATE DB 权限的角色
-- When:    全新集群首次部署；已有集群升级时幂等安全
--
-- 两条执行路径：
--   1. Docker:  postgres 官方镜像将本文件挂载到
--               /docker-entrypoint-initdb.d/  → 首次启动时自动执行
--   2. Manual:  psql -h $HOST -U $SUPERUSER -d postgres -f 本文件
--
-- 设计原则：
--   - 幂等：所有 CREATE 走 IF NOT EXISTS / DO $$ 守卫
--   - 最小权限：应用角色只给 LOGIN + CREATEDB + CREATEROLE，不给 SUPERUSER
--   - 预留 runner：schema guliangiam 提前建好，runner 的 ledgerDDL
--                  （CREATE SCHEMA IF NOT EXISTS）是幂等安全的
--   - 不做 DDL：业务表/扩展/函数/RLS 全部由 runner 嵌入的 V1 执行
-- ============================================================

-- ============================================================
-- 1. 应用角色
-- ============================================================


-- 用途：生产集群 bootstrap — 创建 guliangiam DB + 应用角色 + 扩展（不含业务表）
-- 何时使用：全新集群首次部署；postgres 官方镜像挂载到 /docker-entrypoint-initdb.d/ 时自动执行；或 psql 手工 -f
-- ============================================================
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'guliangiam') THEN
        CREATE ROLE guliangiam LOGIN;
    END IF;
END $$;

-- 角色属性（幂等重复设置安全）
ALTER ROLE guliangiam WITH LOGIN CREATEDB CREATEROLE;
-- 注意：不给 SUPERUSER、不给 REPLICATION——应用不需要

-- ============================================================
-- 2. 应用数据库（切换到 postgres 库执行，避免不能 drop 自己）
-- ============================================================
-- 说明：本文件可能在 docker-entrypoint-initdb.d 里自动执行时
--       已经以 POSTGRES_DB 指定的库为目标；此时下面的
--       CREATE DATABASE 会 "already exists, skipping"，符合预期。

SELECT 'CREATE DATABASE guliangiam OWNER guliangiam'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'guliangiam')\gexec

-- 切换到应用库（如果当前就在 guliangiam，这条是 no-op）
\connect guliangiam

-- 再设一次 owner（幂等，防止 DB 已存在但 owner 不对）
ALTER DATABASE guliangiam OWNER TO guliangiam;

-- 给应用库基础权限
GRANT CONNECT ON DATABASE guliangiam TO guliangiam;

-- ============================================================
-- 3. 应用 schema（提前建好，避免 runner CREATE SCHEMA 权限问题）
-- ============================================================

CREATE SCHEMA IF NOT EXISTS guliangiam AUTHORIZATION guliangiam;

-- 应用角色在 schema 上的完整操作权限（业务全部表建在这个 schema 下）
GRANT USAGE, CREATE ON SCHEMA guliangiam TO guliangiam;

-- public schema 通常只给 USAGE（V1 里的扩展/函数/触发器全在 guliangiam schema）
GRANT USAGE ON SCHEMA public TO guliangiam;

-- ============================================================
-- 4. 默认权限：未来新建的表/序列/函数/触发器自动授权
--    （runner 执行 V1 时会创建所有业务对象，这些 ALTER DEFAULT
--     PRIVILEGES 确保它们一出生就是 guliangiam 可读写的）
-- ============================================================

ALTER DEFAULT PRIVILEGES IN SCHEMA guliangiam
    GRANT ALL PRIVILEGES ON TABLES TO guliangiam;

ALTER DEFAULT PRIVILEGES IN SCHEMA guliangiam
    GRANT ALL PRIVILEGES ON SEQUENCES TO guliangiam;

ALTER DEFAULT PRIVILEGES IN SCHEMA guliangiam
    GRANT EXECUTE ON FUNCTIONS TO guliangiam;

ALTER DEFAULT PRIVILEGES IN SCHEMA guliangiam
    GRANT USAGE ON TYPES TO guliangiam;

-- ============================================================
-- 5. 安全加固
-- ============================================================

-- 禁止 public 对应用库的 CONNECT（最小化攻击面）
REVOKE CONNECT ON DATABASE guliangiam FROM PUBLIC;

-- 禁止 public 在 guliangiam schema 里 CREATE（只允许 USAGE）
REVOKE CREATE ON SCHEMA guliangiam FROM PUBLIC;


-- ============================================================
-- 完成；输出状态（不影响退出码）
-- ============================================================
DO $$
DECLARE
    r RECORD;
BEGIN
    RAISE NOTICE '✅ GuliangIAM bootstrap complete';
    RAISE NOTICE '   Role guliangiam: LOGIN=%, CREATEDB=%, CREATEROLE=%',
        (SELECT rolcanlogin FROM pg_roles WHERE rolname='guliangiam'),
        (SELECT rolcreatedb FROM pg_roles WHERE rolname='guliangiam'),
        (SELECT rolcreaterole FROM pg_roles WHERE rolname='guliangiam');
    RAISE NOTICE '   Database guliangiam owner: %',
        (SELECT pg_catalog.pg_get_userbyid(datdba) FROM pg_database WHERE datname='guliangiam');
    RAISE NOTICE '   Schema guliangiam owner: %',
        (SELECT pg_catalog.pg_get_userbyid(nspowner) FROM pg_namespace WHERE nspname='guliangiam');
    RAISE NOTICE '';
    RAISE NOTICE 'Next step:';
    RAISE NOTICE '  DSN="postgres://guliangiam:$PASSWORD@$HOST:5432/guliangiam"';
    RAISE NOTICE '  AUTO_MIGRATE=true go run ./cmd/migrate up --dsn "$DSN"';
END $$;
