## 变更内容

<!-- 一句话说明本次改动做什么、为什么。 -->

## 类型

- [ ] 新功能
- [ ] Bug 修复
- [ ] 重构
- [ ] 文档/CI
- [ ] 其他

## 检查清单

- [ ] `cd backend && go build ./... && go test ./...` 通过
- [ ] `cd frontend && npm run build` 通过（含 vue-tsc 类型检查）
- [ ] 未提交敏感文件（`config.yaml`、`frontend/.env.*`、题库原始数据）
- [ ] 涉及迁移：附 `NNN_rollback_*.sql`，回滚方式已在描述说明

## 回滚方式

<!-- 本改动如何回滚；若是迁移/数据修复必须写明。 -->