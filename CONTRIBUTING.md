# 贡献指南

感谢你愿意为这个项目贡献代码。请阅读以下约定，让提交更顺畅。

## 环境准备

- Go >= 1.24，Node.js >= 18，MySQL >= 8.0，Redis >= 7
- Docker（可选，用 `docker-compose up -d` 一键起依赖）
- 首次运行：按 `backend/config/config.example.yaml` 复制出 `config.yaml` 填好本地值，
  建库后 `cd backend && go run ./cmd/migrate`（自动应用合并建库基线）；前端复制
  `frontend/.env.example` 为 `.env.development` / `.env.production`。

## 分支与提交

- 功能分支从 `main` 切，不要直接在 `main` 上改。
- commit message 用约定式前缀 + 中文描述，例如：
  - `feat(notes): 笔记页支持图片粘贴上传`
  - `fix(wrongbook): 移动端搜索框被 el-input 宽度变量压扁`
  - `refactor(auth): 抽离 OAuth 登录态恢复`
- 每个 commit 尽量小而独立，能单独回滚；不要夹带无关改动。
- 涉及生产库的改动（迁移、数据修复）必须在 PR 描述里写清回滚方式。

## 数据库迁移约定

- 公开仓库的 `backend/migrations/` 顶层只放**结构变更与配置型维表种子**，编号必须大于 `001`
  —— `000_baseline_schema.sql` / `001_baseline_seed.sql` 是合并建库基线，占用了这两个号。
  任何针对题目 / 解析 / 试卷的 DML 一律进私有数据仓库，不进公开仓库。
- 新增迁移文件名为 `NNN_描述.sql`（数字前缀递增），用 `go run ./cmd/migrate` 应用；
  迁移要幂等、可回滚（附 `NNN_rollback_描述.sql`）。合并基线本身是 squash 产物、没有 rollback，
  这一条不适用于它。
- 只做纯增量的题目补录，不修改已有数据；按题干查重而非题号。
- `knowledge_point` 格式为 `主类>子类`，主类沿用已有分类，不要新建。

## 测试与提交前检查

```bash
cd backend && go build ./... && go test ./...
cd frontend && npm run build   # 含 vue-tsc 类型检查
```

- 新增逻辑尽量带单测（后端用 sqlmock 测 repo/service，前端 vitest）。
- 不要提交：`backend/config/config.yaml`、`frontend/.env.*`（实际环境变量值）、
  任何题库原始数据文件。

## 题库数据

本仓库只包含代码，不随仓库分发题库数据（题目、解析、历史迁移数据 SQL）。
克隆后跑一遍迁移得到的是结构完整的空库，需要题库数据请自备，导入约定见 `README.md` 的「数据说明」一节。