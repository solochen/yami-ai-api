-- =============================================================================
-- 媒体地址批量改写（对象存储切换 / 加 CDN 域名时使用）
--
-- 作用：把数据库里所有 text/varchar/jsonb/json 列中的旧媒体地址前缀
--       （例如 https://www.orbit-e.com/minio-store/starai-works/）
--       替换成新地址前缀（例如 https://img.orbit-e.com/）。
--
-- 使用步骤：
--   1. 先备份数据库（pg_dump），或在可回滚的窗口内执行。
--   2. 修改下面的 v_old_prefix / v_new_prefix。
--   3. 保持 v_dry_run = true 先跑一遍，看 NOTICE 输出的命中行数；
--      脚本在 dry run 模式下不会写入任何数据。
--   4. 确认无误后把 v_dry_run 改成 false，再跑一遍。
--   5. 重跑是幂等的：第二次执行命中 0 行。
--
-- 执行方式（生产容器内）：
--   docker compose -f infra/docker/docker-compose.prod.yml \
--     --env-file .env.production exec -T postgres \
--     psql -U starai -d starai -v ON_ERROR_STOP=1 \
--     -f /tmp/rewrite-media-urls.sql
--   # 先把本文件复制进容器：docker compose cp scripts/rewrite-media-urls.sql postgres:/tmp/
-- =============================================================================

\set ON_ERROR_STOP on

BEGIN;

DO $$
DECLARE
  -- ▼▼▼ 只需要改这三处 ▼▼▼
  v_old_prefix text := 'https://www.orbit-e.com/minio-store/starai-works/';
  v_new_prefix text := 'https://img.orbit-e.com/';
  v_dry_run    boolean := true;
  -- ▲▲▲ 只需要改这三处 ▲▲▲

  r record;
  v_rows bigint;
  v_total bigint := 0;
BEGIN
  RAISE NOTICE 'old = %', v_old_prefix;
  RAISE NOTICE 'new = %', v_new_prefix;
  RAISE NOTICE 'dry_run = %', v_dry_run;

  FOR r IN
    SELECT c.table_name, c.column_name, c.data_type
    FROM information_schema.columns c
    JOIN information_schema.tables t
      ON t.table_schema = c.table_schema
     AND t.table_name = c.table_name
    WHERE c.table_schema = 'public'
      AND t.table_type = 'BASE TABLE'
      AND c.data_type IN ('text', 'character varying', 'character', 'jsonb', 'json')
    ORDER BY c.table_name, c.column_name
  LOOP
    IF r.data_type IN ('jsonb', 'json') THEN
      EXECUTE format(
        'SELECT count(*) FROM public.%I WHERE %I::text LIKE %L',
        r.table_name, r.column_name, '%' || v_old_prefix || '%'
      ) INTO v_rows;

      IF v_rows > 0 THEN
        RAISE NOTICE '  %.% (%): % 行', r.table_name, r.column_name, r.data_type, v_rows;
        IF NOT v_dry_run THEN
          EXECUTE format(
            'UPDATE public.%I SET %I = replace(%I::text, %L, %L)::%s WHERE %I::text LIKE %L',
            r.table_name, r.column_name, r.column_name,
            v_old_prefix, v_new_prefix, r.data_type,
            r.column_name, '%' || v_old_prefix || '%'
          );
        END IF;
        v_total := v_total + v_rows;
      END IF;
    ELSE
      EXECUTE format(
        'SELECT count(*) FROM public.%I WHERE %I LIKE %L',
        r.table_name, r.column_name, '%' || v_old_prefix || '%'
      ) INTO v_rows;

      IF v_rows > 0 THEN
        RAISE NOTICE '  %.% (%): % 行', r.table_name, r.column_name, r.data_type, v_rows;
        IF NOT v_dry_run THEN
          EXECUTE format(
            'UPDATE public.%I SET %I = replace(%I, %L, %L) WHERE %I LIKE %L',
            r.table_name, r.column_name, r.column_name,
            v_old_prefix, v_new_prefix,
            r.column_name, '%' || v_old_prefix || '%'
          );
        END IF;
        v_total := v_total + v_rows;
      END IF;
    END IF;
  END LOOP;

  RAISE NOTICE '合计命中 % 行', v_total;
  IF v_dry_run THEN
    RAISE NOTICE 'DRY RUN：未修改任何数据。确认后把 v_dry_run 改成 false 再执行。';
  ELSE
    RAISE NOTICE '已提交改写（事务结束前仍可 ROLLBACK）。';
  END IF;
END $$;

-- 确认无误后提交；dry run 时为无操作提交。
COMMIT;
