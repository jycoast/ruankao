<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, shallowRef, computed, watch } from 'vue'
import { Transformer } from 'markmap-lib'
import { Markmap } from 'markmap-view'
import { Toolbar } from 'markmap-toolbar'
import 'markmap-toolbar/dist/style.css'
import { ElMessage } from 'element-plus'
import { knowledgeApi, favoriteApi } from '@/api'
import { useSubjectStore } from '@/stores/subject'
import { useUserStore } from '@/stores/user'
import { useRoute, useRouter } from 'vue-router'
import { renderMd } from '@/utils/markdown'
import { useImagePreview } from '@/composables/useImagePreview'
import { ARCHITECT_MINDMAP_MD } from '@/data/architectMindmap'
import { buildTree, treeToMarkdown } from '@/utils/knowledgeTree'
import { MINDMAP_BG_KEY, MINDMAP_BG_PRESETS, findPreset, branchColor, type MindmapBgPreset } from '@/data/mindmapThemes'

const subjectStore = useSubjectStore()
const userStore = useUserStore()
const route = useRoute()
const router = useRouter()

const wrapRef = ref<HTMLElement | null>(null)
const svgRef = ref<SVGSVGElement | null>(null)
const toolbarRef = ref<HTMLElement | null>(null)
const containerRef = ref<HTMLElement | null>(null)
const isFullscreen = ref(false)
const mm = shallowRef<Markmap | null>(null)

// ── 画布背景色预设（localStorage 持久化，key 见 data/mindmapThemes）──
function loadSavedBg(): string {
  try {
    const saved = localStorage.getItem(MINDMAP_BG_KEY)
    // 只认合法 key，防旧版本/localStorage 脏数据
    if (saved && findPreset(saved).key === saved) return saved
  } catch {
    /* 隐私模式等场景读不到就回落默认 */
  }
  return 'default'
}
const bgKey = ref<string>(loadSavedBg())
const bgPreset = computed<MindmapBgPreset>(() => findPreset(bgKey.value))
// 是否处于自定义背景档（default 档不涂底、文字/链接走全局明暗）
const hasCustomBg = computed(() => !!bgPreset.value.bg)
const bgPopoverVisible = ref(false)
// 选中后重建 markmap：markmap 取色在建实例时固化，不能运行时热更
function pickBg(key: string) {
  if (key === bgKey.value) return
  bgKey.value = key
  try {
    localStorage.setItem(MINDMAP_BG_KEY, key)
  } catch {
    /* 写不进去不阻塞切换 */
  }
  bgPopoverVisible.value = false
  // 仅重建 markmap（无 fetch），仍走骨架屏：销毁→重建之间画布短暂空白，骨架覆盖更平滑
  loadingMindmap.value = true
  renderMindmap()
}
// 自定义档位把 CSS 变量设在 .mm-card 根上（画布与详情抽屉都是其后代，共同消费），
// 变量带上 link/warn 兜底，避免个别档位漏配导致样式缺失
const cardStyle = computed(() => {
  const p = bgPreset.value
  if (!p.bg) return undefined
  const ink = p.ink ?? '#334155'
  // ink 是 #RRGGBB，追加两位 alpha 得到 #RRGGBBAA 二级色（次级文字/发丝分隔线）
  const withAlpha = (a: number) => `${ink}${Math.round(a * 255).toString(16).padStart(2, '0')}`
  return {
    '--mm-bg': p.bg,
    '--mm-ink': ink,
    '--mm-link': p.link ?? '#2563eb',
    '--mm-warn': p.warn ?? '#e11d48',
    '--mm-muted': withAlpha(0.55), // 头部提示等次级文字
    '--mm-line': withAlpha(0.2), // 卡片外框 / 头部下分隔线
  } as Record<string, string>
})

// ⓘ 详情联动：按 id / 名称查（DB 驱动用 id，静态回退用名称）
const byId = ref<Record<string, any>>({})
const byName = ref<Record<string, any>>({})
// 导图数据源：DB 层级生成，DB 空/失败时回退静态
const mindmapMd = ref<string>('')
const noData = ref(false)

// 加载占位：从拿到科目 id 到 markmap 节点渲染完整个过程都为 true，骨架屏淡入/淡出。
// 设计为单一标志位而非三态：进入加载路径一律置 true，renderMindmap 完成（或放弃）置 false，
// 这样 onMounted 的 load+render 串行调用时不会被中间的错误复位打断。
const loadingMindmap = ref(false)
// 详情抽屉
const drawerOpen = ref(false)
const current = ref<any>(null)
const isFavorited = ref(false)
const { visible, urlList, initialIndex, onContainerClick, close } = useImagePreview()
// 抽屉宽度：移动端近全屏，桌面固定宽
const drawerSize = computed(() => (window.innerWidth < 640 ? '90%' : '440px'))

async function loadKnowledge() {
  // 进入新一次加载前先清掉上一科目留下的状态：noData 和 mindmapMd 都会跨请求残留，
  // 不清的话中间访问过一个「无数据 + 无 sprint」的科目（如信息系统项目管理师）后再回到
  // 有数据科目，noData 还是 true → 模板 v-if 永远走"暂无"分支，SVG 不可见。
  // 这一行是修该 bug 的关键，其它分支只是在各自条件下重新赋值。
  noData.value = false
  mindmapMd.value = ''
  loadingMindmap.value = true
  // 飞行中可能再切科目：发起时记下当前 id，响应回来对一下，不一致就丢弃。
  // 不这么做的话 A→B→A 快速切换时，后请求先回、A 后回，A 渲染完又被 B 覆盖，或反过来。
  const sid = subjectStore.currentSubjectId
  try {
    // listAll 分页拉全量：公开接口单页钳 50，单次请求会静默截断尾部知识点
    const items = await knowledgeApi.listAll(sid)
    if (subjectStore.currentSubjectId !== sid) return // 已切到其它科目，本响应作废
    const mId: Record<string, any> = {}
    const mName: Record<string, any> = {}
    for (const it of items) {
      mId[it.id] = it
      mName[it.name] = it
    }
    byId.value = mId
    byName.value = mName
    if (items.length) {
      // 由 DB 层级生成导图
      const roots = buildTree(items)
      const title = subjectStore.currentSubject()?.name || '当前科目'
      mindmapMd.value = treeToMarkdown(roots, title)
    } else if (subjectStore.currentSubject()?.code === 'sad') {
      // 仅架构设计师(code='sad')保留内置静态导图兜底。
      // 旧实现用 hasSprint 触发会被系统分析师(has_sprint=1 但无知识点)误命中,
      // 把架构设计师的根节点硬编码内容显示成系统分析师的——按 code 精准匹配。
      mindmapMd.value = ARCHITECT_MINDMAP_MD
    } else {
      // 其它科目暂无知识导图
      noData.value = true
    }
  } catch {
    // API 失败:按发起时的 sid 快照判断是否走架构设计师兜底。
    // 用 sid 而非 currentSubject() 是为了飞行中切了科目时不把旧兜底套到新科目上。
    const code = subjectStore.subjects.find(s => s.id === sid)?.code
    if (code === 'sad') {
      mindmapMd.value = ARCHITECT_MINDMAP_MD
    } else {
      noData.value = true
    }
  }
}

async function openDetail(token: string) {
  // token 为纯数字 → 按 id 查；否则按名称查
  const kp = /^\d+$/.test(token) ? byId.value[token] : byName.value[token]
  if (!kp) {
    ElMessage.info('该主题暂无详情内容')
    return
  }
  current.value = kp
  drawerOpen.value = true
  isFavorited.value = false
  // 知识点对游客开放，但收藏不是。未登录时调用会拿到 401，
  // 被拦截器当成登录过期把访客踢走。
  if (!userStore.isLoggedIn) return
  try {
    const fRes: any = await favoriteApi.check({ target_type: 3, target_id: kp.id })
    isFavorited.value = fRes.data.exists
  } catch {}
}

async function toggleFavorite() {
  const kp = current.value
  if (!kp) return
  if (!userStore.isLoggedIn) {
    router.push({ path: '/login', query: { redirect: route.fullPath } })
    return
  }
  if (isFavorited.value) {
    await favoriteApi.remove(3, kp.id)
  } else {
    await favoriteApi.add({ target_type: 3, target_id: kp.id })
  }
  isFavorited.value = !isFavorited.value
}

// 捕获阶段拦截 ⓘ 链接点击，避免 hash 路由跳转与节点折叠
function onCaptureClick(e: MouseEvent) {
  const target = e.target as HTMLElement
  const a = target.closest?.('a[href^="#kp:"]') as HTMLAnchorElement | null
  if (!a) return
  e.preventDefault()
  e.stopPropagation()
  const raw = (a.getAttribute('href') || '').slice(4) // 去掉 "#kp:"
  let token = raw
  try { token = decodeURIComponent(raw) } catch {}
  openDetail(token)
}

function refit() {
  mm.value?.fit()
}

function toggleFullscreen() {
  const el = containerRef.value
  if (!el) return
  if (!document.fullscreenElement) {
    el.requestFullscreen?.()
  } else {
    document.exitFullscreen?.()
  }
}

function onFullscreenChange() {
  isFullscreen.value = !!document.fullscreenElement
  // 尺寸变了，稍后重新适配画布
  setTimeout(() => mm.value?.fit(), 120)
}

async function renderMindmap() {
  if (!mindmapMd.value) {
    // 没数据（noData 路径或上游 catch 后仍未生成 md）：不再继续，结束加载态
    loadingMindmap.value = false
    return
  }
  try {
    // 重建前先销毁旧实例，避免 markmap-view 持有过期数据
    mm.value?.destroy()
    mm.value = null
    if (toolbarRef.value) toolbarRef.value.innerHTML = ''
    const { root } = new Transformer().transform(mindmapMd.value)
    const p = bgPreset.value
    // 自定义背景档给 markmap 一套取色（主分支锁色）；default 档不传，沿用 markmap 默认
    const branchColors = p.accents.length
      ? (node: { state?: { path?: string | null } }) => branchColor(p.accents, node.state?.path)
      : undefined
    mm.value = Markmap.create(
      svgRef.value as SVGSVGElement,
      {
        pan: true,
        zoom: true,
        initialExpandLevel: 2,
        maxInitialScale: 1,
        spacingVertical: 8,
        paddingX: 16,
        duration: 200, // 折叠/展开动画更快
        maxWidth: 280, // 限制节点宽度，避免超宽 foreignObject 重绘
        ...(branchColors ? { color: branchColors } : {}),
      },
      root,
    )
    await mm.value.fit()

    // 工具栏（缩放/适应/展开），移动端点按兜底
    if (toolbarRef.value) {
      const tb = Toolbar.create(mm.value as Markmap)
      tb.setBrand(false)
      toolbarRef.value.append(tb.el)
    }
  } finally {
    // 无论 Markmap.create/fit 成功还是抛错，都退出加载态，避免骨架屏卡住
    loadingMindmap.value = false
  }
}

onMounted(async () => {
  await loadKnowledge()
  await renderMindmap()

  wrapRef.value?.addEventListener('click', onCaptureClick, true)
  window.addEventListener('resize', refit)
  document.addEventListener('fullscreenchange', onFullscreenChange)
})

// 切科目后重新拉数据并重建 markmap；不依赖整页刷新
watch(() => subjectStore.currentSubjectId, async (id) => {
  if (!id) return
  await loadKnowledge()
  await renderMindmap()
})

onBeforeUnmount(() => {
  wrapRef.value?.removeEventListener('click', onCaptureClick, true)
  window.removeEventListener('resize', refit)
  document.removeEventListener('fullscreenchange', onFullscreenChange)
  mm.value?.destroy()
})
</script>

<template>
  <div>
    <div
      ref="containerRef"
      class="mm-card flex flex-col rounded-xl border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900"
      :class="{ 'is-fullscreen': isFullscreen, 'mm-theme-custom': hasCustomBg }"
      :style="cardStyle"
    >
      <div class="mm-hdr flex items-center justify-between border-b border-slate-100 px-4 py-3 dark:border-slate-800">
        <div class="min-w-0">
          <div class="mm-hdr-title text-sm font-semibold text-slate-900 dark:text-white">{{ (subjectStore.currentSubject()?.name || "当前科目") }} · 知识导图</div>
          <div class="mm-hdr-hint mt-0.5 text-xs text-slate-400">点圆点折叠/展开 · 拖拽平移 · 双指或滚轮缩放 · 点 ⓘ 看详情</div>
        </div>
        <div class="flex shrink-0 items-center gap-2">
          <el-popover
            v-model:visible="bgPopoverVisible"
            :teleported="false"
            placement="bottom-end"
            :width="180"
            trigger="click"
          >
            <template #reference>
              <button
                class="flex items-center gap-1.5 rounded-lg border px-2.5 py-1 text-xs font-medium transition"
                :class="hasCustomBg
                  ? 'border-blue-300 bg-blue-50 text-blue-600 dark:border-blue-500/40 dark:bg-blue-500/10 dark:text-blue-300'
                  : 'border-slate-200 text-slate-600 hover:border-blue-300 hover:text-blue-600 dark:border-slate-700 dark:text-slate-300'"
              >
                <span
                  class="inline-block h-3 w-3 rounded-full border border-black/10 dark:border-white/25"
                  :style="hasCustomBg
                    ? { backgroundColor: bgPreset.bg }
                    : { background: 'linear-gradient(135deg, #ffffff 45%, #0f172a 55%)' }"
                />
                背景
              </button>
            </template>
            <div class="py-0.5">
              <button
                v-for="p in MINDMAP_BG_PRESETS"
                :key="p.key"
                class="flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left text-xs transition hover:bg-slate-100 dark:hover:bg-slate-700/60"
                @click="pickBg(p.key)"
              >
                <span
                  class="inline-block h-4 w-4 shrink-0 rounded border border-black/10 dark:border-white/25"
                  :style="p.bg
                    ? { backgroundColor: p.bg }
                    : { background: 'linear-gradient(135deg, #ffffff 45%, #0f172a 55%)' }"
                />
                <span
                  class="flex-1"
                  :class="bgKey === p.key ? 'font-medium text-blue-600 dark:text-blue-400' : 'text-slate-600 dark:text-slate-300'"
                >{{ p.name }}</span>
                <el-icon v-if="bgKey === p.key" :size="13" class="text-blue-600 dark:text-blue-400"><Check /></el-icon>
              </button>
            </div>
          </el-popover>
          <button
            class="mm-hdr-btn rounded-lg border border-slate-200 px-2.5 py-1 text-xs font-medium text-slate-600 transition hover:border-blue-300 hover:text-blue-600 dark:border-slate-700 dark:text-slate-300"
            @click="refit"
          >
            适应屏幕
          </button>
          <button
            class="mm-hdr-btn flex items-center gap-1 rounded-lg border border-slate-200 px-2.5 py-1 text-xs font-medium text-slate-600 transition hover:border-blue-300 hover:text-blue-600 dark:border-slate-700 dark:text-slate-300"
            @click="toggleFullscreen"
          >
            <el-icon :size="13"><component :is="isFullscreen ? 'Aim' : 'FullScreen'" /></el-icon>
            {{ isFullscreen ? '退出全屏' : '全屏' }}
          </button>
        </div>
      </div>

      <div ref="wrapRef" class="mm-wrap relative">
        <!-- SVG 必须常驻 DOM：Markmap.create 拿 svgRef.value 写入节点,加载态下 v-if 卸载会
             让 svgRef === null,Markmap 实例损坏、节点永远画不出来。这是 v-if/Transition
             改造的硬约束,任何"等节点就绪再显示"的思路都得绕开它。 -->
        <svg
          ref="svgRef"
          class="mm-svg block w-full text-slate-700 dark:text-slate-200 transition-opacity duration-300"
          :class="loadingMindmap || noData ? 'opacity-0' : 'opacity-100'"
        />
        <!-- 覆盖层：仅在加载中或无数据时盖在 SVG 上。加载态显示骨架脉冲(避免白屏),
             无数据态显示"暂无"提示。两者都用 absolute inset-0,不影响 SVG 的 ref。 -->
        <Transition name="mm-fade">
          <div
            v-if="loadingMindmap"
            key="loading"
            class="mm-skel absolute inset-0 flex items-center justify-center"
          >
            <!-- 视觉中心:大号旋转环 + 主标题/副标题,字号与对比度都比边角提示高 -->
            <div class="flex flex-col items-center gap-3">
              <svg
                class="h-10 w-10 animate-spin text-blue-500"
                viewBox="0 0 24 24"
                fill="none"
                aria-hidden="true"
              >
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"
                />
              </svg>
              <div class="text-base font-semibold text-slate-700 dark:text-slate-200">正在加载中，请稍后</div>
              <div class="text-xs text-slate-400 dark:text-slate-500">知识点数据与导图渲染中…</div>
            </div>
          </div>
          <div
            v-else-if="noData"
            key="nodata"
            class="absolute inset-0 flex items-center justify-center bg-white/85 text-sm text-slate-400 backdrop-blur-[2px] dark:bg-slate-900/85"
          >
            该科目暂无知识导图
          </div>
        </Transition>
        <div ref="toolbarRef" class="absolute bottom-3 right-3 z-10" />
      </div>

      <!-- 详情抽屉：置于全屏容器内，全屏时才能正常显示 -->
      <el-drawer
        v-model="drawerOpen"
        :append-to-body="false"
        :title="current?.name || '知识点详情'"
        direction="rtl"
        :size="drawerSize"
      >
      <div v-if="current" class="mm-detail space-y-6 pb-8">
        <div class="flex justify-end">
          <el-button :type="isFavorited ? 'warning' : 'default'" size="small" @click="toggleFavorite">
            <el-icon class="mr-1"><Star /></el-icon>{{ isFavorited ? '已收藏' : '收藏' }}
          </el-button>
        </div>

        <section v-if="current.content">
          <h3 class="mm-title mb-2 font-semibold text-slate-900 dark:text-slate-100">内容</h3>
          <div class="md-body text-sm leading-relaxed text-slate-600 dark:text-slate-300" v-html="renderMd(current.content)" @click="onContainerClick" />
        </section>

        <section v-if="current.key_points">
          <h3 class="mm-title mm-title-link mb-2 font-semibold text-blue-600 dark:text-blue-400">考点</h3>
          <div class="md-body text-sm leading-relaxed text-slate-600 dark:text-slate-300" v-html="renderMd(current.key_points)" @click="onContainerClick" />
        </section>

        <section v-if="current.easy_mistakes">
          <h3 class="mm-title mm-title-warn mb-2 font-semibold text-rose-600 dark:text-rose-400">易错点</h3>
          <div class="md-body text-sm leading-relaxed text-slate-600 dark:text-slate-300" v-html="renderMd(current.easy_mistakes)" @click="onContainerClick" />
        </section>
      </div>
      </el-drawer>

      <!-- 图片预览:teleported=false 保证全屏导图下可见 -->
      <el-image-viewer
        v-if="visible"
        :url-list="urlList"
        :initial-index="initialIndex"
        :hide-on-click-modal="true"
        :teleported="false"
        @close="close"
      />
    </div>
  </div>
</template>

<style scoped>
.mm-wrap {
  height: 72vh;
  min-height: 420px;
}
.mm-svg {
  height: 100%;
  /* 让 d3-zoom 独占触摸手势，避免与页面滚动冲突 */
  touch-action: none;
}
/* 骨架屏淡出 → SVG/noData 淡入：mode="out-in" 走完一个再下一个，整体约 480ms。
   ease-out 进入比线性更自然，符合"内容抵达"的心理预期。 */
.mm-fade-enter-active,
.mm-fade-leave-active {
  transition: opacity 240ms ease-out;
}
.mm-fade-enter-from,
.mm-fade-leave-to {
  opacity: 0;
}
/* 性能：把被 d3-zoom 变换的节点组提升为合成层，平移=移动图层而非重绘 SVG */
.mm-wrap :deep(.markmap > g) {
  will-change: transform;
}
/* 节点内容用 foreignObject，隔离重排范围 */
.mm-wrap :deep(.markmap-foreign) {
  contain: layout style;
}
/* 全屏：卡片铺满整屏，画布区自适应剩余高度 */
.mm-card.is-fullscreen {
  border-radius: 0;
  border: none;
  height: 100%;
}
.mm-card.is-fullscreen .mm-wrap {
  height: auto;
  flex: 1 1 auto;
  min-height: 0;
}
/* 节点文字颜色随明暗模式 */
.mm-wrap :deep(.markmap-foreign) {
  color: inherit;
}
.mm-wrap :deep(.markmap-foreign a) {
  color: #2563eb;
  text-decoration: none;
  white-space: nowrap;
}
.dark .mm-wrap :deep(.markmap-foreign a) {
  color: #60a5fa;
}
/* 自定义背景档（.mm-theme-custom）：整张卡（含头部操作区）+ 详情抽屉随预设，不再跟全局明暗。
   标记类与 CSS 变量设在 .mm-card 根上，头部/画布/抽屉共用一套底色才统一。
   这些规则多类 + scoped 属性，优先级高于 Tailwind 单类与 .dark 变体，靠后放置兜底。 */
.mm-card.mm-theme-custom {
  background-color: var(--mm-bg);
  border-color: var(--mm-line);
}
/* 头部：标题用主墨色、提示用次级墨色、下分隔线与按钮描边用发丝墨色 */
.mm-card.mm-theme-custom .mm-hdr {
  border-color: var(--mm-line);
}
.mm-card.mm-theme-custom .mm-hdr-title {
  color: var(--mm-ink);
}
.mm-card.mm-theme-custom .mm-hdr-hint {
  color: var(--mm-muted);
}
.mm-card.mm-theme-custom .mm-hdr-btn {
  color: var(--mm-ink);
  border-color: var(--mm-line);
}
.mm-card.mm-theme-custom .mm-hdr-btn:hover {
  color: var(--mm-link);
  border-color: var(--mm-link);
}
.mm-card.mm-theme-custom .mm-wrap .mm-svg {
  color: var(--mm-ink);
}
.mm-card.mm-theme-custom .mm-wrap :deep(.markmap) {
  --markmap-circle-open-bg: var(--mm-bg);
}
.mm-card.mm-theme-custom .mm-wrap :deep(.markmap-foreign a) {
  color: var(--mm-link);
}
/* 详情抽屉：面板同底色，标题/关闭按钮/正文随预设 */
.mm-card.mm-theme-custom :deep(.el-drawer) {
  background-color: var(--mm-bg);
}
.mm-card.mm-theme-custom :deep(.el-drawer__title),
.mm-card.mm-theme-custom :deep(.el-drawer__close-btn) {
  color: var(--mm-ink);
}
.mm-card.mm-theme-custom .mm-detail .mm-title {
  color: var(--mm-ink);
}
.mm-card.mm-theme-custom .mm-detail .mm-title-link {
  color: var(--mm-link);
}
.mm-card.mm-theme-custom .mm-detail .mm-title-warn {
  color: var(--mm-warn);
}
.mm-card.mm-theme-custom .mm-detail .md-body {
  color: var(--mm-ink);
}
.mm-card.mm-theme-custom .mm-detail .md-body a {
  color: var(--mm-link);
}
</style>

<style>
/* 详情富文本排版（非 scoped，作用于 v-html 内容） */
.md-body ul { list-style: disc; padding-left: 1.25rem; margin: 0.25rem 0; }
.md-body ol { list-style: decimal; padding-left: 1.25rem; margin: 0.25rem 0; }
.md-body p { margin: 0.4rem 0; }
.md-body table { border-collapse: collapse; margin: 0.5rem 0; }
.md-body th, .md-body td { border: 1px solid #e2e8f0; padding: 4px 8px; }
.md-body img { cursor: zoom-in; }
/* Tailwind preflight 把 h1-h6 重置成与正文同字号同字重，markdown 标题需显式恢复层级。
   颜色不写死，继承 .md-body 的字色（自定义背景档随 --mm-ink，跟随主题档随明暗）。 */
.md-body h1, .md-body h2, .md-body h3,
.md-body h4, .md-body h5, .md-body h6 {
  font-weight: 600;
  line-height: 1.4;
  margin: 0.55em 0 0.35em;
}
.md-body h1 { font-size: 1.4rem; }
.md-body h2 { font-size: 1.22rem; }
.md-body h3 { font-size: 1.08rem; }
.md-body h4 { font-size: 1rem; }
.md-body h5, .md-body h6 { font-size: 0.95rem; }
</style>
