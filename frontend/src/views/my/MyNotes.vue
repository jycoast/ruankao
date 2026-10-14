<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Search } from '@element-plus/icons-vue'
import NoteList from '@/components/notes/NoteList.vue'
import NoteDetail from '@/components/notes/NoteDetail.vue'

const route = useRoute()

// 防抖只有这一处负责：inputKeyword 是输入框的实时值，appliedKeyword 才是下发给列表的。
// 放进列表里做的话，「回车立即触发」还得在外面中断它，两处定时器容易打架。
const inputKeyword = ref('')
const appliedKeyword = ref('')
let timer: ReturnType<typeof setTimeout> | null = null

watch(inputKeyword, (v) => {
  if (timer) clearTimeout(timer)
  timer = setTimeout(() => {
    appliedKeyword.value = v
  }, 300)
})

// 回车 / 清空：立即生效，并取消待触发的那次防抖
function applyNow() {
  if (timer) clearTimeout(timer)
  appliedKeyword.value = inputKeyword.value
}

const selectedId = computed(() => {
  const n = Number(route.params.id)
  return Number.isFinite(n) && n > 0 ? n : undefined
})

// 窄屏抽屉：选中任一条笔记后自动收起
const drawer = ref(false)
watch(selectedId, () => {
  drawer.value = false
})

// 用 matchMedia 而不是纯 CSS：抽屉的开合是 JS 状态，需要知道当前断点。
// 断点与 Tailwind 的 lg（1024px）保持一致。
// 加 typeof window 守卫：本仓有基于无头 Chrome 的预渲染流程，setup 期不保证有 window。
const isNarrow = ref(typeof window !== 'undefined' && window.matchMedia('(max-width: 1023px)').matches)
const mql = typeof window !== 'undefined' ? window.matchMedia('(max-width: 1023px)') : null
const onMql = (e: MediaQueryListEvent) => {
  isNarrow.value = e.matches
  if (!e.matches) drawer.value = false
}
mql?.addEventListener('change', onMql)

onUnmounted(() => {
  if (timer) clearTimeout(timer)
  mql?.removeEventListener('change', onMql)
})
</script>

<template>
  <div class="notes-shell flex flex-col px-3 sm:px-4">
    <div class="mb-3 flex shrink-0 flex-wrap items-center gap-3">
      <el-button v-if="isNarrow" :icon="Search" @click="drawer = true">笔记列表</el-button>
      <h2 class="text-lg font-bold text-gray-900 dark:text-gray-100">我的笔记</h2>
      <el-input
        v-model="inputKeyword"
        placeholder="搜索笔记 / 题目 / 套卷…"
        clearable
        class="ml-auto w-full sm:!w-72"
        @keyup.enter="applyNow"
        @clear="applyNow"
      >
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
    </div>

    <div class="flex min-h-0 flex-1 gap-4">
      <!-- 宽屏：常驻左栏 -->
      <aside v-if="!isNarrow" class="flex w-[380px] shrink-0 flex-col">
        <NoteList :keyword="appliedKeyword" />
      </aside>

      <!-- 窄屏：左栏收进抽屉 -->
      <el-drawer v-else v-model="drawer" title="我的笔记" direction="ltr" size="86%">
        <div class="flex h-full min-h-0 flex-col">
          <NoteList :keyword="appliedKeyword" />
        </div>
      </el-drawer>

      <!-- min-w-0：本节是 flex item，默认 min-width:auto 会按内容撑宽。
           笔记正文或题干里一张宽图 / 一段不换行的代码块，就能把这一行撑破，
           整页出现**横向**滚动条（本页要求只有两栏各自纵向滚动）。 -->
      <section class="min-h-0 min-w-0 flex-1 overflow-y-auto rounded-lg bg-white shadow dark:bg-gray-800">
        <NoteDetail :question-id="selectedId" />
      </section>
    </div>
  </div>
</template>

<style scoped>
/* 顶栏 h-14 = 3.5rem，另需扣掉刘海安全区（AppLayout 的 pt-[env(safe-area-inset-top)]）。
   还要减掉 AppLayout <main> 的 py-6（上下各 1.5rem，合计 3rem）：不减的话整页会多出
   3rem 溢出，页面自身就出现滚动条，而本页要求左右两栏各自滚动、页面不滚。
   不写进 Tailwind 任意值是怕 env() 里的连字符被解析器切错。
   本规则是 ≥1280px（Tailwind xl）那一档：该宽度下 <header> 里只剩 h-14 那一行
   （实测 57px = 56 顶栏 + 1px 底边框；这里仍减 3.5rem，残余 1px 溢出）。
   更窄的档位 <header> 会多出下面的行，高度改由下面的 @media 覆盖。 */
.notes-shell {
  height: calc(100vh - 3.5rem - 3rem - env(safe-area-inset-top, 0px));
}

/* < 1280px（Tailwind xl）：<header> 里还多一行移动端导航
   （AppLayout 的 nav.no-scrollbar.xl:hidden）。
   实测 <header> 高度 = 89px（56 顶栏 + 32 导航行 + 1px 底边框）= 5.5625rem，
   比 h-14 多 33px；不减就会把两栏底部切掉。 */
@media (max-width: 1279px) {
  .notes-shell {
    height: calc(100vh - 5.5625rem - 3rem - env(safe-area-inset-top, 0px));
  }
}

/* < 640px（Tailwind sm）：再多一行移动端科目选择器
   （AppLayout 的 div.mb-1.block.sm:hidden）。
   实测 <header> 高度 = 131px（56 顶栏 + 4 选择器 mb-1 + 38 选择器行 + 32 导航行
   + 1px 底边框）= 8.1875rem。 */
@media (max-width: 639px) {
  .notes-shell {
    height: calc(100vh - 8.1875rem - 3rem - env(safe-area-inset-top, 0px));
  }
}
</style>
