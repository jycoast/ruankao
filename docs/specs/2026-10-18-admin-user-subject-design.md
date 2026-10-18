# 用户管理页展示用户当前科目

- 日期：2026-10-18
- 状态：设计已确认，待实施
- 影响面：**纯前端**；**无接口变更、无数据变更、无新依赖**

## 背景与目标

管理后台「用户管理」(`/admin/users`) 只能看到用户名、邮箱、昵称、角色、状态、会员信息，看不到用户当前选择的考试科目。运营需要据此判断用户的使用方向（如系统架构设计师 / 软件设计师）。

目标：在用户列表中新增「当前科目」列，显示该用户 `subject_id` 对应的科目名称。

## 现状

- `users.subject_id` 已在数据库中持久化（`backend/internal/domain/model/model.go`），用户在前台切换科目时通过 `PUT /user/subject` 同步到服务端。
- `/admin/users` 的响应项 `AdminUserItem` 内嵌 `model.User`，因此**接口已经返回 `subject_id`**，无需后端改动。
- 前端 `src/types/index.ts` 的 `User` 接口已有 `subject_id?: number`。
- `AdminLayout` 挂载时已调用 `subjectStore.loadSubjects()` 拉取 `/subjects`（仅返回 `status = 1` 的启用科目），用户管理页可直接复用该 store。
- 科目列表是异步加载的，`subjectStore.subjects` 初始为空，加载完成后自动更新。

## 决策

### 1. 复用前端科目 store 映射名称

不新增接口字段、不做后端 join：`AdminService.ListUsers` 已经返回 `subject_id`，名称从 `subjectStore.subjects` 按 id 查即可。零额外请求，科目列表加载完成后表格自动刷新显示。

取舍：已下线（`status = 0`）的科目不在 `/subjects` 返回列表中，这类用户显示 `#<id>` 兜底；相比为此改后端 join，收益很小。

### 2. 展示位置与形式

`frontend/src/views/admin/UserManage.vue` 用户表格在「昵称」与「角色」之间新增一列：

```
<el-table-column label="当前科目" min-width="150" show-overflow-tooltip>
  <template #default="{ row }: { row: any }">{{ subjectNameOf(row) }}</template>
</el-table-column>
```

- 值：科目名称（如「系统架构设计师」）。
- 兜底：`subject_id` 查不到名称时显示 `#<id>`；`subject_id` 为空或 0 时显示 `—`。
- 纯文本，不用彩色标签，与用户名/昵称列风格一致。
- `min-width` + `show-overflow-tooltip`：长科目名单行省略，悬停看全文，延续本轮管理后台表格统一规则。

### 3. 名称解析函数

页面内新增一个小函数：

```ts
// 用户当前科目：复用布局层已加载的科目列表映射名称。
// 科目已下线/列表未加载时退化为 #id，避免显示空白。
function subjectNameOf(user: User): string {
  const sid = Number(user.subject_id) || 0
  if (!sid) return '—'
  return subjectStore.subjects.find((s) => s.id === sid)?.name || `#${sid}`
}
```

`UserManage.vue` 新增 `import { useSubjectStore } from '@/stores/subject'` 与 `const subjectStore = useSubjectStore()`。

## 明确不做

- 不加科目筛选下拉（不做 `subject_id` 查询过滤，不改后端）。
- 不改 `/admin/users` 接口与 `AdminUserItem`。
- 不改其他管理页面。
- 不引入科目图标或彩色标签。

## 边界情况

| 场景 | 显示 |
|---|---|
| 用户科目正常且启用 | 科目名称 |
| 科目已下线 / 列表尚未加载完 | `#<id>` |
| `subject_id` 为空或 0 | `—` |
| 科目名称过长 | 单行省略，悬停 tooltip 看全文 |
| 游客 / 未登录 | 不在用户列表中，不涉及 |

## 测试

- 无新增纯逻辑，不新增单测；解析函数依赖 `subjectStore`，属于页面内展示逻辑，按项目惯例靠 `vue-tsc` + 浏览器手测覆盖。
- 验证命令（工作目录 `frontend/`）：`npx vue-tsc -b`、`npm test`（现有 5 个文件 / 62 例应保持全绿）。
- 浏览器手测：正常科目显示名称；把某科目置为下线后该用户显示 `#id`；空 `subject_id` 显示 `—`；窄屏下长名称单行省略且悬停可见全文。
