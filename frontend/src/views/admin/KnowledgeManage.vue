<script setup lang="ts">
import { ref, computed, onMounted, nextTick, watch } from 'vue'
import { knowledgeApi } from '@/api'
import { useSubjectStore } from '@/stores/subject'
import { buildTree, type KPNode } from '@/utils/knowledgeTree'
import { ElMessage, ElMessageBox } from 'element-plus'

const subjectStore = useSubjectStore()
const items = ref<any[]>([])
const loading = ref(false)
// 整树版本号（= 加载时最新的 updated_at）：拖拽提交时带上做乐观锁，
// 期间被其他管理员改过则后端 409 拒绝，防止旧快照整树覆写
const treeVersion = ref('')

const dialogVisible = ref(false)
const editingId = ref<number | null>(null)
const saving = ref(false)

function emptyForm() {
  return { name: '', parent_id: 0, sort_order: 0, content: '', key_points: '', easy_mistakes: '' }
}
const form = ref(emptyForm())

async function load() {
  loading.value = true
  try {
    // 整树管理必须拿全量快照：走 /admin/knowledge（page_size 上限 1000）。
    // 公开接口 page_size 被钳到 50/管理员 200，尾部节点被静默截断后，
    // 子节点因父级不在快照里会被组树逻辑顶成顶级节点
    const res: any = await knowledgeApi.adminList({ page: 1, page_size: 1000, subject_id: subjectStore.currentSubjectId })
    items.value = res.data.items || []
    // 兜底：单科目超过 1000 节点时继续翻页取全量
    const total = Number(res.data?.total || 0)
    for (let p = 2; items.value.length < total; p++) {
      const next: any = await knowledgeApi.adminList({ page: p, page_size: 1000, subject_id: subjectStore.currentSubjectId })
      const more = next.data?.items || []
      if (!more.length) break
      items.value.push(...more)
    }
    let maxTs = 0
    for (const i of items.value) {
      const t = Date.parse(i.updated_at || '')
      if (t > maxTs) {
        maxTs = t
        treeVersion.value = i.updated_at
      }
    }
    if (!treeInitialized) {
      // 首载：默认只展开到第 2 级（顶层章节展开即可看到二级，更深默认折叠）
      expandedKeys.value = items.value
        .filter((i: any) => Number(i.parent_id) === 0)
        .map((i: any) => Number(i.id))
      treeInitialized = true
    }
    // 数据重建后同步展开状态：搜索时全展开，否则回到用户原来的展开/收起
    await syncExpansion()
  } finally {
    loading.value = false
  }
}
onMounted(load)
// 切换科目后强制刷新数据(整页 reload 是首选;此处 watch 是兜底,防止 reload 因任何原因被吞
// 时还停留在旧科目数据上,例如刷新抢在 userApi.updateSubject HTTP 完成前,服务端回滚后导致)
watch(() => subjectStore.currentSubjectId, () => load())

const tree = computed<KPNode[]>(() => buildTree(items.value))

// ---- 查询：按名称（标题）或详情（内容/考点/易错点）过滤树 ----
// 只留命中节点 + 它们到根的整条祖先链 —— 层级路径正是这个功能要给人看的东西。
const query = ref('')
const searching = computed(() => query.value.trim().length > 0)

// id → 命中位置。名称算标题命中，三个详情字段算详情命中
const matches = computed(() => {
  const map = new Map<number, 'name' | 'detail' | 'both'>()
  const q = query.value.trim().toLowerCase()
  if (!q) return map
  for (const it of items.value) {
    const inName = String(it.name || '').toLowerCase().includes(q)
    const inDetail = (['content', 'key_points', 'easy_mistakes'] as const).some((k) =>
      String(it[k] || '').toLowerCase().includes(q),
    )
    if (inName || inDetail) map.set(Number(it.id), inName && inDetail ? 'both' : inName ? 'name' : 'detail')
  }
  return map
})

const filteredTree = computed<KPNode[]>(() => {
  if (!searching.value) return tree.value
  const byId = new Map<number, KPNode>()
  const walk = (nodes: KPNode[]) => {
    for (const n of nodes) {
      byId.set(n.id, n)
      walk(n.children)
    }
  }
  walk(tree.value)

  // 命中节点往上爬到根，沿途全部保留。父级不在快照里（数据异常）时自然停住
  const keep = new Set<number>()
  for (const id of matches.value.keys()) {
    let cur = byId.get(id)
    while (cur && !keep.has(cur.id)) {
      keep.add(cur.id)
      const pid = Number(cur.parent_id || 0)
      cur = pid ? byId.get(pid) : undefined
    }
  }
  const prune = (nodes: KPNode[]): KPNode[] =>
    nodes.filter((n) => keep.has(n.id)).map((n) => ({ ...n, children: prune(n.children) }))
  return prune(tree.value)
})

const MATCH_LABEL = { name: '标题命中', detail: '详情命中', both: '标题+详情' } as const
function matchLabel(id: number): string {
  const m = matches.value.get(Number(id))
  return m ? MATCH_LABEL[m] : ''
}

// 命中的关键字在名称里高亮。切成片段用 span 渲染而不是 v-html —— 名称是后台录入的文本
function highlightSegments(name: string) {
  const raw = String(name || '')
  const q = query.value.trim()
  if (!q) return [{ text: raw, hit: false }]
  const lower = raw.toLowerCase()
  const lq = q.toLowerCase()
  const out: { text: string; hit: boolean }[] = []
  let i = 0
  for (;;) {
    const idx = lower.indexOf(lq, i)
    if (idx < 0) {
      if (i < raw.length) out.push({ text: raw.slice(i), hit: false })
      break
    }
    if (idx > i) out.push({ text: raw.slice(i, idx), hit: false })
    out.push({ text: raw.slice(idx, idx + q.length), hit: true })
    i = idx + q.length
  }
  return out
}

// 上级选择器数据（el-tree-select 用；含「无（顶级）」）
const parentOptions = computed(() => {
  // 编辑时排除自身及其后代，避免成环
  const excluded = new Set<number>()
  if (editingId.value) {
    const collect = (id: number) => {
      excluded.add(id)
      items.value.filter((i) => Number(i.parent_id) === id).forEach((c) => collect(c.id))
    }
    collect(editingId.value)
  }
  const toOpt = (nodes: KPNode[]): any[] =>
    nodes
      .filter((n) => !excluded.has(n.id))
      .map((n) => ({ value: n.id, label: n.name, children: toOpt(n.children) }))
  return [{ value: 0, label: '无（顶级章节）', children: [] }, ...toOpt(tree.value)]
})

// 上级选择器输入过滤：按名称关键字匹配（中文直接包含，英文忽略大小写），
// 不匹配的节点隐藏，命中节点的祖先链自动保留展开
function filterParentNode(query: string, data: any) {
  if (!query) return true
  return String(data.label || '').toLowerCase().includes(query.toLowerCase())
}

function openCreate(parentId = 0) {
  editingId.value = null
  form.value = { ...emptyForm(), parent_id: parentId }
  dialogVisible.value = true
}

// 打开编辑弹窗时按 id 拉一次最新数据：页面列表可能已是旧快照
// （其他管理员刚挪过该节点），用旧 parent_id 回填会在保存时把层级改回去
async function openEdit(node: any) {
  editingId.value = node.id
  let src: any = node
  try {
    const res: any = await knowledgeApi.getById(node.id)
    if (res.data?.id) src = res.data
  } catch {
    // 拉取失败退回列表快照，至少不影响编辑
  }
  form.value = {
    name: src.name,
    parent_id: Number(src.parent_id || 0),
    sort_order: src.sort_order || 0,
    content: src.content || '',
    key_points: src.key_points || '',
    easy_mistakes: src.easy_mistakes || '',
  }
  dialogVisible.value = true
}

async function save() {
  if (!form.value.name.trim()) {
    ElMessage.warning('请输入名称')
    return
  }
  saving.value = true
  try {
    const payload = { ...form.value, subject_id: subjectStore.currentSubjectId }
    if (editingId.value) {
      await knowledgeApi.update(editingId.value, payload)
    } else {
      await knowledgeApi.create(payload)
    }
    ElMessage.success('保存成功')
    dialogVisible.value = false
    load()
  } catch {
    // 失败原因已由拦截器弹出（如后端层级校验拦截），弹窗保持打开可继续修改
  } finally {
    saving.value = false
  }
}

function childCount(id: number): number {
  return items.value.filter((i) => Number(i.parent_id) === id).length
}

const treeRef = ref<any>()

// 展开状态记忆：首载默认只展开到第 2 级；用户手动展开/收起通过
// node-expand/node-collapse 同步；保存/删除后重载恢复到操作前状态。
const expandedKeys = ref<number[]>([])
let treeInitialized = false

// 树数据重建（重载 / 查询变化）后同步展开状态：
// 搜索态全部撑开 —— 路径要一眼看得见；否则恢复到用户原来的展开/收起状态
async function syncExpansion() {
  await nextTick()
  const store = treeRef.value?.store
  if (!store) return
  if (!searching.value) {
    store.setDefaultExpandedKeys(expandedKeys.value)
    return
  }
  for (const n of Object.values(store.nodesMap || {}) as any[]) {
    if (n?.childNodes?.length && !n.expanded) n.expand()
  }
}

// 查询变化后重排展开状态；清空查询回到用户自己的展开记忆
watch(query, () => syncExpansion())

function onNodeExpand(data: any) {
  // 搜索态的全展开是程序撑开的，不该写进用户的展开记忆，否则清空查询后整棵树都是开的
  if (searching.value) return
  const id = Number(data.id)
  if (!expandedKeys.value.includes(id)) expandedKeys.value = [...expandedKeys.value, id]
}

// node-collapse 在折叠前触发，node.childNodes 仍是完整子树；
// 收起祖先时连同其后代一起从记忆里移除，避免重载后后代把祖先重新撑开。
function onNodeCollapse(data: any, node: any) {
  if (searching.value) return
  const drop = new Set<number>()
  const walk = (n: any) => {
    if (!n || !n.data) return
    drop.add(Number(n.data.id))
    for (const c of n.childNodes || []) walk(c)
  }
  walk(node)
  expandedKeys.value = expandedKeys.value.filter((k) => !drop.has(k))
}

// el-tree 拖拽：允许挪到同级或拖入其他节点做子项，但禁止拖进自己或自己的子孙
const allowDrop = (draggingNode: any, dropNode: any, type: string) => {
  if (type === 'inner') {
    let cur = dropNode
    while (cur) {
      if (cur.data.id === draggingNode.data.id) return false
      cur = cur.parent
    }
    return true
  }
  return true
}

/** 从 el-tree 的 node store 读整棵树（node-drop 时已反映本次拖拽），序列化为 {id,parent_id,sort_order} */
const collectTree = (node: any, parentId = 0, out: any[] = []): any[] => {
  node.childNodes.forEach((child: any, idx: number) => {
    out.push({ id: child.data.id, parent_id: parentId, sort_order: idx })
    collectTree(child, child.data.id, out)
  })
  return out
}

async function handleNodeDrop() {
  const root = treeRef.value?.store?.root
  if (!root) return
  const flat = collectTree(root)
  try {
    const res: any = await knowledgeApi.reorder(flat, treeVersion.value)
    // 记下提交后的新版本，供下次拖拽做冲突比对（不整页刷新，不打断展开状态）
    if (res.data?.version) treeVersion.value = res.data.version
    // 本地同步父级/序号，tree 是 computed 会自动重排，且不打断当前展开状态
    const byId = new Map(items.value.map((i: any) => [i.id, i]))
    flat.forEach((f: any) => {
      const it = byId.get(f.id)
      if (it) {
        it.parent_id = f.parent_id
        it.sort_order = f.sort_order
      }
    })
  } catch (e: any) {
    if (e?.code === 409) {
      // 期间其他管理员改过树：本次拖拽被拒绝，刷新到最新数据后重新操作
      ElMessage.warning('知识点树刚被其他人修改，已刷新为最新数据，请重新拖拽')
    } else if (typeof e?.code === 'number') {
      // 后端业务错误（400/500 等），直接展示后端给的原因
      ElMessage.error(e?.message || '排序保存失败，已刷新为服务器最新数据')
    } else {
      // 网络/超时类失败：后端事务可能仍在执行并最终提交，以刷新后的服务器数据为准
      ElMessage.error('排序保存超时，已刷新为服务器最新数据，请核对后再操作')
    }
    load()
  }
}

async function handleDelete(node: any) {
  const n = childCount(node.id)
  if (n > 0) {
    ElMessage.warning(`该节点下有 ${n} 个子项，请先处理子项再删除`)
    return
  }
  await ElMessageBox.confirm(`确认删除「${node.name}」？`, '提示', { type: 'warning' })
  try {
    await knowledgeApi.delete(node.id)
    ElMessage.success('删除成功')
    load()
  } catch {
    // 失败原因已由拦截器弹出（如后端拦截的「子节点仍存在」），保持当前状态即可
  }
}
</script>

<template>
  <div>
    <div class="mb-4 flex items-center justify-between">
      <h2 class="text-xl font-bold">
        知识点管理
        <el-tag size="small" class="ml-2">{{ subjectStore.currentSubject()?.name || '当前科目' }}</el-tag>
      </h2>
      <el-button type="primary" @click="openCreate(0)">新增顶级章节</el-button>
    </div>

    <p class="mb-3 text-sm text-gray-400">当前仅展示 / 维护「{{ subjectStore.currentSubject()?.name }}」的知识点，切换右上角科目可管理其他科目。层级结构会自动同步到前台「知识导图」。<span class="text-emerald-600 dark:text-emerald-400">拖拽节点可调整顺序 / 层级，松手即保存</span>。</p>

    <div class="mb-3 flex flex-wrap items-center gap-3">
      <el-input
        v-model="query"
        clearable
        placeholder="按名称或详情（内容/考点/易错点）查节点"
        class="max-w-md"
      >
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <span v-if="searching" class="text-sm text-gray-500 dark:text-gray-400">
        命中 <b class="text-emerald-600 dark:text-emerald-400">{{ matches.size }}</b> 个节点，下方只列命中节点及其到根的完整路径
      </span>
      <span v-else-if="!loading" class="text-xs text-gray-400">共 {{ items.length }} 个节点</span>
    </div>

    <el-tree
      ref="treeRef"
      v-loading="loading"
      :data="filteredTree"
      node-key="id"
      :props="{ label: 'name', children: 'children' }"
      :default-expanded-keys="expandedKeys"
      :expand-on-click-node="false"
      :draggable="!searching"
      :allow-drop="allowDrop"
      @node-drop="handleNodeDrop"
      @node-expand="onNodeExpand"
      @node-collapse="onNodeCollapse"
    >
      <template #default="{ data }">
        <div class="flex w-full items-center justify-between gap-2 pr-2">
          <span class="flex items-center gap-2">
            <span :class="data.parent_id ? '' : 'font-semibold'"><template
              v-for="(seg, i) in highlightSegments(data.name)"
              :key="i"
            ><span :class="seg.hit ? 'rounded bg-amber-200 text-amber-900 dark:bg-amber-500/40 dark:text-amber-50' : ''">{{ seg.text }}</span></template></span>
            <el-tag v-if="searching && matchLabel(data.id)" size="small" effect="plain" :type="matchLabel(data.id) === '详情命中' ? 'info' : 'warning'">
              {{ matchLabel(data.id) }}
            </el-tag>
            <span v-if="childCount(data.id)" class="rounded-full bg-gray-100 px-1.5 text-xs text-gray-500 dark:bg-gray-700 dark:text-gray-300">{{ childCount(data.id) }}</span>
            <span v-if="data.content || data.key_points || data.easy_mistakes" class="text-xs text-violet-500">有详情</span>
          </span>
          <span class="flex items-center gap-1">
            <el-button size="small" text type="primary" @click.stop="openCreate(data.id)">+子项</el-button>
            <el-button size="small" text @click.stop="openEdit(data)">编辑</el-button>
            <el-button size="small" text type="danger" @click.stop="handleDelete(data)">删除</el-button>
          </span>
        </div>
      </template>
    </el-tree>

    <div v-if="searching && !matches.size" class="py-12 text-center text-gray-400">
      没有匹配「{{ query.trim() }}」的知识点
    </div>
    <div v-else-if="!loading && !tree.length" class="py-12 text-center text-gray-400">暂无知识点</div>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑知识点' : '新增知识点'" width="640px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="章节名或知识点名" />
        </el-form-item>
        <div class="flex gap-3">
          <el-form-item label="上级" class="flex-1">
            <el-tree-select
              v-model="form.parent_id"
              :data="parentOptions"
              :props="{ label: 'label', children: 'children', value: 'value' } as any"
              node-key="value"
              check-strictly
              default-expand-all
              filterable
              :filter-node-method="filterParentNode"
              class="w-full"
            />
          </el-form-item>
          <el-form-item label="排序">
            <el-input-number v-model="form.sort_order" :min="0" />
          </el-form-item>
        </div>
        <el-form-item label="内容">
          <FullScreenEditor
            v-model="form.content"
            label="内容"
            :rows="4"
            placeholder="讲解（可选，支持 Markdown）；可直接粘贴图片自动上传"
          />
        </el-form-item>
        <el-form-item label="考点">
          <FullScreenEditor
            v-model="form.key_points"
            label="考点"
            :rows="3"
            placeholder="重点考点（可选，支持 Markdown）"
          />
        </el-form-item>
        <el-form-item label="易错点">
          <FullScreenEditor
            v-model="form.easy_mistakes"
            label="易错点"
            :rows="3"
            placeholder="易错点（可选，支持 Markdown）"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
