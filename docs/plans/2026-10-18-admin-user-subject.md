# 用户管理页展示当前科目 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在管理后台用户列表新增「当前科目」列，把用户的 `subject_id` 显示为科目名称。

**Architecture:** 纯前端单文件改动。`/admin/users` 已返回 `subject_id`，`AdminLayout` 挂载时已加载科目列表，页面只需从 `useSubjectStore().subjects` 按 id 映射名称，并加一个带兜底的展示函数。

**Tech Stack:** Vue 3 + TypeScript + Element Plus 2.14 + Pinia + vitest 5（`environment: 'node'`）

## Global Constraints

- **纯前端**：无接口变更、无数据库变更、无新依赖、无新组件。
- 不改 `/admin/users` 接口与 `AdminUserItem`。
- 不加科目筛选下拉。
- 只改 `frontend/src/views/admin/UserManage.vue` 一个文件。
- 列样式延续本轮管理后台表格规则：`min-width` + `show-overflow-tooltip`，纯文本不用彩色标签。
- 兜底显示：查不到名称显示 `#<id>`；`subject_id` 为空或 0 显示 `—`。
- 单测运行命令：`npm test`（工作目录 `frontend/`）；类型检查：`npx vue-tsc -b`。
- 中文注释，风格与同目录 `QuestionManage.vue` / `ArticleManage.vue` 一致。

---

## File Structure

| 文件 | 责任 |
|---|---|
| `frontend/src/views/admin/UserManage.vue`（修改） | 引入科目 store，新增名称解析函数，表格新增「当前科目」列 |

---

### Task 1: 用户表格新增「当前科目」列

**Files:**
- Modify: `frontend/src/views/admin/UserManage.vue`（script 顶部导入；script 末尾新增函数；表格列见下）

**Interfaces:**
- Consumes:
  - `useSubjectStore()` from `@/stores/subject`，返回 `{ subjects: Ref<Subject[]> }`，其中 `Subject = { id: number; name: string; ... }`
  - `User` from `@/types`，已有 `subject_id?: number`
- Produces: 无（页面内部函数，不被其他模块引用）

- [ ] **Step 1: 引入科目 store**

把文件顶部这一段：

```ts
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { adminApi } from '@/api'
import type { User } from '@/types'

const users = ref<User[]>([])
```

改为：

```ts
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { adminApi } from '@/api'
import type { User } from '@/types'
import { useSubjectStore } from '@/stores/subject'

// 科目列表由 AdminLayout 挂载时统一加载，这里直接复用，不再重复请求。
const subjectStore = useSubjectStore()

const users = ref<User[]>([])
```

- [ ] **Step 2: 新增名称解析函数**

在 script 末尾、`</script>` 之前（即 `resetPassword` 函数结束后）新增：

```ts
// 用户当前科目：复用布局层已加载的科目列表映射名称。
// 科目已下线（不在 /subjects 返回列表里）/ 列表尚未加载完时退化为 #id，避免显示空白。
function subjectNameOf(user: User): string {
  const sid = Number(user.subject_id) || 0
  if (!sid) return '—'
  return subjectStore.subjects.find((s) => s.id === sid)?.name || `#${sid}`
}
```

- [ ] **Step 3: 表格新增列**

把「昵称」列与「角色」列之间插入新列。定位到这一段：

```vue
      <el-table-column prop="nickname" label="昵称" min-width="110" show-overflow-tooltip />
      <el-table-column prop="role" label="角色" width="90">
```

改为：

```vue
      <el-table-column prop="nickname" label="昵称" min-width="110" show-overflow-tooltip />
      <el-table-column label="当前科目" min-width="150" show-overflow-tooltip>
        <template #default="{ row }: { row: any }">{{ subjectNameOf(row) }}</template>
      </el-table-column>
      <el-table-column prop="role" label="角色" width="90">
```

注意：`show-overflow-tooltip` 对自定义 `#default` 模板同样生效（Element Plus 按 `.cell` 内容判断是否溢出），无需额外写 `title`。

- [ ] **Step 4: 类型检查与全量测试**

Run（工作目录 `frontend/`）:

```bash
npx vue-tsc -b
npm test
```

Expected: 两者均退出码 0；测试 5 个文件、62 个用例全绿。

- [ ] **Step 5: 浏览器手测**

Run:

```bash
npm run dev
```

用管理员账号打开 `http://localhost:3000/admin/users`，逐项确认：

1. 「昵称」与「角色」之间有「当前科目」列，正常用户显示科目名称（如「系统架构设计师」）。
2. 切换前台科目后回到该页，同一用户显示新科目名称。
3. 把某科目在数据库/后台置为下线（`subjects.status = 0`）后刷新：该科目用户显示 `#<id>`，页面不报错。
4. `subject_id` 为 0 的用户显示 `—`。
5. 缩小窗口至窄屏，长科目名单行省略，悬停可看全文；行高不因长名称变高。

- [ ] **Step 6: Commit**

```bash
git add frontend/src/views/admin/UserManage.vue
git commit -m "feat(frontend): 用户管理列表显示用户当前科目"
```

---

## Self-Review

**1. Spec coverage**

| Spec 要求 | 对应任务 |
|---|---|
| 复用 `subjectStore.subjects` 映射名称 | Task 1 Step 1、2 |
| 列位置在昵称与角色之间 | Task 1 Step 3 |
| `min-width=150` + `show-overflow-tooltip` | Task 1 Step 3 |
| 兜底：`#<id>` / `—` | Task 1 Step 2 |
| 不改接口、不加筛选、不改其他页面 | Global Constraints 约束；File Structure 仅一个文件 |
| 无新增单测，靠 vue-tsc + 手测 | Task 1 Step 4、5 |

**2. Placeholder scan**：无 TBD/TODO；所有改动步骤都给出完整代码与定位锚点；验收步骤有明确命令与预期。

**3. Type consistency**：`subjectNameOf(user: User): string` 在 Step 2 定义，Step 3 模板调用一致；`subjectStore.subjects` 的元素类型 `Subject` 含 `id: number` / `name: string`，与 `@/stores/subject` 导出的接口一致；`user.subject_id` 在 `@/types` 的 `User` 中为 `number | undefined`，用 `Number(...) || 0` 归一。
