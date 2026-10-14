-- 001: 建库基线 —— 必需种子数据
--
-- 与 000_baseline_schema.sql 配套：000 建结构，本文件写入让应用可用的最小参考数据。
--
-- 幂等：全部语句可在已有库上重复执行，且已存在的行一律不覆盖
--（保持仓库「纯补充、不修改已有数据」的约定）。
--
-- 收录口径：
--   subjects          —— 必需。questions.subject_id / papers.subject_id / users.subject_id
--                        三处列默认值都写死为 1，科目行缺失会让默认值指向不存在的科目。
--   membership_plans  —— 必需。plan_key 是下单时的连接键，缺行则创建订单直接失败。
--   site_settings     —— 必需。仓储层虽容忍行不存在（按「未维护」处理），但仍按 090 补齐。
--   ai_configs        —— 让后台「AI 配置」页有行可编辑；缺行时运行时按「未配置」处理。
--   exam_schedules    —— 前端考试倒计时的默认数据。
--
-- 不含 user_subject_stats：它是 059 的派生回填，空库上的正解就是 0 行。
--
-- 取值口径：subject 与 membership_plans 取线上库当前值，而非重放历史迁移的结果。
-- 两者在 subjects 上有两处已知偏离（线上被手工改过、全仓无对应迁移），见下方注释。
SET NAMES utf8mb4;

-- 科目。id 必须固定为 1..10：三处列默认值引用 id=1，前端按 code 取值。
--
-- 与 007_subjects.sql 的 INSERT 相比，线上有两处无迁移来源的手工改动，此处按线上值收录：
--   * sort_order：007 写 sd=2 / sa=3，线上是 sd=3 / sa=2。
--   * status：007 的 INSERT 不含该列（默认 1），线上 npd / spm / dse / prog 四行为 0。
-- 其余字段可完全由迁移解释：has_essay / has_sprint 来自 058，level 来自 087。
--
-- 注意 subjects 没有 code 唯一键（只有 PRIMARY(id)），因此幂等靠显式 id 命中主键，
-- 不能依赖 code 去重。
INSERT INTO subjects (id, name, code, icon, sort_order, status, has_essay, has_sprint, level) VALUES
  (1,  '系统架构设计师',         'sad',  '', 1,  1, 1, 1, 1),
  (2,  '软件设计师',             'sd',   '', 3,  1, 0, 0, 2),
  (3,  '系统分析师',             'sa',   '', 2,  1, 1, 1, 1),
  (4,  '信息系统项目管理师',     'pmp',  '', 4,  1, 1, 0, 1),
  (5,  '网络规划设计师',         'npd',  '', 5,  0, 0, 0, 2),
  (6,  '系统规划与管理师',       'spm',  '', 6,  0, 0, 0, 1),
  (7,  '数据库系统工程师',       'dse',  '', 7,  0, 0, 0, 2),
  (8,  '网络工程师',             'ne',   '', 8,  1, 0, 0, 2),
  (9,  '程序员',                 'prog', '', 9,  0, 0, 0, 3),
  (10, '系统集成项目管理工程师', 'jc',   '', 10, 1, 0, 0, 2)
ON DUPLICATE KEY UPDATE id = id;

-- 会员套餐。plan_key 上有唯一键，幂等靠它。
-- 取值与 064_membership_plans.sql 的初始数据一致，也与线上当前值一致（无偏离）。
INSERT INTO membership_plans (id, plan_key, name, days, amount, ldc_amount, sort_order, enabled) VALUES
  (1, 'month',  '月度会员', 30,  2900,  2900,  1, 1),
  (2, 'season', '季度会员', 90,  6900,  6900,  2, 1),
  (3, 'exam',   '考季会员', 180, 9900,  9900,  3, 1),
  (4, 'year',   '年度会员', 365, 19800, 19800, 4, 1)
ON DUPLICATE KEY UPDATE plan_key = plan_key;

-- 站点级开关：单行表，固定 id=1。与 090_site_settings.sql 一致。
INSERT IGNORE INTO site_settings (id, maintenance_enabled) VALUES (1, 0);

-- 全局 AI 配置：单行表，固定 id=1，先留空，由后台「AI 配置」页填写。
-- free_trial_limit 不显式给值，走列默认 5（054_ai_free_trial.sql 加的）。
INSERT INTO ai_configs (id, base_url, api_key, model, temperature, max_tokens, enabled, created_at, updated_at)
VALUES (1, '', '', '', 0.70, 2048, 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE id = id;

-- 考试排期：无唯一键，裸 INSERT 不幂等，用会话变量守卫「仅在表为空时写入」。
-- 不用 WHERE NOT EXISTS (SELECT 1 FROM exam_schedules)：INSERT ... SELECT 引用目标表
-- 会触发 MySQL ERROR 1093。
SET @exam_schedules_empty := (SELECT COUNT(*) FROM exam_schedules);

INSERT INTO exam_schedules (label, exam_date)
SELECT s.label, s.exam_date FROM (
            SELECT '2026年秋季' AS label, '2026-11-07' AS exam_date
  UNION ALL SELECT '2027年春季', '2027-05-22'
  UNION ALL SELECT '2027年秋季', '2027-11-06'
  UNION ALL SELECT '2028年春季', '2028-05-27'
) AS s
WHERE @exam_schedules_empty = 0;
