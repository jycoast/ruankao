<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { questionNoteApi } from '@/api'
import { useUserStore } from '@/stores/user'
import { loginGuestRoute } from '@/utils/guestGate'
import type { NoteWithQuestion } from '@/types'
import NoteListItem from './NoteListItem.vue'

// keyword 是**已经提交**的检索词（防抖在外壳里做）。列表只负责按它重载。
const props = defineProps<{ keyword: string }>()

const PAGE_SIZE = 20

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const isGuest = computed(() => userStore.isGuest)

const items = ref<NoteWithQuestion[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(false)
const hasMore = ref(false)
const firstLoaded = ref(false)
// 最近一次请求是否失败。失败后**不能**再落进「你还没有记过笔记」那类空态：
// 那是在骗用户说没有数据，而实际是没请求到。
const loadFailed = ref(false)

// 请求序号：每次发起自增，响应回来时若已不是最新一次就直接丢弃。
// 不这样做的后果：reset()（检索词变化）会在旧的 loadMore 请求还在飞的时候把 page 置回 1，
// 旧响应回来时若按「page.value === 1 就替换」判断，会把旧页数据当成第 1 页盖上去。
let reqSeq = 0

const scrollEl = ref<HTMLElement | null>(null)
const sentinel = ref<HTMLElement | null>(null)
let observer: IntersectionObserver | null = null

const emptyText = computed(() => {
  if (props.keyword.trim()) return '未找到匹配的笔记'
  return isGuest.value ? '登录后即可查看你的笔记' : '你还没有记过笔记'
})

async function fetchPage() {
  const seq = ++reqSeq
  const reqPage = page.value
  loading.value = true
  loadFailed.value = false
  try {
    const res: any = await questionNoteApi.myNotes({
      page: reqPage,
      page_size: PAGE_SIZE,
      keyword: props.keyword.trim() || undefined,
    })
    if (seq !== reqSeq) return // 已有更新的请求在途或已完成，这次结果作废
    const chunk: NoteWithQuestion[] = res.data.items || []
    total.value = res.data.total || 0
    // 第 1 页替换、之后追加。用 reqPage（发起时的页码），不能用 page.value ——
    // 它在 await 期间可能已被 reset() 改掉。
    items.value = reqPage === 1 ? chunk : items.value.concat(chunk)
    hasMore.value = items.value.length < total.value
  } catch {
    // 必须接住：fetchPage() 在 onMounted / reset / loadMore 里都是裸调用，
    // 不 catch 就是一条未处理的 Promise rejection。错误提示由 axios 拦截器统一弹，
    // 这里只落一个标记，交给模板的失败态（第 1 页失败=整块空态，翻页失败=底部重试行）。
    // 同样只认最新一次请求，旧请求的失败不能污染新请求的状态。
    if (seq === reqSeq) loadFailed.value = true
  } finally {
    // 只有最新一次请求有资格改 loading/firstLoaded，否则旧请求的 finally
    // 会把新请求的 loading 提前抹掉。
    if (seq === reqSeq) {
      loading.value = false
      firstLoaded.value = true
    }
  }
}

function reset() {
  page.value = 1
  items.value = []
  total.value = 0
  hasMore.value = false
  firstLoaded.value = false
  fetchPage()
}

function loadMore() {
  // loadFailed 时不再自动往下翻：否则哨兵再次触发会 page += 1 跳过失败的那一页，
  // 失败的那 20 条就永久缺一块。失败后只由「重试」按钮推进。
  if (!hasMore.value || loading.value || loadFailed.value) return
  page.value += 1
  fetchPage()
}

// 重试：第 1 页失败（items 为空）就整列重来；翻页失败时 page 已指向失败的那一页，
// 直接重发即可（items 非空，走追加分支）。
function retry() {
  if (items.value.length) fetchPage()
  else reset()
}

// 检索词一变就重置回第 1 页（防抖已在外壳里，这里立即执行）
watch(() => props.keyword, reset)

onMounted(() => {
  fetchPage()
  // root 绑列表自己的滚动容器，不绑 viewport —— 抽屉里复用同一个组件时，
  // ref 指向的仍是该实例自己的容器，两种布局都对。
  observer = new IntersectionObserver(
    (entries) => {
      if (entries[0]?.isIntersecting) loadMore()
    },
    { root: scrollEl.value, rootMargin: '200px' },
  )
  // 哨兵**始终渲染**，不随 hasMore 卸载：否则 hasMore 由 false 变 true 时要重新
  // observe，多一套「元素还没挂上就 observe」的竞态。改成常驻 + 回调里判 hasMore。
  if (sentinel.value) observer.observe(sentinel.value)
})

onUnmounted(() => observer?.disconnect())
</script>

<template>
  <div ref="scrollEl" class="min-h-0 flex-1 overflow-y-auto pr-1">
    <!-- 第 1 页就失败：整块失败态。必须排在普通空态**之前** —— 否则一个加载失败
         会被 firstLoaded=true 带进「你还没有记过笔记」，等于告诉用户没数据。 -->
    <div v-if="firstLoaded && loadFailed && !items.length" class="py-10">
      <el-empty description="笔记加载失败">
        <el-button type="primary" plain size="small" @click="retry">重试</el-button>
      </el-empty>
    </div>

    <div v-else-if="firstLoaded && !items.length" class="py-10">
      <el-empty :description="emptyText">
        <el-button v-if="isGuest" type="primary" plain size="small" @click="router.push(loginGuestRoute())">
          去登录
        </el-button>
      </el-empty>
    </div>

    <template v-else>
      <SectionTitle class="px-1">共 {{ total }} 条笔记</SectionTitle>

      <div class="space-y-3">
        <NoteListItem
          v-for="n in items"
          :key="n.id"
          :note="n"
          :active="String(n.question_id) === String(route.params.id)"
        />
      </div>

      <div v-if="loading" class="space-y-3 pt-3">
        <el-skeleton
          v-for="i in 3"
          :key="`sk-${i}`"
          :rows="2"
          animated
          class="rounded-lg border border-gray-200 p-3 dark:border-gray-700"
        />
      </div>
      <!-- 翻页失败：列表已有内容，只在底部挂一条可重试的提示，不整块替换掉已有卡片 -->
      <div v-else-if="loadFailed" class="py-4 text-center text-xs text-gray-500 dark:text-gray-400">
        加载失败
        <el-button link type="primary" size="small" @click="retry">重试</el-button>
      </div>
      <div v-else-if="!hasMore && items.length" class="py-4 text-center text-xs text-gray-500 dark:text-gray-400">
        没有更多了
      </div>
    </template>

    <!-- 无限滚动哨兵：常驻，不要加 v-if -->
    <div ref="sentinel" class="h-px w-full"></div>
  </div>
</template>
